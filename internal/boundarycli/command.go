package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func runCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printCommandHelp(stdout)
		return 0
	}

	switch args[0] {
	case "classify":
		return runCommandClassify(args[1:], stdout, stderr)
	case "run":
		return runCommandRun(args[1:], stdout, stderr)
	case "install":
		return runCommandInstall(args[1:], stdout, stderr)
	case "uninstall":
		return runCommandUninstall(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command subcommand %q\n\n", args[0])
		printCommandHelp(stderr)
		return 1
	}
}

func printCommandHelp(w io.Writer) {
	fmt.Fprintf(w, `Boundary Command Preview

Purpose:
  Classify and govern project-local command paths routed through Boundary.

Usage:
  boundary command <subcommand> [flags]

Commands:
  classify        Classify a command without executing it
  run             Evaluate and run a wrapper-routed command
  install         Install project-local command shims
  uninstall       Remove project-local command shims

Examples:
  boundary command classify -- git status
  boundary command classify -- git push origin main
  boundary command classify --json -- rm -rf dist
  boundary command run -- git status
  boundary command install --project

Notes:
  - classify never executes commands.
  - run executes only after the preview command policy allows or warns.
  - install and uninstall touch only the project .boundary/bin directory.
  - Command Boundary governs only commands routed through Boundary.
`)
}

func runCommandClassify(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary command classify", stderr, commandHelp{
		Purpose: "Classify a command without executing it.",
		Usage:   "boundary command classify [--json] -- <command> [args...]",
		Common: []string{
			"boundary command classify -- git status",
			"boundary command classify -- git push origin main",
			"boundary command classify --json -- rm -rf dist",
		},
		Notes: []string{
			"classify never executes the command.",
			"secret-looking arguments are redacted in output.",
		},
	})
	jsonOut := fs.Bool("json", false, "write JSON classification output")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	argv := fs.Args()
	if len(argv) == 0 {
		fmt.Fprintln(stderr, "command classify: command is required after --")
		return 1
	}

	classification, err := commandboundary.Classify(argv)
	if err != nil {
		fmt.Fprintf(stderr, "command classify: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(classification); err != nil {
			fmt.Fprintf(stderr, "command classify: %v\n", err)
			return 1
		}
		return 0
	}
	writeCommandClassification(stdout, classification)
	return 0
}

func writeCommandClassification(w io.Writer, classification commandboundary.Classification) {
	fmt.Fprintf(w, "Command: %s\n", classification.RedactedCommandLine())
	fmt.Fprintf(w, "Class: %s\n", classification.ClassLabel())
	fmt.Fprintf(w, "Risk: %s\n", classification.Risk)
	fmt.Fprintf(w, "Recommended action: %s\n", classification.RecommendedAction)
	fmt.Fprintf(w, "Reason: %s\n", classification.Reason)
}
