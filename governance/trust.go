package governance

import "context"

// TrustState represents the circuit breaker state of an agent.
type TrustState int

const (
	TrustStateTrusted    TrustState = 0
	TrustStateEvaluating TrustState = 1
	TrustStateIsolated   TrustState = 2
	TrustStateTerminated TrustState = 3
)

// String returns the human-readable trust state name.
func (s TrustState) String() string {
	switch s {
	case TrustStateTrusted:
		return "TRUSTED"
	case TrustStateEvaluating:
		return "EVALUATING"
	case TrustStateIsolated:
		return "ISOLATED"
	case TrustStateTerminated:
		return "TERMINATED"
	default:
		return "UNKNOWN"
	}
}

// Blocked returns true if the state prevents tool execution.
func (s TrustState) Blocked() bool {
	return s == TrustStateIsolated || s == TrustStateTerminated
}

// TrustChecker looks up the trust/circuit-breaker state for an agent. It is
// Stage 1 of the pipeline (governance/pipeline.go): a nil checker or an empty
// AgentID skips the stage. The two in-repo implementations both live in this
// package: StandaloneTrustBackend (trust_beta.go), the in-process Beta(alpha,
// beta) evaluator used in standalone mode, and RedisTrustBackend
// (trust_redis.go), which reads/writes the fulcrum-trust Redis IPC state in
// kernel mode.
//
// Contract for the implementer: a returned error is treated as fail-closed —
// the pipeline denies the request. An absent trust record is NOT an error;
// return TrustStateTrusted (score 1) for an agent the backend has never seen.
type TrustChecker interface {
	CheckAgentState(ctx context.Context, agentID string) (TrustState, error)
}
