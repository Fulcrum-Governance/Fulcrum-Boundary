package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// InspectJSONRPCResponse flags malformed JSON-RPC responses and upstream errors.
func InspectJSONRPCResponse(resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil || len(resp.Content) == 0 {
		return &governance.ResponseInspection{Safe: true}, nil
	}
	var decoded any
	if err := json.Unmarshal(resp.Content, &decoded); err != nil {
		return &governance.ResponseInspection{
			Safe:     false,
			Concerns: []string{"upstream response is not valid JSON"},
		}, nil
	}
	inspection := &governance.ResponseInspection{Safe: true}
	if hasJSONRPCError(decoded) {
		inspection.Concerns = append(inspection.Concerns, "upstream returned JSON-RPC error")
	}
	if resp.ExitCode >= 500 {
		inspection.Safe = false
		inspection.Concerns = append(inspection.Concerns, fmt.Sprintf("upstream HTTP status %d", resp.ExitCode))
	}
	return inspection, nil
}

func hasJSONRPCError(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		_, ok := v["error"]
		return ok
	case []any:
		for _, item := range v {
			if hasJSONRPCError(item) {
				return true
			}
		}
	}
	return false
}
