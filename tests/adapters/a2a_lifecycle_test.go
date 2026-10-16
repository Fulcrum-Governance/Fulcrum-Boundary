package adapters_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/a2a"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestA2AGovernedLifecycleAllowedTaskForwardsWithMetadata(t *testing.T) {
	forwarder := &a2a.MemoryForwarder{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, &collectingAuditPublisher{})
	adapter := a2a.NewForwardingAdapter("tenant-1", forwarder)

	resp, err := adapter.GovernTask(context.Background(), taskEnvelope("task-allow", "summarize"), pipeline)
	if err != nil {
		t.Fatalf("GovernTask: %v", err)
	}
	if resp.Status != a2a.StatusAllowed {
		t.Fatalf("expected allowed response, got %+v", resp)
	}
	if len(forwarder.Snapshot()) != 1 {
		t.Fatal("allowed task was not forwarded exactly once")
	}
	if resp.Governance == nil || resp.Governance.Action != "allow" || resp.Governance.RequestID == "" {
		t.Fatalf("expected governance metadata, got %+v", resp.Governance)
	}
}

func TestA2AGovernedLifecycleMalformedRequestUnsupportedFailClosed(t *testing.T) {
	adapter := a2a.NewForwardingAdapter("tenant-1", &a2a.MemoryForwarder{})
	resp, err := adapter.GovernTask(context.Background(), []byte(`{not-json`), governance.NewPipeline(governance.PipelineConfig{}, nil, nil, nil))
	if err != nil {
		t.Fatalf("GovernTask: %v", err)
	}
	if resp.Status != a2a.StatusUnsupported {
		t.Fatalf("expected unsupported fail-closed response, got %+v", resp)
	}
	if resp.Governance == nil || resp.Governance.Action != "deny" {
		t.Fatalf("expected fail-closed governance metadata, got %+v", resp.Governance)
	}
}

func TestA2AGovernedLifecycleUnknownMandatoryFieldUnsupportedFailClosed(t *testing.T) {
	adapter := a2a.NewForwardingAdapter("tenant-1", &a2a.MemoryForwarder{})
	body := []byte(`{
		"task_id":"task-unknown",
		"sender_agent_id":"agent-1",
		"action":"send",
		"required_fields":["future_mandatory_field"]
	}`)
	resp, err := adapter.GovernTask(context.Background(), body, governance.NewPipeline(governance.PipelineConfig{}, nil, nil, nil))
	if err != nil {
		t.Fatalf("GovernTask: %v", err)
	}
	if resp.Status != a2a.StatusUnsupported || !strings.Contains(resp.Error.Message, "unsupported required field") {
		t.Fatalf("expected unsupported required field response, got %+v", resp)
	}
}

func taskEnvelope(id, action string) a2a.TaskEnvelope {
	return a2a.TaskEnvelope{
		TaskID:        id,
		SenderAgentID: "agent-1",
		Receiver:      "worker-1",
		Action:        action,
		Input:         map[string]any{"text": "hello"},
	}
}

type collectingAuditPublisher struct {
	mu     sync.Mutex
	events []governance.AuditEvent
}

func (p *collectingAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *collectingAuditPublisher) Events() []governance.AuditEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]governance.AuditEvent, len(p.events))
	copy(out, p.events)
	return out
}
