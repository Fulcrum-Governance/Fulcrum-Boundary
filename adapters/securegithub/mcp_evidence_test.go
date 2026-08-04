package securegithub

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestMCPJSONLEvidenceSinkUsesDistinctPrivateFiles(t *testing.T) {
	dir := t.TempDir()
	decisions := filepath.Join(dir, "evidence", "decisions.jsonl")
	forward := filepath.Join(dir, "evidence", "forward.jsonl")
	if err := os.MkdirAll(filepath.Dir(decisions), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decisions, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(decisions, 0o644); err != nil {
		t.Fatal(err)
	}
	sink, err := NewMCPJSONLEvidenceSink(decisions, forward)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(decisions); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("existing decision evidence was not secured at readiness: info=%v err=%v", info, err)
	}
	record := governance.BuildDecisionRecord(governance.AuditEvent{Transport: governance.TransportMCP, Action: "deny"})
	if err := sink.WriteDecision(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	if err := sink.WriteForwardEvent(context.Background(), MCPForwardEvent{Outcome: "blocked_before_forward"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{decisions, forward} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %o, want 600", path, info.Mode().Perm())
		}
	}
	if _, err := NewMCPJSONLEvidenceSink(decisions, decisions); err == nil {
		t.Fatal("same decision and forward path was accepted")
	}
	symlink := filepath.Join(dir, "evidence", "symlink.jsonl")
	if err := os.Symlink(decisions, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := NewMCPJSONLEvidenceSink(symlink, filepath.Join(dir, "other.jsonl")); err == nil {
		t.Fatal("symlink evidence path was accepted")
	}
}
