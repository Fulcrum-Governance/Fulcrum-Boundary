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
	sink, err := NewMCPJSONLEvidenceSink(decisions, forward)
	if err != nil {
		t.Fatal(err)
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
}
