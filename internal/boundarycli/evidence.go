package boundarycli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/evidence"
)

func runEvidence(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printEvidenceHelp(stdout)
		return 0
	}
	switch args[0] {
	case "bundle":
		return runEvidenceBundle(args[1:], stdout, stderr)
	case "verify":
		return runEvidenceVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evidence: unknown subcommand %q\n\n", args[0])
		printEvidenceHelp(stderr)
		return 1
	}
}

func printEvidenceHelp(w io.Writer) {
	fmt.Fprint(w, `Bundle and verify local Boundary evidence artifacts.

Usage:
  boundary evidence <command> [flags]

Commands:
  bundle    Create a local fixture-safe evidence bundle
  verify    Verify a Boundary evidence bundle manifest and artifact hashes

Common usage:
  boundary evidence bundle
  boundary evidence bundle --from .boundary --out boundary-evidence
  boundary evidence bundle --include-demo --json
  boundary evidence verify boundary-evidence
  boundary evidence verify boundary-evidence --json

Notes:
  - Evidence bundles are local artifacts; they do not prove production deployment protection.
  - Default bundle contents require no credentials, no network calls, and no live mutation.
`)
}

func runEvidenceBundle(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary evidence bundle", stderr, commandHelp{
		Purpose: "Create a local fixture-safe Boundary evidence bundle.",
		Usage:   "boundary evidence bundle [--from .boundary] [--out boundary-evidence] [--include-demo] [--json]",
		Common: []string{
			"boundary evidence bundle",
			"boundary evidence bundle --from .boundary",
			"boundary evidence bundle --out boundary-evidence",
			"boundary evidence bundle --include-demo",
			"boundary evidence bundle --json",
		},
		Notes: []string{
			"Bundle generation writes local files only and requires no credentials, no network calls, and no live mutation.",
			"Fixture-safe outputs include version, selftest, and doctor; --include-demo adds the action-boundary demo.",
			"Existing source artifacts are copied from --from when that directory exists.",
		},
	})
	source := fs.String("from", ".boundary", "source directory containing existing Boundary artifacts")
	out := fs.String("out", "boundary-evidence", "output directory for the evidence bundle")
	includeDemo := fs.Bool("include-demo", false, "include fixture-only action-boundary demo artifacts")
	jsonOutput := fs.Bool("json", false, "emit machine-readable manifest JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "evidence bundle: unexpected argument %q\n", fs.Arg(0))
		return 1
	}
	result, err := evidence.Bundle(context.Background(), evidence.BundleOptions{
		SourceDir:   *source,
		OutDir:      *out,
		IncludeDemo: *includeDemo,
	})
	if err != nil {
		fmt.Fprintf(stderr, "evidence bundle: %v\n", err)
		return 1
	}
	if *jsonOutput {
		if err := writeIndentedJSON(stdout, result.Manifest); err != nil {
			fmt.Fprintf(stderr, "evidence bundle: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "Boundary evidence bundle")
	fmt.Fprintf(stdout, "status: pass\n")
	fmt.Fprintf(stdout, "output: %s\n", result.Manifest.Output)
	fmt.Fprintf(stdout, "manifest: %s\n", result.ManifestPath)
	fmt.Fprintf(stdout, "artifacts: %d\n", len(result.Manifest.Artifacts))
	fmt.Fprintln(stdout, "credentials: none")
	fmt.Fprintln(stdout, "network: none")
	fmt.Fprintln(stdout, "live mutation: none")
	if len(result.Manifest.Warnings) > 0 {
		fmt.Fprintln(stdout, "warnings:")
		for _, warning := range result.Manifest.Warnings {
			fmt.Fprintf(stdout, "- %s\n", warning)
		}
	}
	return 0
}

func runEvidenceVerify(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary evidence verify", stderr, commandHelp{
		Purpose: "Verify a Boundary evidence bundle manifest and artifact hashes.",
		Usage:   "boundary evidence verify <bundle-dir> [--json]",
		Common: []string{
			"boundary evidence verify boundary-evidence",
			"boundary evidence verify boundary-evidence --json",
		},
		Notes: []string{
			"Verification checks manifest schema, artifact existence, SHA-256 hashes, JSON schemas, record parseability, and summary references.",
			"Verification is local-only and does not call remote services.",
		},
	})
	fs.Bool("json", false, "emit machine-readable verification JSON")
	jsonOutput, positional, help, err := parseEvidenceVerifyArgs(args)
	if help {
		fs.Usage()
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "evidence verify: %v\n", err)
		return 1
	}
	if len(positional) != 1 {
		fmt.Fprintln(stderr, "evidence verify: bundle directory is required")
		return 1
	}
	result, err := evidence.Verify(evidence.VerifyOptions{BundleDir: positional[0]})
	if err != nil {
		fmt.Fprintf(stderr, "evidence verify: %v\n", err)
		return 1
	}
	if jsonOutput {
		if err := writeIndentedJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "evidence verify: %v\n", err)
			return 1
		}
		if result.Status != "pass" {
			return 1
		}
		return 0
	}
	writeEvidenceVerifyText(stdout, result)
	if result.Status != "pass" {
		return 1
	}
	return 0
}

func parseEvidenceVerifyArgs(args []string) (jsonOutput bool, positional []string, help bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--json", "-json":
			jsonOutput = true
		case "--help", "-h", "help":
			return false, nil, true, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return false, nil, false, fmt.Errorf("unknown flag %s", arg)
			}
			positional = append(positional, arg)
		}
	}
	return jsonOutput, positional, false, nil
}

func writeEvidenceVerifyText(w io.Writer, result *evidence.VerifyResult) {
	fmt.Fprintln(w, "Boundary evidence verify")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "bundle: %s\n", result.Bundle)
	fmt.Fprintf(w, "artifacts: %d\n", result.ArtifactCount)
	fmt.Fprintf(w, "verified_artifacts: %d\n", result.VerifiedArtifacts)
	fmt.Fprintf(w, "parsed_records: %d\n", result.ParsedRecords)
	for _, check := range result.Checks {
		fmt.Fprintf(w, "- %s %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
	}
}

func writeIndentedJSON(w io.Writer, payload any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
