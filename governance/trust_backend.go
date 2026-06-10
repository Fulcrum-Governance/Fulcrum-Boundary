package governance

import (
	"context"
	"time"
)

// TrustSnapshot is a point-in-time view of an agent's trust state as held by a
// TrustBackend. Alpha, Beta, and InteractionCount are populated by the Beta
// evaluator (StandaloneTrustBackend) and may be zero for backends that do not
// model them (e.g. the Redis backend, which stores only state and score).
type TrustSnapshot struct {
	// AgentID is the agent this snapshot describes.
	AgentID string `json:"agent_id"`
	// State is the circuit-breaker state derived from the score.
	State TrustState `json:"state"`
	// Score is the trust score in [0,1]; 1 for an unknown (never-seen) agent.
	Score float64 `json:"score"`
	// Alpha is the Beta-distribution success parameter (Beta evaluator only).
	Alpha float64 `json:"alpha,omitempty"`
	// Beta is the Beta-distribution failure parameter (Beta evaluator only).
	Beta float64 `json:"beta,omitempty"`
	// InteractionCount is the number of recorded decisions for the agent.
	InteractionCount int `json:"interaction_count,omitempty"`
	// Known is false when the backend has no stored record for the agent and
	// returned the default trusted snapshot.
	Known bool `json:"known"`
	// LastUpdated is when the agent's record was last written; zero if unknown.
	LastUpdated time.Time `json:"last_updated,omitempty"`
}

// TrustDecisionUpdate is the result of recording one decision against an
// agent's trust state. It captures the before/after snapshots so callers can
// detect and audit a circuit-breaker state change.
type TrustDecisionUpdate struct {
	// Before is the agent's snapshot prior to applying the decision.
	Before TrustSnapshot `json:"before"`
	// After is the agent's snapshot after applying the decision.
	After TrustSnapshot `json:"after"`
	// Outcome is how the decision was scored (success / failure / partial).
	Outcome TrustOutcome `json:"outcome"`
	// Transition is true when Before.State and After.State differ.
	Transition bool `json:"transition"`
}

// TrustBackend is the read/write trust store the pipeline uses when the
// configured TrustChecker also tracks per-decision trust (the pipeline
// type-asserts its checker to TrustBackend to record outcomes). It extends
// TrustChecker with snapshot reads, decision recording, and operator
// reset/terminate controls. StandaloneTrustBackend and RedisTrustBackend both
// implement it.
//
// The TrustChecker contract carries over: a returned error is fail-closed, and
// an unknown agent is reported as trusted (not an error).
type TrustBackend interface {
	TrustChecker
	// GetAgentTrust returns the current snapshot for the agent. An unknown
	// agent yields a trusted snapshot with Known == false (not an error).
	GetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error)
	// RecordDecision folds one governance decision into the agent's trust
	// state and returns the before/after update.
	RecordDecision(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) (TrustDecisionUpdate, error)
	// ResetAgentTrust clears any stored state, returning the agent to the
	// default trusted snapshot.
	ResetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error)
	// TerminateAgent forces the agent into the TERMINATED state, blocking all
	// further execution until reset.
	TerminateAgent(ctx context.Context, agentID string) (TrustSnapshot, error)
}
