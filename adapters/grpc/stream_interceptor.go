package grpc

import (

	"fmt"
	"sync"


	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type wrappedServerStream struct {
	grpclib.ServerStream
	info     *grpclib.StreamServerInfo
	pipeline *governance.Pipeline
	adapter  *Adapter

	mu    sync.Mutex
	fatal error
}

func (w *wrappedServerStream) RecvMsg(m any) error {
	w.mu.Lock()
	if w.fatal != nil {
		w.mu.Unlock()
		return w.fatal
	}
	w.mu.Unlock()

	if err := w.ServerStream.RecvMsg(m); err != nil {
		return err
	}

	ctx := w.Context()
	md, _ := metadata.FromIncomingContext(ctx)

	req, err := w.adapter.ParseRequest(ctx, &CallInfo{
		Method:   w.info.FullMethod,
		Metadata: md,
		Action:   "grpc/stream-recv",
	})
	if err != nil {
		decision := failClosedDecision("", fmt.Sprintf("parse request: %v", err))
		_ = emitDecisionTrailer(ctx, decision)
		w.mu.Lock()
		w.fatal = status.Errorf(codes.InvalidArgument, "governance: %s", decision.Reason)
		w.mu.Unlock()
		return w.fatal
	}
	ensureRequestIdentity(req)

	if w.pipeline == nil {
		decision := failClosedDecision(req.RequestID, "pipeline is required")
		decision.EnvelopeID = req.EnvelopeID
		_ = emitDecisionTrailer(ctx, decision)
		w.mu.Lock()
		w.fatal = status.Errorf(codes.PermissionDenied, "governance: %s", decision.Reason)
		w.mu.Unlock()
		return w.fatal
	}

	decision, err := w.pipeline.Evaluate(ctx, req)
	if err != nil {
		decision = failClosedDecision(req.RequestID, fmt.Sprintf("evaluate: %v", err))
		decision.EnvelopeID = req.EnvelopeID
		_ = emitDecisionTrailer(ctx, decision)
		// TODO: FUL-464 use CHECK_INDETERMINATE compatible mapping when available
		w.mu.Lock()
		w.fatal = status.Errorf(codes.PermissionDenied, "governance: %s", decision.Reason)
		w.mu.Unlock()
		return w.fatal
	}

	if !decision.Allowed() {
		_ = emitDecisionTrailer(ctx, decision)
		reason := decision.Reason
		if reason == "" {
			reason = decision.Action
		}
		w.mu.Lock()
		w.fatal = status.Errorf(codes.PermissionDenied, "governance: %s", reason)
		w.mu.Unlock()
		return w.fatal
	}

	// For stream-recv, we don't currently have a standard way to inspect the incoming message
	// easily, but we emit trailer for allow for traceability if needed.
	// However, gRPC trailers are only sent once at the end of the stream.
	// We update the trailer but there's a risk it gets overwritten.
	// Emitting it for each allowed message might overwrite previous ones.

	// Ensure we only emit decision trailers on error, or at stream end if we wanted to.
	return nil
}

// StreamInterceptor returns a grpc.StreamServerInterceptor that evaluates each
// incoming message through the pipeline before delivering it to the handler.
// Denied messages terminate the stream with codes.PermissionDenied.
// Governance trailers are emitted on termination.
func StreamInterceptor(pipeline *governance.Pipeline, adapter *Adapter) grpclib.StreamServerInterceptor {
	if adapter == nil {
		adapter = &Adapter{}
	}
	return func(srv any, ss grpclib.ServerStream, info *grpclib.StreamServerInfo, handler grpclib.StreamHandler) error {
		if info == nil || info.FullMethod == "" {
			decision := failClosedDecision("", "gRPC full method is required")
			_ = emitDecisionTrailer(ss.Context(), decision)
			return status.Errorf(codes.InvalidArgument, "governance: %s", decision.Reason)
		}
		ws := &wrappedServerStream{
			ServerStream: ss,
			info:         info,
			pipeline:     pipeline,
			adapter:      adapter,
		}
		return handler(srv, ws)
	}
}
