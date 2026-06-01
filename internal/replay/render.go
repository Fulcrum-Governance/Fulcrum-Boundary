package replay

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON emits the boundary.replay.v1 envelope as two-space-indented JSON,
// matching the doctor/selftest/explain JSON convention.
func WriteJSON(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("replay result is required")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// WriteText renders the replay Result as a human-readable account: the hash
// gates, the recorded-vs-reproduced decision-defining fields, an overall
// match/mismatch line, and the fixed limitation footer. It states explicitly
// that replay reproduces the decision, not enforcement.
func WriteText(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("replay result is required")
	}
	fmt.Fprintln(w, "Boundary replay")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "record schema_version: %s\n", result.RecordSchemaVersion)
	if result.RecordID != "" {
		fmt.Fprintf(w, "record_id: %s\n", result.RecordID)
	}
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w, "live mutation: none")

	fmt.Fprintln(w, "\nHash gates:")
	for _, check := range result.HashChecks {
		fmt.Fprintf(w, "  %s: %s\n", check.Field, matchLabel(check.Matched))
		fmt.Fprintf(w, "    recorded:   %s\n", check.Recorded)
		fmt.Fprintf(w, "    recomputed: %s\n", check.Recomputed)
	}

	fmt.Fprintln(w, "\nDecision fields (recorded vs reproduced):")
	for _, check := range result.FieldChecks {
		fmt.Fprintf(w, "  %s: %s\n", check.Field, matchLabel(check.Matched))
		fmt.Fprintf(w, "    recorded:   %q\n", check.Recorded)
		fmt.Fprintf(w, "    reproduced: %q\n", check.Reproduced)
	}

	if result.Matched {
		fmt.Fprintln(w, "\nresult: MATCH — the recorded request reproduced the recorded decision")
	} else {
		fmt.Fprintln(w, "\nresult: MISMATCH — the reproduced decision differs from the record")
		for _, reason := range result.Mismatches {
			fmt.Fprintf(w, "  - %s\n", reason)
		}
	}

	fmt.Fprintln(w, "\nWhat this does not prove:")
	for _, line := range result.DoesNotProve {
		fmt.Fprintf(w, "- %s\n", line)
	}
	return nil
}

// matchLabel renders a boolean match outcome as a stable token.
func matchLabel(matched bool) string {
	if matched {
		return "match"
	}
	return "MISMATCH"
}
