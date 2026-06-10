package governance

import (
	"context"
	"time"
)

// AuditEvent represents a governance audit record emitted after every
// pipeline evaluation, regardless of outcome. The pipeline populates it from
// the request and the final GovernanceDecision and hands it to an
// AuditPublisher; BuildDecisionRecord renders it into the hash-verifiable
// decision record. Fields are descriptive context, not attestation: see the
// route-context field docs below and ExecutionClaim for the honesty caveats.
type AuditEvent struct {
	// EventType labels the record; empty for an ordinary decision, or
	// "trust_transition" for the event emitted on a trust state change.
	EventType string `json:"event_type,omitempty"`
	// RequestID is the per-request identifier shared with the decision.
	RequestID string `json:"request_id"`
	// Transport is the transport the request arrived on.
	Transport TransportType `json:"transport"`
	// ToolName is the tool, command, or operation that was evaluated.
	ToolName string `json:"tool_name"`
	// Action is the final decision verdict (allow/deny/warn/escalate/
	// require_approval). For a dry-run deny this still records the real deny.
	Action string `json:"action"`
	// Reason is the human-readable explanation for the verdict.
	Reason string `json:"reason,omitempty"`
	// MatchedRule names the static policy or evaluator policy that decided.
	MatchedRule string `json:"matched_rule,omitempty"`
	// PolicyFile is the source file of the matched policy, when known.
	PolicyFile string `json:"policy_file,omitempty"`
	// PolicyBundleHash identifies the active policy bundle.
	PolicyBundleHash string `json:"policy_bundle_hash,omitempty"`
	// GatewayVersion is the released Boundary version that emitted the record.
	GatewayVersion string `json:"gateway_version,omitempty"`
	// BoundaryBuildDigest identifies the binary/image that emitted the record.
	BoundaryBuildDigest string `json:"boundary_build_digest,omitempty"`
	// RequestHash is the content hash of the canonical request.
	RequestHash string `json:"request_hash,omitempty"`
	// RawShapeHash is the hash of the raw request shape, when computed.
	RawShapeHash string `json:"raw_shape_hash,omitempty"`
	// DecisionHash is the hash of the decision record, when computed.
	DecisionHash string `json:"decision_hash,omitempty"`
	// TrustState is the agent's circuit-breaker state at decision time.
	TrustState string `json:"trust_state,omitempty"`
	// Signature is the detached signature over the decision record, if signed.
	Signature string `json:"signature,omitempty"`
	// SignatureKeyID identifies the key that produced Signature.
	SignatureKeyID string `json:"signature_key_id,omitempty"`
	// TraceID carries the caller's distributed-trace identifier.
	TraceID string `json:"trace_id,omitempty"`
	// TrustScore is the agent's trust score in [0,1] at decision time.
	TrustScore float64 `json:"trust_score"`
	// EnvelopeID is the governance envelope identifier for the request.
	EnvelopeID string `json:"envelope_id"`
	// AgentID is the acting agent, when present.
	AgentID string `json:"agent_id,omitempty"`
	// TenantID is the owning tenant, when present.
	TenantID string `json:"tenant_id,omitempty"`
	// Metadata carries optional sink-specific key/value context.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Timestamp is when the event was emitted.
	Timestamp time.Time `json:"timestamp"`
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

// AuditPublisher publishes governance audit events. The pipeline emits exactly
// one AuditEvent per evaluation (plus a trust_transition event on a state
// change), regardless of the decision outcome.
//
// Contract for the implementer: Publish must not block the caller — it runs on
// the governance hot path. Treat it as fire-and-forget: buffer, drop, or hand
// off asynchronously rather than performing synchronous network I/O. Errors are
// the publisher's to absorb; Publish has no return value and must not panic.
//
// In-repo implementations: noopAuditPublisher (the default, silently discards),
// SlogAuditPublisher (slog_audit.go, structured slog records), and
// kernel.NATSAuditPublisher (kernel mode, publishes to NATS JetStream).
type AuditPublisher interface {
	Publish(ctx context.Context, event AuditEvent)
}

// noopAuditPublisher silently discards events.
type noopAuditPublisher struct{}

func (noopAuditPublisher) Publish(context.Context, AuditEvent) {}
