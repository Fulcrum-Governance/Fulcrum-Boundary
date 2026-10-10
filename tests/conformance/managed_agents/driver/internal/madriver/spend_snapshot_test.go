package madriver

import (
	"errors"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// TestSpendGuardCountsPricedSpendAfterSnapshot proves the spend estimate never
// under-reports once a list_cost snapshot has arrived: priced model requests
// and per-event usage costs observed after the snapshot must still count
// against the ceiling, not wait for the next session.usage.
func TestSpendGuardCountsPricedSpendAfterSnapshot(t *testing.T) {
	// A $0.50 snapshot followed by $0.30 of priced model spend on a $1.00
	// ceiling must abort at the 80-percent guard line.
	g := newSpendGuard(1.00, 1024, 8, time.Minute)
	err := g.observe(managedagents.Event{
		Type: "session.usage",
		Data: map[string]any{"usage": map[string]any{"list_cost": "50"}},
	})
	if err != nil {
		t.Fatalf("list_cost snapshot: %v", err)
	}
	err = g.observe(managedagents.Event{
		Type: "span.model_request_end",
		Data: map[string]any{"model_usage": map[string]any{
			"model": "claude-sonnet-4-5-20250929", "input_tokens": 95_000, "output_tokens": 1_000,
		}},
	})
	if !errors.Is(err, ErrSpendAbort) {
		t.Fatalf("post-snapshot spend of $0.30 on a $0.50 snapshot must abort at 80%% of $1.00, got err=%v used=%v", err, g.used())
	}

	// The same holds for the per-event Usage.CostUSD signal: a $0.40 event
	// cost after a $0.50 snapshot pushes the estimate to $0.90.
	g = newSpendGuard(1.00, 1024, 8, time.Minute)
	if err := g.observe(managedagents.Event{
		Type: "session.usage",
		Data: map[string]any{"usage": map[string]any{"list_cost": "50"}},
	}); err != nil {
		t.Fatalf("list_cost snapshot: %v", err)
	}
	err = g.observe(managedagents.Event{
		Type:  managedagents.EventAgentToolUse,
		Usage: &managedagents.Usage{CostUSD: 0.40},
	})
	if !errors.Is(err, ErrSpendAbort) {
		t.Fatalf("post-snapshot event cost of $0.40 on a $0.50 snapshot must abort, got err=%v used=%v", err, g.used())
	}
}

// TestSpendGuardPostSnapshotBaselineResets covers the re-baseline: a new
// list_cost snapshot that sets a higher maximum covers the priced spend seen
// so far, so the post-snapshot accrual restarts from zero rather than
// double-counting it.
func TestSpendGuardPostSnapshotBaselineResets(t *testing.T) {
	g := newSpendGuard(10.00, 1024, 8, time.Minute)
	observe := func(event managedagents.Event) {
		if err := g.observe(event); err != nil {
			t.Fatalf("observe %q: %v", event.Type, err)
		}
	}
	observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "50"}}})
	observe(managedagents.Event{
		Type: "span.model_request_end",
		Data: map[string]any{"model_usage": map[string]any{
			"model": "claude-sonnet-4-5-20250929", "input_tokens": 95_000, "output_tokens": 1_000,
		}},
	})
	if got, want := g.used(), 0.80; got != want {
		t.Fatalf("snapshot $0.50 + post-snapshot $0.30 = %v, want %v", got, want)
	}
	// A new snapshot of $0.90 already includes that $0.30; the estimate must
	// become $0.90, not $0.50 + $0.30 + $0.90.
	observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "90"}}})
	if got, want := g.used(), 0.90; got != want {
		t.Fatalf("re-baselined estimate = %v, want %v", got, want)
	}
}
