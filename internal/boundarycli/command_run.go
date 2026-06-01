package boundarycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func runCommandRun(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary command run", stderr, commandHelp{
		Purpose: "Evaluate a wrapper-routed command before execution.",
		Usage:   "boundary command run [--record-out PATH] -- <command> [args...]",
		Common: []string{
			"boundary command run -- git status",
			"boundary command run -- rm -rf dist",
			"boundary command run --record-out .boundary/command/decision-records.jsonl -- git status",
		},
		Notes: []string{
			"Commands are parsed as argv; no shell interpolation is performed.",
			"deny and require_approval decisions do not execute the command.",
			"Decision records do not store raw secret-looking arguments.",
		},
	})
	recordOut := fs.String("record-out", commandboundary.DefaultDecisionRecordPath, "JSONL command decision record path")
	agentID := fs.String("agent-id", "", "agent identity to attach to the command decision")
	tenantID := fs.String("tenant-id", "local", "tenant identity to attach to the command decision")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintln(stderr, "command run: command is required after --")
		return 1
	}

	executor := commandboundary.Executor{RecordPath: *recordOut}
	result, err := executor.Run(context.Background(), commandboundary.RunRequest{
		Argv:       argv,
		AgentID:    *agentID,
		TenantID:   *tenantID,
		RecordPath: *recordOut,
	})
	if err != nil {
		fmt.Fprintf(stderr, "command run: %v\n", err)
		return 1
	}

	if len(result.Stdout) > 0 {
		_, _ = stdout.Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = stderr.Write(result.Stderr)
		if !strings.HasSuffix(string(result.Stderr), "\n") {
			fmt.Fprintln(stderr)
		}
	}
	fmt.Fprintf(stderr, "boundary: action=%s executed=%t class=%s\n", result.Decision.Action, result.Executed, result.Classification.ClassLabel())
	if !result.Executed && result.Decision.Reason != "" {
		fmt.Fprintf(stderr, "boundary: reason=%s\n", result.Decision.Reason)
	}
	printRecordPath(stderr, result.RecordPath)
	return result.ExitCode
}
