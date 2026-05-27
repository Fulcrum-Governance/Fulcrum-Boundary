package governance

const (
	DefaultTrustIsolationThreshold = 0.3
	DefaultTrustDegradedThreshold  = 0.6
)

func TrustStateFromScore(score, isolationThreshold, degradedThreshold float64) TrustState {
	if score < isolationThreshold {
		return TrustStateIsolated
	}
	if score < degradedThreshold {
		return TrustStateEvaluating
	}
	return TrustStateTrusted
}

func TrustScore(alpha, beta float64) float64 {
	if alpha+beta <= 0 {
		return 0
	}
	return alpha / (alpha + beta)
}
