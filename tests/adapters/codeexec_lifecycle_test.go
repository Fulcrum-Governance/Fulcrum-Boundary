package adapters_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/codeexec"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestCodeExecGovernedLifecycleDeniedCodeDoesNotExecute(t *testing.T) {
	executor := &codeExecRecordingExecutor{}
	adapter := codeexec.NewAdapterWithExecutor("tenant-1", executor, codeexec.LocalProcessBoundary("test-boundary"))
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{
			Name:   "deny-code-exec",
			Tool:   "code_exec",
			Action: "deny",
			Reason: "code execution blocked",
		}},
	}, nil, nil, &collectingAuditPublisher{})

	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "print('should not run')",
		Language: "python",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("denied code reached executor")
	}
	if resp.ExitCode != 126 || resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("expected denied CodeExec response, got %+v", resp)
	}
}

func TestCodeExecGovernedLifecycleAllowedCodeExecutesOnceAndRecords(t *testing.T) {
	executor := &codeExecRecordingExecutor{}
	auditor := &collectingAuditPublisher{}
	adapter := codeexec.NewAdapterWithExecutor("tenant-1", executor, codeexec.LocalProcessBoundary("test-boundary"))
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, auditor)

	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "print('ok')",
		Language: "python",
		AgentID:  "agent-1",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" || resp.Metadata["x-fulcrum-request-id"] == "" {
		t.Fatalf("governance metadata missing: %+v", resp.Metadata)
	}
	if resp.Metadata["codeexec_secure_sandbox"] != "false" {
		t.Fatalf("local-process boundary must not claim secure sandbox: %+v", resp.Metadata)
	}
	if got := len(auditor.Events()); got != 1 {
		t.Fatalf("expected one decision record, got %d", got)
	}
}

func TestCodeExecGovernedLifecycleSandboxPolicyDeniesBeforeExecute(t *testing.T) {
	executor := &codeExecRecordingExecutor{}
	adapter := codeexec.NewAdapterWithExecutor("tenant-1", executor, codeexec.LocalProcessBoundary("test-boundary"))
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, &collectingAuditPublisher{})

	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "import subprocess\nsubprocess.run(['whoami'])",
		Language: "python",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("sandbox-policy-denied code reached executor")
	}
	if resp.ExitCode != 126 || resp.Metadata["x-fulcrum-rule"] != "codeexec-sandbox-policy" {
		t.Fatalf("expected sandbox policy denial, got %+v", resp)
	}
}

func TestCodeExecGovernedLifecyclePipelineErrorFailsClosed(t *testing.T) {
	executor := &codeExecRecordingExecutor{}
	auditor := &collectingAuditPublisher{}
	adapter := codeexec.NewAdapterWithExecutor("tenant-1", executor, codeexec.LocalProcessBoundary("test-boundary"))
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, errorEvaluator{}, auditor)

	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "print('should not run')",
		Language: "python",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("pipeline error allowed code execution")
	}
	if resp.ExitCode != 126 || resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("expected fail-closed denied response, got %+v", resp)
	}
	if events := auditor.Events(); len(events) != 1 || events[0].Action != "deny" {
		t.Fatalf("expected denied audit event, got %+v", events)
	}
}

func TestCodeExecDirectExecutionBypassLimitation(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "direct-codeexec.txt")
	if err := os.WriteFile(marker, []byte("bypass"), 0o600); err != nil {
		t.Fatalf("direct host write failed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("direct host write did not create marker: %v", err)
	}
}

type codeExecRecordingExecutor struct {
	calls int
}

func (e *codeExecRecordingExecutor) Execute(_ context.Context, _ *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	e.calls++
	return &governance.ToolResponse{
		Content:     []byte("stdout\n"),
		ContentType: "text/plain",
		ExitCode:    0,
		Metadata: map[string]string{
			"stderr":  "",
			"timeout": "false",
		},
	}, nil
}
