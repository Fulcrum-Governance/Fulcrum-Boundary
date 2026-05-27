package a2a

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestAdapter_Type(t *testing.T) {
	if NewAdapter("t").Type() != governance.TransportA2A {
		t.Fatal("Type should be TransportA2A")
	}
}

func TestAdapter_ParseRequest_FromStruct(t *testing.T) {
	a := NewAdapter("tenant-X")
	msg := &TaskMessage{
		TaskID:    "task-123",
		AgentCard: AgentCard{AgentID: "agent-7", Name: "Worker", Endpoint: "https://x"},
		Action:    "execute_query",
		Input:     map[string]any{"query": "SELECT 1"},
	}
	req, err := a.ParseRequest(context.Background(), msg)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Transport != governance.TransportA2A {
		t.Errorf("transport = %s", req.Transport)
	}
	if req.ToolName != "execute_query" {
		t.Errorf("ToolName = %q, want execute_query", req.ToolName)
	}
	if req.AgentID != "agent-7" {
		t.Errorf("AgentID = %q", req.AgentID)
	}
	if req.TenantID != "tenant-X" {
		t.Errorf("TenantID = %q", req.TenantID)
	}
	if req.TraceID != "task-123" {
		t.Errorf("TraceID = %q", req.TraceID)
	}
	if req.Action != "a2a/task" {
		t.Errorf("Action = %q", req.Action)
	}
	if v, _ := req.Arguments["query"].(string); v != "SELECT 1" {
		t.Errorf("Arguments did not propagate: %+v", req.Arguments)
	}
}

func TestAdapter_ParseRequest_FromValueStruct(t *testing.T) {
	a := NewAdapter("t")
	msg := TaskMessage{Action: "ping", AgentCard: AgentCard{AgentID: "a"}}
	req, err := a.ParseRequest(context.Background(), msg)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.ToolName != "ping" {
		t.Fatal("value-struct path failed")
	}
}

func TestAdapter_ParseRequest_FromJSONBytes(t *testing.T) {
	a := NewAdapter("tenant-Y")
	body, _ := json.Marshal(TaskMessage{
		TaskID:    "t-1",
		AgentCard: AgentCard{AgentID: "agent-9"},
		Action:    "summarize",
		Input:     map[string]any{"doc": "hello"},
	})
	req, err := a.ParseRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.ToolName != "summarize" || req.AgentID != "agent-9" || req.TenantID != "tenant-Y" {
		t.Fatalf("JSON path produced wrong fields: %+v", req)
	}
}

func TestAdapter_ParseRequest_FromRawMessage(t *testing.T) {
	a := NewAdapter("t")
	body := json.RawMessage(`{"action":"x","agent_card":{"agent_id":"a"}}`)
	req, err := a.ParseRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.ToolName != "x" {
		t.Fatal("RawMessage path failed")
	}
}

func TestAdapter_ParseRequest_Errors(t *testing.T) {
	a := NewAdapter("t")
	if _, err := a.ParseRequest(context.Background(), 42); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if _, err := a.ParseRequest(context.Background(), []byte("{not-json")); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if _, err := a.ParseRequest(context.Background(), &TaskMessage{Action: ""}); err == nil {
		t.Fatal("expected error for missing Action")
	}
}

func TestAdapter_ParseRequest_FromTaskEnvelopeJSON(t *testing.T) {
	a := NewAdapter("tenant-Z")
	body := []byte(`{
		"task_id":"task-z",
		"sender_agent_id":"agent-z",
		"receiver":"worker-z",
		"action":"summarize",
		"input":{"text":"hello"},
		"required_fields":["task_id","sender_agent_id","action"]
	}`)
	req, err := a.ParseRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.ToolName != "summarize" || req.AgentID != "agent-z" || req.TraceID != "task-z" {
		t.Fatalf("TaskEnvelope JSON path produced wrong fields: %+v", req)
	}
}

func TestAdapter_ParseRequest_FromJSONRPCMessageSend(t *testing.T) {
	a := NewAdapter("tenant-jsonrpc")
	body := []byte(`{
		"jsonrpc":"2.0",
		"id":"1",
		"method":"message/send",
		"params":{
			"message":{"taskId":"task-9","messageId":"msg-1","parts":[{"kind":"text","text":"run report"}]},
			"metadata":{"sender_agent_id":"agent-json","receiver":"agent-worker","action":"report.generate"}
		}
	}`)
	req, err := a.ParseRequest(context.Background(), body)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.ToolName != "report.generate" || req.AgentID != "agent-json" {
		t.Fatalf("JSON-RPC path produced wrong fields: %+v", req)
	}
	if got, _ := req.Arguments["text"].(string); got != "run report" {
		t.Fatalf("JSON-RPC text part did not propagate: %+v", req.Arguments)
	}
}

func TestAdapter_ParseRequest_UnknownRequiredFieldFailsClosed(t *testing.T) {
	a := NewAdapter("tenant-Z")
	body := []byte(`{
		"task_id":"task-z",
		"sender_agent_id":"agent-z",
		"action":"summarize",
		"required_fields":["future_mandatory_field"]
	}`)
	_, err := a.ParseRequest(context.Background(), body)
	if err == nil || !strings.Contains(err.Error(), "unsupported required field") {
		t.Fatalf("expected unsupported required field error, got %v", err)
	}
}

func TestAdapter_ForwardGoverned_DenialDoesNotForward(t *testing.T) {
	forwarder := &MemoryForwarder{}
	a := NewForwardingAdapter("t", forwarder)
	req := &governance.GovernanceRequest{
		Transport: governance.TransportA2A,
		AgentID:   "agent-1",
		ToolName:  "send",
		Arguments: map[string]any{"task_id": "task-denied"},
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{Action: "deny", Reason: "blocked"})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if len(forwarder.Snapshot()) != 0 {
		t.Fatal("denied task reached downstream forwarder")
	}
	if !strings.Contains(string(resp.Content), `"status":"denied"`) {
		t.Fatalf("expected transport-shaped denial, got %s", string(resp.Content))
	}
}

func TestAdapter_ForwardGoverned_AllowedForwardsAndAddsMetadata(t *testing.T) {
	forwarder := &MemoryForwarder{}
	a := NewForwardingAdapter("t", forwarder)
	req := &governance.GovernanceRequest{
		Transport: governance.TransportA2A,
		AgentID:   "agent-1",
		ToolName:  "send",
		Arguments: map[string]any{"task_id": "task-allowed", "payload": "ok"},
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{Action: "allow", RequestID: "req-1", EnvelopeID: "env-1", MatchedRule: "allow-send"})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if len(forwarder.Snapshot()) != 1 {
		t.Fatal("allowed task was not forwarded exactly once")
	}
	if !strings.Contains(string(resp.Content), `"matched_rule":"allow-send"`) {
		t.Fatalf("expected governance metadata in response, got %s", string(resp.Content))
	}
}

func TestAdapter_InspectResponse_FlagsPolicySignals(t *testing.T) {
	a := NewAdapter("t")
	insp, err := a.InspectResponse(context.Background(), &governance.ToolResponse{
		Content: []byte(`{"output":{"message":"secret token leaked"}}`),
	})
	if err != nil {
		t.Fatalf("InspectResponse: %v", err)
	}
	if insp == nil || insp.Safe {
		t.Fatalf("expected unsafe inspection, got %+v", insp)
	}
}

func TestAdapter_EmitGovernanceMetadata(t *testing.T) {
	a := NewAdapter("t")
	resp := &governance.ToolResponse{}
	err := a.EmitGovernanceMetadata(context.Background(), resp, &governance.GovernanceDecision{
		Action:      "allow",
		RequestID:   "req-1",
		EnvelopeID:  "env-1",
		MatchedRule: "allow-send",
	})
	if err != nil {
		t.Fatalf("EmitGovernanceMetadata: %v", err)
	}
	if resp.Metadata["x-fulcrum-rule"] != "allow-send" {
		t.Fatalf("metadata not attached: %+v", resp.Metadata)
	}
}
