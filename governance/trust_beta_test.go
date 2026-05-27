package governance

import (
	"context"
	"math"
	"testing"
)

func TestStandaloneTrustBackendMatchesFulcrumTrustBetaMath(t *testing.T) {
	backend := NewStandaloneTrustBackend(StandaloneTrustConfig{})
	req := &GovernanceRequest{AgentID: "agent-1", Transport: TransportMCP}
	allow := &GovernanceDecision{Action: "allow"}
	deny := &GovernanceDecision{Action: "deny"}

	update, err := backend.RecordDecision(context.Background(), req, allow)
	if err != nil {
		t.Fatal(err)
	}
	if update.After.Alpha != 2 || update.After.Beta != 1 || math.Abs(update.After.Score-0.666666) > 0.001 {
		t.Fatalf("success update not equivalent to fulcrum-trust: %#v", update.After)
	}

	update, err = backend.RecordDecision(context.Background(), req, deny)
	if err != nil {
		t.Fatal(err)
	}
	if update.After.Alpha != 2 || update.After.Beta != 2 || update.After.Score != 0.5 {
		t.Fatalf("failure update not equivalent to fulcrum-trust: %#v", update.After)
	}
}

func TestTrustStateFromScore(t *testing.T) {
	for _, tc := range []struct {
		score float64
		want  TrustState
	}{
		{0.85, TrustStateTrusted},
		{0.45, TrustStateEvaluating},
		{0.29, TrustStateIsolated},
	} {
		if got := TrustStateFromScore(tc.score, 0.3, 0.6); got != tc.want {
			t.Fatalf("score %.2f -> %s, want %s", tc.score, got, tc.want)
		}
	}
}
