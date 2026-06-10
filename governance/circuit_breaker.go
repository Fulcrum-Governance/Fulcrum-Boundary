package governance

// Default score thresholds used by the standalone trust evaluator: a score
// below DefaultTrustIsolationThreshold isolates the agent; a score below
// DefaultTrustDegradedThreshold marks it degraded (EVALUATING).
const (
	DefaultTrustIsolationThreshold = 0.3
	DefaultTrustDegradedThreshold  = 0.6
)

// TrustStateFromScore maps a trust score to a circuit-breaker state: below
// isolationThreshold → TrustStateIsolated, below degradedThreshold →
// TrustStateEvaluating, otherwise TrustStateTrusted. Callers supply the
// thresholds (the defaults above are typical).
func TrustStateFromScore(score, isolationThreshold, degradedThreshold float64) TrustState {
	if score < isolationThreshold {
		return TrustStateIsolated
	}
	if score < degradedThreshold {
		return TrustStateEvaluating
	}
	return TrustStateTrusted
}

// TrustScore returns the Beta-distribution mean alpha/(alpha+beta), the trust
// score in [0,1]. It returns 0 when alpha+beta is non-positive (undefined
// mean).
func TrustScore(alpha, beta float64) float64 {
	if alpha+beta <= 0 {
		return 0
	}
	return alpha / (alpha + beta)
}
