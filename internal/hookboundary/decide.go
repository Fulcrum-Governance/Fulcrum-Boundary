package hookboundary

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

// Result is the outcome of deciding one PreToolUse event.
//
// Decide never fails: every error path is folded into a Result carrying a
// verdict, so the caller's job is to write Stdout and exit 0. Fault and
// SinkError are reported for diagnostics, not as a signal to retry.
type Result struct {
	// Route is the Boundary surface the tool was routed to. RouteNone means
	// the tool is not governed here.
	Route Route
	// Verdict is Boundary's final verdict after any fault or sink degradation.
	Verdict Verdict
	// Reason is the human-readable explanation, empty for a silent allow of an
	// ungoverned tool.
	Reason string
	// Stdout is the exact JSON to write to stdout, nil for a silent allow.
	Stdout []byte
	// Record is the persisted decision record, nil when nothing was decided
	// (an ungoverned tool) or when persisting failed.
	Record *governance.DecisionRecordV1
	// Paths names where the record was persisted; zero when Record is nil.
	Paths Paths
	// Sequence is this process's decision sequence number for the event, 0 for
	// an ungoverned tool.
	Sequence uint64
	// Fault is the internal fault that forced the fail-mode verdict, nil on an
	// ordinary decision.
	Fault error
	// SinkError is the record-persistence failure that degraded the verdict,
	// nil when the record was written.
	SinkError error
}

// Decide reads one Claude Code PreToolUse event from r, routes it to the
// matching Boundary preview classifier, persists the decision record, and
// returns the hook's stdout bytes.
//
// Order is load-bearing: the record is persisted BEFORE the decision is handed
// back for stdout, so a decision Claude Code acts on is a decision Boundary
// already wrote down. A tool this hook does not govern is allowed silently and
// leaves no record — nothing was decided.
//
// The verdict never depends on the caller reading Result.Fault: faults are
// already resolved through cfg.FailMode, and a record-sink failure is already
// folded in. Decide does not execute the command, write the file, or touch the
// network.
func Decide(cfg Config, r io.Reader) Result {
	cfg = cfg.withDefaults()

	raw, err := ReadEvent(r)
	if err != nil {
		return faultResult(cfg, raw, "", "", err)
	}
	event, err := ParseEvent(raw)
	if err != nil {
		return faultResult(cfg, raw, "", "", err)
	}
	if event.HookEventName != "" && event.HookEventName != HookEventName {
		return faultResult(cfg, raw, "", event.SessionID,
			fmt.Errorf("hook_event_name %q is not %q", sanitizeLabel(event.HookEventName, maxLabelLen), HookEventName))
	}
	tool := strings.TrimSpace(event.ToolName)
	if tool == "" {
		return faultResult(cfg, raw, "", event.SessionID, errors.New("event carries no tool_name"))
	}

	route := RouteFor(tool)
	if route == RouteNone {
		// Not a governed surface. Allow silently, record nothing: Boundary made
		// no decision about this call and must not imply it did.
		return Result{Route: RouteNone, Verdict: VerdictAllow}
	}

	input, err := event.Input()
	if err != nil {
		return faultResult(cfg, raw, tool, event.SessionID, err)
	}
	if route == RouteCommand {
		return decideCommand(cfg, raw, event, input)
	}
	return decideEdit(cfg, raw, event, input)
}

// decideCommand routes a shell tool call through Command Boundary.
//
// The governed request is built from the line's AGGREGATE classification — the
// most restrictive segment the decomposer found — so a compound line is governed
// by its worst part. argv_hash binds the record to the exact command LINE
// Boundary observed, hashed as a single element, because the line (not a
// pre-split argv) is what was classified here.
//
// The aggregate is then run past the control-surface guard, which denies a write
// to the files that decide how this agent is governed. Without it the hook would
// deny `.claude/settings.json` as an Edit tool call and permit the same write as
// `cp evil .claude/settings.json` — self-protection closed on one of the two
// routes the same hook governs is not closed.
func decideCommand(cfg Config, raw []byte, event Event, input ToolInput) Result {
	line := strings.TrimSpace(input.Command)
	if line == "" {
		return faultResult(cfg, raw, event.ToolName, event.SessionID,
			errors.New("shell tool_input carries no command to classify"))
	}
	classified, err := ClassifyBashLine(line)
	if err != nil {
		return faultResult(cfg, raw, event.ToolName, event.SessionID, fmt.Errorf("classify command: %w", err))
	}
	aggregate := classified.Aggregate
	verdict, ok := ParseVerdict(string(aggregate.RecommendedAction))
	if !ok {
		return faultResult(cfg, raw, event.ToolName, event.SessionID,
			fmt.Errorf("command classifier returned unrecognized action %q", aggregate.RecommendedAction))
	}
	reason := commandReason(verdict, classified)
	matchedRule := "command-preview-posture:" + string(aggregate.Class)
	if surface, ok := commandControlSurface(classified); ok {
		verdict = verdict.AtLeastAsStrict(VerdictDeny)
		reason = controlSurfaceReason(surface)
		matchedRule = "command-control-surface:" + string(aggregate.Class)
	}

	observed := []string{line}
	request := commandboundary.BuildGovernanceRequest(commandboundary.RunRequest{
		Argv:     observed,
		CWD:      sanitizeLabel(event.CWD, maxLabelLen),
		AgentID:  cfg.AgentID,
		TenantID: cfg.TenantID,
	}, aggregate, commandboundary.HashArgv(observed))
	request.Transport = TransportHook

	return decided(cfg, recordInput{
		Config:      cfg,
		Request:     request,
		Tool:        event.ToolName,
		SessionID:   event.SessionID,
		Verdict:     verdict,
		Reason:      reason,
		MatchedRule: matchedRule,
	}, RouteCommand)
}

// controlSurfaceTarget names a governance control surface a command was about to
// write, and why that path is one.
type controlSurfaceTarget struct {
	// Path is the redacted argument that named the control surface.
	Path string
	// Reason is editboundary's statement of why the path is a control surface.
	Reason string
	// Segment is the redacted text of the segment that would have written it.
	Segment string
}

// commandControlSurface reports the governance control surface a classified
// command line writes to, if any.
//
// It is the command route's half of the self-protection the edit route gets from
// editboundary's E7 control-surface classes, and it uses the same path shapes —
// editboundary.ControlSurfacePath — so both routes agree on what a control
// surface is. An agent that can rewrite `.claude/settings.json`, a file under
// `.claude/hooks/`, `.boundary/`, or the hook script itself can turn the hook off
// before its next tool call, whichever tool it uses to do it.
//
// Only segments whose class MUTATES a file are scanned, so reading or listing a
// control surface is untouched: `cat .claude/settings.json` and `boundary
// verify-record .boundary/hook/records/x.json` keep the verdicts their own
// classes give them. Redirect segments are covered because the redirect target is
// classified as the write it is.
//
// Within a scanned segment the argument POSITION is not analyzed, so a control
// surface named as the SOURCE of a write is refused too:
// `cp .claude/settings.json backup.json` denies even though it only reads the
// settings. That is deliberate. Telling source from destination needs a
// per-command table of argument grammars, and a table that is wrong about one
// command is wrong in the permissive direction; over-refusing a backup is the
// cost of not guessing.
//
// It scans REDACTED arguments, which is exact for these shapes — none of them
// look like a secret, so none of them are rewritten — and it never sees an
// argument the classifier redacted away. Like ControlSurfacePaths it is a set of
// shapes, not an inventory: a command that reaches a control surface some other
// way (an interpreter's inline program, a path built at runtime) is not matched.
func commandControlSurface(classified commandboundary.LineClassification) (controlSurfaceTarget, bool) {
	for _, segment := range classified.Segments {
		if !segment.Class.MutatesFiles() {
			continue
		}
		for _, arg := range segment.ArgsRedacted {
			reason, ok := editboundary.ControlSurfacePath(arg)
			if !ok {
				continue
			}
			return controlSurfaceTarget{Path: arg, Reason: reason, Segment: segment.Segment}, true
		}
	}
	return controlSurfaceTarget{}, false
}

// controlSurfaceReason renders the operator-facing explanation for a command
// denied by the control-surface guard. It names the surface and the segment that
// would have written it, so the prompt explains itself.
func controlSurfaceReason(surface controlSurfaceTarget) string {
	return fmt.Sprintf(
		"Fulcrum Boundary (Command Boundary preview) denied this command: it writes %q, a governance control surface (%s) — %s. "+
			"This is a routed pre-execution deny; the command was not run.",
		surface.Path, surface.Reason, surface.Segment)
}

// decideEdit routes a file-mutation tool call through Edit Boundary. The target
// path is resolved against cfg.ProjectRoot first, so a write outside the project
// is classified as one instead of as its relative tail.
func decideEdit(cfg Config, raw []byte, event Event, input ToolInput) Result {
	path := input.EditPath()
	if path == "" {
		return faultResult(cfg, raw, event.ToolName, event.SessionID,
			errors.New("edit tool_input carries no file_path or notebook_path to classify"))
	}
	inspection, patch, err := InspectEditPath(path, cfg.ProjectRoot)
	if err != nil {
		return faultResult(cfg, raw, event.ToolName, event.SessionID, fmt.Errorf("inspect edit: %w", err))
	}
	verdict, ok := ParseVerdict(string(inspection.RecommendedAction))
	if !ok {
		return faultResult(cfg, raw, event.ToolName, event.SessionID,
			fmt.Errorf("edit classifier returned unrecognized action %q", inspection.RecommendedAction))
	}

	request := editboundary.BuildGovernanceRequest(editboundary.ApplyRequest{
		Patch:    patch,
		CWD:      sanitizeLabel(event.CWD, maxLabelLen),
		AgentID:  cfg.AgentID,
		TenantID: cfg.TenantID,
		DryRun:   true,
	}, inspection)
	request.Transport = TransportHook

	return decided(cfg, recordInput{
		Config:      cfg,
		Request:     request,
		Tool:        event.ToolName,
		SessionID:   event.SessionID,
		Verdict:     verdict,
		Reason:      editReason(verdict, inspection),
		MatchedRule: "edit-preview-posture:" + string(inspection.HighestClass),
	}, RouteEdit)
}

// faultResult resolves an internal fault through the fail mode and records it.
//
// The record is a parse-rejection record: it carries raw_shape_hash over the
// exact bytes Boundary observed instead of a request_hash, because no governed
// request was ever built. It states that Boundary saw an input it could not turn
// into a decision — not that the tool call was safe or unsafe.
func faultResult(cfg Config, raw []byte, tool, sessionID string, fault error) Result {
	verdict := cfg.FailMode.Verdict()
	shape := raw
	if shape == nil {
		shape = []byte{}
	}
	result := decided(cfg, recordInput{
		Config:      cfg,
		RawShape:    shape,
		EventType:   EventTypeParseRejected,
		Tool:        tool,
		SessionID:   sessionID,
		Verdict:     verdict,
		Reason:      faultReason(cfg.FailMode, fault),
		MatchedRule: "hook-failmode:" + string(cfg.FailMode),
	}, RouteNone)
	result.Fault = fault
	return result
}

// decided builds the decision record, persists it, and renders the hook output.
//
// A record-sink failure never resolves to a silent allow, whatever the fail mode
// says. The fail mode governs whether Boundary could CLASSIFY; the sink is
// whether Boundary could RECORD, and an unrecorded decision is escalated to the
// user instead of waved through. A deny stands as a deny: escalation may only
// strengthen the verdict, never soften one Boundary already reached.
func decided(cfg Config, in recordInput, route Route) Result {
	in.Config = cfg
	in.Sequence = NextSequence()
	record := buildRecord(in)

	paths, err := Sink{Dir: cfg.Dir}.Write(record)
	if err != nil {
		verdict := in.Verdict.AtLeastAsStrict(VerdictRequireApproval)
		reason := in.Reason + " " + sinkFailureNote(err)
		return Result{
			Route:     route,
			Verdict:   verdict,
			Reason:    reason,
			Stdout:    BuildOutput(verdict, reason),
			Sequence:  in.Sequence,
			SinkError: err,
		}
	}
	return Result{
		Route:    route,
		Verdict:  in.Verdict,
		Reason:   in.Reason,
		Stdout:   BuildOutput(in.Verdict, in.Reason),
		Record:   &record,
		Paths:    paths,
		Sequence: in.Sequence,
	}
}

// commandReason renders the operator-facing explanation for a command verdict.
//
// For a compound line it names the OFFENDING SEGMENT, not the leading command:
// the aggregate reason already states which segment set the verdict and how many
// segments the line decomposed into, and the trailing parenthetical is that
// segment's own redacted text. When the line could not be decomposed the reason
// says so, so an unexpected prompt is self-explaining. Redaction is
// pattern-based and not a guarantee.
func commandReason(verdict Verdict, classified commandboundary.LineClassification) string {
	label := fmt.Sprintf("[%s]: %s", classified.Aggregate.Class, classified.Aggregate.Reason)
	// A single-segment line's reason does not name the command, so the redacted
	// command line is what tells the operator what was decided. A compound
	// line's aggregate reason already names the offending segment, and a line
	// whose verdict came from the undecomposable floor has no segment to name;
	// appending either would only repeat text or read as an empty parenthetical.
	if len(classified.Segments) == 1 && classified.Aggregate.Command != "" {
		label += " (" + classified.RedactedCommandLine() + ")"
	}
	switch verdict {
	case VerdictDeny:
		return "Fulcrum Boundary (Command Boundary preview) denied this command " + label +
			". This is a routed pre-execution deny; the command was not run."
	case VerdictRequireApproval:
		return "Fulcrum Boundary (Command Boundary preview) flagged this command " + label +
			". It requires approval before it runs."
	case VerdictWarn:
		return "Fulcrum Boundary (Command Boundary preview) warned on this command " + label +
			". Boundary does not block it; approve it yourself if you meant to run it."
	default:
		return "Fulcrum Boundary (Command Boundary preview) allowed this command " + label + "."
	}
}

// editReason renders the operator-facing explanation for an edit verdict. The
// path it names is the inspection's own path, so a secret-bearing path appears
// redacted rather than echoed back and written into a record.
func editReason(verdict Verdict, inspection editboundary.Inspection) string {
	path, reason := editFindingDetail(inspection)
	label := fmt.Sprintf("to %q [%s]: %s", path, inspection.HighestClass, reason)
	switch verdict {
	case VerdictDeny:
		return "Fulcrum Boundary (Edit Boundary preview) denied this edit " + label +
			". This is a routed pre-execution deny; the file was not written."
	case VerdictRequireApproval:
		return "Fulcrum Boundary (Edit Boundary preview) flagged this edit " + label +
			". It requires approval before the file is written."
	case VerdictWarn:
		return "Fulcrum Boundary (Edit Boundary preview) warned on this edit " + label +
			". Boundary does not block it; approve it yourself if you meant to write it."
	default:
		return "Fulcrum Boundary (Edit Boundary preview) allowed this edit " + label + "."
	}
}

// editFindingDetail returns the path and reason of the finding that set the
// inspection's highest class, falling back to the first finding.
func editFindingDetail(inspection editboundary.Inspection) (path, reason string) {
	if len(inspection.Findings) == 0 {
		return "", "no file mutation detected"
	}
	chosen := inspection.Findings[0]
	for _, finding := range inspection.Findings {
		if finding.Class == inspection.HighestClass {
			chosen = finding
			break
		}
	}
	return chosen.Path, chosen.Reason
}

// faultReason renders the operator-facing explanation for an internal fault. It
// names the fail mode in force and how to change it, so an unexpected prompt is
// self-explaining.
func faultReason(mode FailMode, fault error) string {
	detail := "unknown fault"
	if fault != nil {
		detail = fault.Error()
	}
	switch mode {
	case FailModeOpen:
		return "Fulcrum Boundary hook fault, failing open and allowing: " + detail +
			". Set " + EnvFailMode + "=ask or =closed to stop allowing on faults."
	case FailModeClosed:
		return "Fulcrum Boundary hook fault, failing closed: " + detail +
			". Set " + EnvFailMode + "=ask or =open to change the fault posture."
	default:
		return "Fulcrum Boundary hook fault, asking before this tool call: " + detail +
			". Set " + EnvFailMode + "=open to allow or =closed to deny on faults."
	}
}

// sinkFailureNote explains a degraded verdict caused by an unwritable record.
//
// It states the failure WITHOUT the record path. This text reaches the model
// transcript through permissionDecisionReason and is persisted in every later
// record's reason; where the operator keeps decision records is the operator's
// business, and echoing an absolute path back to the agent hands it a filesystem
// location it did not have. The full error, path included, stays on Result.
// SinkError and reaches stderr under BOUNDARY_HOOK_DEBUG.
func sinkFailureNote(err error) string {
	return "Fulcrum Boundary could not persist the decision record (" + sinkFailureDetail(err) +
		"), so this call is escalated rather than allowed unrecorded. Set " + EnvDebug +
		" for the full error."
}

// sinkFailureDetail renders a sink failure without naming the record path. An
// error the sink did not produce falls back to a fixed phrase rather than being
// printed, so no path can reach the transcript through an unexpected error type.
func sinkFailureDetail(err error) string {
	var writeErr *WriteError
	if errors.As(err, &writeErr) {
		return writeErr.Summary()
	}
	return "record sink write failed"
}
