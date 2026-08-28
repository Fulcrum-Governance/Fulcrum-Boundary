package hookboundary

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

// SessionEndEventName is the Claude Code hook event SessionEnd summarizes. An
// event that names a different hook is a wiring mistake, and SessionEnd answers
// a wiring mistake by writing nothing at all.
const SessionEndEventName = "SessionEnd"

// SessionSummarySchemaVersion identifies the shape of one session summary line.
const SessionSummarySchemaVersion = "boundary.hook.session-summary.v1"

// SessionSummaryLogName is the append-only session summary log, one line per
// ended session, written beside the decision records it counts.
const SessionSummaryLogName = "session-summaries.jsonl"

// ReceiptHint is the directory a reader should look in for session receipts. It
// is a POINTER, not an assertion: nothing here creates that directory, writes to
// it, or checks that anything is in it. A summary carries the hint so a later
// reader knows where the receipt lane looks, and finding nothing there is an
// ordinary outcome.
//
// It is deliberately NOT under .boundary/. That tree is a governance control
// surface (editboundary.ControlSurfacePath denies any write with a .boundary
// component, at any position, as E7), so a receipt could never be written there
// by any routed tool call — a hint pointing inside it would name a directory
// that is empty by construction. The receipt lane writes to this sibling path
// instead; see skills/report/SKILL.md.
const ReceiptHint = ".boundary-receipts/"

// maxSessionReasonLen caps the SessionEnd reason label carried into a summary.
// The reason is a short host-supplied word ("clear", "logout"); the cap bounds a
// producer that sends something else.
const maxSessionReasonLen = 64

// SessionEndEvent is the subset of a Claude Code SessionEnd event this package
// reads. Unknown fields are ignored by design: SessionEnd carries transcript
// paths and host metadata that a decision summary has no business reading.
type SessionEndEvent struct {
	// HookEventName is the hook that fired. Empty is tolerated (older clients);
	// a value other than SessionEndEventName is a wiring mistake.
	HookEventName string `json:"hook_event_name"`
	// SessionID is the session that ended. It is untrusted input: it is
	// sanitized and length-capped before it reaches the summary, and it is
	// never used to build a filesystem path.
	SessionID string `json:"session_id"`
	// Reason is the host's word for why the session ended, e.g. "clear".
	Reason string `json:"reason"`
}

// SessionSummary is one line of the session summary log: what Boundary decided
// during one Claude Code session, counted from the decision records it wrote.
//
// It is a COUNT OVER RECORDS, not a second source of truth. Every number here is
// recomputable from decision-records.jsonl, and the records — not this line —
// are the hash-verifiable artifact. A summary is not hashed, not signed, and not
// consumed by verify-record.
type SessionSummary struct {
	SchemaVersion string `json:"schema_version"`
	// UTC is when the summary was written.
	UTC time.Time `json:"utc"`
	// SessionID is the sanitized session label, which is also the session half
	// of the trace ids it counted. It is not the raw event value.
	SessionID string `json:"session_id"`
	// TracePrefix is the exact trace_id prefix the counts were matched on, so
	// the join is reproducible by anyone holding the same log.
	TracePrefix string `json:"trace_prefix"`
	// Reason is the host's sanitized word for why the session ended, when it
	// supplied one.
	Reason string `json:"reason,omitempty"`
	// Decisions is how many records carried this session's trace prefix,
	// including any whose action was not one of the four below.
	Decisions int `json:"decisions"`
	// Denied counts records whose action was deny.
	Denied int `json:"denied"`
	// Asked counts records whose action was require_approval. Warn is counted
	// separately even though both reach the user as an "ask", because the
	// verdict Boundary recorded is what a later reader must be able to recover.
	Asked int `json:"asked"`
	// Warned counts records whose action was warn.
	Warned int `json:"warned"`
	// Allowed counts records whose action was allow.
	Allowed int `json:"allowed"`
	// UnreadableLines counts lines in the decision log that could not be read
	// as records, and are therefore counted in nothing above. It is present
	// only when the reader hit some, so a summary never under-counts silently.
	UnreadableLines int `json:"unreadable_lines,omitempty"`
	// ReceiptHint names where a reader should look for session receipts. It is
	// a pointer, not a claim that anything is there.
	ReceiptHint string `json:"receipt_hint"`
}

// SessionEndResult is the outcome of one SessionEnd run, for diagnostics only.
//
// SessionEnd never fails a session: the caller writes nothing to stdout and
// exits 0 whatever this says. Fault explains why no summary was written, so a
// debug run can show it.
type SessionEndResult struct {
	// Written reports whether a summary line was appended.
	Written bool
	// Path is the summary log that was appended to, empty when nothing was
	// written.
	Path string
	// Summary is the line that was written, nil when nothing was.
	Summary *SessionSummary
	// Fault is why nothing was written, nil on success.
	Fault error
}

// SessionEnd reads one Claude Code SessionEnd event from r and appends one
// summary line counting what Boundary decided during that session.
//
// It is deliberately silent and deliberately harmless. SessionEnd output is NOT
// a decision — nothing is being gated, the tools already ran — so this writes
// nothing to stdout, and every failure path returns a Fault instead of an error
// the caller could turn into a non-zero exit. A session that ends must not be
// disturbed by its own bookkeeping.
//
// Two failures are treated differently on purpose:
//
//   - An event that cannot be read or parsed, or that names another hook,
//     writes NOTHING. There is no session to attribute a summary to, and a
//     summary attributed to the wrong session is worse than no summary.
//   - A decision log that exists but cannot be READ also writes nothing. A
//     summary claiming zero decisions over a log the reader could not open
//     would be a false statement about what happened; an absent log is
//     different and honestly counts as zero.
//
// A session in which Boundary decided nothing still gets a line. "This session
// ended and Boundary recorded nothing for it" is a fact worth keeping, and it is
// what makes the summary log a complete per-session series rather than a series
// with silent gaps.
func SessionEnd(cfg Config, r io.Reader) SessionEndResult {
	cfg = cfg.withDefaults()

	raw, err := ReadEvent(r)
	if err != nil {
		return SessionEndResult{Fault: err}
	}
	var event SessionEndEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return SessionEndResult{Fault: fmt.Errorf("parse SessionEnd event: %w", err)}
	}
	if event.HookEventName != "" && event.HookEventName != SessionEndEventName {
		return SessionEndResult{Fault: fmt.Errorf("hook_event_name %q is not %q",
			sanitizeLabel(event.HookEventName, maxLabelLen), SessionEndEventName)}
	}

	session := sanitizeLabel(event.SessionID, maxLabelLen)
	if session == "" {
		session = noSessionLabel
	}
	prefix := session + "#"

	summary, err := summarizeSession(cfg.Dir, session, prefix, sanitizeLabel(event.Reason, maxSessionReasonLen), cfg.Now())
	if err != nil {
		return SessionEndResult{Fault: err}
	}
	path, err := (Sink{Dir: cfg.Dir}).WriteSessionSummary(summary)
	if err != nil {
		return SessionEndResult{Fault: err}
	}
	return SessionEndResult{Written: true, Path: path, Summary: &summary}
}

// summarizeSession counts the decision records belonging to one session.
//
// Records are matched on the trace_id PREFIX, which is how a Claude Code session
// appears in the record stream: TraceID composes "<sanitized session>#<sequence>"
// and the hook runs once per tool call, so every record from one session shares
// the part before "#". Matching the prefix (not the whole field) is what joins a
// session to the many single-decision processes that served it.
func summarizeSession(dir, session, prefix, reason string, now time.Time) (SessionSummary, error) {
	summary := SessionSummary{
		SchemaVersion: SessionSummarySchemaVersion,
		UTC:           now.UTC(),
		SessionID:     session,
		TracePrefix:   prefix,
		Reason:        reason,
		ReceiptHint:   ReceiptHint,
	}
	logPath := filepath.Join(dir, DecisionLogName)
	err := scanDecisionLog(logPath, func(entry logEntry, ok bool) {
		if !ok {
			summary.UnreadableLines++
			return
		}
		if !hasTracePrefix(entry.TraceID, prefix) {
			return
		}
		summary.Decisions++
		switch Verdict(entry.Action) {
		case VerdictDeny:
			summary.Denied++
		case VerdictRequireApproval:
			summary.Asked++
		case VerdictWarn:
			summary.Warned++
		case VerdictAllow:
			summary.Allowed++
		}
	})
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// No log at all: nothing was decided here, which is a countable zero
		// rather than a failure. UnreadableLines is untouched.
		return summary, nil
	case err != nil:
		return SessionSummary{}, fmt.Errorf("read decision log: %w", err)
	}
	return summary, nil
}

// hasTracePrefix reports whether a record's trace_id belongs to the session
// identified by prefix. The prefix already ends in "#", so this cannot match a
// session id that merely starts with another one's.
func hasTracePrefix(traceID, prefix string) bool {
	return strings.HasPrefix(traceID, prefix)
}

// WriteSessionSummary appends one summary line to the session summary log under
// Dir and returns the path it was appended to.
//
// It reuses the sink's discipline rather than restating it: the directory is
// created owner-only or refused when it is a symlink or not a directory
// (ensureRecordDir), and the log itself is refused when it is a symlink and
// created 0600 when it is absent (appendJSONLine). A summary names the sessions
// an operator ran and how often Boundary blocked them, so it is no more shareable
// than the records it counts.
func (s Sink) WriteSessionSummary(summary SessionSummary) (string, error) {
	dir := s.Dir
	if dir == "" {
		dir = DefaultRecordDir
	}
	if err := ensureRecordDir(dir); err != nil {
		return "", err
	}
	body, err := json.Marshal(summary)
	if err != nil {
		return "", &WriteError{Op: "encode session summary", Path: dir, Err: err}
	}
	path := filepath.Join(dir, SessionSummaryLogName)
	if err := appendJSONLine(path, "session summary log", body); err != nil {
		return "", err
	}
	return path, nil
}
