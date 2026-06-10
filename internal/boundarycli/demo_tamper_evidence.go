package boundarycli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	boundarydemo "github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
)

// runTamperEvidenceDemo runs `boundary demo tamper-evidence`: a convenience
// wrapper that emits a hash-verifiable decision record, verifies it, tampers a
// single field, and shows that re-verification fails on the decision_hash
// mismatch. It is fixture-only — no credentials, no network, no live mutation —
// and is the one-command form of the manual "forge the receipt" sequence the
// README documents.
func runTamperEvidenceDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo tamper-evidence", stderr, commandHelp{
		Purpose: "Run a fixture-only tamper-evidence demo: emit a hash-verifiable record, verify it, forge one field, and show the mismatch.",
		Usage:   "boundary demo tamper-evidence [--json]",
		Common: []string{
			"boundary demo tamper-evidence",
			"boundary demo tamper-evidence --json",
		},
		Notes: []string{
			"Fixture mode uses no credentials, no network, and no live mutation.",
			"This demonstrates hash-verifiable tamper detection, not tamper-proof or immutable storage; verification confirms a record matches its decision_hash, it does not attest deployment topology.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	result, err := boundarydemo.RunTamperEvidence(context.Background(), boundarydemo.TamperEvidenceOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "tamper-evidence demo: %v\n", err)
		return 1
	}

	var report bytes.Buffer
	if *jsonOutput {
		if err := boundarydemo.WriteTamperEvidenceJSON(&report, result); err != nil {
			fmt.Fprintf(stderr, "tamper-evidence demo: %v\n", err)
			return 1
		}
	} else {
		color := boundarydemo.NewColorizer(stdout)
		if err := boundarydemo.WriteTamperEvidenceTextColor(&report, result, color); err != nil {
			fmt.Fprintf(stderr, "tamper-evidence demo: %v\n", err)
			return 1
		}
	}
	if _, err := io.Copy(stdout, &report); err != nil {
		fmt.Fprintf(stderr, "tamper-evidence demo: %v\n", err)
		return 1
	}

	if !result.Passed {
		return 1
	}
	return 0
}
