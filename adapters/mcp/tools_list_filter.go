package mcp

import (
	"context"
	"encoding/json"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func filterToolsList(ctx context.Context, body []byte, pipeline *governance.Pipeline, identity Identity) []byte {
	if pipeline == nil {
		return body
	}
	var resp map[string]any
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}
	result, _ := resp["result"].(map[string]any)
	if result == nil {
		return body
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		return body
	}
	filtered := make([]any, 0, len(tools))
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		name, _ := tool["name"].(string)
		if name == "" {
			continue
		}
		decision, err := pipeline.Evaluate(ctx, &governance.GovernanceRequest{
			Transport: governance.TransportMCP,
			ToolName:  name,
			Action:    "tools/call",
			AgentID:   identity.AgentID,
			TenantID:  identity.TenantID,
			TraceID:   identity.TraceID,
		})
		if err == nil && decision.Allowed() {
			filtered = append(filtered, item)
		}
	}
	result["tools"] = filtered
	encoded, err := json.Marshal(resp)
	if err != nil {
		return body
	}
	return encoded
}
