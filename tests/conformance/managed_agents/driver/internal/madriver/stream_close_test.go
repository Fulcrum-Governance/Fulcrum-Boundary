package madriver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// openTestStream runs a real httptest SSE exchange: the handler answers
// session creation, then streams body and returns (closing the response).
func openTestStream(t *testing.T, body string) managedagents.EventSource {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events/stream") {
			w.Header().Set("content-type", "application/json")
			fmt.Fprint(w, `{"id":"sess_stream","agent":"agt"}`)
			return
		}
		w.Header().Set("content-type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}))
	t.Cleanup(srv.Close)

	up := newLiveUpstream(srv.URL, "fake")
	t.Cleanup(func() { _ = up.Close() })
	sess, err := up.CreateSession(context.Background(), CreateSessionParams{AgentID: "a", EnvironmentID: "e", BudgetUSD: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stream, err := up.OpenEventStream(context.Background(), sess.ID)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	return stream
}

// TestSSESourceAbruptCloseIsAnError is the core defect: a stream that ends
// without the upstream signalling session completion is an unexpected close
// and must surface an error — not io.EOF, which SessionProxy treats as a
// clean finish that skips the session interrupt.
func TestSSESourceAbruptCloseIsAnError(t *testing.T) {
	stream := openTestStream(t, "data: {\"type\":\"session.status_running\"}\n\n")
	event, err := stream.Next(context.Background())
	if err != nil || event.Type != "session.status_running" {
		t.Fatalf("first event: type=%q err=%v", event.Type, err)
	}
	_, err = stream.Next(context.Background())
	if err == nil {
		t.Fatal("a stream closing without a completion signal must produce an error")
	}
	if errors.Is(err, io.EOF) {
		t.Fatal("abrupt close must not surface as io.EOF (that reads as a clean finish)")
	}
	if !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("abrupt close error = %v, want ErrStreamClosed", err)
	}
}

// TestSSESourceDoneIsCleanEOF guards the clean path: the [DONE] sentinel ends
// the stream as io.EOF exactly as before.
func TestSSESourceDoneIsCleanEOF(t *testing.T) {
	stream := openTestStream(t, "data: {\"type\":\"session.status_running\"}\n\ndata: [DONE]\n\n")
	if _, err := stream.Next(context.Background()); err != nil {
		t.Fatalf("first event: %v", err)
	}
	if _, err := stream.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("[DONE] must end the stream as io.EOF, got %v", err)
	}
}

// TestSSESourceTerminalIdleThenCloseIsClean covers upstreams that end the
// session with a terminal idle status and then close without a sentinel: the
// completion event was seen, so the close is clean.
func TestSSESourceTerminalIdleThenCloseIsClean(t *testing.T) {
	stream := openTestStream(t, "data: {\"type\":\"session.status_running\"}\n\ndata: {\"type\":\"session.status_idle\",\"stop_reason\":{\"type\":\"end_turn\"}}\n\n")
	for i, want := range []string{"session.status_running", "session.status_idle"} {
		event, err := stream.Next(context.Background())
		if err != nil || event.Type != want {
			t.Fatalf("event %d: type=%q err=%v, want %q", i, event.Type, err, want)
		}
	}
	if _, err := stream.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("close after a terminal idle event must be a clean io.EOF, got %v", err)
	}
}

// TestSSESourceRequiresActionIdleThenCloseIsAnError: an idle stop_reason of
// requires_action means the session is parked awaiting confirmations — the
// session is still live, so a close there is unexpected.
func TestSSESourceRequiresActionIdleThenCloseIsAnError(t *testing.T) {
	stream := openTestStream(t, "data: {\"type\":\"session.status_idle\",\"stop_reason\":{\"type\":\"requires_action\",\"event_ids\":[\"e1\"]}}\n\n")
	if _, err := stream.Next(context.Background()); err != nil {
		t.Fatalf("idle event: %v", err)
	}
	if _, err := stream.Next(context.Background()); !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("close while requires_action is pending = %v, want ErrStreamClosed", err)
	}
}
