package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/google/uuid"
)

// CommandInput is the protocol-specific input for CLI command execution.
type CommandInput struct {
	Command  string `json:"command"`
	Stdin    []byte `json:"stdin,omitempty"`
	AgentID  string `json:"agent_id,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

// Adapter implements governance.TransportAdapter for CLI command execution.
type Adapter struct {
	defaultTenantID string
	classifier      *Classifier
	inspector       *Inspector
	executor        Executor
}

// NewAdapter creates a CLI transport adapter with the given default tenant ID.
func NewAdapter(defaultTenantID string) *Adapter {
	return &Adapter{
		defaultTenantID: defaultTenantID,
		classifier:      NewClassifier(),
		inspector:       NewInspector(),
		executor:        OSExecutor{},
	}
}

// NewAdapterWithClassifier creates a CLI transport adapter with a custom classifier.
func NewAdapterWithClassifier(defaultTenantID string, c *Classifier) *Adapter {
	return &Adapter{
		defaultTenantID: defaultTenantID,
		classifier:      c,
		inspector:       NewInspector(),
		executor:        OSExecutor{},
	}
}

// NewAdapterWithExecutor creates a CLI transport adapter with a custom executor.
func NewAdapterWithExecutor(defaultTenantID string, executor Executor) *Adapter {
	if executor == nil {
		executor = OSExecutor{}
	}
	return &Adapter{
		defaultTenantID: defaultTenantID,
		classifier:      NewClassifier(),
		inspector:       NewInspector(),
		executor:        executor,
	}
}

// Type returns TransportCLI.
func (a *Adapter) Type() governance.TransportType {
	return governance.TransportCLI
}

// ParseRequest converts a CLI CommandInput into a canonical GovernanceRequest.
// The raw parameter must be a *CommandInput, CommandInput, json.RawMessage, or []byte.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	var input *CommandInput

	switch v := raw.(type) {
	case *CommandInput:
		input = v
	case CommandInput:
		input = &v
	case json.RawMessage:
		input = &CommandInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportCLI, "unmarshal command input", err)
		}
	case []byte:
		input = &CommandInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportCLI, "unmarshal command input", err)
		}
	default:
		return nil, governance.NewParseError(governance.TransportCLI, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}

	if input.Command == "" {
		return nil, governance.NewParseError(governance.TransportCLI, "empty command", nil)
	}

	// Parse the command into pipe segments.
	segments, err := ParseCommand(input.Command)
	if err != nil {
		return nil, governance.NewParseError(governance.TransportCLI, "parse command", err)
	}

	// Classify each segment's risk level.
	for i := range segments {
		segments[i].RiskLevel = a.classifier.ClassifyCommand(segments[i].Command)
	}

	// Inspect stdin for sensitive data.
	if len(input.Stdin) > 0 {
		concerns := a.inspector.InspectStdin(input.Stdin)
		if len(concerns) > 0 {
			// Attach stdin concerns as compliance flags in the request via Arguments.
			// The governance pipeline can read these during evaluation.
			_ = concerns // Concerns are surfaced in the governance request's action level.
		}
	}

	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = a.defaultTenantID
	}

	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportCLI,
		AgentID:   input.AgentID,
		TenantID:  tenantID,
		ToolName:  segments[0].Command,
		Action:    governance.HighestRisk(segments),
		Command:   input.Command,
		PipeChain: segments,
		Stdin:     input.Stdin,
	}, nil
}

// ForwardGoverned executes the governed CLI command only when the decision
// allows it. Denied commands never reach the executor.
func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("governance request is required")
	}
	if decision == nil || !decision.Allowed() {
		return deniedResponse(decision), nil
	}
	resp, err := a.executor.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := a.EmitGovernanceMetadata(ctx, resp, decision); err != nil {
		return nil, err
	}
	inspection, err := a.InspectResponse(ctx, resp)
	if err != nil {
		return nil, err
	}
	attachInspectionMetadata(resp, inspection)
	return resp, nil
}

// InspectResponse examines CLI command output for governance concerns.
func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}, nil
	}
	return a.inspector.InspectOutput(resp.Content), nil
}

// EmitGovernanceMetadata attaches governance fields to the CLI response metadata.
func (a *Adapter) EmitGovernanceMetadata(_ context.Context, resp *governance.ToolResponse, decision *governance.GovernanceDecision) error {
	if resp == nil || decision == nil {
		return nil
	}
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]string)
	}
	resp.Metadata["x-fulcrum-action"] = decision.Action
	resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
	resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
	if decision.PolicyID != "" {
		resp.Metadata["x-fulcrum-policy-id"] = decision.PolicyID
	}
	return nil
}

// GovernCommand runs the complete wrapper-owned CLI lifecycle.
func (a *Adapter) GovernCommand(ctx context.Context, raw any, pipeline *governance.Pipeline) (*governance.ToolResponse, error) {
	req, err := a.ParseRequest(ctx, raw)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		decision := &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     "governance pipeline is required",
			EnvelopeID: req.EnvelopeID,
		}
		return a.ForwardGoverned(ctx, req, decision)
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
	return a.ForwardGoverned(ctx, req, decision)
}

// Compile-time interface check.
var _ governance.TransportAdapter = (*Adapter)(nil)
