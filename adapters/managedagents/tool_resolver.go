package managedagents

import (
	"context"
	"fmt"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// ToolResolver evaluates always_ask tool events and emits the corresponding
// user.tool_confirmation upstream.
type ToolResolver struct {
	Adapter   *Adapter
	Pipeline  *governance.Pipeline
	Tracker   *ThreadTracker
	Forwarder ConfirmationForwarder
}

func (r *ToolResolver) Resolve(ctx context.Context, event Event) (ToolConfirmation, *governance.GovernanceDecision, error) {
	if r.Adapter == nil {
		return ToolConfirmation{}, nil, fmt.Errorf("managed agents adapter is required")
	}
	if r.Pipeline == nil {
		return ToolConfirmation{}, nil, fmt.Errorf("governance pipeline is required")
	}
	req, err := r.Adapter.ParseRequest(ctx, event)
	if err != nil {
		return ToolConfirmation{}, nil, err
	}
	decision, err := r.Pipeline.Evaluate(ctx, req)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: req.EnvelopeID,
		}
	}
	if decision.Allowed() && r.Tracker != nil {
		if err := r.Tracker.Reserve(event.SessionID, event.SessionThreadID, estimatedCost(event)); err != nil {
			decision.Action = "deny"
			decision.Reason = err.Error()
		}
	}
	confirmation := confirmationFromDecision(event, decision)
	if r.Forwarder != nil {
		if err := r.Forwarder.SendConfirmation(ctx, event.SessionID, confirmation); err != nil {
			return confirmation, decision, err
		}
	}
	return confirmation, decision, nil
}

func confirmationFromDecision(event Event, decision *governance.GovernanceDecision) ToolConfirmation {
	confirmation := ToolConfirmation{
		Type:            ConfirmationEventType,
		ToolUseID:       event.ID,
		Result:          ConfirmationAllow,
		SessionThreadID: event.SessionThreadID,
		ProcessedAt:     time.Now().UTC(),
		Governance:      metadataFromDecision(decision),
	}
	if decision == nil || !decision.Allowed() {
		confirmation.Result = ConfirmationDeny
		if decision != nil {
			confirmation.DenyMessage = decision.Reason
		}
		if confirmation.DenyMessage == "" {
			confirmation.DenyMessage = "denied by Boundary"
		}
	}
	return confirmation
}

func estimatedCost(event Event) float64 {
	if event.Usage != nil && event.Usage.CostUSD > 0 {
		return event.Usage.CostUSD
	}
	if cost := floatFromMap(event.Input, "estimated_cost_usd"); cost > 0 {
		return cost
	}
	if cost := floatFromMap(event.Data, "estimated_cost_usd"); cost > 0 {
		return cost
	}
	return 0
}
