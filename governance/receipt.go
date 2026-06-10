package governance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gowebpki/jcs"
	"gopkg.in/yaml.v3"
)

// BuildDecisionRecord assembles a finished DecisionRecordV1 from an internal
// AuditEvent. It copies the event's decision-defining and identity/context
// fields, defaults Timestamp to time.Now().UTC() when the event leaves it zero,
// fixes SchemaVersion (DecisionRecordSchemaV2 when any route-context field is
// populated, otherwise DecisionRecordSchemaVersion), then computes DecisionHash
// over the assembled record and derives RecordID from it. The schema version is
// chosen before hashing so it is covered by decision_hash, and a record with no
// route-context marshals byte-for-byte as a schema_version "1" record. The
// returned record reflects what Boundary decided; it is not evidence that any
// action was executed or prevented. See docs/DECISION_RECORDS.md.
func BuildDecisionRecord(event AuditEvent) DecisionRecordV1 {
	ts := event.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	record := DecisionRecordV1{
		SchemaVersion:       DecisionRecordSchemaVersion,
		EventType:           firstReceiptString(event.EventType, "governance_decision"),
		Timestamp:           ts.UTC(),
		BoundaryVersion:     event.GatewayVersion,
		BoundaryBuildDigest: event.BoundaryBuildDigest,
		Adapter:             event.Transport,
		AgentID:             event.AgentID,
		TenantID:            event.TenantID,
		TraceID:             event.TraceID,
		Tool:                event.ToolName,
		Action:              event.Action,
		Reason:              event.Reason,
		DecisionMode:        event.DecisionMode,
		MatchedRule:         event.MatchedRule,
		PolicyFile:          event.PolicyFile,
		PolicyBundleHash:    event.PolicyBundleHash,
		RequestHash:         event.RequestHash,
		RawShapeHash:        event.RawShapeHash,
		TrustScore:          event.TrustScore,
		TrustState:          event.TrustState,
		Signature:           event.Signature,
		SignatureKeyID:      event.SignatureKeyID,
		AdapterID:           event.AdapterID,
		RouteID:             event.RouteID,
		TopologyProfile:     event.TopologyProfile,
		ExecutionClaim:      event.ExecutionClaim,
	}
	// Emit "2" only when route-context is populated; otherwise keep "1" so the
	// record stays byte-compatible with existing V1 tooling and its
	// decision_hash is identical to a pre-V2 record. The version is fixed
	// BEFORE hashing so it is covered by decision_hash.
	if record.HasRouteContext() {
		record.SchemaVersion = DecisionRecordSchemaV2
	}
	record.DecisionHash = ComputeDecisionHash(record)
	record.RecordID = recordID(record.DecisionHash)
	return record
}

// ComputeDecisionHash returns the stable decision_hash of a record: the
// SHA-256, lowercase hex, "sha256:"-prefixed digest of the record's canonical
// JSON with record_id, decision_hash, signature, and signature_key_id blanked
// first. Blanking those four makes the hash self-excluding (it does not depend
// on its own value or the derived record_id) and signature-excluding (it covers
// content, not the optional operator signature). It is computed over the same
// superset struct for both schema versions, so route-context fields are covered
// when present. The result is an unkeyed integrity digest: recomputing it on
// altered inputs yields a new, internally valid hash, so it detects tampering
// but does not attest authorship or that the verdict was correct. See
// docs/RECEIPTS.md.
func ComputeDecisionHash(record DecisionRecordV1) string {
	record.RecordID = ""
	record.DecisionHash = ""
	record.Signature = ""
	record.SignatureKeyID = ""
	encoded := mustCanonicalJSON(record)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ComputeRequestHash returns the stable request_hash of an in-memory governed
// request: the SHA-256, lowercase hex, "sha256:"-prefixed digest of the
// request's canonical JSON. Because the input is canonicalized, key ordering and
// whitespace do not change the digest. Use ComputeRawRequestHash to hash raw
// request bytes (for example a file supplied to verification) to the same value.
func ComputeRequestHash(req *GovernanceRequest) string {
	encoded := mustCanonicalJSON(req)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ComputeRawRequestHash returns the stable request_hash for raw request bytes by
// round-tripping them through canonical JSON before hashing, so the digest is
// independent of key ordering and whitespace and matches ComputeRequestHash for
// the same logical request. It is the verify-time counterpart used by
// VerifyDecisionRecord and boundary verify-record --request. It returns an error
// if raw is not valid JSON.
func ComputeRawRequestHash(raw []byte) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	encoded := mustCanonicalJSON(value)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// ComputeRawShapeHash returns the raw_shape_hash recorded on parse rejections:
// the SHA-256, lowercase hex, "sha256:"-prefixed digest of the trimmed raw input
// bytes. Unlike ComputeRequestHash and ComputeRawRequestHash it does not
// canonicalize the input (the bytes never parsed into a governed request), so it
// is whitespace-trim-sensitive but otherwise byte-exact. It records that
// Boundary observed and rejected an input shape; it does not imply any
// downstream tool was reached. It appears in place of request_hash on
// event_type=parse_rejected records. See docs/DECISION_RECORDS.md.
func ComputeRawShapeHash(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	sum := sha256.Sum256(trimmed)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// PolicyBundleHashFromDir returns the stable policy_bundle_hash for a policy
// directory: every regular .yaml/.yml file is normalized from YAML to canonical
// JSON, the normalized documents are sorted, and the sorted set is hashed to a
// SHA-256, lowercase hex, "sha256:"-prefixed digest. The hash covers policy
// content only — file modification time, directory order, and file metadata are
// excluded, and symlinks and non-YAML files are skipped, so a policy delivered
// by a symlink or a non-YAML mechanism is outside the bundle hash. It is the
// verify-time counterpart used by VerifyDecisionRecord and boundary
// verify-record --policies. It returns an error if the directory cannot be read
// or any file cannot be read or canonicalized. See docs/RECEIPTS.md.
func PolicyBundleHashFromDir(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read policy directory %s: %w", dir, err)
	}
	var docs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// #nosec G304 -- path is assembled from os.ReadDir entries in the operator-selected policy directory; symlinks are skipped above.
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read policy file %s: %w", path, err)
		}
		canonical, err := canonicalYAML(body)
		if err != nil {
			return "", fmt.Errorf("canonicalize policy file %s: %w", path, err)
		}
		docs = append(docs, string(canonical))
	}
	sort.Strings(docs)
	encoded := mustCanonicalJSON(docs)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// SupportedDecisionRecordSchemaVersion reports whether v is a decision-record
// schema version this build can verify. Both "1" (no route-context) and "2"
// (additive route-context) are accepted; decision_hash is recomputed over the
// same superset struct for either, so a V1 record still verifies under a V2
// build and a V2 record verifies with its route-context fields covered.
func SupportedDecisionRecordSchemaVersion(v string) bool {
	return v == DecisionRecordSchemaVersion || v == DecisionRecordSchemaV2
}

// VerifyDecisionRecord recomputes a record's stable hashes and reports the first
// mismatch as an error, or nil when every applicable check passes. It runs the
// checks in this fixed order, stopping at the first failure: schema_version must
// be supported (see SupportedDecisionRecordSchemaVersion); request_hash, only
// when rawRequest is non-nil; policy_bundle_hash, only when policyDir is
// non-empty; boundary_build_digest, only when binaryDigest is non-empty; and
// decision_hash, always. The three cross-checks are what bind a record to a
// specific request, policy bundle, and build: called with rawRequest nil,
// policyDir "", and binaryDigest "" this confirms only schema_version and
// decision_hash self-consistency — that the record has not been altered since
// emission — and does not bind it to the request, policy bundle, or build that
// ran.
//
// This is integrity verification, not authenticity: it detects tampering with
// the covered inputs but does not check the optional signature fields and does
// not prove who produced the record or that the verdict was correct. A passing
// check is not evidence the action was executed or prevented. See
// docs/RECEIPTS.md.
func VerifyDecisionRecord(record DecisionRecordV1, rawRequest []byte, policyDir, binaryDigest string) error {
	if !SupportedDecisionRecordSchemaVersion(record.SchemaVersion) {
		return fmt.Errorf("schema_version unsupported: got %q want one of %q, %q", record.SchemaVersion, DecisionRecordSchemaVersion, DecisionRecordSchemaV2)
	}
	if rawRequest != nil {
		requestHash, err := ComputeRawRequestHash(rawRequest)
		if err != nil {
			return fmt.Errorf("request hash: %w", err)
		}
		if record.RequestHash != requestHash {
			return fmt.Errorf("request_hash mismatch: got %s want %s", record.RequestHash, requestHash)
		}
	}
	if policyDir != "" {
		policyHash, err := PolicyBundleHashFromDir(policyDir)
		if err != nil {
			return err
		}
		if record.PolicyBundleHash != policyHash {
			return fmt.Errorf("policy_bundle_hash mismatch: got %s want %s", record.PolicyBundleHash, policyHash)
		}
	}
	if binaryDigest != "" && record.BoundaryBuildDigest != binaryDigest {
		return fmt.Errorf("boundary_build_digest mismatch: got %s want %s", record.BoundaryBuildDigest, binaryDigest)
	}
	decisionHash := ComputeDecisionHash(record)
	if record.DecisionHash != decisionHash {
		return fmt.Errorf("decision_hash mismatch: got %s want %s", record.DecisionHash, decisionHash)
	}
	return nil
}

func canonicalYAML(body []byte) ([]byte, error) {
	var value any
	if err := yaml.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	return json.Marshal(normalizeYAML(value))
}

func normalizeYAML(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = normalizeYAML(value)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[fmt.Sprint(key)] = normalizeYAML(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = normalizeYAML(value)
		}
		return out
	default:
		return typed
	}
}

// mustCanonicalJSON returns the RFC 8785 (JSON Canonicalization Scheme)
// canonical form of value. It is the single canonicalization helper behind
// every stable hash in this package (ComputeDecisionHash, ComputeRequestHash,
// ComputeRawRequestHash, and PolicyBundleHashFromDir), so all of them produce
// digests an independent, stock JCS implementation in any language can
// reproduce.
//
// The two ways Go's default encoder diverges from JCS are corrected by routing
// through jcs.Transform: object keys are reordered lexicographically by UTF-16
// code unit (Go emits struct-declaration order), and the HTML escaping
// json.Marshal applies to "<", ">", and "&" is undone (JCS leaves those bytes
// literal). Numbers are re-emitted with the ECMAScript Number-to-string
// (shortest round-trip) algorithm rather than Go's, which matters for arbitrary
// float64 values such as a non-trivial trust_score. Field declaration order in
// the structs is irrelevant: JCS sorts at encode time, so the on-the-wire shape
// of a record is unchanged.
//
// It panics only on inputs json.Marshal itself cannot encode (the same failure
// mode as before) or if the marshaled bytes are not valid JSON for Transform,
// which cannot happen for json.Marshal output; callers pass marshalable values.
func mustCanonicalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	canonical, err := jcs.Transform(encoded)
	if err != nil {
		panic(err)
	}
	return canonical
}

func recordID(hash string) string {
	trimmed := strings.TrimPrefix(hash, "sha256:")
	if len(trimmed) < 12 {
		return "rec_" + trimmed
	}
	return "rec_" + trimmed[:12]
}

func firstReceiptString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
