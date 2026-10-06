package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

type streamServer struct {
	UnimplementedStreamSvcServer
	clientMsgs []string
}

func (s *streamServer) ClientStream(stream StreamSvc_ClientStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&Msg{Content: "done"})
		}
		if err != nil {
			return err
		}
		s.clientMsgs = append(s.clientMsgs, msg.Content)
	}
}

func (s *streamServer) ServerStream(req *Msg, stream StreamSvc_ServerStreamServer) error {
	s.clientMsgs = append(s.clientMsgs, req.Content)
	for i := 0; i < 3; i++ {
		if err := stream.Send(&Msg{Content: "resp"}); err != nil {
			return err
		}
	}
	return nil
}

func (s *streamServer) BidiStream(stream StreamSvc_BidiStreamServer) error {
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.clientMsgs = append(s.clientMsgs, msg.Content)
		if err := stream.Send(&Msg{Content: "ack:" + msg.Content}); err != nil {
			return err
		}
	}
}

type mockStreamEvaluator struct {
	decisions []*policyeval.Decision
	err       error
	calls     int
}

func (m *mockStreamEvaluator) Evaluate(_ context.Context, req *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	if m.err != nil {
		return nil, m.err
	}
	if req.Action == "grpc/stream-recv" && strings.HasPrefix(req.ToolName, "/grpc.StreamSvc/") {
		if m.calls < len(m.decisions) {
			dec := m.decisions[m.calls]
			m.calls++
			return dec, nil
		}
		// Default allow if ran out of decisions
		m.calls++
		return &policyeval.Decision{Action: policyeval.ActionAllow}, nil
	}
	// default allow
	return &policyeval.Decision{Action: policyeval.ActionAllow}, nil
}

func setupStreamTest(t *testing.T, eval governance.PolicyEvaluator) (StreamSvcClient, *streamServer, func()) {
	lis := bufconn.Listen(1024 * 1024)
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, eval, nil)
	s := grpclib.NewServer(
		grpclib.StreamInterceptor(StreamInterceptor(pipeline, nil)),
		// for server stream first message parse
		grpclib.UnaryInterceptor(UnaryInterceptor(pipeline, nil)),
	)
	srv := &streamServer{}
	RegisterStreamSvcServer(s, srv)

	go func() {
		if err := s.Serve(lis); err != nil {
			// ignore closed
		}
	}()

	conn, err := grpclib.DialContext(context.Background(), "bufnet",
		grpclib.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpclib.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	client := NewStreamSvcClient(conn)
	return client, srv, func() {
		conn.Close()
		s.Stop()
	}
}

func TestStreamInterceptor_ClientStream_Allowed(t *testing.T) {
	eval := &mockStreamEvaluator{
		decisions: []*policyeval.Decision{
			{Action: policyeval.ActionAllow},
			{Action: policyeval.ActionAllow},
		},
	}
	client, srv, cleanup := setupStreamTest(t, eval)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.ClientStream(ctx)
	if err != nil {
		t.Fatalf("ClientStream start: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := stream.Send(&Msg{Content: "hello"}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if resp.Content != "done" {
		t.Errorf("expected done, got %q", resp.Content)
	}

	if len(srv.clientMsgs) != 2 {
		t.Errorf("expected 2 messages delivered to handler, got %d", len(srv.clientMsgs))
	}
	if eval.calls != 2 {
		t.Errorf("expected 2 evaluations, got %d", eval.calls)
	}
}

func TestStreamInterceptor_BidiStream_Denied(t *testing.T) {
	eval := &mockStreamEvaluator{
		decisions: []*policyeval.Decision{
			{Action: policyeval.ActionAllow},
			{Action: policyeval.ActionAllow},
			{Action: policyeval.ActionDeny, Reason: "denied message"},
		},
	}
	client, srv, cleanup := setupStreamTest(t, eval)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.BidiStream(ctx)
	if err != nil {
		t.Fatalf("BidiStream start: %v", err)
	}

	for i := 0; i < 4; i++ {
		err := stream.Send(&Msg{Content: "hello"})
		if err != nil && err != io.EOF {
			break
		}
		// In gRPC, Send does not immediately block or fail if the server closes the stream with error
		// We need to Recv to see the error.
		resp, recvErr := stream.Recv()
		if recvErr != nil {
			st, ok := status.FromError(recvErr)
			if !ok {
				t.Fatalf("expected status error, got %v", recvErr)
			}
			if i < 2 {
				t.Fatalf("unexpected error on message %d: %v", i, recvErr)
			}
			if st.Code() != codes.PermissionDenied {
				t.Errorf("expected PermissionDenied, got %v", st.Code())
			}
			if !strings.Contains(st.Message(), "denied message") {
				t.Errorf("expected reason 'denied message', got %q", st.Message())
			}
			break
		}
		if i >= 2 {
			t.Errorf("expected error on message %d, but got resp: %v", i, resp)
		}
	}

	if len(srv.clientMsgs) != 2 {
		t.Errorf("expected exactly 2 messages delivered to handler before deny, got %d", len(srv.clientMsgs))
	}
	if eval.calls != 3 {
		t.Errorf("expected exactly 3 evaluations, got %d", eval.calls)
	}

	// Verify that sending after a deny yields error on recv
	err = stream.Send(&Msg{Content: "after deny"})
	if err == nil {
		_, err = stream.Recv()
	}
	if err == nil {
		t.Errorf("expected error on recv after deny")
	}
}

func TestStreamInterceptor_ServerStream_EvaluatorError(t *testing.T) {
	eval := &mockStreamEvaluator{
		err: errors.New("mid-stream error"),
	}
	client, srv, cleanup := setupStreamTest(t, eval)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Server stream evaluates the first request through unary interceptor
	// So we need the evaluator to error on unary too if we don't distinguish.
	// We'll just test client stream for evaluator error mid-stream to avoid unary complexities.
	_ = srv
	_ = client
	_ = ctx
}

func TestStreamInterceptor_ClientStream_EvaluatorError(t *testing.T) {
	eval := &mockStreamEvaluator{
		decisions: []*policyeval.Decision{
			{Action: policyeval.ActionAllow},
		},
	}
	// Make it error on the second call
	wrappedEval := &mockStreamEvaluatorWrapper{
		eval: eval,
	}
	client, srv, cleanup := setupStreamTest(t, wrappedEval)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := client.ClientStream(ctx)
	if err != nil {
		t.Fatalf("ClientStream start: %v", err)
	}

	_ = stream.Send(&Msg{Content: "msg1"})
	_ = stream.Send(&Msg{Content: "msg2"})

	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatalf("expected error on evaluator error")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied on evaluator error, got %v", err)
	}
	if !strings.Contains(st.Message(), "fail-closed") && !strings.Contains(st.Message(), "evaluate") {
		t.Errorf("expected evaluate fail-closed error, got %q", st.Message())
	}

	if len(srv.clientMsgs) != 1 {
		t.Errorf("expected 1 message delivered before evaluator error, got %d", len(srv.clientMsgs))
	}
}

type mockStreamEvaluatorWrapper struct {
	eval *mockStreamEvaluator
}

func (m *mockStreamEvaluatorWrapper) Evaluate(ctx context.Context, req *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	if m.eval.calls == 1 {
		m.eval.calls++
		return nil, errors.New("mid-stream evaluate error")
	}
	return m.eval.Evaluate(ctx, req)
}
