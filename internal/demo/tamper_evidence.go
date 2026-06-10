package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// TamperEvidenceSchemaVersion identifies the JSON shape of the tamper-evidence
// demo result.
const TamperEvidenceSchemaVersion = "boundary.demo.tamper_evidence.v1"

// TamperEvidenceOptions configures the tamper-evidence demo. Now is injected so
// tests get deterministic timing; it does not affect the verification outcome.
type TamperEvidenceOptions struct {
	Now time.Time
}

// TamperEvidenceResult is the structured outcome of the "forge the receipt"
// sequence: emit a hash-verifiable decision record, verify it, tamper exactly
// one field, and show that re-verification fails on a decision_hash mismatch.
// It is fixture-only and performs no live mutation, no network, and needs no
// credentials.
type TamperEvidenceResult struct {
	SchemaVersion       string `json:"schema_version"`
	Status              string `json:"status"`
	Passed              bool   `json:"passed"`
	FixtureOnly         bool   `json:"fixture_only"`
	RequiresCredentials bool   `json:"requires_credentials"`
	RequiresNetwork     bool   `json:"requires_network"`
	MutatesLiveSystems  bool   `json:"mutates_live_systems"`

	RecordID string `json:"record_id"`
	// TamperedField names the single field changed to forge the record.
	TamperedField string `json:"tampered_field"`
	OriginalValue string `json:"original_value"`
	ForgedValue   string `json:"forged_value"`
	// OriginalVerified is true when the untouched record verifies (the baseline
	// the demo depends on).
	OriginalVerified bool `json:"original_verified"`
	// TamperDetected is true when re-verifying the forged record fails. The demo
	// passes only when the original verifies AND the forgery is detected.
	TamperDetected bool `json:"tamper_detected"`
	// StoredHash is the decision_hash carried in the forged record (unchanged by
	// the tamper); RecomputedHash is what verification recomputes from the forged
	// body. They differ, which is exactly what the verifier reports.
	StoredHash     string `json:"stored_hash"`
	RecomputedHash string `json:"recomputed_hash"`
	// VerifyError is the verifier's message for the forged record (the
	// "decision_hash mismatch: got ... want ..." line).
	VerifyError string   `json:"verify_error,omitempty"`
	Proof       []string `json:"proof"`
	Limitations []string `json:"limitations"`
}

// RunTamperEvidence runs the forge-the-receipt sequence end to end and returns
// its structured result. The source record is the Secure GitHub fixture's
// write-after-taint denial — a real, hash-complete DecisionRecordV1 — so the
// demo verifies an actual governed record rather than a hand-built fake.
func RunTamperEvidence(ctx context.Context, opts TamperEvidenceOptions) (*TamperEvidenceResult, error) {
	proof, err := runSecureGitHubFixture(ctx)
	if err != nil {
		return nil, err
	}
	record := proof.write.DecisionRecord
	if record.RecordID == "" || record.DecisionHash == "" {
		return nil, fmt.Errorf("tamper-evidence demo: source record is missing record_id or decision_hash")
	}

	// Step 1: the untouched record must verify (decision_hash matches its body).
	originalErr := governance.VerifyDecisionRecord(record, nil, "", "")
	originalVerified := originalErr == nil

	// Step 2: forge exactly one field. Flipping the verdict from deny to allow is
	// the strongest "forged receipt" story: an attacker tries to make a denial
	// read as an approval. We leave decision_hash untouched (the attacker does
	// not recompute it), which is what the verifier catches.
	originalAction := record.Action
	forgedAction := "allow"
	if originalAction == "allow" {
		forgedAction = "deny"
	}
	forged := record
	forged.Action = forgedAction

	storedHash := forged.DecisionHash
	recomputedHash := governance.ComputeDecisionHash(forged)

	// Step 3: re-verify the forged record; it must fail on the hash mismatch.
	tamperErr := governance.VerifyDecisionRecord(forged, nil, "", "")
	tamperDetected := tamperErr != nil
	verifyError := ""
	if tamperErr != nil {
		verifyError = tamperErr.Error()
	}

	passed := originalVerified && tamperDetected && storedHash != recomputedHash
	status := "pass"
	if !passed {
		status = "fail"
	}

	return &TamperEvidenceResult{
		SchemaVersion:       TamperEvidenceSchemaVersion,
		Status:              status,
		Passed:              passed,
		FixtureOnly:         true,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		RecordID:            record.RecordID,
		TamperedField:       "action",
		OriginalValue:       originalAction,
		ForgedValue:         forgedAction,
		OriginalVerified:    originalVerified,
		TamperDetected:      tamperDetected,
		StoredHash:          storedHash,
		RecomputedHash:      recomputedHash,
		VerifyError:         verifyError,
		Proof: []string{
			"A governed denial record is emitted with a decision_hash bound to its body.",
			"The untouched record verifies: the stored decision_hash matches the recomputed hash.",
			"Forging a single field (the verdict) leaves the stored decision_hash unchanged.",
			"Re-verification recomputes the hash from the forged body and reports the mismatch.",
		},
		Limitations: []string{
			"Hash-verifiable detection is not tamper-proof or immutable storage: it detects a forged record, it does not prevent one from being written elsewhere.",
			"Verification confirms a record's body matches its decision_hash; it does not attest that the deployment topology forced the route through Boundary.",
			"The demo is fixture-only: no credentials, no network, and no live mutation.",
		},
	}, nil
}

// WriteTamperEvidenceJSON writes the result as indented JSON.
func WriteTamperEvidenceJSON(w io.Writer, result *TamperEvidenceResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// WriteTamperEvidenceText renders the plain (no-color) human report.
func WriteTamperEvidenceText(w io.Writer, result *TamperEvidenceResult) error {
	return WriteTamperEvidenceTextColor(w, result, nil)
}

// WriteTamperEvidenceTextColor renders the human report, styling the verify
// outcomes and hashes through c (nil renders plain). The narrative walks the
// three steps so the output reads as "forge the receipt, the hash catches it",
// not a bare boolean.
func WriteTamperEvidenceTextColor(w io.Writer, result *TamperEvidenceResult, c *Colorizer) error {
	if result == nil {
		return fmt.Errorf("tamper-evidence result is required")
	}
	fmt.Fprintln(w, c.Bold("Tamper-evidence demo: forge the receipt (fixture-only)"))
	fmt.Fprintln(w, "fixture-only: true   credentials: none   network: none   live mutation: none")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "record id: %s\n", c.Dim(result.RecordID))
	fmt.Fprintf(w, "1. emit + verify original     -> %s\n", colorVerifyState(c, result.OriginalVerified, "verified", "FAILED"))
	fmt.Fprintf(w, "2. tamper one field           -> %s: %q -> %q\n", result.TamperedField, result.OriginalValue, result.ForgedValue)
	fmt.Fprintf(w, "3. re-verify forged record    -> %s\n", colorTamperOutcome(c, result.TamperDetected))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "stored decision hash:     %s\n", c.Dim(result.StoredHash))
	fmt.Fprintf(w, "recomputed decision hash: %s\n", c.Dim(result.RecomputedHash))
	if result.VerifyError != "" {
		fmt.Fprintf(w, "verifier: %s\n", result.VerifyError)
	}
	fmt.Fprintln(w, "\n"+c.Bold("What this proves:"))
	for _, proof := range result.Proof {
		fmt.Fprintf(w, "- %s\n", proof)
	}
	fmt.Fprintln(w, "\n"+c.Bold("What this does not prove:"))
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "- %s\n", limitation)
	}
	return nil
}

// colorVerifyState renders a boolean verify outcome with an explicit label:
// green for the ok case, red for the not-ok case.
func colorVerifyState(c *Colorizer, ok bool, okLabel, failLabel string) string {
	if ok {
		return c.Pass(okLabel)
	}
	return c.Fail(failLabel)
}

// colorTamperOutcome renders the re-verify step. Detecting the forgery is the
// success path here, so "tamper DETECTED" is green and a missed forgery is red.
func colorTamperOutcome(c *Colorizer, detected bool) string {
	if detected {
		return c.Pass("tamper DETECTED (verification rejects forged record)")
	}
	return c.Fail("tamper NOT detected")
}
