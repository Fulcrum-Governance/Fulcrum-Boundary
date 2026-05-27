package governance

import (
	"context"
	"time"
)

type TrustSnapshot struct {
	AgentID          string     `json:"agent_id"`
	State            TrustState `json:"state"`
	Score            float64    `json:"score"`
	Alpha            float64    `json:"alpha,omitempty"`
	Beta             float64    `json:"beta,omitempty"`
	InteractionCount int        `json:"interaction_count,omitempty"`
	Known            bool       `json:"known"`
	LastUpdated      time.Time  `json:"last_updated,omitempty"`
}

type TrustDecisionUpdate struct {
	Before     TrustSnapshot `json:"before"`
	After      TrustSnapshot `json:"after"`
	Outcome    TrustOutcome  `json:"outcome"`
	Transition bool          `json:"transition"`
}

type TrustBackend interface {
	TrustChecker
	GetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error)
	RecordDecision(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) (TrustDecisionUpdate, error)
	ResetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error)
	TerminateAgent(ctx context.Context, agentID string) (TrustSnapshot, error)
}
