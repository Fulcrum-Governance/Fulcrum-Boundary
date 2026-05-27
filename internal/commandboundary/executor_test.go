package commandboundary

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls []ExecutionRequest
	out   ExecutionResult
}

func (r *fakeRunner) Run(_ context.Context, req ExecutionRequest) (ExecutionResult, error) {
	r.calls = append(r.calls, req)
	return r.out, nil
}

func TestExecutorDeniedCommandDoesNotExecute(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	runner := &fakeRunner{}
	executor := Executor{Runner: runner, RecordPath: recordPath}

	result, err := executor.Run(context.Background(), RunRequest{
		Argv: []string{"rm", "-rf", "dist"},
		CWD:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Executed {
		t.Fatal("denied command executed")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
	if result.Decision.Action != "deny" {
		t.Fatalf("action = %q, want deny", result.Decision.Action)
	}
	if result.ExitCode != 126 {
		t.Fatalf("exit = %d, want 126", result.ExitCode)
	}
	record := readSingleCommandRecord(t, recordPath)
	if record.Executed {
		t.Fatalf("record executed = true")
	}
	if record.Class != ClassDestructiveMutation {
		t.Fatalf("record class = %s, want %s", record.Class, ClassDestructiveMutation)
	}
}

func TestExecutorAllowedCommandExecutesOnce(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	cwd := t.TempDir()
	runner := &fakeRunner{out: ExecutionResult{Stdout: []byte("ok\n"), ExitCode: 0}}
	executor := Executor{Runner: runner, RecordPath: recordPath}

	result, err := executor.Run(context.Background(), RunRequest{
		Argv: []string{"pwd"},
		CWD:  cwd,
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !result.Executed {
		t.Fatal("allowed command did not execute")
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.Command != "pwd" || len(call.Args) != 0 || call.CWD != cwd {
		t.Fatalf("runner call = %#v", call)
	}
	if result.Decision.Action != "allow" {
		t.Fatalf("action = %q, want allow", result.Decision.Action)
	}
	record := readSingleCommandRecord(t, recordPath)
	if !record.Executed || record.ExitCode != 0 {
		t.Fatalf("record execution fields = %#v", record)
	}
}

func TestExecutorRequireApprovalDoesNotExecute(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	runner := &fakeRunner{}
	executor := Executor{Runner: runner, RecordPath: recordPath}

	result, err := executor.Run(context.Background(), RunRequest{
		Argv: []string{"git", "push", "origin", "main"},
		CWD:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if result.Executed {
		t.Fatal("require_approval command executed")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner calls = %d, want 0", len(runner.calls))
	}
	if result.Decision.Action != "require_approval" {
		t.Fatalf("action = %q, want require_approval", result.Decision.Action)
	}
}

func TestExecutorRecordRedactsSecretArgs(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	executor := Executor{Runner: &fakeRunner{}, RecordPath: recordPath}

	_, err := executor.Run(context.Background(), RunRequest{
		Argv: []string{"cat", ".env"},
		CWD:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	body, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), ".env") {
		t.Fatalf("record leaked secret-looking arg: %s", string(body))
	}
	if !strings.Contains(string(body), "sha256:") {
		t.Fatalf("record missing argv hash: %s", string(body))
	}
}

func TestExecutorPassesShellMetacharactersAsLiteralArgs(t *testing.T) {
	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	runner := &fakeRunner{out: ExecutionResult{ExitCode: 0}}
	executor := Executor{Runner: runner, RecordPath: recordPath}

	_, err := executor.Run(context.Background(), RunRequest{
		Argv: []string{"pwd", ";", "rm", "-rf", "dist"},
		CWD:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("runner calls = %d, want 1", len(runner.calls))
	}
	got := strings.Join(runner.calls[0].Args, " ")
	if got != "; rm -rf dist" {
		t.Fatalf("literal args = %q", got)
	}
}

func readSingleCommandRecord(t *testing.T, path string) CommandDecisionRecord {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 {
		t.Fatalf("record lines = %d, want 1:\n%s", len(lines), string(body))
	}
	var record CommandDecisionRecord
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("decode record: %v\n%s", err, lines[0])
	}
	return record
}
