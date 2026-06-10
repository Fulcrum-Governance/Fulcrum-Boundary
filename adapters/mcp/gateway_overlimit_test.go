package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// captureAuditor records published audit events so the over-limit path can be
// asserted to emit a parse rejection rather than silently dropping.
type captureAuditor struct {
	events []governance.AuditEvent
}

func (c *captureAuditor) Publish(_ context.Context, event governance.AuditEvent) {
	c.events = append(c.events, event)
}

// echoForwarder is an upstream stub that must never be reached on the
// over-limit path.
type echoForwarder struct {
	called bool
}

func (e *echoForwarder) Forward(context.Context, []byte) (*governance.ToolResponse, error) {
	e.called = true
	return &governance.ToolResponse{Content: []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)}, nil
}

// TestGateway_OverLimitBodyRejectedWithJSONRPCError verifies that a request
// body exceeding the configured cap is rejected with a JSON-RPC error envelope
// and an audited parse rejection, never buffered whole and never forwarded
// upstream.
func TestGateway_OverLimitBodyRejectedWithJSONRPCError(t *testing.T) {
	auditor := &captureAuditor{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{GatewayVersion: "test"}, nil, nil, auditor)
	forwarder := &echoForwarder{}
	gateway := NewGateway(pipeline, forwarder, "tenant-1")
	gateway.MaxRequestBytes = 64 // small cap to keep the test cheap

	// A syntactically valid JSON-RPC envelope whose body far exceeds the cap.
	oversized := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{"blob":"` +
		strings.Repeat("A", 4096) + `"}}}`

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(oversized))
	req.Header.Set(governance.HeaderGovernanceAgentID, "agent-1")
	rec := httptest.NewRecorder()

	gateway.ServeHTTP(rec, req)

	// The gateway returns JSON-RPC errors over HTTP 200 (its established
	// convention), not a bare 500.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JSON-RPC error envelope); body=%s", rec.Code, rec.Body.String())
	}
	if forwarder.called {
		t.Fatal("over-limit request must not be forwarded upstream")
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON (possible panic/truncation): %v; body=%s", err, rec.Body.String())
	}
	if resp.JSONRPC != "2.0" {
		t.Fatalf("jsonrpc field = %q, want 2.0", resp.JSONRPC)
	}
	if resp.Error == nil {
		t.Fatalf("expected JSON-RPC error envelope, got: %s", rec.Body.String())
	}
	if resp.Error.Code != -32600 {
		t.Fatalf("error code = %d, want -32600 (invalid request)", resp.Error.Code)
	}

	if len(auditor.events) != 1 {
		t.Fatalf("audit events = %d, want 1 parse rejection", len(auditor.events))
	}
	event := auditor.events[0]
	if event.EventType != "parse_rejected" {
		t.Fatalf("event type = %q, want parse_rejected", event.EventType)
	}
	if event.AgentID != "agent-1" || event.TenantID != "tenant-1" {
		t.Fatalf("identity not propagated to rejection event: %#v", event)
	}
	if !strings.Contains(event.Reason, "limit") {
		t.Fatalf("rejection reason should cite the limit, got %q", event.Reason)
	}
}

// TestGateway_BodyWithinLimitNotRejectedAsOverLimit is the control: a request
// comfortably under the cap is processed normally and reaches the forwarder.
func TestGateway_BodyWithinLimitNotRejectedAsOverLimit(t *testing.T) {
	auditor := &captureAuditor{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{GatewayVersion: "test"}, nil, nil, auditor)
	forwarder := &echoForwarder{}
	gateway := NewGateway(pipeline, forwarder, "tenant-1")
	gateway.MaxRequestBytes = 1 << 16 // 64 KiB

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"safe_tool","arguments":{"x":1}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()

	gateway.ServeHTTP(rec, req)

	if !forwarder.called {
		t.Fatalf("within-limit request should reach the forwarder; status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, e := range auditor.events {
		if e.EventType == "parse_rejected" {
			t.Fatalf("within-limit request must not emit a parse rejection: %#v", e)
		}
	}
}

// TestGateway_DefaultMaxRequestBytesUsedWhenUnset pins the documented default
// so an unconfigured gateway is still bounded.
func TestGateway_DefaultMaxRequestBytesUsedWhenUnset(t *testing.T) {
	g := &Gateway{}
	if got := g.maxRequestBytes(); got != DefaultMaxRequestBytes {
		t.Fatalf("unset gateway cap = %d, want default %d", got, DefaultMaxRequestBytes)
	}
	if DefaultMaxRequestBytes != 4<<20 {
		t.Fatalf("DefaultMaxRequestBytes = %d, want 4 MiB", DefaultMaxRequestBytes)
	}
}
