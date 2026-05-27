// Package a2a provides a preview TransportAdapter for governed
// Agent-to-Agent (A2A) task/message envelopes.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Adapter implements governance.TransportAdapter for A2A messages.
type Adapter struct {
	// TenantID is applied to every parsed request when the inbound message
	// does not carry tenant information of its own.
	TenantID string

	forwarder Forwarder
}

var _ governance.TransportAdapter = (*Adapter)(nil)

// NewAdapter returns an A2A adapter scoped to a tenant.
func NewAdapter(tenantID string) *Adapter {
	return &Adapter{TenantID: tenantID}
}

// NewForwardingAdapter returns an A2A adapter that owns governed forwarding.
func NewForwardingAdapter(tenantID string, forwarder Forwarder) *Adapter {
	return &Adapter{TenantID: tenantID, forwarder: forwarder}
}

// Type returns TransportA2A.
func (a *Adapter) Type() governance.TransportType { return governance.TransportA2A }

// ParseRequest converts supported A2A envelopes into a GovernanceRequest.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	envelope, rawPayload, err := ParseTaskEnvelope(raw)
	if err != nil {
		return nil, err
	}
	return GovernanceRequestFromEnvelope(envelope, rawPayload, a.TenantID)
}

// ForwardGoverned forwards allowed A2A tasks through the configured forwarder
// and returns a transport-shaped denial when governance denies the request.
func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("a2a request is required")
	}
	envelope := envelopeFromRequest(req)
	if decision == nil || !decision.Allowed() {
		return toolResponseFromTaskResponse(DeniedTaskResponse(envelope, decision))
	}
	if a.forwarder == nil {
		return nil, fmt.Errorf("a2a forwarding requires a configured forwarder")
	}
	response, err := a.forwarder.ForwardTask(ctx, envelope)
	if err != nil {
		return nil, err
	}
	AttachGovernanceMetadata(response, decision)
	return toolResponseFromTaskResponse(response)
}

// InspectResponse examines downstream A2A output for policy-relevant signals.
func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	return InspectResponse(resp), nil
}

// EmitGovernanceMetadata attaches governance metadata to a ToolResponse.
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
	if decision.MatchedRule != "" {
		resp.Metadata["x-fulcrum-rule"] = decision.MatchedRule
	}
	return nil
}

// GovernTask runs the complete preview A2A lifecycle: parse, evaluate, deny or
// forward, inspect, attach metadata, and return a transport-shaped response.
func (a *Adapter) GovernTask(ctx context.Context, raw any, pipeline *governance.Pipeline) (*TaskResponse, error) {
	envelope, rawPayload, err := ParseTaskEnvelope(raw)
	if err != nil {
		return UnsupportedTaskResponse("", err), nil
	}
	req, err := GovernanceRequestFromEnvelope(envelope, rawPayload, a.TenantID)
	if err != nil {
		return UnsupportedTaskResponse(envelope.TaskID, err), nil
	}
	if pipeline == nil {
		decision := &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     "governance pipeline is required",
			EnvelopeID: req.EnvelopeID,
		}
		return DeniedTaskResponse(*envelope, decision), nil
	}
	decision, err := pipeline.Evaluate(ctx, req)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: req.EnvelopeID,
		}
	}
	if decision == nil || !decision.Allowed() {
		return DeniedTaskResponse(*envelope, decision), nil
	}
	if a.forwarder == nil {
		return ErrorTaskResponse(*envelope, "forwarding_error", "a2a forwarding requires a configured forwarder"), nil
	}
	response, err := a.forwarder.ForwardTask(ctx, *envelope)
	if err != nil {
		return ErrorTaskResponse(*envelope, "forwarding_error", err.Error()), nil
	}
	AttachGovernanceMetadata(response, decision)
	toolResp, err := toolResponseFromTaskResponse(response)
	if err != nil {
		return ErrorTaskResponse(*envelope, "response_error", err.Error()), nil
	}
	inspection := InspectResponse(toolResp)
	if inspection != nil && !inspection.Safe {
		if response.Governance == nil {
			response.Governance = MetadataFromDecision(decision)
		}
		response.Governance.InspectionConcerns = append(response.Governance.InspectionConcerns, inspection.Concerns...)
	}
	return response, nil
}

func toolResponseFromTaskResponse(response *TaskResponse) (*governance.ToolResponse, error) {
	if response == nil {
		response = &TaskResponse{Status: StatusError}
	}
	body, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &governance.ToolResponse{
		Content:     body,
		ContentType: "application/json",
		Metadata: map[string]string{
			"a2a_status":  response.Status,
			"a2a_task_id": response.TaskID,
		},
	}, nil
}
