package madriver

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// TestLogOutputPseudonymizesSessionID proves no line written to the run log
// carries the raw upstream session id — every mention must appear as the
// same "sess-…" pseudonym the transcript uses. Covers both the per-run
// summary line and the interrupt-failure warning (where the raw id can also
// hide inside the upstream error text).
func TestLogOutputPseudonymizesSessionID(t *testing.T) {
	t.Run("summary line", func(t *testing.T) {
		up := &fakeUpstream{events: []managedagents.Event{
			{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
			{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001",
				Data: map[string]any{"usage": map[string]any{"list_cost": "5"}}},
			{ID: "e3", Type: managedagents.EventStatusIdle, SessionID: "sess_fake_0001"},
		}}
		var log bytes.Buffer
		if _, err := Run(context.Background(), liveTestConfig(t), liveDeps(up), &log); err != nil {
			t.Fatalf("run: %v", err)
		}
		assertSessionIDScrubbed(t, log.String())
	})

	t.Run("interrupt warning", func(t *testing.T) {
		up := &fakeUpstream{
			stopErr: errors.New("upstream rejected stop for sess_fake_0001"),
			events: []managedagents.Event{
				{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
				{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001",
					Data: map[string]any{"usage": map[string]any{"list_cost": "500"}}},
			},
		}
		var log bytes.Buffer
		// The run fails (spend abort + rejected interrupt); only the log
		// matters here.
		_, _ = Run(context.Background(), liveTestConfig(t), liveDeps(up), &log)
		assertSessionIDScrubbed(t, log.String())
	})
}

func assertSessionIDScrubbed(t *testing.T, log string) {
	t.Helper()
	if strings.Contains(log, "sess_fake_0001") {
		t.Fatalf("raw session id leaked into run log:\n%s", log)
	}
	if !strings.Contains(log, pseudonym("sess", "sess_fake_0001")) {
		t.Fatalf("log must carry the session pseudonym, got:\n%s", log)
	}
	if LooksSecret(log) {
		t.Fatalf("log carries secret-shaped content:\n%s", log)
	}
}

// TestLogOutputRedactsSecretShapes proves the log writer runs the same
// redactor as the transcript: an upstream error carrying a secret-shaped
// token must not reach the log verbatim.
func TestLogOutputRedactsSecretShapes(t *testing.T) {
	up := &fakeUpstream{
		streamErr: errors.New("stream failed with token sk-ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE"),
		events:    nil,
	}
	var log bytes.Buffer
	_, _ = Run(context.Background(), liveTestConfig(t), liveDeps(up), &log)
	if strings.Contains(log.String(), "sk-ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE") {
		t.Fatalf("secret-shaped token leaked into run log:\n%s", log.String())
	}
}
