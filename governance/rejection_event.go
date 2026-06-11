package governance

import (
	"context"
	"time"
)

type ParseRejectionEvent struct {
	Adapter         TransportType
	RawPayload      []byte
	RejectionReason string
	AgentID         string
	TenantID        string
	TraceID         string
	GatewayVersion  string
	BuildDigest     string
}

func (p *Pipeline) PublishParseRejection(ctx context.Context, event ParseRejectionEvent) {
	if p == nil {
		return
	}
	gatewayVersion := event.GatewayVersion
	if gatewayVersion == "" {
		gatewayVersion = p.gatewayVersion
	}
	buildDigest := event.BuildDigest
	if buildDigest == "" {
		buildDigest = p.buildDigest
	}
	auditEvent := AuditEvent{
		EventType:           "parse_rejected",
		Transport:           event.Adapter,
		Action:              "deny",
		Reason:              event.RejectionReason,
		TraceID:             event.TraceID,
		AgentID:             event.AgentID,
		TenantID:            event.TenantID,
		GatewayVersion:      gatewayVersion,
		BoundaryBuildDigest: buildDigest,
		RawShapeHash:        ComputeRawShapeHash(event.RawPayload),
		TrustState:          TrustStateTrusted.String(),
		Timestamp:           time.Now().UTC(),
		DecisionMode:        DecisionModeDeterministic,
	}
	// Parse rejections are decision records too: sign them under the same
	// configured signer so a signing deployment never emits an unsigned record.
	p.signAuditEvent(&auditEvent)
	p.auditor.Publish(ctx, auditEvent)
}
