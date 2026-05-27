package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Compile-time interface check.
var _ governance.TransportAdapter = (*Adapter)(nil)

func TestAdapter_Type(t *testing.T) {
	a := NewAdapter("tenant-1")
	if a.Type() != governance.TransportCLI {
		t.Errorf("expected TransportCLI, got %s", a.Type())
	}
}

func TestAdapter_ParseRequest_Struct(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{
		Command:  "ls -la /tmp",
		AgentID:  "agent-1",
		TenantID: "tenant-1",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if req.Transport != governance.TransportCLI {
		t.Errorf("Transport = %s, want %s", req.Transport, governance.TransportCLI)
	}
	if req.ToolName != "ls" {
		t.Errorf("ToolName = %q, want %q", req.ToolName, "ls")
	}
	if req.Action != "read" {
		t.Errorf("Action = %q, want %q", req.Action, "read")
	}
	if req.Command != "ls -la /tmp" {
		t.Errorf("Command = %q, want %q", req.Command, "ls -la /tmp")
	}
	if req.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", req.AgentID, "agent-1")
	}
	if req.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", req.TenantID, "tenant-1")
	}
	if req.RequestID == "" {
		t.Error("RequestID should be generated")
	}
	if len(req.PipeChain) != 1 {
		t.Fatalf("PipeChain length = %d, want 1", len(req.PipeChain))
	}
	if req.PipeChain[0].Command != "ls" {
		t.Errorf("PipeChain[0].Command = %q, want %q", req.PipeChain[0].Command, "ls")
	}
	if req.PipeChain[0].RiskLevel != "read" {
		t.Errorf("PipeChain[0].RiskLevel = %q, want %q", req.PipeChain[0].RiskLevel, "read")
	}
}

func TestAdapter_ParseRequest_ValueStruct(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := CommandInput{
		Command: "echo hello",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ToolName != "echo" {
		t.Errorf("ToolName = %q, want %q", req.ToolName, "echo")
	}
}

func TestAdapter_ParseRequest_JSON(t *testing.T) {
	a := NewAdapter("default-tenant")
	data := json.RawMessage(`{"command":"cat /etc/hosts | grep localhost","agent_id":"agent-2"}`)

	req, err := a.ParseRequest(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ToolName != "cat" {
		t.Errorf("ToolName = %q, want %q", req.ToolName, "cat")
	}
	if req.AgentID != "agent-2" {
		t.Errorf("AgentID = %q, want %q", req.AgentID, "agent-2")
	}
	if len(req.PipeChain) != 2 {
		t.Fatalf("PipeChain length = %d, want 2", len(req.PipeChain))
	}
	if req.Action != "read" {
		t.Errorf("Action = %q, want %q (both cat and grep are read)", req.Action, "read")
	}
}

func TestAdapter_ParseRequest_Bytes(t *testing.T) {
	a := NewAdapter("default-tenant")
	data := []byte(`{"command":"rm -rf /tmp/test"}`)

	req, err := a.ParseRequest(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.ToolName != "rm" {
		t.Errorf("ToolName = %q, want %q", req.ToolName, "rm")
	}
	if req.Action != "destructive" {
		t.Errorf("Action = %q, want %q", req.Action, "destructive")
	}
}

func TestAdapter_ParseRequest_DefaultTenant(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{
		Command: "pwd",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.TenantID != "default-tenant" {
		t.Errorf("TenantID = %q, want %q", req.TenantID, "default-tenant")
	}
}

func TestAdapter_ParseRequest_PipeChainClassification(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{
		Command: "cat /etc/passwd | grep root | tee /tmp/output",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(req.PipeChain) != 3 {
		t.Fatalf("PipeChain length = %d, want 3", len(req.PipeChain))
	}

	// cat = read, grep = read, tee = write
	expected := []struct {
		cmd  string
		risk string
	}{
		{"cat", "read"},
		{"grep", "read"},
		{"tee", "write"},
	}
	for i, exp := range expected {
		if req.PipeChain[i].Command != exp.cmd {
			t.Errorf("PipeChain[%d].Command = %q, want %q", i, req.PipeChain[i].Command, exp.cmd)
		}
		if req.PipeChain[i].RiskLevel != exp.risk {
			t.Errorf("PipeChain[%d].RiskLevel = %q, want %q", i, req.PipeChain[i].RiskLevel, exp.risk)
		}
	}

	// Highest risk should be write
	if req.Action != "write" {
		t.Errorf("Action = %q, want %q", req.Action, "write")
	}
}

func TestAdapter_ParseRequest_WithStdin(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{
		Command: "cat",
		Stdin:   []byte("some input data"),
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(req.Stdin) != "some input data" {
		t.Errorf("Stdin = %q, want %q", req.Stdin, "some input data")
	}
}

func TestAdapter_ParseRequest_EmptyCommand(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{Command: ""}
	_, err := a.ParseRequest(context.Background(), input)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestAdapter_ParseRequest_InvalidJSON(t *testing.T) {
	a := NewAdapter("default-tenant")
	data := json.RawMessage(`{invalid}`)
	_, err := a.ParseRequest(context.Background(), data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAdapter_ParseRequest_UnsupportedType(t *testing.T) {
	a := NewAdapter("default-tenant")
	_, err := a.ParseRequest(context.Background(), 42)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestAdapter_ParseRequest_UnbalancedQuotes(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{Command: `echo "hello`}
	_, err := a.ParseRequest(context.Background(), input)
	if err == nil {
		t.Error("expected error for unbalanced quotes")
	}
}

func TestAdapter_InspectResponse_Nil(t *testing.T) {
	a := NewAdapter("default-tenant")
	insp, err := a.InspectResponse(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !insp.Safe {
		t.Error("expected safe=true for nil response")
	}
}

func TestAdapter_InspectResponse_CleanOutput(t *testing.T) {
	a := NewAdapter("default-tenant")
	resp := &governance.ToolResponse{
		Content: []byte("total 0\ndrwxr-xr-x  2 user staff 64 Jan  1 00:00 .\n"),
	}
	insp, err := a.InspectResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !insp.Safe {
		t.Errorf("expected safe=true for clean output, got concerns: %v", insp.Concerns)
	}
}

func TestAdapter_InspectResponse_SensitiveOutput(t *testing.T) {
	a := NewAdapter("default-tenant")
	resp := &governance.ToolResponse{
		Content: []byte("DATABASE_URL=" + "postgresql://u:p@db:5432/prod\n"),
	}
	insp, err := a.InspectResponse(context.Background(), resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if insp.Safe {
		t.Error("expected safe=false for sensitive output")
	}
	if !insp.SensitiveData {
		t.Error("expected SensitiveData=true")
	}
}

func TestAdapter_EmitGovernanceMetadata(t *testing.T) {
	a := NewAdapter("default-tenant")
	resp := &governance.ToolResponse{Content: []byte("ok")}
	decision := &governance.GovernanceDecision{
		Action:     "allow",
		EnvelopeID: "env-1",
		RequestID:  "req-1",
		PolicyID:   "pol-1",
	}

	err := a.EmitGovernanceMetadata(context.Background(), resp, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" {
		t.Errorf("x-fulcrum-action = %q, want %q", resp.Metadata["x-fulcrum-action"], "allow")
	}
	if resp.Metadata["x-fulcrum-envelope-id"] != "env-1" {
		t.Errorf("x-fulcrum-envelope-id = %q, want %q", resp.Metadata["x-fulcrum-envelope-id"], "env-1")
	}
	if resp.Metadata["x-fulcrum-request-id"] != "req-1" {
		t.Errorf("x-fulcrum-request-id = %q, want %q", resp.Metadata["x-fulcrum-request-id"], "req-1")
	}
	if resp.Metadata["x-fulcrum-policy-id"] != "pol-1" {
		t.Errorf("x-fulcrum-policy-id = %q, want %q", resp.Metadata["x-fulcrum-policy-id"], "pol-1")
	}
}

func TestAdapter_EmitGovernanceMetadata_NoPolicyID(t *testing.T) {
	a := NewAdapter("default-tenant")
	resp := &governance.ToolResponse{Content: []byte("ok")}
	decision := &governance.GovernanceDecision{
		Action:     "deny",
		EnvelopeID: "env-2",
		RequestID:  "req-2",
	}

	err := a.EmitGovernanceMetadata(context.Background(), resp, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.Metadata["x-fulcrum-policy-id"]; ok {
		t.Error("x-fulcrum-policy-id should not be set when PolicyID is empty")
	}
}

func TestAdapter_EmitGovernanceMetadata_NilInputs(t *testing.T) {
	a := NewAdapter("default-tenant")
	if err := a.EmitGovernanceMetadata(context.Background(), nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_EmitGovernanceMetadata_NilResp(t *testing.T) {
	a := NewAdapter("default-tenant")
	decision := &governance.GovernanceDecision{Action: "allow"}
	if err := a.EmitGovernanceMetadata(context.Background(), nil, decision); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_EmitGovernanceMetadata_NilDecision(t *testing.T) {
	a := NewAdapter("default-tenant")
	resp := &governance.ToolResponse{Content: []byte("ok")}
	if err := a.EmitGovernanceMetadata(context.Background(), resp, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdapter_ForwardGoverned_ReturnsError(t *testing.T) {
	a := NewAdapter("default-tenant")
	_, err := a.ForwardGoverned(context.Background(), nil, nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestAdapter_ForwardGoverned_DeniedDoesNotExecute(t *testing.T) {
	executor := &recordingExecutor{}
	a := NewAdapterWithExecutor("default-tenant", executor)
	req := &governance.GovernanceRequest{
		Transport: governance.TransportCLI,
		Command:   "echo denied",
		PipeChain: []governance.PipeSegment{{Command: "echo", Args: []string{"denied"}}},
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{Action: "deny", Reason: "blocked"})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("denied command reached executor")
	}
	if resp.ExitCode != 126 {
		t.Fatalf("denied response exit code = %d, want 126", resp.ExitCode)
	}
}

func TestAdapter_ForwardGoverned_AllowedExecutesOnceWithMetadata(t *testing.T) {
	executor := &recordingExecutor{response: &governance.ToolResponse{Content: []byte("ok\n")}}
	a := NewAdapterWithExecutor("default-tenant", executor)
	req := &governance.GovernanceRequest{
		Transport: governance.TransportCLI,
		Command:   "echo ok",
		PipeChain: []governance.PipeSegment{{Command: "echo", Args: []string{"ok"}}},
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{
		Action:     "allow",
		RequestID:  "req-1",
		EnvelopeID: "env-1",
		PolicyID:   "pol-1",
	})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" || resp.Metadata["x-fulcrum-policy-id"] != "pol-1" {
		t.Fatalf("governance metadata missing: %+v", resp.Metadata)
	}
}

func TestAdapter_WithCustomClassifier(t *testing.T) {
	c := NewClassifier()
	c.Overrides["my-tool"] = RiskRead

	a := NewAdapterWithClassifier("default-tenant", c)
	input := &CommandInput{
		Command: "my-tool --flag",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Action != "read" {
		t.Errorf("Action = %q, want %q (custom classifier)", req.Action, "read")
	}
	if req.PipeChain[0].RiskLevel != "read" {
		t.Errorf("PipeChain[0].RiskLevel = %q, want %q", req.PipeChain[0].RiskLevel, "read")
	}
}

func TestOSExecutor_ExecutesWithoutShellAndAddsGovernedEnv(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{Command: "printf ${BOUNDARY_GOVERNED}:${BOUNDARY_TRANSPORT}"}
	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	resp, err := a.ForwardGoverned(context.Background(), req, &governance.GovernanceDecision{Action: "allow", RequestID: "req-1", EnvelopeID: "env-1"})
	if err != nil {
		t.Fatalf("ForwardGoverned: %v", err)
	}
	if string(resp.Content) != "${BOUNDARY_GOVERNED}:${BOUNDARY_TRANSPORT}" {
		t.Fatalf("command appears to have been shell-expanded, got %q", string(resp.Content))
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" {
		t.Fatalf("governance metadata missing: %+v", resp.Metadata)
	}
}

type recordingExecutor struct {
	calls    int
	response *governance.ToolResponse
}

func (e *recordingExecutor) Execute(_ context.Context, _ *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	e.calls++
	if e.response != nil {
		return e.response, nil
	}
	return &governance.ToolResponse{Content: []byte("ok\n"), Metadata: map[string]string{}}, nil
}

func TestAdapter_DestructivePipeChain(t *testing.T) {
	a := NewAdapter("default-tenant")
	input := &CommandInput{
		Command: "find /tmp -name '*.log' | xargs rm",
	}

	req, err := a.ParseRequest(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// find = read, xargs = admin (unknown), rm = destructive
	// But xargs is the command, not rm. xargs is unknown → admin.
	// The pipe chain is: find | xargs rm
	// Segment 1: command=find, args=[-name, *.log] → read
	// Segment 2: command=xargs, args=[rm] → admin (unknown default)
	// Highest risk = admin
	if req.Action != "admin" {
		t.Errorf("Action = %q, want %q", req.Action, "admin")
	}
}
