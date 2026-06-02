package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarytest"
)

func runTest(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary test", stderr, commandHelp{
		Purpose: "Run local policy-as-code test cases against Boundary policy bundles.",
		Usage:   "boundary test [--path .boundary/tests] [--format text|json]",
		Common: []string{
			"boundary test --path .boundary/tests",
			"boundary test --path .boundary/tests --format json",
		},
		Notes: []string{
			"Test is local-only and fixture-safe: no credentials, no network, no live mutation.",
			"Each case names a local policy bundle, a GovernanceRequest fixture, and an expected verdict.",
			"Test evaluates requests through the existing Boundary governance pipeline and exits non-zero on any mismatch.",
			"Test reports policy verdicts for routed request fixtures; it does not prove production route enforcement or deployment bypass resistance.",
		},
	})
	path := fs.String("path", ".boundary/tests", "directory containing YAML policy test cases")
	format := fs.String("format", "text", "output format: text or json")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON (alias for --format json)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: boundary test [--path .boundary/tests] [--format text|json]")
		return 1
	}

	outputFormat := *format
	if *jsonOutput {
		outputFormat = "json"
	}
	if outputFormat != "text" && outputFormat != "json" {
		fmt.Fprintf(stderr, "test: unsupported format %q (want text or json)\n", outputFormat)
		return 1
	}

	result, err := boundarytest.Run(boundarytest.Options{Path: *path})
	if err != nil {
		fmt.Fprintf(stderr, "test: %v\n", err)
		return 1
	}
	if outputFormat == "json" {
		if err := boundarytest.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "test: %v\n", err)
			return 1
		}
	} else if err := boundarytest.WriteText(stdout, result); err != nil {
		fmt.Fprintf(stderr, "test: %v\n", err)
		return 1
	}
	if result.Status != "pass" {
		return 1
	}
	return 0
}
