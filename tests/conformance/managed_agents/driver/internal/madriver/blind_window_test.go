package madriver

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// blockingSource never yields an event: it waits on the context exactly like
// a silent SSE stream whose bytes stop arriving.
type blockingSource struct{}

func (blockingSource) Next(ctx context.Context) (managedagents.Event, error) {
	<-ctx.Done()
	return managedagents.Event{}, ctx.Err()
}

// TestGuardedSourceTripsBlindLimitOnSilentStream proves the usage-blind
// window is a time limit, not only an event limit: a stream that produces no
// events at all must trip ErrSpendUnknown at --usage-blind-window rather than
// hanging until the transport's (longer) idle timeout.
func TestGuardedSourceTripsBlindLimitOnSilentStream(t *testing.T) {
	guard := newSpendGuard(5.00, 1024, 100, 40*time.Millisecond)
	src := &guardedSource{inner: blockingSource{}, guard: guard, maxTurns: 10}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()
	_, err := src.Next(ctx)
	if !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("a silent stream must trip the blind limit, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("blind limit took %s, want ~40ms", elapsed)
	}
}

// TestSpendGuardBlindDeadlineUsesInjectedClock covers the time check itself:
// the blind deadline anchors at the first observed stream activity (or last
// usage signal) plus the window, evaluated on the guard's clock.
func TestSpendGuardBlindDeadlineUsesInjectedClock(t *testing.T) {
	start := time.Now()
	now := start
	g := newSpendGuard(5.00, 1024, 100, time.Minute)
	g.now = func() time.Time { return now }
	if g.blindExceeded() {
		t.Fatal("an unarmed guard has no blind deadline")
	}
	g.arm()
	if g.blindExceeded() {
		t.Fatal("just-armed guard must not be blind yet")
	}
	now = start.Add(61 * time.Second)
	if !g.blindExceeded() {
		t.Fatal("61s after stream start with no usage signal must exceed the 60s window")
	}
	// A usage signal re-anchors the window on the signal's own timestamp.
	now = start.Add(2 * time.Hour)
	if err := g.observe(managedagents.Event{
		Type: "session.usage",
		Data: map[string]any{"usage": map[string]any{"list_cost": "10"}},
	}); err != nil {
		t.Fatalf("usage event: %v", err)
	}
	if g.blindExceeded() {
		t.Fatal("the blind window must re-anchor on the latest usage signal")
	}
	now = now.Add(61 * time.Second)
	if !g.blindExceeded() {
		t.Fatal("61s after the last usage signal must exceed the window")
	}
}

// TestSilentStreamTripsBlindLimitInRun covers the whole run path: the session
// is interrupted and the run reports stopped_spend_unknown instead of waiting
// out the stream idle timeout.
func TestSilentStreamTripsBlindLimitInRun(t *testing.T) {
	up := &fakeUpstream{block: true}
	cfg := liveTestConfig(t)
	cfg.UsageBlindWindow = 40 * time.Millisecond
	cfg.Timeout = 5 * time.Second

	start := time.Now()
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("silent stream must fail closed with ErrSpendUnknown, got %v", err)
	}
	if res.StopReason != StopSpendUnknown {
		t.Fatalf("stop=%q, want %q", res.StopReason, StopSpendUnknown)
	}
	if got := up.StopCalls(); got != 1 {
		t.Fatalf("blind-limit stop must interrupt the session exactly once, got %d", got)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("run took %s on a 40ms blind window", elapsed)
	}
}
