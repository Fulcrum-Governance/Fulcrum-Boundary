// Package managedagents provides a Boundary adapter for Anthropic Managed
// Agents session events. The package keeps the upstream API behind small local
// interfaces so tests and deployments can govern sessions without embedding
// Anthropic SDK types in the governance core.
package managedagents

import (
	"encoding/json"
	"time"
)

const (
	EventAgentToolUse       = "agent.tool_use"
	EventAgentMCPToolUse    = "agent.mcp_tool_use"
	EventThreadCreated      = "session.thread_created"
	EventThreadStatusPrefix = "session.thread_status_"
	EventStatusIdle         = "session.status_idle"
	EventToolResult         = "agent.tool_result"
	ConfirmationEventType   = "user.tool_confirmation"

	ConfirmationAllow = "allow"
	ConfirmationDeny  = "deny"
)

// Event is the local wire shape Boundary needs from the Managed Agents SSE
// stream. Unknown upstream fields stay in Data so the proxy can preserve them.
type Event struct {
	ID              string          `json:"id,omitempty"`
	Type            string          `json:"type"`
	TenantID        string          `json:"tenant_id,omitempty"`
	UserID          string          `json:"user_id,omitempty"`
	AgentID         string          `json:"agent_id,omitempty"`
	SessionID       string          `json:"session_id,omitempty"`
	SessionThreadID string          `json:"session_thread_id,omitempty"`
	ParentThreadID  string          `json:"parent_thread_id,omitempty"`
	ToolName        string          `json:"tool_name,omitempty"`
	Input           map[string]any  `json:"input,omitempty"`
	StopReason      *StopReason     `json:"stop_reason,omitempty"`
	Usage           *Usage          `json:"usage,omitempty"`
	Governance      *Metadata       `json:"governance,omitempty"`
	Data            map[string]any  `json:"data,omitempty"`
	Raw             json.RawMessage `json:"-"`
}

// StopReason captures the requires_action event IDs emitted while a session is
// idle and waiting for tool confirmations.
type StopReason struct {
	Type     string   `json:"type,omitempty"`
	EventIDs []string `json:"event_ids,omitempty"`
}

// Usage is the per-event cost signal Boundary tracks against session budgets.
type Usage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// ToolConfirmation is the event Boundary sends upstream to allow or deny an
// always_ask tool invocation.
type ToolConfirmation struct {
	Type            string    `json:"type"`
	ToolUseID       string    `json:"tool_use_id"`
	Result          string    `json:"result"`
	DenyMessage     string    `json:"deny_message,omitempty"`
	SessionThreadID string    `json:"session_thread_id,omitempty"`
	ProcessedAt     time.Time `json:"processed_at,omitempty"`
	Governance      *Metadata `json:"governance,omitempty"`
}

// SessionCreateRequest is the customer-facing request Boundary proxies before
// opening or streaming a Managed Agents session upstream.
type SessionCreateRequest struct {
	TenantID      string  `json:"tenant_id"`
	UserID        string  `json:"user_id,omitempty"`
	AgentID       string  `json:"agent_id"`
	SessionID     string  `json:"session_id,omitempty"`
	BudgetCeiling float64 `json:"budget_ceiling,omitempty"`
}
