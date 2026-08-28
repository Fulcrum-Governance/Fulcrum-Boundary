package hookboundary

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

// sessionNow is a fixed clock so the summary timestamp is assertable.
var sessionNow = time.Date(2026, 4, 2, 9, 30, 0, 0, time.UTC)

func sessionConfig(dir string) Config {
	return Config{Dir: dir, Now: func() time.Time { return sessionNow }}
}

// runSessionEnd feeds one SessionEnd event and returns the result.
func runSessionEnd(t *testing.T, dir, event string) SessionEndResult {
	t.Helper()
	return SessionEnd(sessionConfig(dir), strings.NewReader(event))
}

// readSummaries decodes every line of the session summary log.
func readSummaries(t *testing.T, dir string) []SessionSummary {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, SessionSummaryLogName))
	if err != nil {
		t.Fatalf("read summary log: %v", err)
	}
	var summaries []SessionSummary
	for i, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		var summary SessionSummary
		if err := json.Unmarshal([]byte(line), &summary); err != nil {
			t.Fatalf("summary line %d is not JSON: %v", i, err)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func seedSessionLog(t *testing.T, dir string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, DecisionLogName), []byte(body), 0o600); err != nil {
		t.Fatalf("write decision log: %v", err)
	}
}

func TestSessionEndCountsOnlyThisSessionsDecisions(t *testing.T) {
	dir := t.TempDir()
	seedSessionLog(t, dir,
		recordLine(t, "sess-a#1", "deny", sessionNow),
		recordLine(t, "sess-a#2", "require_approval", sessionNow),
		recordLine(t, "sess-a#3", "warn", sessionNow),
		recordLine(t, "sess-a#4", "allow", sessionNow),
		recordLine(t, "sess-a#5", "allow", sessionNow),
		recordLine(t, "sess-b#1", "deny", sessionNow),
	)

	result := runSessionEnd(t, dir, `{"hook_event_name":"SessionEnd","session_id":"sess-a","reason":"clear"}`)
	if !result.Written {
		t.Fatalf("no summary written: %v", result.Fault)
	}
	summaries := readSummaries(t, dir)
	if len(summaries) != 1 {
		t.Fatalf("summaries = %d, want 1", len(summaries))
	}
	got := summaries[0]
	if got.Decisions != 5 {
		t.Fatalf("decisions = %d, want 5 (the other session must not be counted)", got.Decisions)
	}
	if got.Denied != 1 || got.Asked != 1 || got.Warned != 1 || got.Allowed != 2 {
		t.Fatalf("counts = denied %d asked %d warned %d allowed %d, want 1/1/1/2",
			got.Denied, got.Asked, got.Warned, got.Allowed)
	}
	if got.SessionID != "sess-a" || got.TracePrefix != "sess-a#" {
		t.Fatalf("session = %q prefix = %q", got.SessionID, got.TracePrefix)
	}
	if got.ReceiptHint != ReceiptHint {
		t.Fatalf("receipt_hint = %q, want %q", got.ReceiptHint, ReceiptHint)
	}
	if !got.UTC.Equal(sessionNow) {
		t.Fatalf("utc = %s, want %s", got.UTC, sessionNow)
	}
	if got.Reason != "clear" {
		t.Fatalf("reason = %q", got.Reason)
	}
}

// ReceiptHint pointed at .boundary/receipts/ — a directory Boundary itself
// denies every write to, so the one machine-readable pointer in the summary log
// named a path that is empty by construction. Comparing it to the constant
// cannot catch that; comparing it to the classifier can.
func TestReceiptHintIsAPathAWriteCanReach(t *testing.T) {
	if reason, denied := editboundary.ControlSurfacePath(ReceiptHint + "2026-01-01-session-receipt.md"); denied {
		t.Fatalf("ReceiptHint %q points inside a governance control surface (%s); a receipt can never be written there",
			ReceiptHint, reason)
	}
	if strings.HasPrefix(ReceiptHint, ".boundary/") {
		t.Fatalf("ReceiptHint = %q, which is under the .boundary evidence tree rather than beside it", ReceiptHint)
	}
}

// A prefix match must not leak between sessions whose ids share a leading
// substring. The "#" separator is what makes the join exact.
func TestSessionEndDoesNotMatchASessionIDPrefix(t *testing.T) {
	dir := t.TempDir()
	seedSessionLog(t, dir,
		recordLine(t, "sess#1", "deny", sessionNow),
		recordLine(t, "sess-longer#1", "deny", sessionNow),
	)

	runSessionEnd(t, dir, `{"session_id":"sess"}`)
	got := readSummaries(t, dir)[0]
	if got.Decisions != 1 {
		t.Fatalf("decisions = %d, want 1; a longer session id was folded in", got.Decisions)
	}
}

// The end-to-end join: a real decision written by Decide must be countable by
// SessionEnd using the same raw session id the event carried.
func TestSessionEndCountsRecordsWrittenByDecide(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, ProjectRoot: t.TempDir()}
	for _, event := range []string{
		`{"session_id":"live/session:1","tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`,
		`{"session_id":"live/session:1","tool_name":"Bash","tool_input":{"command":"git status"}}`,
	} {
		if result := Decide(cfg, strings.NewReader(event)); result.Record == nil {
			t.Fatalf("event %s wrote no record: %+v", event, result)
		}
	}

	result := runSessionEnd(t, dir, `{"session_id":"live/session:1"}`)
	if !result.Written {
		t.Fatalf("no summary written: %v", result.Fault)
	}
	got := readSummaries(t, dir)[0]
	if got.Decisions != 2 || got.Denied != 1 || got.Allowed != 1 {
		t.Fatalf("counts = %+v, want 2 decisions (1 deny, 1 allow)", got)
	}
}

// A session id with characters the record layer sanitizes must still join: the
// summary matches on the sanitized label, exactly as TraceID composes it.
func TestSessionEndMatchesTheSanitizedSessionLabel(t *testing.T) {
	dir := t.TempDir()
	raw := "sess i%d"
	seedSessionLog(t, dir, recordLine(t, TraceID(raw, 1), "deny", sessionNow))

	body, err := json.Marshal(map[string]string{"session_id": raw})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	runSessionEnd(t, dir, string(body))
	got := readSummaries(t, dir)[0]
	if got.Decisions != 1 {
		t.Fatalf("decisions = %d, want the sanitized label to join", got.Decisions)
	}
	if strings.ContainsAny(got.SessionID, " %") {
		t.Fatalf("session_id = %q, want the sanitized label", got.SessionID)
	}
}

// A session that decided nothing still gets a line: a complete series with a
// zero is more useful than a series with a silent gap.
func TestSessionEndWritesAZeroSummaryWithNoDecisionLog(t *testing.T) {
	dir := t.TempDir()

	result := runSessionEnd(t, dir, `{"session_id":"quiet"}`)
	if !result.Written {
		t.Fatalf("no summary written: %v", result.Fault)
	}
	got := readSummaries(t, dir)[0]
	if got.Decisions != 0 || got.Denied != 0 || got.Allowed != 0 {
		t.Fatalf("counts = %+v, want zeros", got)
	}
}

func TestSessionEndAppendsOneLinePerCall(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"one", "two", "three"} {
		if result := runSessionEnd(t, dir, `{"session_id":"`+id+`"}`); !result.Written {
			t.Fatalf("session %q wrote nothing: %v", id, result.Fault)
		}
	}
	if summaries := readSummaries(t, dir); len(summaries) != 3 {
		t.Fatalf("summaries = %d, want 3", len(summaries))
	}
}

// A summary names sessions and how often Boundary blocked them, so it is no more
// shareable than the records it counts.
func TestSessionEndWritesOwnerOnlyArtifacts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hook")
	result := runSessionEnd(t, dir, `{"session_id":"perms"}`)
	if !result.Written {
		t.Fatalf("no summary written: %v", result.Fault)
	}
	for _, path := range []string{dir, result.Path} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode = %v, want no group or world bits", path, info.Mode().Perm())
		}
	}
}

// Malformed input writes NOTHING. A summary attributed to a session that cannot
// be identified is worse than no summary.
func TestSessionEndIsSilentOnAMalformedEvent(t *testing.T) {
	for _, event := range []string{`{"session_id":`, "", "   ", "not json at all", `["a","b"]`} {
		t.Run(strings.TrimSpace(event), func(t *testing.T) {
			dir := t.TempDir()
			result := runSessionEnd(t, dir, event)
			if result.Written {
				t.Fatalf("event %q wrote a summary", event)
			}
			if result.Fault == nil {
				t.Fatalf("event %q reported no fault", event)
			}
			if _, err := os.Stat(filepath.Join(dir, SessionSummaryLogName)); !os.IsNotExist(err) {
				t.Fatalf("summary log exists after a malformed event: %v", err)
			}
		})
	}
}

func TestSessionEndIgnoresAForeignHookEvent(t *testing.T) {
	dir := t.TempDir()
	result := runSessionEnd(t, dir, `{"hook_event_name":"SessionStart","session_id":"nope"}`)
	if result.Written {
		t.Fatal("a SessionStart event wrote a session summary")
	}
	if result.Fault == nil || !strings.Contains(result.Fault.Error(), SessionEndEventName) {
		t.Fatalf("fault = %v, want it to name the expected hook", result.Fault)
	}
}

// An event with no session_id is a tolerated SHAPE, not a parse failure: it
// joins the records that also carried no session.
func TestSessionEndToleratesAMissingSessionID(t *testing.T) {
	dir := t.TempDir()
	seedSessionLog(t, dir, recordLine(t, TraceID("", 1), "deny", sessionNow))

	result := runSessionEnd(t, dir, `{"hook_event_name":"SessionEnd"}`)
	if !result.Written {
		t.Fatalf("no summary written: %v", result.Fault)
	}
	got := readSummaries(t, dir)[0]
	if got.SessionID != noSessionLabel || got.Decisions != 1 {
		t.Fatalf("summary = %+v, want the no-session label with 1 decision", got)
	}
}

// A line the reader could not decode is reported, never silently dropped: an
// under-count that says nothing is a false statement about the session.
func TestSessionEndReportsUnreadableLogLines(t *testing.T) {
	dir := t.TempDir()
	seedSessionLog(t, dir,
		recordLine(t, "sess#1", "deny", sessionNow),
		"}{ corrupted",
		recordLine(t, "sess#2", "allow", sessionNow),
	)

	runSessionEnd(t, dir, `{"session_id":"sess"}`)
	got := readSummaries(t, dir)[0]
	if got.Decisions != 2 {
		t.Fatalf("decisions = %d, want 2", got.Decisions)
	}
	if got.UnreadableLines != 1 {
		t.Fatalf("unreadable_lines = %d, want 1", got.UnreadableLines)
	}
}

// An action outside the four verdicts still counts as a decision, so the total
// never disagrees with the sum of what the log holds.
func TestSessionEndCountsAnUnrecognizedActionInTheTotal(t *testing.T) {
	dir := t.TempDir()
	seedSessionLog(t, dir,
		recordLine(t, "sess#1", "escalate", sessionNow),
		recordLine(t, "sess#2", "deny", sessionNow),
	)

	runSessionEnd(t, dir, `{"session_id":"sess"}`)
	got := readSummaries(t, dir)[0]
	if got.Decisions != 2 || got.Denied != 1 {
		t.Fatalf("summary = %+v, want 2 decisions of which 1 deny", got)
	}
	if got.Allowed != 0 || got.Asked != 0 || got.Warned != 0 {
		t.Fatalf("summary = %+v, an unrecognized action was folded into a verdict bucket", got)
	}
}

func TestSessionEndRefusesASymlinkedSummaryLog(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hook")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(root, "outside.jsonl")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, SessionSummaryLogName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result := runSessionEnd(t, dir, `{"session_id":"sess"}`)
	if result.Written {
		t.Fatal("a summary was written through a symlink")
	}
	if result.Fault == nil || !strings.Contains(result.Fault.Error(), "symlink") {
		t.Fatalf("fault = %v, want a symlink refusal", result.Fault)
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("symlink target was written through: %q", body)
	}
}

func TestSessionEndRefusesASymlinkedRecordDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(root, "hook")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result := runSessionEnd(t, link, `{"session_id":"sess"}`)
	if result.Written {
		t.Fatal("a summary was written into a symlinked directory")
	}
	if entries, err := os.ReadDir(target); err == nil && len(entries) != 0 {
		t.Fatalf("wrote %d entries through the symlink", len(entries))
	}
}

// A decision log that exists but cannot be read must NOT produce a summary
// claiming zero decisions: that would be a false statement about the session.
func TestSessionEndWritesNothingWhenTheDecisionLogCannotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	seedSessionLog(t, dir, recordLine(t, "sess#1", "deny", sessionNow))
	logPath := filepath.Join(dir, DecisionLogName)
	if err := os.Chmod(logPath, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(logPath, 0o600) })

	result := runSessionEnd(t, dir, `{"session_id":"sess"}`)
	if result.Written {
		t.Fatal("a summary was written over an unreadable decision log")
	}
	if result.Fault == nil {
		t.Fatal("no fault reported for an unreadable decision log")
	}
	if _, err := os.Stat(filepath.Join(dir, SessionSummaryLogName)); !os.IsNotExist(err) {
		t.Fatalf("summary log exists: %v", err)
	}
}

// An oversize event is a fault, not a summary: ReadEvent's cap applies here too.
func TestSessionEndRefusesAnOversizeEvent(t *testing.T) {
	dir := t.TempDir()
	oversize := strings.Repeat("x", MaxEventBytes+1)
	result := runSessionEnd(t, dir, oversize)
	if result.Written {
		t.Fatal("an oversize event wrote a summary")
	}
	if result.Fault == nil {
		t.Fatal("no fault reported for an oversize event")
	}
}

func TestSessionSummaryCarriesItsSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	runSessionEnd(t, dir, `{"session_id":"sess"}`)
	if got := readSummaries(t, dir)[0].SchemaVersion; got != SessionSummarySchemaVersion {
		t.Fatalf("schema_version = %q, want %q", got, SessionSummarySchemaVersion)
	}
}

// The session summary must not be mistakable for a decision record: it lands in
// its own log and leaves the record artifacts untouched.
func TestSessionEndDoesNotWriteDecisionRecords(t *testing.T) {
	dir := t.TempDir()
	runSessionEnd(t, dir, `{"session_id":"sess"}`)
	if _, err := os.Stat(filepath.Join(dir, DecisionLogName)); !os.IsNotExist(err) {
		t.Fatalf("session end touched the decision log: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, RecordsDirName)); !os.IsNotExist(err) {
		t.Fatalf("session end created a records directory: %v", err)
	}
}
