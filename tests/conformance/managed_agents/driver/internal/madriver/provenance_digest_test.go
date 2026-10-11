package madriver

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
	maconf "github.com/fulcrum-governance/fulcrum-boundary/tests/conformance/managed_agents"
)

// TestDriverLiveEvidenceSetPassesProvenanceCheck runs a live-mode driver
// session against the fake upstream and validates the emitted evidence set
// with the conformance package's CheckLiveProvenance — including the
// transcript digest recomputation, which only holds when every field the
// driver writes (e.g. confirmation delivered flags) round-trips through the
// shared Transcript schema.
func TestDriverLiveEvidenceSetPassesProvenanceCheck(t *testing.T) {
	up := &fakeUpstream{events: []managedagents.Event{
		{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
		{ID: "tu1", Type: managedagents.EventAgentToolUse, AgentID: "agt_fake", SessionID: "sess_fake_0001",
			ToolName: "safe_tool", Usage: &managedagents.Usage{CostUSD: 0.01},
			Data: map[string]any{"evaluated_permission": "ask"}},
		{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001",
			Data: map[string]any{"usage": map[string]any{"list_cost": "5"}}},
		{ID: "e3", Type: managedagents.EventStatusIdle, SessionID: "sess_fake_0001"},
	}}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("live fake run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := maconf.ParseTranscript(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := maconf.CheckLiveProvenance(tr, res.TranscriptPath); err != nil {
		t.Fatalf("driver-produced live evidence set must pass CheckLiveProvenance: %v", err)
	}
}
