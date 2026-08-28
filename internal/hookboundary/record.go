package hookboundary

import (
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Identity constants stamped on every hook decision record.
const (
	// TransportHook is the transport label recorded for Claude Code PreToolUse
	// hook decisions. It lands in the record's adapter field and distinguishes
	// hook-routed decisions from the cli-transport records that `boundary
	// command run` and `boundary edit apply` write.
	TransportHook governance.TransportType = "claude-code-hook"
	// AdapterID names the adapter that parsed and routed the request. It is
	// descriptive route context, not attestation.
	AdapterID = "claude-code-hook"
	// RoutePrefix prefixes the record's route_id, which is completed with the
	// Claude Code tool name, e.g. "claude-code/PreToolUse/Bash".
	RoutePrefix = "claude-code/PreToolUse/"
	// ExecutionClaimSource names the surface that produced the execution
	// self-report on these records.
	ExecutionClaimSource = "claude-code-pretooluse-hook"
	// EventTypeParseRejected labels the record emitted when Boundary observed
	// an input shape it could not turn into a decision. It carries
	// raw_shape_hash in place of request_hash, matching the parse-rejection
	// records the MCP route emits.
	EventTypeParseRejected = "parse_rejected"
)

// noSessionLabel stands in for a missing session_id so a trace id is always
// parseable as "<session>#<sequence>".
const noSessionLabel = "no-session"

// Length caps for untrusted event-derived strings that reach a record. They
// bound record size; they are not a sanitization guarantee on their own.
const (
	maxLabelLen  = 128
	maxReasonLen = 512
)

// sequence is this process's monotonic hook decision counter.
var sequence atomic.Uint64

// NextSequence returns this process's next hook decision sequence number,
// starting at 1.
//
// The sequence orders decisions WITHIN one hook process. Claude Code spawns the
// hook once per tool call, so in normal use every record carries sequence 1; the
// counter distinguishes decisions only when one process decides several events
// (a batch run, a test, a future long-lived mode). It is not a global counter
// for a session, and two processes in the same session both start at 1.
func NextSequence() uint64 {
	return sequence.Add(1)
}

// TraceID composes the record's trace_id from the Claude Code session and this
// process's decision sequence, as "<session_id>#<sequence>".
//
// trace_id is the record's correlation field, and a Claude Code session is
// exactly a trace spanning many tool calls: records sharing a session share the
// prefix before "#". The session id is untrusted event input, so it is
// sanitized and length-capped first; a missing session id renders as
// "no-session".
func TraceID(sessionID string, seq uint64) string {
	session := sanitizeLabel(sessionID, maxLabelLen)
	if session == "" {
		session = noSessionLabel
	}
	return session + "#" + strconv.FormatUint(seq, 10)
}

// recordInput is everything buildRecord needs to assemble one decision record.
type recordInput struct {
	// Config supplies identity and version fields; it must already have
	// defaults resolved.
	Config Config
	// Request is the governed request the decision was made about. It is nil
	// only on a fault record, where RawShape stands in for it.
	Request *governance.GovernanceRequest
	// RawShape is the raw event bytes, set only on a fault record.
	RawShape []byte
	// EventType is empty for an ordinary decision, EventTypeParseRejected for
	// a fault record.
	EventType string
	// Tool is the Claude Code tool name that was routed.
	Tool string
	// SessionID is the Claude Code session id, if the event carried one.
	SessionID string
	// Sequence is this process's decision sequence number.
	Sequence uint64
	// Verdict is Boundary's verdict.
	Verdict Verdict
	// Reason is the human-readable explanation carried to both the record and
	// the hook's stdout.
	Reason string
	// MatchedRule names the posture entry that produced the verdict.
	MatchedRule string
}

// buildRecord assembles the hash-verifiable decision record for one hook
// decision. The record is built through governance.BuildDecisionRecord, so its
// decision_hash and record_id are computed exactly as every other Boundary
// surface computes them and `boundary verify-record` recomputes them unchanged.
//
// The record carries route context (adapter_id, route_id, execution_claim), so
// governance.BuildDecisionRecord emits it at schema_version "2" — the additive
// superset, not a new schema. execution_claim reports upstream_called=false and
// executed=false: this package decides before execution and never runs the
// command or writes the file. That is the hook's self-report about itself, not
// corroboration that no other path ran the action.
func buildRecord(in recordInput) governance.DecisionRecordV1 {
	tool := sanitizeLabel(in.Tool, maxLabelLen)
	event := governance.AuditEvent{
		EventType:      in.EventType,
		Transport:      TransportHook,
		ToolName:       tool,
		Action:         string(in.Verdict),
		Reason:         clip(in.Reason, maxReasonLen),
		MatchedRule:    in.MatchedRule,
		GatewayVersion: in.Config.BoundaryVersion,
		AgentID:        in.Config.AgentID,
		TenantID:       in.Config.TenantID,
		TraceID:        TraceID(in.SessionID, in.Sequence),
		Timestamp:      in.Config.Now().UTC(),
		DecisionMode:   governance.DecisionModeDeterministic,
		AdapterID:      AdapterID,
		ExecutionClaim: &governance.ExecutionClaim{
			UpstreamCalled: false,
			Executed:       false,
			Source:         ExecutionClaimSource,
		},
	}
	// route_id names a route the request actually travelled. When the event
	// never yielded a tool name there is no such route, so the field stays
	// empty rather than asserting a bare "claude-code/PreToolUse/".
	if tool != "" {
		event.RouteID = RoutePrefix + tool
	}
	switch {
	case in.Request != nil:
		event.RequestID = in.Request.RequestID
		event.EnvelopeID = in.Request.EnvelopeID
		event.RequestHash = governance.ComputeRequestHash(in.Request)
	case in.RawShape != nil:
		// No governed request exists: bind the record to the input shape
		// Boundary observed and rejected instead.
		event.RawShapeHash = governance.ComputeRawShapeHash(in.RawShape)
	}
	return governance.BuildDecisionRecord(event)
}

// sanitizeLabel reduces an untrusted, event-derived string to a conservative
// label safe to carry in a record and to compose into a trace or route id:
// letters, digits, and ".-_:/" survive, every other rune becomes "_", and the
// result is truncated to limit runes. It bounds and normalizes; it does not
// authenticate what the value claims to be.
func sanitizeLabel(raw string, limit int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, r := range trimmed {
		if count >= limit {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_' || r == ':' || r == '/':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		count++
	}
	return b.String()
}

// clip truncates a reason to limit runes, marking the cut so a reader can tell
// the text was shortened rather than produced that way.
func clip(raw string, limit int) string {
	trimmed := strings.TrimSpace(raw)
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

// recordTimestampFormat is the UTC timestamp component of a record file name.
// It is fixed-width and sorts lexicographically in emission order.
const recordTimestampFormat = "20060102T150405.000000000Z"

// recordFileName derives a decision record's file name from Boundary-generated
// values only: the record's own UTC timestamp and its record_id (which is
// derived from decision_hash). No event-supplied string reaches a path.
func recordFileName(record governance.DecisionRecordV1, fallback time.Time) string {
	ts := record.Timestamp
	if ts.IsZero() {
		ts = fallback
	}
	id := sanitizeLabel(record.RecordID, 64)
	if id == "" {
		id = "rec_unknown"
	}
	return ts.UTC().Format(recordTimestampFormat) + "-" + id + ".json"
}
