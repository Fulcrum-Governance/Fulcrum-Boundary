package proofreceipt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const ReceiptVersion = "proof-receipt-v0.1"

const (
	ResultPass = "pass"
	ResultFail = "fail"
)

// Invariant is one checker-validated invariant line in a proof receipt. It
// names the upstream Lean theorem, the predicate that was checked, the
// canonical hash of the witness inputs the checker consumed, and the result.
type Invariant struct {
	TheoremID  string `json:"theorem_id"`
	Predicate  string `json:"predicate"`
	InputsHash string `json:"inputs_hash"`
	Result     string `json:"result"`
}

// ProofReceipt is the proof-receipt-v0.1 sidecar (see package doc). It carries
// no decision_mode field by construction; the bound record's mode stays
// deterministic/classified/human_approved, and Boundary never emits `proved`.
type ProofReceipt struct {
	ReceiptVersion   string      `json:"receipt_version"`
	DecisionHash     string      `json:"decision_hash"`
	CheckerID        string      `json:"checker_id"`
	CheckerBuildHash string      `json:"checker_build_hash"`
	Invariants       []Invariant `json:"invariants"`
	RecordedAt       time.Time   `json:"recorded_at"`
}

// CanonicalInputsHash returns the "sha256:"-prefixed SHA-256 of the RFC 8785
// canonical JSON of v, the witness-input digest stored in Invariant.InputsHash.
// It uses governance.CanonicalJSONBytes so the digest is reproducible by the
// stock JCS verifiers in verifiers/.
func CanonicalInputsHash(v any) string {
	sum := sha256.Sum256(governance.CanonicalJSONBytes(v))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// New builds a proof-receipt-v0.1 bound to record by its decision_hash, with the
// given checker identity and invariant lines, stamping RecordedAt to
// recordedAt.UTC() (time.Now().UTC() when zero). It does not mutate record.
func New(record governance.DecisionRecordV1, checkerID, checkerBuildHash string, invariants []Invariant, recordedAt time.Time) ProofReceipt {
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	return ProofReceipt{
		ReceiptVersion:   ReceiptVersion,
		DecisionHash:     record.DecisionHash,
		CheckerID:        checkerID,
		CheckerBuildHash: checkerBuildHash,
		Invariants:       invariants,
		RecordedAt:       recordedAt.UTC(),
	}
}

// VerifyBinding reports an error unless r.ReceiptVersion is ReceiptVersion and
// r.DecisionHash equals record's recomputed decision_hash. It binds the sidecar
// to the record without re-encoding the record; it does not re-run the checker.
func (r ProofReceipt) VerifyBinding(record governance.DecisionRecordV1) error {
	if r.ReceiptVersion != ReceiptVersion {
		return fmt.Errorf("receipt_version unsupported: got %q want %q", r.ReceiptVersion, ReceiptVersion)
	}
	want := governance.ComputeDecisionHash(record)
	if r.DecisionHash != want {
		return fmt.Errorf("decision_hash binding mismatch: receipt %s record %s", r.DecisionHash, want)
	}
	return nil
}

// WriteJSON writes r as an indented JSON object to path (0600, parent dir 0700),
// mirroring demo.WriteDecisionRecordJSON so a sidecar lands next to the
// decision-record.json it binds to.
func WriteJSON(path string, r ProofReceipt) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- path is an internally constructed or operator-selected artifact path.
	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if openErr != nil {
		return openErr
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ReadJSON reads a proof-receipt-v0.1 object from path. It rejects any JSON
// that contains fields not defined by the proof-receipt-v0.1 schema (e.g. a
// rogue "decision_mode" field), returning a wrapped error so callers can
// detect non-conformant sidecars before VerifyBinding.
func ReadJSON(path string) (ProofReceipt, error) {
	// #nosec G304 -- path is an internally constructed or operator-selected artifact path.
	body, err := os.ReadFile(path)
	if err != nil {
		return ProofReceipt{}, err
	}
	var r ProofReceipt
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return ProofReceipt{}, fmt.Errorf("proof-receipt-v0.1 schema violation: %w", err)
	}
	return r, nil
}

// AttachAll builds a proof-receipt-v0.1 for record from the given invariant
// lines and returns it bound by record.DecisionHash. It is the single entry
// point a demo or adapter uses to attach budget/privilege/trust invariants at
// once. It does not mutate record and does not recompute or alter decision_hash;
// the returned receipt is a separate artifact that VerifyBinding re-checks
// against the verbatim record. Invariants with an empty TheoremID are dropped so
// a caller can pass a fixed-length slice with some checks skipped.
func AttachAll(record governance.DecisionRecordV1, checkerID, checkerBuildHash string, invariants []Invariant, recordedAt time.Time) ProofReceipt {
	kept := make([]Invariant, 0, len(invariants))
	for _, inv := range invariants {
		if inv.TheoremID == "" {
			continue
		}
		kept = append(kept, inv)
	}
	return New(record, checkerID, checkerBuildHash, kept, recordedAt)
}
