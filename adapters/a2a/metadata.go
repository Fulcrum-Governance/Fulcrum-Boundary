package a2a

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

func MetadataFromDecision(decision *governance.GovernanceDecision) *GovernanceMetadata {
	if decision == nil {
		return nil
	}
	return &GovernanceMetadata{
		Action:       decision.Action,
		Reason:       decision.Reason,
		RequestID:    decision.RequestID,
		EnvelopeID:   decision.EnvelopeID,
		MatchedRule:  decision.MatchedRule,
		DecisionMode: string(decision.DecisionMode),
		TrustScore:   decision.TrustScore,
	}
}

func AttachGovernanceMetadata(response *TaskResponse, decision *governance.GovernanceDecision) {
	if response == nil || decision == nil {
		return
	}
	response.Governance = MetadataFromDecision(decision)
}
