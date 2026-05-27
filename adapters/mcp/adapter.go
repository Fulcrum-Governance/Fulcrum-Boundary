// Package mcp provides the MCP (Model Context Protocol) transport adapter
// for Fulcrum Boundary.
//
// It converts JSON-RPC tools/call requests into canonical GovernanceRequests
// and delegates governance evaluation to the shared pipeline.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/google/uuid"
)

// ToolCallInput is the protocol-specific input parsed from an MCP tools/call request.
type ToolCallInput struct {
	ToolName  string         `json:"tool_name"`
	Arguments map[string]any `json:"arguments"`
	AgentID   string         `json:"agent_id,omitempty"`
	TenantID  string         `json:"tenant_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Method    string         `json:"method,omitempty"`
	Params    struct {
		Name      string         `json:"name,omitempty"`
		Arguments map[string]any `json:"arguments,omitempty"`
	} `json:"params,omitempty"`
}

// Adapter implements governance.TransportAdapter for MCP JSON-RPC.
type Adapter struct {
	defaultTenantID string
	forwarder       Forwarder
}

// NewAdapter creates an MCP transport adapter.
func NewAdapter(defaultTenantID string) *Adapter {
	return &Adapter{defaultTenantID: defaultTenantID}
}

// NewProxyAdapter creates an MCP adapter that can forward allowed JSON-RPC
// requests through the supplied forwarder.
func NewProxyAdapter(defaultTenantID string, forwarder Forwarder) *Adapter {
	return &Adapter{defaultTenantID: defaultTenantID, forwarder: forwarder}
}

// Type returns TransportMCP.
func (a *Adapter) Type() governance.TransportType {
	return governance.TransportMCP
}

// ParseRequest converts an MCP ToolCallInput into a canonical GovernanceRequest.
// The raw parameter must be a *ToolCallInput or a json.RawMessage that can be
// unmarshaled into one.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	var input *ToolCallInput

	switch v := raw.(type) {
	case *ToolCallInput:
		input = v
	case ToolCallInput:
		input = &v
	case json.RawMessage:
		input = &ToolCallInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportMCP, "unmarshal tool call", err)
		}
	case []byte:
		input = &ToolCallInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportMCP, "unmarshal tool call", err)
		}
	default:
		return nil, governance.NewParseError(governance.TransportMCP, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}

	toolName := input.ToolName
	if toolName == "" {
		toolName = input.Params.Name
	}
	args := input.Arguments
	if len(args) == 0 {
		args = input.Params.Arguments
	}
	method := input.Method
	if method == "" {
		method = "tools/call"
	}

	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = a.defaultTenantID
	}

	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportMCP,
		AgentID:   input.AgentID,
		TenantID:  tenantID,
		ToolName:  toolName,
		Action:    method,
		Arguments: args,
		TraceID:   input.TraceID,
	}, nil
}

// ForwardGoverned forwards the governed request to the upstream MCP server.
func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if decision == nil || !decision.Allowed() {
		return nil, fmt.Errorf("MCP request was not allowed by governance")
	}
	if a.forwarder == nil {
		return nil, fmt.Errorf("MCP forwarding requires a configured forwarder")
	}
	return a.forwarder.Forward(ctx, req.RawPayload)
}

// InspectResponse checks MCP tool output for governance concerns.
func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}, nil
	}
	return InspectJSONRPCResponse(resp)
}

// EmitGovernanceMetadata attaches governance headers to the MCP response.
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
	return nil
}
