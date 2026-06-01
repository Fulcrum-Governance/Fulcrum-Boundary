package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/replay"
)

func runReplay(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary replay", stderr, commandHelp{
		Purpose: "Re-evaluate a recorded request locally and compare the reproduced decision against the record.",
		Usage:   "boundary replay [--json] <record.json> --request <request.json> --policies <dir>",
		Common: []string{
			"boundary replay record.json --request request.json --policies ./policies/",
			"boundary replay --json record.json --request request.json --policies ./policies/",
		},
		Notes: []string{
			"Replay is local-only and fixture-safe: no credentials, no network, no live mutation.",
			"The record carries request_hash but not the request body, so --request supplies the recorded GovernanceRequest JSON.",
			"Replay recomputes request_hash, recomputes policy_bundle_hash when the record carries one, rebuilds the request, and re-runs the same pipeline.",
			"Replay compares action, reason, decision_mode, matched_rule, and policy_file — not action alone; it exits non-zero on any mismatch.",
			"Replay reproduces the decision, not enforcement: a reproduced deny does not prove the action was blocked, and a match does not prove the verdict was correct.",
			"Replay does not prove no upstream bytes moved; direct access to the same tool is a bypass a record cannot see.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	requestPath := fs.String("request", "", "canonical GovernanceRequest JSON that was recorded (required)")
	policyDir := fs.String("policies", "", "policy directory the request is re-evaluated against (required)")
	// Parse with the positional record path allowed in any position. Go's flag
	// package stops at the first non-flag token, so a natural invocation like
	// `replay record.json --request r.json --policies dir` would otherwise leave
	// the flags unparsed. Collect the single positional and the flags in one pass.
	positionals, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if len(positionals) != 1 {
		fmt.Fprintln(stderr, "usage: boundary replay [--json] <record.json> --request <request.json> --policies <dir>")
		return 1
	}

	result, err := replay.Run(replay.Options{
		RecordPath:  positionals[0],
		RequestPath: *requestPath,
		PolicyDir:   *policyDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "replay: %v\n", err)
		return 1
	}

	if *jsonOutput {
		if err := replay.WriteJSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "replay: %v\n", err)
			return 1
		}
	} else if err := replay.WriteText(stdout, result); err != nil {
		fmt.Fprintf(stderr, "replay: %v\n", err)
		return 1
	}

	// Exit non-zero when the reproduced decision or a hash gate did not match,
	// even though emission succeeded — mirroring boundary selftest's
	// result-driven exit. The body (text or JSON) is still written so the caller
	// sees exactly which gate or field diverged.
	if !result.Matched {
		return 1
	}
	return 0
}
