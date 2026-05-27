package managedagents

import (
	"context"
	"fmt"

	"github.com/fulcrum-governance/boundary/governance"
)

// Adapter implements governance.TransportAdapter for Managed Agents events.
type Adapter struct {
	defaultTenantID string
	forwarder       ConfirmationForwarder
}

var _ governance.TransportAdapter = (*Adapter)(nil)

func NewAdapter(defaultTenantID string) *Adapter {
	return &Adapter{defaultTenantID: defaultTenantID}
}

func NewProxyAdapter(defaultTenantID string, forwarder ConfirmationForwarder) *Adapter {
	return &Adapter{defaultTenantID: defaultTenantID, forwarder: forwarder}
}

func (a *Adapter) Type() governance.TransportType {
	return governance.TransportManagedAgents
}

// ForwardGoverned sends an allow or deny confirmation for a governed tool-use
// event. The raw request must include the event fields required to identify the
// upstream session and tool-use ID.
func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if a.forwarder == nil {
		return nil, fmt.Errorf("managed agents forwarding requires a configured confirmation forwarder")
	}
	if req == nil || req.Arguments == nil {
		return nil, fmt.Errorf("managed agents request arguments are required")
	}
	event := Event{
		ID:              stringArg(req.Arguments, "tool_use_id"),
		SessionID:       stringArg(req.Arguments, "session_id"),
		SessionThreadID: stringArg(req.Arguments, "session_thread_id"),
	}
	confirmation := confirmationFromDecision(event, decision)
	if err := a.forwarder.SendConfirmation(ctx, event.SessionID, confirmation); err != nil {
		return nil, err
	}
	return &governance.ToolResponse{
		ContentType: "application/json",
		Metadata: map[string]string{
			"managed_agents_confirmation": confirmation.Result,
			"tool_use_id":                 confirmation.ToolUseID,
		},
	}, nil
}

func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	return InspectResponse(resp), nil
}

func (a *Adapter) EmitGovernanceMetadata(_ context.Context, resp *governance.ToolResponse, decision *governance.GovernanceDecision) error {
	if resp == nil || decision == nil {
		return nil
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	resp.Metadata["x-fulcrum-action"] = decision.Action
	resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
	resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
	return nil
}

func stringArg(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
