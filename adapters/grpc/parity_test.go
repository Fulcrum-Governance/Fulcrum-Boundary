package grpc

import (
	"context"
	"testing"

	grpclib "google.golang.org/grpc"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
)

// TestAdapterParity runs the shared ADR-047 table (internal/adapterparity)
// through the unary interceptor. The other adapters run it in
// tests/adapter_conformance.
func TestAdapterParity(t *testing.T) {
	const method = "/parity.Service/Call"
	adapterparity.Run(t, adapterparity.Adapter{Name: "grpc", Tool: method, Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, in adapterparity.Input) adapterparity.Outcome {
		info := &grpclib.UnaryServerInfo{FullMethod: method}
		if in == adapterparity.Malformed {
			info.FullMethod = ""
		}
		stream := &captureServerTransportStream{method: info.FullMethod}
		ctx = grpclib.NewContextWithServerTransportStream(ctx, stream)
		calls := 0
		_, _ = UnaryInterceptor(p, NewAdapter("tenant-parity"))(ctx, nil, info, func(context.Context, any) (any, error) {
			calls++
			return "ok", nil
		})
		return adapterparity.Outcome{Action: firstMetadataValue(stream.trailer, TrailerAction), Executed: calls}
	}})
}
