package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/hookboundary"
)

func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printHookHelp(stdout)
		return 0
	}

	switch args[0] {
	case "pretooluse":
		return runHookPreToolUse(args[1:], stdin, stdout, stderr)
	case "doctor":
		return runHookDoctor(args[1:], stdout, stderr)
	case "sessionend":
		return runHookSessionEnd(args[1:], stdin, stderr)
	default:
		fmt.Fprintf(stderr, "unknown hook subcommand %q\n\n", args[0])
		printHookHelp(stderr)
		return 1
	}
}

func printHookHelp(w io.Writer) {
	fmt.Fprintf(w, `Boundary Agent Hooks

Purpose:
  Decide an agent hook event read from stdin, before the tool runs, and record
  the verdict.

Usage:
  boundary hook <subcommand> [flags]

Commands:
  pretooluse      Decide a Claude Code PreToolUse event read from stdin
  doctor          Report how the hook is wired, what it recorded, and what it
                  does not govern
  sessionend      Summarize a finished session from a SessionEnd event on stdin

Examples:
  printf '{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}' | boundary hook pretooluse
  printf '{"tool_name":"Write","tool_input":{"file_path":"config/.env"}}' | boundary hook pretooluse
  boundary hook doctor
  printf '{"hook_event_name":"SessionEnd","session_id":"abc"}' | boundary hook sessionend

Notes:
  - The hook classifies; it never runs the command or writes the file.
  - Boundary governs only the tool calls routed to this hook; an unwired tool,
    an MCP tool, or a command a subprocess runs on its own is a bypass.
  - Command Boundary and Edit Boundary are delivered previews, not production
    surfaces.
`)
}

func runHookPreToolUse(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary hook pretooluse", stderr, commandHelp{
		Purpose: "Decide a Claude Code PreToolUse event read from stdin and record the verdict.",
		Usage:   "boundary hook pretooluse [--dir DIR] [--failmode open|closed|ask] [--agent-id ID]",
		Common: []string{
			`printf '{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}' | boundary hook pretooluse`,
			`printf '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | boundary hook pretooluse`,
			"boundary hook pretooluse --failmode closed < event.json",
		},
		Notes: []string{
			"Reads one PreToolUse event as JSON on stdin and writes the decision as JSON on stdout; a plain allow writes nothing.",
			"The JSON decision, not the exit code, is what stops a tool call; this command exits 0 on every decided path.",
			"Bash routes to Command Boundary and Edit/Write/MultiEdit/NotebookEdit route to Edit Boundary; every other tool is allowed silently and leaves no record.",
			"A compound Bash line is decomposed into segments and governed by its most restrictive one; a line the decomposer cannot model escalates to ask rather than being allowed.",
			"Every decided event is recorded to " + hookboundary.DefaultRecordDir + " before the decision reaches stdout; verify one with boundary verify-record.",
			"A deny always blocks. An internal fault follows --failmode (default ask); a record that cannot be written escalates the call rather than allowing it unrecorded.",
			"Command Boundary and Edit Boundary are delivered previews, so treat these verdicts as preview-grade.",
			"Records are hash-verifiable for integrity, not authenticity, and are not proof the action was executed or prevented.",
		},
	})
	env := hookboundary.ConfigFromEnv(os.Getenv, currentVersionInfo().Version)
	dir := fs.String("dir", env.Dir, "directory decision records are written to (default "+hookboundary.DefaultRecordDir+")")
	failMode := fs.String("failmode", string(env.FailMode), "fault posture: open, closed, or ask")
	agentID := fs.String("agent-id", env.AgentID, "agent id recorded on the decision (advisory)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "hook pretooluse: unexpected argument; the event is read from stdin")
		return 1
	}

	cfg := hookboundary.Config{
		Dir:             *dir,
		FailMode:        hookboundary.ParseFailMode(*failMode),
		AgentID:         *agentID,
		BoundaryVersion: env.BoundaryVersion,
	}
	result := hookboundary.Decide(cfg, stdin)
	if len(result.Stdout) > 0 {
		fmt.Fprint(stdout, string(result.Stdout))
	}
	writeHookDiagnostics(stderr, result)
	return 0
}

// runHookDoctor reports the local hook installation.
//
// It exits 1 when a check is BROKEN — nothing wired, hooks switched off, or
// evidence that cannot be written — so a setup script can tell "Boundary is not
// in front of this agent" from "Boundary is wired with a caveat". A warn or an
// unknown exits 0: a partial matcher, an unreadable enterprise policy, or a
// registration that only a plugin manifest declares is information, not a failed
// command. A plugin install must never exit 1 here — the manifest is real
// evidence that the hook may be live, so "unknown" is the honest grade and exit 0
// is the honest signal.
func runHookDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary hook doctor", stderr, commandHelp{
		Purpose: "Report how the Claude Code hook is wired, what it has recorded, and what it does not govern.",
		Usage:   "boundary hook doctor [--dir DIR] [--json]",
		Common: []string{
			"boundary hook doctor",
			"boundary hook doctor --json",
			"boundary hook doctor --dir .boundary/hook",
		},
		Notes: []string{
			"Read-only except one probe: the evidence check writes a marker file into the record directory and removes it, because writability cannot be answered by stat alone. The marker is never a decision record.",
			"Registration is matched by path shape in .claude/settings.json, .claude/settings.local.json, and ~/.claude/settings.json; nothing is executed and no command is resolved on PATH.",
			"Plugin hooks.json manifests are also found by path shape, in the project root and under ~/.claude. A manifest says what a plugin registers when Claude Code has it enabled, which is not readable from the file, so it is reported as unknown rather than wired.",
			"A hook wired through a wrapper this check does not recognize is reported as a peer rather than as Boundary, so a registration is never claimed on a guess.",
			"Other PreToolUse hooks are reported as merge peers: Claude Code takes the most restrictive result (deny > defer > ask > allow), which Boundary neither implements nor verifies.",
			"The bypass list is fixed, not discovered. MCP tools, processes a governed tool spawns, and shell outside Claude Code are never governed here, and enterprise managed settings usually cannot be verified from this machine.",
			"This never reports that Boundary is the only route an agent has to a shell or the filesystem, because it cannot establish that.",
			"Exits 1 when a check is broken, 0 on ok, warn, or unknown.",
		},
	})
	env := hookboundary.ConfigFromEnv(os.Getenv, currentVersionInfo().Version)
	dir := fs.String("dir", env.Dir, "decision record directory to inspect (default "+hookboundary.DefaultRecordDir+")")
	jsonOutput := fs.Bool("json", false, "emit the report as JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "hook doctor: unexpected argument; use --dir to select a record directory")
		return 1
	}

	report := hookboundary.Doctor(hookboundary.DoctorConfig{
		Dir:     *dir,
		Version: currentVersionInfo(),
	})
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "hook doctor: %v\n", err)
			return 1
		}
	} else if err := hookboundary.WriteDoctorText(stdout, report); err != nil {
		fmt.Fprintf(stderr, "hook doctor: %v\n", err)
		return 1
	}
	if report.Status == hookboundary.StateBroken {
		return 1
	}
	return 0
}

// runHookSessionEnd summarizes a finished session.
//
// It writes NOTHING to stdout and exits 0 on every path once the event has been
// read. SessionEnd output is not a decision — the tools already ran, nothing is
// being gated — so a summary that cannot be written is a fault the operator can
// see under BOUNDARY_HOOK_DEBUG rather than an error that disturbs a session that
// is already over. Only a malformed command line exits non-zero, because that is
// a wiring mistake a human made and can fix.
func runHookSessionEnd(args []string, stdin io.Reader, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary hook sessionend", stderr, commandHelp{
		Purpose: "Summarize one finished Claude Code session from a SessionEnd event read from stdin.",
		Usage:   "boundary hook sessionend [--dir DIR]",
		Common: []string{
			`printf '{"hook_event_name":"SessionEnd","session_id":"abc"}' | boundary hook sessionend`,
			"boundary hook sessionend --dir .boundary/hook < event.json",
		},
		Notes: []string{
			"Appends one line to " + hookboundary.SessionSummaryLogName + " counting the decision records whose trace_id belongs to that session.",
			"It writes nothing to stdout and exits 0 once the event is read: SessionEnd output is not a decision, nothing is being gated, and a session that has already ended must not be disturbed by its own bookkeeping.",
			"An event it cannot read, or one naming another hook, writes nothing at all rather than a summary attributed to the wrong session.",
			"The counts are recomputable from " + hookboundary.DecisionLogName + "; a summary is not hashed, not signed, and is not evidence.",
			"A session in which Boundary decided nothing still gets a line, so the summary log has no silent gaps.",
		},
	})
	env := hookboundary.ConfigFromEnv(os.Getenv, currentVersionInfo().Version)
	dir := fs.String("dir", env.Dir, "directory the session summary is written to (default "+hookboundary.DefaultRecordDir+")")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "hook sessionend: unexpected argument; the event is read from stdin")
		return 1
	}

	result := hookboundary.SessionEnd(hookboundary.Config{
		Dir:             *dir,
		BoundaryVersion: env.BoundaryVersion,
	}, stdin)
	writeSessionEndDiagnostics(stderr, result)
	return 0
}

// writeSessionEndDiagnostics prints the session summary trail under
// BOUNDARY_HOOK_DEBUG and stays silent otherwise. Nothing here is printed on the
// ordinary path: a SessionEnd hook that chatters is a SessionEnd hook an
// operator turns off.
func writeSessionEndDiagnostics(stderr io.Writer, result hookboundary.SessionEndResult) {
	if os.Getenv(hookboundary.EnvDebug) == "" {
		return
	}
	if result.Fault != nil {
		fmt.Fprintf(stderr, "boundary hook sessionend: no summary written: %v\n", result.Fault)
		return
	}
	if result.Summary == nil {
		return
	}
	fmt.Fprintf(stderr, "boundary hook sessionend: session=%q decisions=%d denied=%d asked=%d warned=%d allowed=%d\n",
		result.Summary.SessionID, result.Summary.Decisions, result.Summary.Denied,
		result.Summary.Asked, result.Summary.Warned, result.Summary.Allowed)
	fmt.Fprintf(stderr, "boundary hook sessionend: summary %s\n", result.Path)
}

// writeHookDiagnostics writes the hook's stderr commentary. It stays quiet by
// default so a hook in an interactive session is not noisy: it speaks up when a
// fault was allowed through (the operator should know Boundary did not decide)
// and, under BOUNDARY_HOOK_DEBUG, prints the routing and record trail.
func writeHookDiagnostics(stderr io.Writer, result hookboundary.Result) {
	if result.Fault != nil && result.Verdict == hookboundary.VerdictAllow {
		fmt.Fprintf(stderr, "boundary hook: fault, allowing (set %s=ask or =closed to stop): %v\n",
			hookboundary.EnvFailMode, result.Fault)
	}
	if os.Getenv(hookboundary.EnvDebug) == "" {
		return
	}
	fmt.Fprintf(stderr, "boundary hook: route=%q verdict=%q\n", result.Route, result.Verdict)
	if result.SinkError != nil {
		fmt.Fprintf(stderr, "boundary hook: record sink error: %v\n", result.SinkError)
	}
	if result.Record != nil {
		printRecordID(stderr, result.Record.RecordID)
		printRecordPath(stderr, result.Paths.Record)
		printRecordLog(stderr, result.Paths.Log)
	}
}
