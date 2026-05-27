package tests

import (
	"strings"
	"testing"

	"github.com/fulcrum-governance/boundary/governance"
)

func TestProjectPolicyEvalRequestIncludesFullContext(t *testing.T) {
	score := 0.5
	req := &governance.GovernanceRequest{
		RequestID: "req-1",
		Transport: governance.TransportMCP,
		AgentID:   "agent-1",
		TenantID:  "tenant-1",
		ToolName:  "query",
		Action:    "tools/call",
		Arguments: map[string]any{
			"sql":       "SELECT * FROM users",
			"sql_class": "READ",
			"table":     "users",
		},
		EnvelopeID: "env-1",
		TraceID:    "trace-1",
	}

	projected := governance.ProjectPolicyEvalRequest(req, &score, governance.TrustStateEvaluating, "policy-v1")
	if projected.TenantID != "tenant-1" || projected.AgentID != "agent-1" {
		t.Fatalf("missing identity context: %#v", projected)
	}
	if projected.Transport != "mcp" || projected.ToolName != "query" || projected.Action != "tools/call" {
		t.Fatalf("missing action context: %#v", projected)
	}
	if projected.TrustScore == nil || *projected.TrustScore != 0.5 || projected.TrustState != "EVALUATING" {
		t.Fatalf("missing trust context: %#v", projected)
	}
	if projected.RiskClass != "READ" {
		t.Fatalf("expected risk class READ, got %q", projected.RiskClass)
	}
	if len(projected.ResourceIDs) != 1 || projected.ResourceIDs[0] != "users" {
		t.Fatalf("expected users resource, got %#v", projected.ResourceIDs)
	}
	if !strings.HasPrefix(projected.RequestHash, "sha256:") {
		t.Fatalf("expected request hash, got %q", projected.RequestHash)
	}

	ctx := projected.ToProtoContext()
	if ctx.Attributes["argument.sql_class"] != "READ" {
		t.Fatalf("missing argument projection: %#v", ctx.Attributes)
	}
	if ctx.Attributes["request.hash"] != projected.RequestHash {
		t.Fatalf("request hash not copied to attributes: %#v", ctx.Attributes)
	}
}
