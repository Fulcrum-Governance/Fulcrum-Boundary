package commandboundary_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func TestCommandRunDeniedDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "run", "--record-out", recordPath, "--", "rm", "-f", sentinel}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit = %d, want 126 stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("denied command removed sentinel: %v", err)
	}
	if !strings.Contains(stderr.String(), "action=deny executed=false") {
		t.Fatalf("stderr missing deny summary: %s", stderr.String())
	}
	record := readCommandRunRecord(t, recordPath)
	if record.Executed || record.Action != "deny" {
		t.Fatalf("record = %#v", record)
	}
}

func TestCommandRunAllowedExecutesAndRecords(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "run", "--record-out", recordPath, "--", "pwd"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), dir) {
		t.Fatalf("pwd output = %q, want cwd %q", stdout.String(), dir)
	}
	if !strings.Contains(stderr.String(), "action=allow executed=true") {
		t.Fatalf("stderr missing allow summary: %s", stderr.String())
	}
	record := readCommandRunRecord(t, recordPath)
	if !record.Executed || record.Action != "allow" || record.ExitCode != 0 {
		t.Fatalf("record = %#v", record)
	}
}

func TestCommandRunRequireApprovalDoesNotExecute(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "run", "--record-out", recordPath, "--", "git", "push", "origin", "main"}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "action=require_approval executed=false") {
		t.Fatalf("stderr missing require_approval summary: %s", stderr.String())
	}
	record := readCommandRunRecord(t, recordPath)
	if record.Executed || record.Action != "require_approval" {
		t.Fatalf("record = %#v", record)
	}
}

func TestCommandRunDoesNotInvokeShellInterpolation(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	_ = boundarycli.Run([]string{"command", "run", "--record-out", recordPath, "--", "pwd", ";", "rm", "-f", sentinel}, &stdout, &stderr)
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("shell metacharacter was interpreted and removed sentinel: %v", err)
	}
}

func readCommandRunRecord(t *testing.T, path string) commandboundary.CommandDecisionRecord {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1 {
		t.Fatalf("record lines = %d, want 1:\n%s", len(lines), string(body))
	}
	var record commandboundary.CommandDecisionRecord
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("decode record: %v\n%s", err, lines[0])
	}
	return record
}
