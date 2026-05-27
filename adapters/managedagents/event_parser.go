package managedagents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/boundary/governance"
)

// ParseEvent normalizes a Managed Agents event into the local Event shape.
func ParseEvent(raw any) (*Event, error) {
	var event *Event
	switch v := raw.(type) {
	case *Event:
		event = v
	case Event:
		event = &v
	case json.RawMessage:
		event = &Event{Raw: v}
		if err := json.Unmarshal(v, event); err != nil {
			return nil, governance.NewParseError(governance.TransportManagedAgents, "unmarshal event", err)
		}
	case []byte:
		event = &Event{Raw: append([]byte(nil), v...)}
		if err := json.Unmarshal(v, event); err != nil {
			return nil, governance.NewParseError(governance.TransportManagedAgents, "unmarshal event", err)
		}
	default:
		return nil, governance.NewParseError(governance.TransportManagedAgents, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}
	if event == nil || event.Type == "" {
		return nil, governance.NewParseError(governance.TransportManagedAgents, "Event.Type is required", nil)
	}
	return event, nil
}

// GovernanceRequestFromEvent maps a tool-use event into a canonical request.
func GovernanceRequestFromEvent(event *Event, defaultTenantID string) (*governance.GovernanceRequest, error) {
	if event == nil {
		return nil, governance.NewParseError(governance.TransportManagedAgents, "event is nil", nil)
	}
	if event.Type != EventAgentToolUse && event.Type != EventAgentMCPToolUse {
		return nil, governance.NewParseError(governance.TransportManagedAgents, "event is not a governable tool use", nil)
	}
	toolName := event.ToolName
	if toolName == "" {
		if v, ok := event.Input["name"].(string); ok {
			toolName = v
		}
	}
	if toolName == "" {
		return nil, governance.NewParseError(governance.TransportManagedAgents, "tool name is required", nil)
	}
	tenantID := event.TenantID
	if tenantID == "" {
		tenantID = defaultTenantID
	}
	args := make(map[string]any, len(event.Input)+4)
	for k, v := range event.Input {
		args[k] = v
	}
	args["tool_use_id"] = event.ID
	args["session_id"] = event.SessionID
	args["session_thread_id"] = event.SessionThreadID
	args["event_type"] = event.Type

	traceID := event.SessionID
	if traceID == "" {
		traceID = event.ID
	}
	return &governance.GovernanceRequest{
		RequestID:   uuid.New().String(),
		Transport:   governance.TransportManagedAgents,
		AgentID:     event.AgentID,
		TenantID:    tenantID,
		ToolName:    toolName,
		Action:      event.Type,
		Arguments:   args,
		TraceID:     traceID,
		BudgetKey:   budgetKey(event.SessionID, event.SessionThreadID),
		ParentEnvID: event.SessionThreadID,
	}, nil
}

// ParseRequest converts a raw event into a canonical governance request.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	event, err := ParseEvent(raw)
	if err != nil {
		return nil, err
	}
	return GovernanceRequestFromEvent(event, a.defaultTenantID)
}
