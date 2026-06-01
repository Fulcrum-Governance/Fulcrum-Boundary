package tests

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

// TestVerifyRecordAcceptsV1AndV2 proves boundary verify-record accepts both the
// schema_version "1" record (no route-context) and the schema_version "2"
// record (additive route-context), through the real CLI dispatch — confirming
// the dual-version gate is wired end to end.
func TestVerifyRecordAcceptsV1AndV2(t *testing.T) {
	dir := t.TempDir()

	v1 := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:  governance.TransportMCP,
		ToolName:   "query",
		Action:     "allow",
		TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
	})
	if v1.SchemaVersion != "1" {
		t.Fatalf("expected V1 record schema_version 1, got %q", v1.SchemaVersion)
	}

	v2 := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:       governance.TransportMCP,
		ToolName:        "github.create_or_update_file",
		Action:          "deny",
		Reason:          "write-after-taint",
		TrustScore:      1,
		TrustState:      governance.TrustStateTrusted.String(),
		AdapterID:       "securegithub",
		RouteID:         "mcp:github.create_or_update_file",
		TopologyProfile: "single-tenant-routed",
		ExecutionClaim:  &governance.ExecutionClaim{UpstreamCalled: false, Executed: false, Source: "securegithub"},
	})
	if v2.SchemaVersion != "2" {
		t.Fatalf("expected V2 record schema_version 2, got %q", v2.SchemaVersion)
	}

	for name, rec := range map[string]governance.DecisionRecordV1{"v1": v1, "v2": v2} {
		path := filepath.Join(dir, name+".json")
		writeRecord(t, path, rec)
		var stdout, stderr bytes.Buffer
		code := boundarycli.Run([]string{"verify-record", path}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%s record failed to verify: code=%d stderr=%s", name, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "record verification: ok") {
			t.Fatalf("%s record missing success line: %s", name, stdout.String())
		}
	}
}

// TestVerifyRecordV2DetectsRouteContextTamper proves a route-context field is
// covered by decision_hash: altering topology_profile (or any route-context
// field) on a V2 record makes verification fail. Integrity is extended to the
// new fields; it does not add attestation.
func TestVerifyRecordV2DetectsRouteContextTamper(t *testing.T) {
	dir := t.TempDir()
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:       governance.TransportMCP,
		ToolName:        "github.create_or_update_file",
		Action:          "deny",
		TrustScore:      1,
		TrustState:      governance.TrustStateTrusted.String(),
		AdapterID:       "securegithub",
		RouteID:         "mcp:github.create_or_update_file",
		TopologyProfile: "single-tenant-routed",
	})

	// Tamper with topology_profile after emission but keep the stored
	// decision_hash, exactly as an after-the-fact edit would.
	rec.TopologyProfile = "multi-tenant"
	path := filepath.Join(dir, "tampered.json")
	writeRecord(t, path, rec)

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"verify-record", path}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("tampered route-context record verified: stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "decision_hash mismatch") {
		t.Fatalf("expected decision_hash mismatch, got: %s", stderr.String())
	}
}

// TestCommittedV2ExampleVerifies pins the committed V2 example fixture so the
// documented walkthrough cannot silently drift: the example must verify and
// must actually carry the route-context fields the docs describe.
func TestCommittedV2ExampleVerifies(t *testing.T) {
	const example = "../docs/examples/decision-record-v2.example.json"
	body, err := os.ReadFile(example)
	if err != nil {
		t.Fatalf("read committed V2 example: %v", err)
	}
	var rec governance.DecisionRecordV1
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatalf("parse committed V2 example: %v", err)
	}
	if rec.SchemaVersion != "2" {
		t.Fatalf("committed example must be schema_version 2, got %q", rec.SchemaVersion)
	}
	if rec.AdapterID == "" || rec.RouteID == "" || rec.TopologyProfile == "" || rec.ExecutionClaim == nil {
		t.Fatalf("committed V2 example must carry every route-context field: %#v", rec)
	}
	if err := governance.VerifyDecisionRecord(rec, nil, "", ""); err != nil {
		t.Fatalf("committed V2 example failed verification: %v", err)
	}
}

func writeRecord(t *testing.T, path string, rec governance.DecisionRecordV1) {
	t.Helper()
	body, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}
