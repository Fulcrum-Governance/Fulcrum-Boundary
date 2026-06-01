package governance

import (
	"context"
	"log/slog"
)

// SlogAuditPublisher writes governance audit events to a slog.Logger as
// structured records. Allow/warn decisions log at INFO; deny, escalate, and
// require_approval decisions log at WARN. This is the recommended default
// AuditPublisher for development and for production deployments that already
// ship logs to a structured backend.
type SlogAuditPublisher struct {
	Logger *slog.Logger
}

// NewSlogAuditPublisher returns a SlogAuditPublisher that uses the given
// logger. If logger is nil, slog.Default() is used.
func NewSlogAuditPublisher(logger *slog.Logger) *SlogAuditPublisher {
	return &SlogAuditPublisher{Logger: logger}
}

// Publish implements AuditPublisher.
func (p *SlogAuditPublisher) Publish(ctx context.Context, event AuditEvent) {
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}

	level := slog.LevelInfo
	switch event.Action {
	case "deny", "escalate", "require_approval":
		level = slog.LevelWarn
	}
	record := BuildDecisionRecord(event)
	msg := event.EventType
	if msg == "" {
		msg = "governance_decision"
	}

	logger.LogAttrs(ctx, level, msg,
		slog.String("schema_version", record.SchemaVersion),
		slog.String("event_type", record.EventType),
		slog.String("record_id", record.RecordID),
		slog.String("request_id", event.RequestID),
		slog.String("transport", string(event.Transport)),
		slog.String("tool_name", event.ToolName),
		slog.String("action", event.Action),
		slog.String("reason", event.Reason),
		slog.String("decision_mode", string(event.DecisionMode)),
		slog.String("matched_rule", event.MatchedRule),
		slog.String("policy_file", event.PolicyFile),
		slog.String("policy_bundle_hash", event.PolicyBundleHash),
		slog.String("gateway_version", event.GatewayVersion),
		slog.String("boundary_version", event.GatewayVersion),
		slog.String("boundary_build_digest", event.BoundaryBuildDigest),
		slog.String("request_hash", record.RequestHash),
		slog.String("raw_shape_hash", record.RawShapeHash),
		slog.String("decision_hash", record.DecisionHash),
		slog.String("trust_state", record.TrustState),
		slog.String("signature", record.Signature),
		slog.String("signature_key_id", record.SignatureKeyID),
		slog.String("trace_id", event.TraceID),
		slog.String("agent_id", event.AgentID),
		slog.String("tenant_id", event.TenantID),
		slog.Float64("trust_score", event.TrustScore),
		slog.String("envelope_id", event.EnvelopeID),
		slog.Time("timestamp", event.Timestamp),
		// Route-context (schema_version "2"). Emitted for slog/record parity;
		// empty for V1 records. Descriptive only — see ExecutionClaim docs for
		// the asserted-not-attested / self-report-not-corroborated caveats.
		slog.String("adapter_id", record.AdapterID),
		slog.String("route_id", record.RouteID),
		slog.String("topology_profile", record.TopologyProfile),
		slog.Bool("execution_claim_present", record.ExecutionClaim != nil),
	)
}
