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
			`boundary command classify -- 'git status && rm -rf dist'`,
		},
		Notes: []string{
			"classify never executes the command.",
			"secret-looking arguments are redacted in output.",
			"a single argument carrying shell operators is decomposed per segment; the most restrictive segment becomes the verdict.",
			"a compound line the tokenizer cannot decompose reports require_approval, never allow.",
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
	// One unsplit argument carrying shell operators is a compound line, not an
	// argv the caller already split. Decompose it per segment; every other
	// invocation keeps the argv classification path unchanged.
	if len(argv) == 1 && commandboundary.ContainsShellOperators(argv[0]) {
		return runCommandClassifyLine(argv[0], *jsonOut, stdout, stderr)
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

// commandLineClassifyOutput is the JSON form of a compound-line classification.
// It embeds the aggregate Classification so the emitted object stays a superset
// of the single-command payload: a consumer that reads only `class`,
// `recommended_action`, and `reason` keeps reading the enforced verdict.
type commandLineClassifyOutput struct {
	commandboundary.Classification
	LineSchemaVersion string                                  `json:"line_schema_version"`
	Parseable         bool                                    `json:"parseable"`
	Segments          []commandboundary.SegmentClassification `json:"segments"`
}

func runCommandClassifyLine(line string, jsonOut bool, stdout, stderr io.Writer) int {
	classification, err := commandboundary.ClassifyLine(line)
	if err != nil {
		fmt.Fprintf(stderr, "command classify: %v\n", err)
		return 1
	}
	if jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		output := commandLineClassifyOutput{
			Classification:    classification.Aggregate,
			LineSchemaVersion: classification.SchemaVersion,
			Parseable:         classification.Parseable,
			Segments:          classification.Segments,
		}
		if err := enc.Encode(output); err != nil {
			fmt.Fprintf(stderr, "command classify: %v\n", err)
			return 1
		}
		return 0
	}
	writeCommandLineClassification(stdout, classification)
	return 0
}

func writeCommandLineClassification(w io.Writer, classification commandboundary.LineClassification) {
	fmt.Fprintf(w, "Command: %s\n", classification.RedactedCommandLine())
	fmt.Fprintf(w, "Class: %s\n", classification.AggregateClassLabel())
	fmt.Fprintf(w, "Risk: %s\n", classification.Aggregate.Risk)
	fmt.Fprintf(w, "Recommended action: %s\n", classification.Aggregate.RecommendedAction)
	fmt.Fprintf(w, "Reason: %s\n", classification.Aggregate.Reason)
	fmt.Fprintf(w, "Parseable: %t\n", classification.Parseable)
	fmt.Fprintf(w, "Segments: %d\n", len(classification.Segments))
	for i, segment := range classification.Segments {
		fmt.Fprintf(w, "  %d. [%s | %s] %s\n", i+1, segment.ClassLabel(), segment.RecommendedAction, segment.Segment)
	}
}

func writeCommandClassification(w io.Writer, classification commandboundary.Classification) {
	fmt.Fprintf(w, "Command: %s\n", classification.RedactedCommandLine())
	fmt.Fprintf(w, "Class: %s\n", classification.ClassLabel())
	fmt.Fprintf(w, "Risk: %s\n", classification.Risk)
	fmt.Fprintf(w, "Recommended action: %s\n", classification.RecommendedAction)
	fmt.Fprintf(w, "Reason: %s\n", classification.Reason)
}
