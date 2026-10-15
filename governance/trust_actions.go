package governance

type TrustOutcome string

const (
	TrustOutcomeSuccess TrustOutcome = "success"
	TrustOutcomeFailure TrustOutcome = "failure"
	TrustOutcomePartial TrustOutcome = "partial"
	// TrustOutcomeNeutral leaves the agent's trust unchanged. A warn (policy
	// warn or audit) is an allowed call the operator asked to flag; flagging it
	// must not degrade the agent and block its later calls.
	TrustOutcomeNeutral TrustOutcome = "neutral"
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
	case "warn":
		return TrustOutcomeNeutral
	default:
		return TrustOutcomePartial
	}
}
