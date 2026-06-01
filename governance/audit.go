package governance

import (
	"context"
	"time"
)

// AuditEvent represents a governance audit record emitted after every
// pipeline evaluation, regardless of outcome.
type AuditEvent struct {
	EventType           string                 `json:"event_type,omitempty"`
	RequestID           string                 `json:"request_id"`
	Transport           TransportType          `json:"transport"`
	ToolName            string                 `json:"tool_name"`
	Action              string                 `json:"action"`
	Reason              string                 `json:"reason,omitempty"`
	MatchedRule         string                 `json:"matched_rule,omitempty"`
	PolicyFile          string                 `json:"policy_file,omitempty"`
	PolicyBundleHash    string                 `json:"policy_bundle_hash,omitempty"`
	GatewayVersion      string                 `json:"gateway_version,omitempty"`
	BoundaryBuildDigest string                 `json:"boundary_build_digest,omitempty"`
	RequestHash         string                 `json:"request_hash,omitempty"`
	RawShapeHash        string                 `json:"raw_shape_hash,omitempty"`
	DecisionHash        string                 `json:"decision_hash,omitempty"`
	TrustState          string                 `json:"trust_state,omitempty"`
	Signature           string                 `json:"signature,omitempty"`
	SignatureKeyID      string                 `json:"signature_key_id,omitempty"`
	TraceID             string                 `json:"trace_id,omitempty"`
	TrustScore          float64                `json:"trust_score"`
	EnvelopeID          string                 `json:"envelope_id"`
	AgentID             string                 `json:"agent_id,omitempty"`
	TenantID            string                 `json:"tenant_id,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	Timestamp           time.Time              `json:"timestamp"`
	// DecisionMode mirrors GovernanceDecision.DecisionMode so audit sinks
	// can filter or aggregate by epistemic confidence level.
	DecisionMode DecisionMode `json:"decision_mode,omitempty"`

	// Route-context (schema_version "2" decision-record fields). These are
	// descriptive context the adapter already knows; they are copied verbatim
	// into the decision record. They are not attestation — see ExecutionClaim
	// and DecisionRecordV1's route-context field docs for the honesty caveats.

	// AdapterID names the adapter that parsed and routed the request.
	AdapterID string `json:"adapter_id,omitempty"`
	// RouteID names the specific governed route the request traveled.
	RouteID string `json:"route_id,omitempty"`
	// TopologyProfile is the named deployment posture asserted at emission
	// (asserted, not attested).
	TopologyProfile string `json:"topology_profile,omitempty"`
	// ExecutionClaim is the adapter's structured execution self-report
	// (self-report, not corroborated).
	ExecutionClaim *ExecutionClaim `json:"execution_claim,omitempty"`
}

// AuditPublisher publishes governance audit events.
// The concrete implementation uses NATS JetStream (internal/securemcp/audit.go).
type AuditPublisher interface {
	Publish(ctx context.Context, event AuditEvent)
}

// noopAuditPublisher silently discards events.
type noopAuditPublisher struct{}

func (noopAuditPublisher) Publish(context.Context, AuditEvent) {}
