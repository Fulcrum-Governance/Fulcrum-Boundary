package explain

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// buildV1 returns a schema_version "1" record (no route-context).
func buildV1(t *testing.T) governance.DecisionRecordV1 {
	t.Helper()
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:    governance.TransportMCP,
		ToolName:     "query",
		Action:       "allow",
		Reason:       "read-only SELECT permitted by policy",
		DecisionMode: governance.DecisionModeDeterministic,
		MatchedRule:  "allow-select",
		PolicyFile:   "postgres.yaml",
		RequestHash:  "sha256:" + strings.Repeat("a", 64),
		TrustScore:   1,
		TrustState:   governance.TrustStateTrusted.String(),
	})
	if rec.SchemaVersion != governance.DecisionRecordSchemaVersion {
		t.Fatalf("expected V1 record, got schema_version %q", rec.SchemaVersion)
	}
	return rec
}

// buildV2 returns a schema_version "2" record carrying every route-context field.
func buildV2(t *testing.T) governance.DecisionRecordV1 {
	t.Helper()
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:       governance.TransportMCP,
		ToolName:        "github.create_or_update_file",
		Action:          "deny",
		Reason:          "write-after-taint",
		DecisionMode:    governance.DecisionModeDeterministic,
		MatchedRule:     "deny-github-write-after-taint-fixture",
		AdapterID:       "securegithub",
		RouteID:         "mcp:github.create_or_update_file",
		TopologyProfile: "single-tenant-routed",
		ExecutionClaim:  &governance.ExecutionClaim{UpstreamCalled: false, Executed: false, Source: "securegithub"},
		TrustScore:      1,
		TrustState:      governance.TrustStateTrusted.String(),
	})
	if rec.SchemaVersion != governance.DecisionRecordSchemaV2 {
		t.Fatalf("expected V2 record, got schema_version %q", rec.SchemaVersion)
	}
	return rec
}

func mustJSON(t *testing.T, rec governance.DecisionRecordV1) []byte {
	t.Helper()
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestDescribeV1OmitsRouteContext(t *testing.T) {
	result, err := describe(mustJSON(t, buildV1(t)))
	if err != nil {
		t.Fatalf("describe v1: %v", err)
	}
	if result.SchemaVersion != SchemaVersion {
		t.Fatalf("envelope schema_version = %q, want %q", result.SchemaVersion, SchemaVersion)
	}
	if result.RecordSchemaVersion != "1" {
		t.Fatalf("record_schema_version = %q, want 1", result.RecordSchemaVersion)
	}
	if result.RouteContext != nil {
		t.Fatalf("V1 record must not carry route_context: %#v", result.RouteContext)
	}
	if result.Decision.Action != "allow" || result.Decision.MatchedRule != "allow-select" {
		t.Fatalf("decision fields not mapped: %#v", result.Decision)
	}
	if result.RequiresCredentials || result.RequiresNetwork || result.MutatesLiveSystems {
		t.Fatalf("explain must be local-only: %#v", result)
	}
}

func TestDescribeV2CarriesRouteContextAndCaveats(t *testing.T) {
	result, err := describe(mustJSON(t, buildV2(t)))
	if err != nil {
		t.Fatalf("describe v2: %v", err)
	}
	if result.RecordSchemaVersion != "2" {
		t.Fatalf("record_schema_version = %q, want 2", result.RecordSchemaVersion)
	}
	rc := result.RouteContext
	if rc == nil {
		t.Fatal("V2 record must carry route_context")
	}
	if rc.AdapterID != "securegithub" || rc.RouteID != "mcp:github.create_or_update_file" || rc.TopologyProfile != "single-tenant-routed" {
		t.Fatalf("route_context not mapped: %#v", rc)
	}
	if rc.ExecutionClaim == nil || rc.ExecutionClaim.UpstreamCalled || rc.ExecutionClaim.Executed || rc.ExecutionClaim.Source != "securegithub" {
		t.Fatalf("execution_claim not mapped: %#v", rc.ExecutionClaim)
	}
}

func TestDescribeListsHashCoverageWithoutRecomputing(t *testing.T) {
	rec := buildV1(t)
	// Corrupt decision_hash: explain describes the stored value as-is and does
	// not recompute or validate it (that is verify-record's job).
	rec.DecisionHash = "sha256:deadbeef"
	result, err := describe(mustJSON(t, rec))
	if err != nil {
		t.Fatalf("describe must not verify hashes: %v", err)
	}
	var decisionHash *HashDescription
	for i := range result.Hashes {
		if result.Hashes[i].Field == "decision_hash" {
			decisionHash = &result.Hashes[i]
		}
	}
	if decisionHash == nil {
		t.Fatal("decision_hash coverage line missing")
	}
	if decisionHash.Value != "sha256:deadbeef" || !decisionHash.Present {
		t.Fatalf("explain must surface the stored hash verbatim: %#v", decisionHash)
	}
	if !strings.Contains(decisionHash.Covers, "Integrity, not authenticity") {
		t.Fatalf("decision_hash coverage must keep the integrity caveat: %q", decisionHash.Covers)
	}
}

func TestFixedDoesNotProveFooterIsNegationFramed(t *testing.T) {
	result, err := describe(mustJSON(t, buildV1(t)))
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if len(result.DoesNotProve) == 0 {
		t.Fatal("does_not_prove footer must not be empty")
	}
	mustContain := []string{
		"does not verify the record's hashes",
		"does not prove the verdict was correct",
		"does not prove enforcement",
		"self-report, not corroborated",
		"asserted, not attested",
	}
	joined := strings.ToLower(strings.Join(result.DoesNotProve, "\n"))
	for _, want := range mustContain {
		if !strings.Contains(joined, strings.ToLower(want)) {
			t.Fatalf("does_not_prove footer missing %q:\n%s", want, strings.Join(result.DoesNotProve, "\n"))
		}
	}
}

func TestRunRejectsUnsupportedSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"99","action":"deny","decision_hash":"sha256:x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{Path: path}); err == nil {
		t.Fatal("expected unsupported schema_version error")
	}
}

func TestRunRejectsMissingAction(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noaction.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"1","decision_hash":"sha256:x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{Path: path}); err == nil {
		t.Fatal("expected missing-action error")
	}
}

func TestRunRequiresPath(t *testing.T) {
	if _, err := Run(Options{Path: ""}); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestWriteTextV1IsReadableAndOmitsRouteContext(t *testing.T) {
	result, err := describe(mustJSON(t, buildV1(t)))
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteText(&buf, result); err != nil {
		t.Fatalf("write text: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Boundary explain",
		"record schema_version: 1",
		"action: allow",
		"What this does not prove:",
		"does not verify the record's hashes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Route context") {
		t.Fatalf("V1 text must not render route context:\n%s", out)
	}
}

func TestWriteJSONIsStable(t *testing.T) {
	result, err := describe(mustJSON(t, buildV2(t)))
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, result); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var decoded Result
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("explain JSON did not round-trip: %v\n%s", err, buf.String())
	}
	if decoded.SchemaVersion != SchemaVersion || decoded.RecordSchemaVersion != "2" {
		t.Fatalf("round-trip lost identity: %#v", decoded)
	}
}
