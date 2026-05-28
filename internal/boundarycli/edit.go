package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

func runEdit(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printEditHelp(stdout)
		return 0
	}

	switch args[0] {
	case "inspect":
		return runEditInspect(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown edit subcommand %q\n\n", args[0])
		printEditHelp(stderr)
		return 1
	}
}

func printEditHelp(w io.Writer) {
	fmt.Fprintf(w, `Boundary Edit Preview

Purpose:
  Classify proposed file mutations routed through Boundary edit envelopes.

Usage:
  boundary edit <subcommand> [flags]

Commands:
  inspect        Classify a patch without applying it

Examples:
  boundary edit inspect --patch proposed.diff
  boundary edit inspect --patch proposed.diff --json
  boundary edit inspect --from-git-diff
  boundary edit inspect --stdin

Notes:
  - inspect never applies edits.
  - secret-looking paths and content are redacted in output.
  - Edit Boundary governs only proposed mutations routed through Boundary.
`)
}

func runEditInspect(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary edit inspect", stderr, commandHelp{
		Purpose: "Classify a proposed file mutation without applying it.",
		Usage:   "boundary edit inspect (--patch <file> | --from-git-diff | --stdin) [--json]",
		Common: []string{
			"boundary edit inspect --patch proposed.diff",
			"boundary edit inspect --patch proposed.diff --json",
			"boundary edit inspect --from-git-diff",
			"boundary edit inspect --stdin",
		},
		Notes: []string{
			"inspect never applies edits.",
			"secret-looking paths and content are redacted in output.",
		},
	})
	patchPath := fs.String("patch", "", "path to a unified diff or git diff patch file")
	fromGitDiff := fs.Bool("from-git-diff", false, "inspect the current git diff without applying it")
	fromStdin := fs.Bool("stdin", false, "read a patch from stdin")
	jsonOut := fs.Bool("json", false, "write JSON inspection output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	patch, err := readEditInspectPatch(*patchPath, *fromGitDiff, *fromStdin)
	if err != nil {
		fmt.Fprintf(stderr, "edit inspect: %v\n", err)
		return 1
	}
	inspection, err := editboundary.InspectPatch(patch)
	if err != nil {
		fmt.Fprintf(stderr, "edit inspect: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(inspection); err != nil {
			fmt.Fprintf(stderr, "edit inspect: %v\n", err)
			return 1
		}
		return 0
	}
	writeEditInspection(stdout, inspection)
	return 0
}

func readEditInspectPatch(patchPath string, fromGitDiff, fromStdin bool) ([]byte, error) {
	sources := 0
	if patchPath != "" {
		sources++
	}
	if fromGitDiff {
		sources++
	}
	if fromStdin {
		sources++
	}
	if sources != 1 {
		return nil, fmt.Errorf("exactly one patch source is required")
	}
	if patchPath != "" {
		if strings.HasPrefix(strings.TrimSpace(patchPath), "-") {
			return nil, fmt.Errorf("patch path must not start with '-'")
		}
		return os.ReadFile(patchPath)
	}
	if fromStdin {
		return io.ReadAll(os.Stdin)
	}
	cmd := exec.Command("git", "diff", "--no-ext-diff", "--binary")
	return cmd.Output()
}

func writeEditInspection(w io.Writer, inspection editboundary.Inspection) {
	fmt.Fprintln(w, "Edit Boundary Inspection")
	fmt.Fprintf(w, "Files touched: %d\n", inspection.FilesTouched)
	fmt.Fprintf(w, "Highest class: %s\n", inspection.HighestClassLabel())
	fmt.Fprintf(w, "Risk: %s\n", inspection.Risk)
	fmt.Fprintf(w, "Recommended action: %s\n", inspection.RecommendedAction)
	fmt.Fprintln(w, "Findings:")
	if len(inspection.Findings) == 0 {
		fmt.Fprintln(w, "- no file mutations detected")
		return
	}
	for _, finding := range inspection.Findings {
		fmt.Fprintf(w, "- %s %s: %s (%s)\n", finding.Path, finding.Operation, finding.ClassLabel(), finding.Reason)
	}
}
