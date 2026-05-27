package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

var errGRPCEvaluator = errors.New("grpc evaluator failed")

type captureServerTransportStream struct {
	method  string
	headers metadata.MD
	trailer metadata.MD
}

func (s *captureServerTransportStream) Method() string { return s.method }

func (s *captureServerTransportStream) SetHeader(md metadata.MD) error {
	s.headers = metadata.Join(s.headers, md)
	return nil
}

func (s *captureServerTransportStream) SendHeader(md metadata.MD) error {
	s.headers = metadata.Join(s.headers, md)
	return nil
}

func (s *captureServerTransportStream) SetTrailer(md metadata.MD) error {
	s.trailer = metadata.Join(s.trailer, md)
	return nil
}

func newPipeline(t *testing.T, deny bool, denyTool string) *governance.Pipeline {
	t.Helper()
	cfg := governance.PipelineConfig{}
	if deny {
		cfg.StaticPolicies = []governance.StaticPolicyRule{{
			Name:   "deny-test",
			Tool:   denyTool,
			Action: "deny",
			Reason: "blocked by test policy",
		}}
	}
	return governance.NewPipeline(cfg, nil, nil, nil)
}

func TestAdapter_Type(t *testing.T) {
	a := NewAdapter("tenant-default")
	if a.Type() != governance.TransportGRPC {
		t.Fatalf("expected TransportGRPC, got %s", a.Type())
	}
}

func TestAdapter_ParseRequest_FromCallInfo(t *testing.T) {
	a := NewAdapter("tenant-default")
	md := metadata.New(map[string]string{
		DefaultAgentMetadataKey:  "agent-7",
		DefaultTenantMetadataKey: "tenant-7",
		DefaultTraceMetadataKey:  "trace-xyz",
	})
	req, err := a.ParseRequest(context.Background(), &CallInfo{
		Method:   "/svc.Service/DoThing",
		Metadata: md,
	})
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Transport != governance.TransportGRPC {
		t.Errorf("transport = %s, want grpc", req.Transport)
	}
	if req.ToolName != "/svc.Service/DoThing" {
		t.Errorf("tool name = %s", req.ToolName)
	}
	if req.AgentID != "agent-7" || req.TenantID != "tenant-7" || req.TraceID != "trace-xyz" {
		t.Errorf("identity not propagated: %+v", req)
	}
}

func TestAdapter_ParseRequest_TenantFallback(t *testing.T) {
	a := NewAdapter("tenant-default")
	req, err := a.ParseRequest(context.Background(), &CallInfo{
		Method:   "/svc.Service/M",
		Metadata: metadata.MD{}, // empty
	})
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.TenantID != "tenant-default" {
		t.Errorf("expected tenant-default fallback, got %q", req.TenantID)
	}
	if req.AgentID != "" {
		t.Errorf("expected empty agent, got %q", req.AgentID)
	}
}

func TestAdapter_ParseRequest_RejectsUnknownType(t *testing.T) {
	a := NewAdapter("")
	if _, err := a.ParseRequest(context.Background(), "not a CallInfo"); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	if _, err := a.ParseRequest(context.Background(), &CallInfo{}); err == nil {
		t.Fatal("expected error for empty Method")
	}
}

func TestUnaryInterceptor_Allowed(t *testing.T) {
	pipe := newPipeline(t, false, "")
	intercept := UnaryInterceptor(pipe, NewAdapter(""))
	stream := &captureServerTransportStream{method: "/svc.Svc/Allowed"}
	ctx := grpclib.NewContextWithServerTransportStream(context.Background(), stream)

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}
	resp, err := intercept(ctx, nil, &grpclib.UnaryServerInfo{FullMethod: "/svc.Svc/Allowed"}, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("downstream handler was not called")
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if got := firstMetadataValue(stream.trailer, TrailerAction); got != "allow" {
		t.Fatalf("trailer %s = %q, want allow", TrailerAction, got)
	}
	if got := firstMetadataValue(stream.trailer, TrailerRequestID); got == "" {
		t.Fatalf("trailer %s was not set", TrailerRequestID)
	}
	if got := firstMetadataValue(stream.trailer, TrailerEnvelopeID); got == "" {
		t.Fatalf("trailer %s was not set", TrailerEnvelopeID)
	}
	if got := firstMetadataValue(stream.trailer, TrailerSafe); got != "true" {
		t.Fatalf("trailer %s = %q, want true", TrailerSafe, got)
	}
}

func TestUnaryInterceptor_Denied(t *testing.T) {
	pipe := newPipeline(t, true, "/svc.Svc/Forbidden")
	intercept := UnaryInterceptor(pipe, NewAdapter(""))
	stream := &captureServerTransportStream{method: "/svc.Svc/Forbidden"}
	ctx := grpclib.NewContextWithServerTransportStream(context.Background(), stream)

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return nil, nil
	}
	_, err := intercept(ctx, nil, &grpclib.UnaryServerInfo{FullMethod: "/svc.Svc/Forbidden"}, handler)
	if err == nil {
		t.Fatal("expected denial error")
	}
	if called {
		t.Fatal("handler must NOT be called on deny")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T: %v", err, err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %s", st.Code())
	}
	if !strings.Contains(st.Message(), "blocked by test policy") {
		t.Fatalf("expected reason in message, got %q", st.Message())
	}
	if got := firstMetadataValue(stream.trailer, TrailerAction); got != "deny" {
		t.Fatalf("trailer %s = %q, want deny", TrailerAction, got)
	}
	if got := firstMetadataValue(stream.trailer, TrailerRule); got != "deny-test" {
		t.Fatalf("trailer %s = %q, want deny-test", TrailerRule, got)
	}
	if got := firstMetadataValue(stream.trailer, TrailerRequestID); got == "" {
		t.Fatalf("trailer %s was not set", TrailerRequestID)
	}
}

func TestUnaryInterceptor_MetadataExtraction(t *testing.T) {
	pipe := newPipeline(t, false, "")
	intercept := UnaryInterceptor(pipe, NewAdapter(""))

	ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
		DefaultAgentMetadataKey:  "alice",
		DefaultTenantMetadataKey: "acme",
	}))

	// Use a custom interceptor on top to capture the parsed request via a
	// chained handler that inspects the context — but the request itself is
	// only visible inside ParseRequest. Easier: re-call ParseRequest with the
	// same metadata to verify extraction is wired through.
	a := NewAdapter("")
	md, _ := metadata.FromIncomingContext(ctx)
	req, err := a.ParseRequest(ctx, &CallInfo{Method: "/svc/Method", Metadata: md})
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.AgentID != "alice" || req.TenantID != "acme" {
		t.Fatalf("metadata not extracted: %+v", req)
	}

	// And verify the interceptor itself doesn't error when md is present.
	if _, err := intercept(ctx, nil, &grpclib.UnaryServerInfo{FullMethod: "/svc/Method"}, func(ctx context.Context, req any) (any, error) { return nil, nil }); err != nil {
		t.Fatalf("interceptor with metadata failed: %v", err)
	}
}

func TestUnaryInterceptor_NilAdapterDefaults(t *testing.T) {
	pipe := newPipeline(t, false, "")
	// Passing nil adapter must not panic.
	intercept := UnaryInterceptor(pipe, nil)
	if _, err := intercept(context.Background(), nil, &grpclib.UnaryServerInfo{FullMethod: "/svc/M"}, func(ctx context.Context, req any) (any, error) { return "ok", nil }); err != nil {
		t.Fatalf("nil adapter path failed: %v", err)
	}
}

func TestUnaryInterceptor_ResponseInspectionTrailer(t *testing.T) {
	pipe := newPipeline(t, false, "")
	intercept := UnaryInterceptor(pipe, NewAdapter(""))
	stream := &captureServerTransportStream{method: "/svc.Svc/Leaks"}
	ctx := grpclib.NewContextWithServerTransportStream(context.Background(), stream)

	resp, err := intercept(ctx, nil, &grpclib.UnaryServerInfo{FullMethod: "/svc.Svc/Leaks"}, func(ctx context.Context, req any) (any, error) {
		return "api_key=abc123", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "api_key=abc123" {
		t.Fatalf("unexpected response: %v", resp)
	}
	if got := firstMetadataValue(stream.trailer, TrailerSafe); got != "false" {
		t.Fatalf("trailer %s = %q, want false", TrailerSafe, got)
	}
	if got := firstMetadataValue(stream.trailer, TrailerConcerns); !strings.Contains(got, "api_key") {
		t.Fatalf("trailer %s = %q, want api_key concern", TrailerConcerns, got)
	}
}

func TestUnaryInterceptor_PolicyEvaluatorErrorFailsClosed(t *testing.T) {
	pipe := governance.NewPipeline(governance.PipelineConfig{
		FailClosedTransports: []governance.TransportType{governance.TransportGRPC},
	}, nil, grpcErrorEvaluator{}, nil)
	intercept := UnaryInterceptor(pipe, NewAdapter(""))
	stream := &captureServerTransportStream{method: "/svc.Svc/Error"}
	ctx := grpclib.NewContextWithServerTransportStream(context.Background(), stream)

	called := false
	_, err := intercept(ctx, nil, &grpclib.UnaryServerInfo{FullMethod: "/svc.Svc/Error"}, func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	})
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
	if called {
		t.Fatal("handler must not be called on pipeline error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied status, got %v", err)
	}
	if !strings.Contains(st.Message(), "fail-closed") {
		t.Fatalf("expected fail-closed reason, got %q", st.Message())
	}
	if got := firstMetadataValue(stream.trailer, TrailerAction); got != "deny" {
		t.Fatalf("trailer %s = %q, want deny", TrailerAction, got)
	}
}

func TestAdapter_EmitGovernanceMetadata(t *testing.T) {
	a := NewAdapter("")
	resp := &governance.ToolResponse{}
	decision := &governance.GovernanceDecision{
		RequestID:    "req-1",
		Action:       "allow",
		MatchedRule:  "rule-1",
		TrustScore:   0.75,
		EnvelopeID:   "env-1",
		DecisionMode: governance.DecisionModeDeterministic,
	}
	if err := a.EmitGovernanceMetadata(context.Background(), resp, decision); err != nil {
		t.Fatalf("EmitGovernanceMetadata: %v", err)
	}
	if resp.Metadata[TrailerAction] != "allow" || resp.Metadata[TrailerRule] != "rule-1" {
		t.Fatalf("metadata not attached: %+v", resp.Metadata)
	}
	if resp.Metadata[TrailerTrust] != "0.75" {
		t.Fatalf("trust metadata = %q, want 0.75", resp.Metadata[TrailerTrust])
	}
}

type grpcErrorEvaluator struct{}

func (grpcErrorEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return nil, errGRPCEvaluator
}
