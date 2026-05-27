package integration

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestManagedAgentsTracksThreadBudgetsAndTrust(t *testing.T) {
	pipeline := governance.NewPipeline(governance.PipelineConfig{GatewayVersion: "test"}, nil, nil, nil)
	forwarder := &managedagents.MemoryForwarder{}
	tracker := managedagents.NewThreadTracker("sess-1", 0.60)
	adapter := managedagents.NewProxyAdapter("tenant-a", forwarder)
	resolver := &managedagents.ToolResolver{Adapter: adapter, Pipeline: pipeline, Tracker: tracker, Forwarder: forwarder}
	proxy := managedagents.NewSessionProxy(resolver, tracker)

	source := &managedagents.SliceSource{Events: []managedagents.Event{
		{ID: "thread-1", Type: managedagents.EventThreadCreated, SessionID: "sess-1", SessionThreadID: "worker-a", ParentThreadID: "sess-1", Data: map[string]any{"budget_limit": 0.25}},
		{ID: "evt-1", Type: managedagents.EventAgentToolUse, TenantID: "tenant-a", AgentID: "agent-a", SessionID: "sess-1", SessionThreadID: "worker-a", ToolName: "search", Input: map[string]any{"estimated_cost_usd": 0.10}},
		{ID: "evt-2", Type: managedagents.EventAgentToolUse, TenantID: "tenant-a", AgentID: "agent-a", SessionID: "sess-1", SessionThreadID: "worker-a", ToolName: "expensive_search", Input: map[string]any{"estimated_cost_usd": 0.20}},
	}}
	if err := proxy.Proxy(context.Background(), source, &managedagents.SliceSink{}); err != nil {
		t.Fatal(err)
	}

	confirmations := forwarder.Snapshot()
	if len(confirmations) != 2 {
		t.Fatalf("confirmations = %d, want 2", len(confirmations))
	}
	if confirmations[0].Result != managedagents.ConfirmationAllow {
		t.Fatalf("first confirmation = %#v, want allow", confirmations[0])
	}
	if confirmations[1].Result != managedagents.ConfirmationDeny {
		t.Fatalf("second confirmation = %#v, want budget deny", confirmations[1])
	}
	state := tracker.Snapshot()["worker-a"]
	if state.ParentID != "sess-1" || state.BudgetLimit != 0.25 || state.BudgetUsed != 0.10 {
		t.Fatalf("unexpected thread state: %#v", state)
	}

	tracker.SetTrust("worker-a", governance.TrustStateIsolated)
	source = &managedagents.SliceSource{Events: []managedagents.Event{
		{ID: "evt-3", Type: managedagents.EventAgentToolUse, TenantID: "tenant-a", AgentID: "agent-a", SessionID: "sess-1", SessionThreadID: "worker-a", ToolName: "search", Input: map[string]any{"estimated_cost_usd": 0.01}},
	}}
	if err := proxy.Proxy(context.Background(), source, &managedagents.SliceSink{}); err != nil {
		t.Fatal(err)
	}
	confirmations = forwarder.Snapshot()
	if confirmations[2].Result != managedagents.ConfirmationDeny || confirmations[2].DenyMessage == "" {
		t.Fatalf("isolated thread should deny confirmation: %#v", confirmations[2])
	}
}
