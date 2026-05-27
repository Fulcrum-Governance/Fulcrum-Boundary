package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// GovernanceMetadata is attached to MCP responses under result._meta.governance
// when the response shape permits it.
type GovernanceMetadata struct {
	Action         string `json:"action"`
	Reason         string `json:"reason,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
	EnvelopeID     string `json:"envelope_id,omitempty"`
	MatchedRule    string `json:"matched_rule,omitempty"`
	PolicyFile     string `json:"policy_file,omitempty"`
	GatewayVersion string `json:"gateway_version,omitempty"`
}

func metadataFromDecision(decision *governance.GovernanceDecision) GovernanceMetadata {
	if decision == nil {
		return GovernanceMetadata{}
	}
	return GovernanceMetadata{
		Action:         decision.Action,
		Reason:         decision.Reason,
		RequestID:      decision.RequestID,
		EnvelopeID:     decision.EnvelopeID,
		MatchedRule:    decision.MatchedRule,
		PolicyFile:     decision.PolicyFile,
		GatewayVersion: decision.GatewayVersion,
	}
}

func writeHTTPGovernanceHeaders(w http.ResponseWriter, decision *governance.GovernanceDecision) {
	if decision == nil {
		return
	}
	w.Header().Set(governance.HeaderGovernanceAction, decision.Action)
	if decision.Reason != "" {
		w.Header().Set(governance.HeaderGovernanceReason, decision.Reason)
	}
	if decision.EnvelopeID != "" {
		w.Header().Set(governance.HeaderGovernanceEnvelopeID, decision.EnvelopeID)
	}
	if decision.RequestID != "" {
		w.Header().Set(governance.HeaderGovernanceRequestID, decision.RequestID)
	}
	if decision.MatchedRule != "" {
		w.Header().Set(governance.HeaderGovernanceMatchedRule, decision.MatchedRule)
	}
}

func attachGovernanceMetadata(body []byte, decision *governance.GovernanceDecision) []byte {
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		return body
	}
	meta, _ := result["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		result["_meta"] = meta
	}
	meta["governance"] = metadataFromDecision(decision)
	encoded, err := json.Marshal(resp)
	if err != nil {
		return body
	}
	return encoded
}
