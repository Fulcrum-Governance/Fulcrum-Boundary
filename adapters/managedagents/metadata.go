package managedagents

import "github.com/fulcrum-governance/boundary/governance"

// Metadata is attached to proxied session events and upstream confirmations.
type Metadata struct {
	Action         string  `json:"action"`
	Reason         string  `json:"reason,omitempty"`
	RequestID      string  `json:"request_id,omitempty"`
	EnvelopeID     string  `json:"envelope_id,omitempty"`
	MatchedRule    string  `json:"matched_rule,omitempty"`
	PolicyFile     string  `json:"policy_file,omitempty"`
	GatewayVersion string  `json:"gateway_version,omitempty"`
	TrustScore     float64 `json:"trust_score,omitempty"`
}

func metadataFromDecision(decision *governance.GovernanceDecision) *Metadata {
	if decision == nil {
		return nil
	}
	return &Metadata{
		Action:         decision.Action,
		Reason:         decision.Reason,
		RequestID:      decision.RequestID,
		EnvelopeID:     decision.EnvelopeID,
		MatchedRule:    decision.MatchedRule,
		PolicyFile:     decision.PolicyFile,
		GatewayVersion: decision.GatewayVersion,
		TrustScore:     decision.TrustScore,
	}
}
