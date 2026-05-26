package integration

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/boundary/adapters/managedagents"
	"github.com/fulcrum-governance/boundary/governance"
)

func TestManagedAgentsProxyGovernsToolEventsAndAutoResolves(t *testing.T) {
	auditor := &recordingAuditPublisher{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		GatewayVersion: "test",
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "block-prod-delete", Tool: "delete_production_issue", Action: "deny", Reason: "production destructive action"},
		},
	}, nil, nil, auditor)
	forwarder := &managedagents.MemoryForwarder{}
	tracker := managedagents.NewThreadTracker("sess-1", 1.00)
	adapter := managedagents.NewProxyAdapter("tenant-a", forwarder)
	resolver := &managedagents.ToolResolver{Adapter: adapter, Pipeline: pipeline, Tracker: tracker, Forwarder: forwarder}
	proxy := managedagents.NewSessionProxy(resolver, tracker)

	source := &managedagents.SliceSource{Events: []managedagents.Event{
		{ID: "evt-1", Type: managedagents.EventAgentToolUse, TenantID: "tenant-a", AgentID: "agent-a", SessionID: "sess-1", ToolName: "read_issue", Input: map[string]any{"estimated_cost_usd": 0.10}},
		{ID: "evt-2", Type: managedagents.EventAgentMCPToolUse, TenantID: "tenant-a", AgentID: "agent-a", SessionID: "sess-1", ToolName: "delete_production_issue"},
	}}
	sink := &managedagents.SliceSink{}
	if err := proxy.Proxy(context.Background(), source, sink); err != nil {
		t.Fatal(err)
	}

	confirmations := forwarder.Snapshot()
	if len(confirmations) != 2 {
		t.Fatalf("confirmations = %d, want 2", len(confirmations))
	}
	if confirmations[0].Result != managedagents.ConfirmationAllow {
		t.Fatalf("first confirmation = %#v, want allow", confirmations[0])
	}
	if confirmations[1].Result != managedagents.ConfirmationDeny || confirmations[1].DenyMessage == "" {
		t.Fatalf("second confirmation = %#v, want deny with message", confirmations[1])
	}
	if len(sink.Events) != 2 || sink.Events[0].Governance == nil || sink.Events[1].Governance == nil {
		t.Fatalf("proxied events missing governance metadata: %#v", sink.Events)
	}
	if auditor.Count() != 2 {
		t.Fatalf("audit records = %d, want one per tool event", auditor.Count())
	}
	if used := tracker.Snapshot()["sess-1"].BudgetUsed; used != 0.10 {
		t.Fatalf("budget used = %.2f, want 0.10", used)
	}
}

func TestManagedAgentsAdapterParsesAndFailsClosed(t *testing.T) {
	adapter := managedagents.NewAdapter("tenant-a")
	req, err := adapter.ParseRequest(context.Background(), managedagents.Event{
		ID:        "evt-1",
		Type:      managedagents.EventAgentToolUse,
		AgentID:   "agent-a",
		SessionID: "sess-1",
		ToolName:  "danger",
	})
	if err != nil {
		t.Fatal(err)
	}
	if req.Transport != governance.TransportManagedAgents || req.TraceID != "sess-1" || req.ToolName != "danger" {
		t.Fatalf("unexpected governance request: %#v", req)
	}

	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, failingEvaluator{}, nil)
	decision, err := pipeline.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "deny" {
		t.Fatalf("managed agents should fail closed, got %#v", decision)
	}
}

func TestManagedAgentsBypassConfig(t *testing.T) {
	if err := managedagents.VerifyBypassConfig(managedagents.BypassConfig{BoundaryOwnsAPIKey: true}); err != nil {
		t.Fatalf("expected bypass config to pass: %v", err)
	}
	if err := managedagents.VerifyBypassConfig(managedagents.BypassConfig{CustomerCanSendConfirmations: true}); err == nil {
		t.Fatal("expected bypass config to fail when customer can confirm directly")
	}
}

type recordingAuditPublisher struct {
	events []governance.AuditEvent
}

func (p *recordingAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.events = append(p.events, event)
}

func (p *recordingAuditPublisher) Count() int {
	return len(p.events)
}
