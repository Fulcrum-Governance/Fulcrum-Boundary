// Package grpc provides a gRPC unary server interceptor that routes
// every RPC through the governance pipeline before the handler runs.
//
// The adapter lives in its own go.mod so the root Boundary module stays free
// of the google.golang.org/grpc dependency tree. Import this adapter only
// in services that already speak gRPC.
package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Default metadata header keys used to extract identity from a gRPC call.
const (
	DefaultAgentMetadataKey  = "x-agent-id"
	DefaultTenantMetadataKey = "x-tenant-id"
	DefaultTraceMetadataKey  = "x-trace-id"

	TrailerAction     = "governance-action"
	TrailerRule       = "governance-rule"
	TrailerMode       = "governance-mode"
	TrailerTrust      = "governance-trust"
	TrailerRequestID  = "governance-request-id"
	TrailerEnvelopeID = "governance-envelope-id"
	TrailerSafe       = "governance-response-safe"
	TrailerConcerns   = "governance-response-concerns"
)

// CallInfo describes a gRPC unary call in transport-neutral terms.
// It is what ParseRequest expects to receive as its raw input.
type CallInfo struct {
	Method   string      // Full gRPC method name, e.g., "/svc.Service/Method"
	Metadata metadata.MD // Incoming metadata headers
	AgentID  string      // Optional override; falls back to metadata
	TenantID string      // Optional override; falls back to metadata
}

// Adapter implements governance.TransportAdapter for gRPC unary calls.
type Adapter struct {
	// AgentMetadataKey overrides the metadata header used for AgentID.
	// Defaults to DefaultAgentMetadataKey when empty.
	AgentMetadataKey string
	// TenantMetadataKey overrides the metadata header used for TenantID.
	// Defaults to DefaultTenantMetadataKey when empty.
	TenantMetadataKey string
	// DefaultTenantID is used when no tenant header is present.
	DefaultTenantID string
}

// NewAdapter returns an Adapter with default header keys.
func NewAdapter(defaultTenantID string) *Adapter {
	return &Adapter{DefaultTenantID: defaultTenantID}
}

// Type returns TransportGRPC.
func (a *Adapter) Type() governance.TransportType { return governance.TransportGRPC }

// ParseRequest converts a *CallInfo into a canonical GovernanceRequest.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	info, ok := raw.(*CallInfo)
	if !ok {
		return nil, governance.NewParseError(governance.TransportGRPC, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}
	if info.Method == "" {
		return nil, governance.NewParseError(governance.TransportGRPC, "CallInfo.Method is required", nil)
	}

	agentID := info.AgentID
	tenantID := info.TenantID
	traceID := ""
	if info.Metadata != nil {
		if agentID == "" {
			agentID = firstMetadataValue(info.Metadata, a.agentKey())
		}
		if tenantID == "" {
			tenantID = firstMetadataValue(info.Metadata, a.tenantKey())
		}
		traceID = firstMetadataValue(info.Metadata, DefaultTraceMetadataKey)
	}
	if tenantID == "" {
		tenantID = a.DefaultTenantID
	}

	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportGRPC,
		AgentID:   agentID,
		TenantID:  tenantID,
		ToolName:  info.Method,
		Action:    "grpc/unary",
		TraceID:   traceID,
	}, nil
}

// ForwardGoverned is a no-op for gRPC. Forwarding the actual call is the
// responsibility of the surrounding gRPC server interceptor chain.
func (a *Adapter) ForwardGoverned(_ context.Context, _ *governance.GovernanceRequest, _ *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	return nil, nil
}

// InspectResponse performs best-effort inspection over response bytes. Unary
// gRPC handlers may hand Boundary a string or byte payload through
// InspectGRPCResponse; arbitrary protobuf messages remain host-shaped and are
// inspected only through their string form.
func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}, nil
	}
	return inspectResponseContent(string(resp.Content)), nil
}

// EmitGovernanceMetadata attaches trailer-shaped metadata to ToolResponse for
// adapter conformance tests and non-server embeddings. Server interceptors emit
// the same keys as actual gRPC trailers.
func (a *Adapter) EmitGovernanceMetadata(_ context.Context, resp *governance.ToolResponse, decision *governance.GovernanceDecision) error {
	if resp == nil || decision == nil {
		return nil
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	for key, value := range decisionTrailerValues(decision) {
		resp.Metadata[key] = value
	}
	return nil
}

func (a *Adapter) agentKey() string {
	if a.AgentMetadataKey != "" {
		return a.AgentMetadataKey
	}
	return DefaultAgentMetadataKey
}

func (a *Adapter) tenantKey() string {
	if a.TenantMetadataKey != "" {
		return a.TenantMetadataKey
	}
	return DefaultTenantMetadataKey
}

// InspectGRPCResponse converts a unary handler response into the transport
// neutral inspection result Boundary can expose through trailers.
func (a *Adapter) InspectGRPCResponse(ctx context.Context, resp any) (*governance.ResponseInspection, error) {
	content := responseContent(resp)
	return a.InspectResponse(ctx, &governance.ToolResponse{
		Content:     []byte(content),
		ContentType: "text/plain",
	})
}

// UnaryInterceptor returns a grpc.UnaryServerInterceptor that evaluates each
// RPC through pipeline before invoking the actual handler. Denied requests
// return codes.PermissionDenied with the governance reason as the message.
// Governance trailers are emitted for allow, deny, and fail-closed outcomes
// when the current context is backed by a gRPC server transport stream.
//
// adapter may be nil; in that case a default-configured adapter is used.
func UnaryInterceptor(pipeline *governance.Pipeline, adapter *Adapter) grpclib.UnaryServerInterceptor {
	if adapter == nil {
		adapter = &Adapter{}
	}
	return func(ctx context.Context, req any, info *grpclib.UnaryServerInfo, handler grpclib.UnaryHandler) (any, error) {
		if info == nil || info.FullMethod == "" {
			decision := failClosedDecision("", "gRPC full method is required")
			_ = emitDecisionTrailer(ctx, decision)
			return nil, status.Errorf(codes.InvalidArgument, "governance: %s", decision.Reason)
		}
		md, _ := metadata.FromIncomingContext(ctx)
		gReq, err := adapter.ParseRequest(ctx, &CallInfo{
			Method:   info.FullMethod,
			Metadata: md,
		})
		if err != nil {
			decision := failClosedDecision("", fmt.Sprintf("parse request: %v", err))
			_ = emitDecisionTrailer(ctx, decision)
			return nil, status.Errorf(codes.InvalidArgument, "governance: %s", decision.Reason)
		}
		ensureRequestIdentity(gReq)
		if pipeline == nil {
			decision := failClosedDecision(gReq.RequestID, "pipeline is required")
			decision.EnvelopeID = gReq.EnvelopeID
			_ = emitDecisionTrailer(ctx, decision)
			return nil, status.Errorf(codes.PermissionDenied, "governance: %s", decision.Reason)
		}
		decision, err := pipeline.Evaluate(ctx, gReq)
		if err != nil {
			decision = failClosedDecision(gReq.RequestID, fmt.Sprintf("evaluate: %v", err))
			decision.EnvelopeID = gReq.EnvelopeID
			_ = emitDecisionTrailer(ctx, decision)
			return nil, status.Errorf(codes.PermissionDenied, "governance: %s", decision.Reason)
		}
		if !decision.Allowed() {
			_ = emitDecisionTrailer(ctx, decision)
			reason := decision.Reason
			if reason == "" {
				reason = decision.Action
			}
			return nil, status.Errorf(codes.PermissionDenied, "governance: %s", reason)
		}
		resp, err := handler(ctx, req)
		if err != nil {
			_ = emitDecisionTrailer(ctx, decision)
			return nil, err
		}
		inspection, inspectErr := adapter.InspectGRPCResponse(ctx, resp)
		if inspectErr != nil {
			decision := failClosedDecision(gReq.RequestID, fmt.Sprintf("inspect response: %v", inspectErr))
			decision.EnvelopeID = gReq.EnvelopeID
			_ = emitDecisionTrailer(ctx, decision)
			return nil, status.Errorf(codes.PermissionDenied, "governance: %s", decision.Reason)
		}
		_ = emitDecisionAndInspectionTrailer(ctx, decision, inspection)
		return resp, nil
	}
}

func firstMetadataValue(md metadata.MD, key string) string {
	if vs := md.Get(key); len(vs) > 0 {
		return vs[0]
	}
	return ""
}

func ensureRequestIdentity(req *governance.GovernanceRequest) {
	if req == nil {
		return
	}
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	if req.EnvelopeID == "" {
		req.EnvelopeID = uuid.New().String()
	}
}

func emitDecisionTrailer(ctx context.Context, decision *governance.GovernanceDecision) error {
	return grpclib.SetTrailer(ctx, metadata.New(decisionTrailerValues(decision)))
}

func emitDecisionAndInspectionTrailer(ctx context.Context, decision *governance.GovernanceDecision, inspection *governance.ResponseInspection) error {
	values := decisionTrailerValues(decision)
	if inspection != nil {
		values[TrailerSafe] = strconv.FormatBool(inspection.Safe)
		if len(inspection.Concerns) > 0 {
			values[TrailerConcerns] = strings.Join(inspection.Concerns, "; ")
		}
	}
	return grpclib.SetTrailer(ctx, metadata.New(values))
}

func decisionTrailerValues(decision *governance.GovernanceDecision) map[string]string {
	values := map[string]string{
		TrailerAction:     "deny",
		TrailerRule:       "",
		TrailerMode:       "",
		TrailerTrust:      "0",
		TrailerRequestID:  "",
		TrailerEnvelopeID: "",
	}
	if decision == nil {
		return values
	}
	values[TrailerAction] = decision.Action
	values[TrailerRule] = decision.MatchedRule
	values[TrailerMode] = string(decision.DecisionMode)
	values[TrailerTrust] = strconv.FormatFloat(decision.TrustScore, 'f', -1, 64)
	values[TrailerRequestID] = decision.RequestID
	values[TrailerEnvelopeID] = decision.EnvelopeID
	return values
}

func failClosedDecision(requestID, reason string) *governance.GovernanceDecision {
	return &governance.GovernanceDecision{
		RequestID:      requestID,
		Action:         "deny",
		Reason:         reason,
		MatchedRule:    "grpc-fail-closed",
		TrustScore:     0,
		DecisionMode:   governance.DecisionModeDeterministic,
		GatewayVersion: "grpc-interceptor",
	}
}

func responseContent(resp any) string {
	switch v := resp.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func inspectResponseContent(content string) *governance.ResponseInspection {
	lower := strings.ToLower(content)
	inspection := &governance.ResponseInspection{Safe: true}
	concerns := []string{}
	for _, marker := range []string{
		"ignore previous instructions",
		"system prompt",
		"api_key",
		"secret",
		"bearer ",
		"password",
	} {
		if strings.Contains(lower, marker) {
			concerns = append(concerns, "response contains policy-relevant marker: "+marker)
		}
	}
	if len(concerns) > 0 {
		inspection.Safe = false
		inspection.Concerns = concerns
		inspection.InjectionRisk = 0.7
	}
	if strings.Contains(lower, "api_key") || strings.Contains(lower, "secret") || strings.Contains(lower, "bearer ") || strings.Contains(lower, "password") {
		inspection.SensitiveData = true
	}
	return inspection
}
