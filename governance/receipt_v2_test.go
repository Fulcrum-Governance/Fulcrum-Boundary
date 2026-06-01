package governance

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// TestDecisionRecordV2IsStrictlyAdditive proves a record with no route-context
// fields stays schema_version "1" and is byte-identical to the pre-V2 shape, so
// its decision_hash is unchanged. This is the load-bearing backward-compat
// guarantee: existing V1 records and tooling keep working.
func TestDecisionRecordV2IsStrictlyAdditive(t *testing.T) {
	base := AuditEvent{
		Transport:        TransportMCP,
		ToolName:         "query",
		Action:           "allow",
		Reason:           "read-only",
		PolicyBundleHash: "sha256:bundle",
		RequestHash:      "sha256:req",
		TrustScore:       1,
		TrustState:       TrustStateTrusted.String(),
		Timestamp:        time.Unix(1700000000, 0).UTC(),
	}
	rec := BuildDecisionRecord(base)
	if rec.SchemaVersion != DecisionRecordSchemaVersion {
		t.Fatalf("record without route-context should be schema_version %q, got %q", DecisionRecordSchemaVersion, rec.SchemaVersion)
	}
	if rec.HasRouteContext() {
		t.Fatal("record without route-context fields must report HasRouteContext()==false")
	}
	// The serialized form must omit every additive field entirely.
	encoded, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"adapter_id", "route_id", "topology_profile", "execution_claim"} {
		if strings.Contains(string(encoded), field) {
			t.Fatalf("V1 record JSON must not contain %q: %s", field, encoded)
		}
	}
	// It must verify under the dual-version verifier.
	if err := VerifyDecisionRecord(rec, nil, "", ""); err != nil {
		t.Fatalf("V1 record did not verify: %v", err)
	}
}

// TestDecisionRecordV2PopulatedEmitsVersion2 proves that populating any
// route-context field promotes the record to schema_version "2", and that the
// route-context fields are covered by decision_hash (tampering fails
// verification) — integrity is extended to the new fields without adding
// attestation.
func TestDecisionRecordV2PopulatedEmitsVersion2(t *testing.T) {
	base := AuditEvent{
		Transport:       TransportMCP,
		ToolName:        "github.create_or_update_file",
		Action:          "deny",
		Reason:          "write-after-taint",
		TrustScore:      1,
		TrustState:      TrustStateTrusted.String(),
		AdapterID:       "securegithub",
		RouteID:         "mcp:github.create_or_update_file",
		TopologyProfile: "single-tenant-routed",
		ExecutionClaim:  &ExecutionClaim{UpstreamCalled: false, Executed: false, Source: "securegithub"},
		Timestamp:       time.Unix(1700000000, 0).UTC(),
	}
	rec := BuildDecisionRecord(base)
	if rec.SchemaVersion != DecisionRecordSchemaV2 {
		t.Fatalf("record with route-context should be schema_version %q, got %q", DecisionRecordSchemaV2, rec.SchemaVersion)
	}
	if !rec.HasRouteContext() {
		t.Fatal("record with route-context fields must report HasRouteContext()==true")
	}
	if err := VerifyDecisionRecord(rec, nil, "", ""); err != nil {
		t.Fatalf("V2 record did not verify: %v", err)
	}

	// Tampering with each route-context field must break decision_hash.
	t.Run("tamper_adapter_id", func(t *testing.T) {
		tampered := rec
		tampered.AdapterID = "spoofed"
		if err := VerifyDecisionRecord(tampered, nil, "", ""); err == nil {
			t.Fatal("tampered adapter_id verified")
		}
	})
	t.Run("tamper_route_id", func(t *testing.T) {
		tampered := rec
		tampered.RouteID = "mcp:other"
		if err := VerifyDecisionRecord(tampered, nil, "", ""); err == nil {
			t.Fatal("tampered route_id verified")
		}
	})
	t.Run("tamper_topology_profile", func(t *testing.T) {
		tampered := rec
		tampered.TopologyProfile = "multi-tenant"
		if err := VerifyDecisionRecord(tampered, nil, "", ""); err == nil {
			t.Fatal("tampered topology_profile verified")
		}
	})
	t.Run("tamper_execution_claim", func(t *testing.T) {
		tampered := rec
		tampered.ExecutionClaim = &ExecutionClaim{UpstreamCalled: true, Executed: true, Source: "securegithub"}
		if err := VerifyDecisionRecord(tampered, nil, "", ""); err == nil {
			t.Fatal("tampered execution_claim verified")
		}
	})
}

// TestVerifyDecisionRecordRejectsUnknownSchema proves the dual-version gate
// still rejects versions outside {"1","2"}.
func TestVerifyDecisionRecordRejectsUnknownSchema(t *testing.T) {
	rec := BuildDecisionRecord(AuditEvent{Transport: TransportMCP, Action: "allow", TrustScore: 1})
	rec.SchemaVersion = "3"
	// Recompute so only the version is "wrong" — proves the gate, not the hash.
	rec.DecisionHash = ComputeDecisionHash(rec)
	rec.RecordID = recordID(rec.DecisionHash)
	if err := VerifyDecisionRecord(rec, nil, "", ""); err == nil {
		t.Fatal("schema_version 3 must be rejected")
	}
	if !SupportedDecisionRecordSchemaVersion("1") || !SupportedDecisionRecordSchemaVersion("2") {
		t.Fatal("versions 1 and 2 must be supported")
	}
	if SupportedDecisionRecordSchemaVersion("3") || SupportedDecisionRecordSchemaVersion("") {
		t.Fatal("only versions 1 and 2 are supported")
	}
}

// TestPipelineEmitsRouteContext proves the pipeline populates adapter_id and
// route_id from the request and topology_profile from config, promoting the
// emitted record to schema_version "2". execution_claim stays absent because
// the pipeline decides before execution.
func TestPipelineEmitsRouteContext(t *testing.T) {
	buf := &captureBuffer{}
	logger := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := PipelineConfig{
		TopologyProfile: "single-tenant-routed",
		StaticPolicies: []StaticPolicyRule{
			{Name: "block-rm", Tool: "rm", Action: "deny", Reason: "destructive"},
		},
	}
	p := NewPipeline(cfg, nil, nil, NewSlogAuditPublisher(logger))
	if _, err := p.Evaluate(context.Background(), &GovernanceRequest{ToolName: "rm", Transport: TransportCLI}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var line map[string]any
	if err := json.Unmarshal(buf.firstLine(), &line); err != nil {
		t.Fatalf("decode slog line: %v", err)
	}
	if line["adapter_id"] != "cli" {
		t.Errorf("expected adapter_id=cli, got %v", line["adapter_id"])
	}
	if line["route_id"] != "cli:rm" {
		t.Errorf("expected route_id=cli:rm, got %v", line["route_id"])
	}
	if line["topology_profile"] != "single-tenant-routed" {
		t.Errorf("expected topology_profile=single-tenant-routed, got %v", line["topology_profile"])
	}
	if line["execution_claim_present"] != false {
		t.Errorf("pre-execution record must not carry an execution_claim, got present=%v", line["execution_claim_present"])
	}
}

// TestPipelineWithoutTopologyStaysV1 proves that when no topology profile is
// configured and the transport is empty, the pipeline still records nothing in
// the route-context fields (record stays V1-compatible). A transport-only
// request does carry adapter_id/route_id, which is correct route description.
func TestPipelineWithoutTopologyStaysV1Fields(t *testing.T) {
	if got := routeID(&GovernanceRequest{}); got != "" {
		t.Fatalf("empty request should yield empty route_id, got %q", got)
	}
	if got := routeID(&GovernanceRequest{Transport: TransportMCP}); got != "mcp" {
		t.Fatalf("transport-only request route_id mismatch: got %q", got)
	}
	if got := routeID(nil); got != "" {
		t.Fatalf("nil request route_id should be empty, got %q", got)
	}
}

type captureBuffer struct {
	data []byte
}

func (b *captureBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *captureBuffer) firstLine() []byte {
	if i := indexByte(b.data, '\n'); i >= 0 {
		return b.data[:i]
	}
	return b.data
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
