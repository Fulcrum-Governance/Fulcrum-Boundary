package madriver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// TestDenyNotHeldIsFailedEnforcement proves that when Boundary resolves a deny
// for a tool call the upstream never paused for (evaluated_permission
// "allow"), the run cannot report the deny as enforced: the run errors, the
// paid session is interrupted, and the transcript records the confirmation
// as delivered=false/enforced=false rather than a successful deny.
func TestDenyNotHeldIsFailedEnforcement(t *testing.T) {
	up := &fakeUpstream{
		confirmErr: ErrConfirmationNotAsked,
		events: []managedagents.Event{
			{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
			{ID: "tu1", Type: managedagents.EventAgentToolUse, AgentID: "agt_fake", SessionID: "sess_fake_0001",
				ToolName: defaultDenyTool, Usage: &managedagents.Usage{CostUSD: 0.01},
				Data: map[string]any{"evaluated_permission": "allow"}},
			{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001",
				Data: map[string]any{"usage": map[string]any{"list_cost": "5"}}},
			{ID: "e3", Type: managedagents.EventStatusIdle, SessionID: "sess_fake_0001"},
		},
	}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if !errors.Is(err, ErrConfirmationNotEnforced) {
		t.Fatalf("a deny the upstream never held must fail the run as not-enforced, got err=%v stop=%s", err, res.StopReason)
	}
	if up.StopCalls() == 0 {
		t.Fatal("a deny that could not be enforced must interrupt the paid session")
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if len(tr.Confirmations) == 0 {
		t.Fatal("the resolved deny must still be recorded in the transcript")
	}
	for _, c := range tr.Confirmations {
		if c.Result != managedagents.ConfirmationDeny {
			continue
		}
		if c.Delivered == nil || *c.Delivered {
			t.Fatalf("not-held deny must record delivered=false: %+v", c)
		}
		if c.Enforced == nil || *c.Enforced {
			t.Fatalf("not-held deny must record enforced=false: %+v", c)
		}
	}
}

// TestAllowNotHeldStaysBenign proves the not-asked path still succeeds for an
// allow: nothing had to be enforced — the upstream already ran the call —
// so the run completes and the transcript records delivered=false.
func TestAllowNotHeldStaysBenign(t *testing.T) {
	up := &fakeUpstream{
		confirmErr: ErrConfirmationNotAsked,
		events: []managedagents.Event{
			{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
			{ID: "tu1", Type: managedagents.EventAgentToolUse, AgentID: "agt_fake", SessionID: "sess_fake_0001",
				ToolName: "safe_tool", Usage: &managedagents.Usage{CostUSD: 0.01},
				Data: map[string]any{"evaluated_permission": "allow"}},
			{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001",
				Data: map[string]any{"usage": map[string]any{"list_cost": "5"}}},
			{ID: "e3", Type: managedagents.EventStatusIdle, SessionID: "sess_fake_0001"},
		},
	}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("an allow the upstream never held is not a failed enforcement: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	for _, c := range tr.Confirmations {
		if c.Delivered == nil || *c.Delivered {
			t.Fatalf("not-held confirmation must record delivered=false: %+v", c)
		}
	}
}
