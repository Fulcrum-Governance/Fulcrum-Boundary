package madriver

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// TestStopFailurePropagatesOnAbortPaths proves a rejected upstream interrupt
// cannot masquerade as a clean abort: the run must fail and say the paid
// session may still be running, on both the deliberate-abort paths and the
// error paths.
func TestStopFailurePropagatesOnAbortPaths(t *testing.T) {
	usage := func() managedagents.Event {
		return managedagents.Event{Type: "session.usage", SessionID: "sess_fake_0001",
			Data: map[string]any{"usage": map[string]any{"list_cost": "80"}}}
	}
	stopFailure := errors.New("upstream rejected interrupt")

	t.Run("clean spend abort with failed interrupt", func(t *testing.T) {
		up := &fakeUpstream{stopErr: stopFailure, events: []managedagents.Event{usage()}}
		cfg := liveTestConfig(t)
		cfg.MaxSpendUSD = 0.10 // 80% of ceiling < the $0.80 snapshot
		res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
		if err == nil {
			t.Fatal("a failed upstream stop must not report a successful abort")
		}
		if !strings.Contains(err.Error(), "may still be running") {
			t.Fatalf("run error must say the session may still be running, got: %v", err)
		}
		if got := up.StopCalls(); got != 1 {
			t.Fatalf("interrupt attempted %d times, want 1", got)
		}
		if res.StopReason != StopSpendAbort {
			t.Fatalf("stop=%q, want %q recorded even when the interrupt fails", res.StopReason, StopSpendAbort)
		}
	})

	t.Run("stream error with failed interrupt", func(t *testing.T) {
		up := &fakeUpstream{streamErr: errors.New("upstream went away"), stopErr: stopFailure}
		cfg := liveTestConfig(t)
		_, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
		if err == nil {
			t.Fatal("run must fail")
		}
		if !strings.Contains(err.Error(), "upstream went away") {
			t.Fatalf("run error must keep the stream error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "may still be running") {
			t.Fatalf("run error must report the failed interrupt, got: %v", err)
		}
	})
}
