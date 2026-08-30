package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/explain"
)

func runExplain(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary explain", stderr, commandHelp{
		Purpose: "Describe a decision record (schema_version 1 or 2) without evaluating, executing, or verifying it.",
		Usage:   "boundary explain [--json] <record.json>",
		Common: []string{
			"boundary explain docs/examples/decision-record.example.json",
			"boundary explain --json docs/examples/decision-record-v2.example.json",
		},
		Notes: []string{
			"Explain is local-only and read-only: no credentials, no network, no live mutation.",
			"Explain renders a record; it does not verify the record's hashes. Run boundary verify-record to recompute them.",
			"Explain does not prove the verdict was correct and does not prove enforcement; direct access to the same tool is a bypass a record cannot see.",
			"Flags and the record path may appear in either order, mirroring boundary verify-record.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	// Interspersed on purpose, mirroring verify-record: the two record-reading
	// verbs must accept the same shapes, and Command Boundary's first-party
	// classification recognizes exactly the shapes the CLIs accept.
	positionals, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if len(positionals) != 1 {
		fmt.Fprintln(stderr, "usage: boundary explain [--json] <record.json>")
		return 1
	}

	result, err := explain.Run(explain.Options{Path: positionals[0]})
	if err != nil {
		fmt.Fprintf(stderr, "explain: %v\n", err)
		return 1
	}

	if *jsonOutput {
		if err := explain.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "explain: %v\n", err)
			return 1
		}
		return 0
	}
	if err := explain.WriteText(stdout, result); err != nil {
		fmt.Fprintf(stderr, "explain: %v\n", err)
		return 1
	}
	return 0
}
