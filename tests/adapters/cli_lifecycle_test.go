package adapters_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/cli"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestCLIGovernedLifecycleDeniedCommandDoesNotExecute(t *testing.T) {
	executor := &cliRecordingExecutor{}
	adapter := cli.NewAdapterWithExecutor("tenant-1", executor)
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{
			Name:   "deny-rm",
			Tool:   "rm",
			Action: "deny",
			Reason: "destructive command blocked",
		}},
	}, nil, nil, &collectingAuditPublisher{})

	resp, err := adapter.GovernCommand(context.Background(), cli.CommandInput{Command: "rm /tmp/important"}, pipeline)
	if err != nil {
		t.Fatalf("GovernCommand: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("denied command reached executor")
	}
	if resp.ExitCode != 126 || resp.Metadata["cli_denied"] != "true" {
		t.Fatalf("expected denied CLI response, got %+v", resp)
	}
}

func TestCLIGovernedLifecycleAllowedCommandExecutesOnceAndRecords(t *testing.T) {
	executor := &cliRecordingExecutor{}
	auditor := &collectingAuditPublisher{}
	adapter := cli.NewAdapterWithExecutor("tenant-1", executor)
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, auditor)

	resp, err := adapter.GovernCommand(context.Background(), cli.CommandInput{Command: "echo ok", AgentID: "agent-1"}, pipeline)
	if err != nil {
		t.Fatalf("GovernCommand: %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("executor calls = %d, want 1", executor.calls)
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" || resp.Metadata["x-fulcrum-request-id"] == "" {
		t.Fatalf("governance metadata missing: %+v", resp.Metadata)
	}
	if got := len(auditor.Events()); got != 1 {
		t.Fatalf("expected one decision record, got %d", got)
	}
}

func TestCLIGovernedLifecyclePipelineErrorFailsClosed(t *testing.T) {
	executor := &cliRecordingExecutor{}
	auditor := &collectingAuditPublisher{}
	adapter := cli.NewAdapterWithExecutor("tenant-1", executor)
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, errorEvaluator{}, auditor)

	resp, err := adapter.GovernCommand(context.Background(), cli.CommandInput{Command: "echo should-not-run"}, pipeline)
	if err != nil {
		t.Fatalf("GovernCommand: %v", err)
	}
	if executor.calls != 0 {
		t.Fatal("pipeline error allowed command execution")
	}
	if resp.ExitCode != 126 || resp.Metadata["cli_denied"] != "true" {
		t.Fatalf("expected fail-closed denied response, got %+v", resp)
	}
	if events := auditor.Events(); len(events) != 1 || events[0].Action != "deny" {
		t.Fatalf("expected denied audit event, got %+v", events)
	}
}

func TestCLIDirectShellBypassLimitation(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "direct-shell.txt")
	cmd := exec.Command("sh", "-c", "printf bypass > \"$1\"", "sh", marker)
	if err := cmd.Run(); err != nil {
		t.Fatalf("direct shell command failed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("direct shell command did not create marker: %v", err)
	}
}

type cliRecordingExecutor struct {
	calls int
}

func (e *cliRecordingExecutor) Execute(_ context.Context, _ *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	e.calls++
	return &governance.ToolResponse{
		Content:     []byte("ok\n"),
		ContentType: "text/plain",
		Metadata:    map[string]string{},
	}, nil
}
