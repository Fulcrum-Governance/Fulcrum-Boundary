package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type failingRedisStore struct{}

func (failingRedisStore) Get(context.Context, string) (string, error) {
	return "", fmt.Errorf("redis down")
}

func (failingRedisStore) Set(context.Context, string, string, time.Duration) error {
	return fmt.Errorf("redis down")
}

func (failingRedisStore) Del(context.Context, string) error {
	return fmt.Errorf("redis down")
}

func TestKernelTrustTimeoutFailsClosed(t *testing.T) {
	trust := governance.NewRedisTrustBackend(failingRedisStore{}, governance.KernelTrustConfig{FailClosed: true})
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, trust, nil, nil)
	decision, err := pipeline.Evaluate(context.Background(), &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		AgentID:   "agent-1",
		ToolName:  "query",
	})
	if err != nil {
		t.Fatal(err)
	}
	// ADR-047: a trust lookup that cannot produce a valid result is
	// CHECK_INDETERMINATE — it blocks execution but is neither allow nor a
	// substantive policy denial.
	if decision.Action != governance.ActionCheckIndeterminate || decision.TrustScore != 0 {
		t.Fatalf("expected fail-closed trust check_indeterminate, got %#v", decision)
	}
	if decision.Allowed() {
		t.Fatal("check_indeterminate must not allow execution")
	}
	if decision.Check == nil || decision.Check.Category != governance.FailureUnavailable {
		t.Fatalf("expected trust lookup failure category unavailable, got %#v", decision.Check)
	}
}
