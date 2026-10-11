package madriver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// Anthropic's Managed Agents docs (platform.claude.com/docs/en/managed-agents/
// workflow-runs) say a session.status_idle can carry stop reasons beyond
// end_turn and requires_action (the list is not closed), and that a
// retries_exhausted idle can occur even when the server is about to start a
// new turn by itself. Treating any non-requires_action idle as a completed
// session would skip the interrupt while a paid session keeps running.

var uncleanIdleReasons = []string{"retries_exhausted", "budget_reached", "refusal", "brand_new_reason_xyz"}

func idleBody(reason string) string {
	return fmt.Sprintf("data: {\"type\":\"session.status_idle\",\"stop_reason\":{\"type\":%q}}\n\n", reason)
}

// newIdleServer serves one SSE stream body and counts user.interrupt events.
func newIdleServer(t *testing.T, streamBody string) (*httptest.Server, *int32) {
	t.Helper()
	var interrupts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/events/stream"):
			w.Header().Set("content-type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, streamBody)
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/events"):
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "user.interrupt") {
				atomic.AddInt32(&interrupts, 1)
			}
			w.Header().Set("content-type", "application/json")
			fmt.Fprint(w, `{}`)
		default:
			w.Header().Set("content-type", "application/json")
			fmt.Fprint(w, `{"id":"sess_idle","agent":"agt"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &interrupts
}

// TestSSESourceUncleanIdleReasonsAreErrors: only end_turn is a clean end. Any
// other idle stop reason (retries_exhausted, budget_reached, refusal, an
// unknown string) must surface as an error naming the reason, never as a
// delivered event that later closes as a clean io.EOF.
func TestSSESourceUncleanIdleReasonsAreErrors(t *testing.T) {
	for _, reason := range uncleanIdleReasons {
		t.Run(reason, func(t *testing.T) {
			stream := openTestStream(t, idleBody(reason))
			_, err := stream.Next(context.Background())
			if err == nil {
				t.Fatalf("idle with stop_reason %q must be an unclean close, got a normal event", reason)
			}
			if errors.Is(err, io.EOF) {
				t.Fatalf("idle with stop_reason %q must not read as a clean finish (io.EOF)", reason)
			}
			if !strings.Contains(err.Error(), reason) {
				t.Fatalf("error %q must record the stop reason %q", err, reason)
			}
		})
	}
}

// TestSSESourceIdleWithoutStopReasonIsUnclean: an idle with no stop_reason
// cannot be shown to be a natural finish, so it fails closed.
func TestSSESourceIdleWithoutStopReasonIsUnclean(t *testing.T) {
	stream := openTestStream(t, "data: {\"type\":\"session.status_idle\"}\n\n")
	_, err := stream.Next(context.Background())
	if err == nil {
		t.Fatal("an idle event with no stop_reason must be an unclean close")
	}
	if errors.Is(err, io.EOF) {
		t.Fatal("an idle event with no stop_reason must not read as a clean finish")
	}
}

// TestRunInterruptsSessionOnUncleanIdle ties the fix to a real run over the
// live SSE source: an unclean idle interrupts the session exactly once, fails
// the run, and records the reason instead of reporting completed.
func TestRunInterruptsSessionOnUncleanIdle(t *testing.T) {
	for _, reason := range uncleanIdleReasons {
		t.Run(reason, func(t *testing.T) {
			srv, interrupts := newIdleServer(t, idleBody(reason))
			deps := liveDeps(newLiveUpstream(srv.URL, "fake"))
			res, err := Run(context.Background(), liveTestConfig(t), deps, io.Discard)
			if err == nil {
				t.Fatalf("idle with stop_reason %q must fail the run (stop=%q)", reason, res.StopReason)
			}
			if got := atomic.LoadInt32(interrupts); got != 1 {
				t.Fatalf("unclean idle %q must interrupt the session exactly once, got %d", reason, got)
			}
			if res.StopReason == StopCompleted {
				t.Fatalf("unclean idle %q must not be reported as completed", reason)
			}
			if !strings.Contains(res.StopReason, reason) {
				t.Fatalf("stop reason %q must record the idle reason %q", res.StopReason, reason)
			}
		})
	}
}

// TestInterruptInducedEndTurnIsNotNaturalCompletion: an interrupt also produces
// an end_turn idle, indistinguishable by value from a natural finish. Once the
// driver has sent its own interrupt, an end_turn must not count as the
// session's natural completion, so a following close is unclean.
func TestInterruptInducedEndTurnIsNotNaturalCompletion(t *testing.T) {
	srv, interrupts := newIdleServer(t, idleBody("end_turn"))
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
	if err := up.Stop(context.Background(), sess.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if atomic.LoadInt32(interrupts) != 1 {
		t.Fatalf("the test needs the driver's own interrupt on the wire, got %d", atomic.LoadInt32(interrupts))
	}
	event, err := stream.Next(context.Background())
	if err != nil || event.Type != managedagents.EventStatusIdle {
		t.Fatalf("the end_turn idle may still be delivered: type=%q err=%v", event.Type, err)
	}
	if _, err := stream.Next(context.Background()); errors.Is(err, io.EOF) || err == nil {
		t.Fatalf("an end_turn after the driver's own interrupt must not count as natural completion; close = %v, want an unclean error", err)
	}
}
