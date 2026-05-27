package a2a

import "encoding/json"

const (
	StatusAllowed     = "allowed"
	StatusDenied      = "denied"
	StatusUnsupported = "unsupported"
	StatusError       = "error"
)

// TaskMessage is the legacy local schema accepted by early Boundary A2A
// parser tests. New callers should prefer TaskEnvelope.
type TaskMessage struct {
	TaskID    string         `json:"task_id"`
	AgentCard AgentCard      `json:"agent_card"`
	Action    string         `json:"action"`
	Input     map[string]any `json:"input"`
}

// AgentCard identifies an A2A participant in the legacy local schema.
type AgentCard struct {
	AgentID  string `json:"agent_id"`
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
}

// TaskEnvelope is Boundary's documented preview A2A envelope. It intentionally
// supports only the stable fields Boundary needs to govern an action.
type TaskEnvelope struct {
	TaskID         string          `json:"task_id,omitempty"`
	ContextID      string          `json:"context_id,omitempty"`
	MessageID      string          `json:"message_id,omitempty"`
	SenderAgentID  string          `json:"sender_agent_id,omitempty"`
	Receiver       string          `json:"receiver,omitempty"`
	Action         string          `json:"action,omitempty"`
	Input          map[string]any  `json:"input,omitempty"`
	Metadata       map[string]any  `json:"metadata,omitempty"`
	RequiredFields []string        `json:"required_fields,omitempty"`
	Raw            json.RawMessage `json:"-"`
}

// jsonRPCRequest covers the A2A JSON-RPC message/send shape documented in the
// protocol snapshot. It is deliberately minimal and converted to TaskEnvelope.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type messageSendParams struct {
	Message  a2aMessage     `json:"message"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type a2aMessage struct {
	Role      string         `json:"role,omitempty"`
	TaskID    string         `json:"taskId,omitempty"`
	ContextID string         `json:"contextId,omitempty"`
	MessageID string         `json:"messageId,omitempty"`
	Parts     []messagePart  `json:"parts,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type messagePart struct {
	Kind string         `json:"kind,omitempty"`
	Text string         `json:"text,omitempty"`
	Data map[string]any `json:"data,omitempty"`
}

// TaskResponse is the transport-shaped A2A response Boundary returns after
// governance and optional downstream forwarding.
type TaskResponse struct {
	TaskID     string              `json:"task_id,omitempty"`
	ContextID  string              `json:"context_id,omitempty"`
	Status     string              `json:"status"`
	Output     map[string]any      `json:"output,omitempty"`
	Artifacts  []Artifact          `json:"artifacts,omitempty"`
	Error      *TaskError          `json:"error,omitempty"`
	Governance *GovernanceMetadata `json:"governance,omitempty"`
}

type Artifact struct {
	Name string `json:"name,omitempty"`
	MIME string `json:"mime_type,omitempty"`
	Text string `json:"text,omitempty"`
}

type TaskError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GovernanceMetadata struct {
	Action             string   `json:"action"`
	Reason             string   `json:"reason,omitempty"`
	RequestID          string   `json:"request_id,omitempty"`
	EnvelopeID         string   `json:"envelope_id,omitempty"`
	MatchedRule        string   `json:"matched_rule,omitempty"`
	DecisionMode       string   `json:"decision_mode,omitempty"`
	TrustScore         float64  `json:"trust_score,omitempty"`
	InspectionConcerns []string `json:"inspection_concerns,omitempty"`
}
