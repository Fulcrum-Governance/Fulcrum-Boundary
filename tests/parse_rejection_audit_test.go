package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fulcrum-governance/boundary/adapters/mcp"
	"github.com/fulcrum-governance/boundary/governance"
)

type captureAuditPublisher struct {
	events []governance.AuditEvent
}

func (p *captureAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.events = append(p.events, event)
}

type unusedForwarder struct{}

func (unusedForwarder) Forward(context.Context, []byte) (*governance.ToolResponse, error) {
	return &governance.ToolResponse{Content: []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)}, nil
}

func TestParseRejectionEmitsReceiptEventWithoutPipelineEvaluation(t *testing.T) {
	auditor := &captureAuditPublisher{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		GatewayVersion: "test-version",
		BuildDigest:    "sha256:test-build",
	}, nil, nil, auditor)
	gateway := mcp.NewGateway(pipeline, unusedForwarder{}, "tenant-1")

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":""}`))
	req.Header.Set(governance.HeaderGovernanceAgentID, "agent-1")
	resp := httptest.NewRecorder()
	gateway.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if len(auditor.events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(auditor.events))
	}
	event := auditor.events[0]
	if event.EventType != "parse_rejected" {
		t.Fatalf("event type = %q", event.EventType)
	}
	if event.RawShapeHash == "" || event.RequestHash != "" {
		t.Fatalf("expected raw shape hash only, got raw=%q request=%q", event.RawShapeHash, event.RequestHash)
	}
	if event.AgentID != "agent-1" || event.TenantID != "tenant-1" {
		t.Fatalf("identity not copied: %#v", event)
	}
	record := governance.BuildDecisionRecord(event)
	if record.EventType != "parse_rejected" || record.RawShapeHash == "" || record.DecisionHash == "" {
		t.Fatalf("parse rejection record missing receipt fields: %#v", record)
	}
}
