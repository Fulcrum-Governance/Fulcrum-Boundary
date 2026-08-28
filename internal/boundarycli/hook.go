package boundarycli

import (
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

Examples:
  printf '{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}' | boundary hook pretooluse
  printf '{"tool_name":"Write","tool_input":{"file_path":"config/.env"}}' | boundary hook pretooluse

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
