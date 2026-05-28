package boundarycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

func runEditApply(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary edit apply", stderr, commandHelp{
		Purpose: "Evaluate and apply a proposed file mutation when policy permits.",
		Usage:   "boundary edit apply (--patch <file> | --from-git-diff | --stdin) [--dry-run] [--require-approval]",
		Common: []string{
			"boundary edit apply --patch proposed.diff",
			"boundary edit apply --patch proposed.diff --dry-run",
			"boundary edit apply --patch proposed.diff --require-approval",
		},
		Notes: []string{
			"patches are classified before any write is attempted.",
			"deny decisions and missing approval never invoke the applier.",
			"dry-run records the decision but never applies the patch.",
			"no shell interpolation or global filesystem interception is performed.",
		},
	})
	patchPath := fs.String("patch", "", "path to a unified diff or git diff patch file")
	fromGitDiff := fs.Bool("from-git-diff", false, "apply the current git diff after governance permits it")
	fromStdin := fs.Bool("stdin", false, "read a patch from stdin")
	dryRun := fs.Bool("dry-run", false, "evaluate and record without applying")
	requireApproval := fs.Bool("require-approval", false, "operator approval acknowledgement for require_approval decisions")
	recordOut := fs.String("record-out", editboundary.DefaultEditDecisionRecordPath, "JSONL edit decision record path")
	agentID := fs.String("agent-id", "", "agent identity to attach to the edit decision")
	tenantID := fs.String("tenant-id", "local", "tenant identity to attach to the edit decision")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	patch, err := readEditInspectPatch(*patchPath, *fromGitDiff, *fromStdin)
	if err != nil {
		fmt.Fprintf(stderr, "edit apply: %v\n", err)
		return 1
	}
	executor := editboundary.ApplyExecutor{RecordPath: *recordOut}
	result, err := executor.Apply(context.Background(), editboundary.ApplyRequest{
		Patch:           patch,
		AgentID:         *agentID,
		TenantID:        *tenantID,
		RecordPath:      *recordOut,
		DryRun:          *dryRun,
		ApprovalPresent: *requireApproval,
	})
	if err != nil {
		fmt.Fprintf(stderr, "edit apply: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Edit Boundary Apply\n")
	fmt.Fprintf(stdout, "Patch: %s\n", result.Inspection.PatchSHA256)
	fmt.Fprintf(stdout, "Highest class: %s\n", result.Inspection.HighestClassLabel())
	fmt.Fprintf(stdout, "Action: %s\n", result.Decision.Action)
	fmt.Fprintf(stdout, "Applied: %t\n", result.Applied)
	fmt.Fprintf(stdout, "Dry run: %t\n", result.Record.DryRun)
	fmt.Fprintf(stdout, "Record: %s\n", result.RecordPath)
	if !result.Applied && result.Record.Reason != "" {
		fmt.Fprintf(stdout, "Reason: %s\n", result.Record.Reason)
	}
	return result.ExitCode
}
