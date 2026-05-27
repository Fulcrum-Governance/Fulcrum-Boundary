package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fulcrum-governance/boundary/governance"
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
	if decision.Action != "deny" || decision.TrustScore != 0 {
		t.Fatalf("expected fail-closed trust denial, got %#v", decision)
	}
}
