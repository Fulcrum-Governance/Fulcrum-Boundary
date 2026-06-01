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

	"gopkg.in/yaml.v3"
)

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

func ComputeDecisionHash(record DecisionRecordV1) string {
	record.RecordID = ""
	record.DecisionHash = ""
	record.Signature = ""
	record.SignatureKeyID = ""
	encoded := mustCanonicalJSON(record)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ComputeRequestHash(req *GovernanceRequest) string {
	encoded := mustCanonicalJSON(req)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ComputeRawRequestHash(raw []byte) (string, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	encoded := mustCanonicalJSON(value)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func ComputeRawShapeHash(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	sum := sha256.Sum256(trimmed)
	return "sha256:" + hex.EncodeToString(sum[:])
}

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

func mustCanonicalJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
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
