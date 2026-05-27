package governance

type TrustOutcome string

const (
	TrustOutcomeSuccess TrustOutcome = "success"
	TrustOutcomeFailure TrustOutcome = "failure"
	TrustOutcomePartial TrustOutcome = "partial"
)

func TrustOutcomeFromDecision(decision *GovernanceDecision) TrustOutcome {
	if decision == nil {
		return TrustOutcomePartial
	}
	switch decision.Action {
	case "allow":
		return TrustOutcomeSuccess
	case "deny":
		return TrustOutcomeFailure
	case "warn", "escalate", "require_approval":
		return TrustOutcomePartial
	default:
		return TrustOutcomePartial
	}
}
