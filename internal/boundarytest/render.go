package boundarytest

import (
	"encoding/json"
	"fmt"
	"io"
)

func WriteJSON(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("boundary test result is required")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteText(w io.Writer, result *Result) error {
	if result == nil {
		return fmt.Errorf("boundary test result is required")
	}
	fmt.Fprintf(w, "boundary test: %s\n", result.Path)
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w, "live mutation: none")
	for _, c := range result.Cases {
		fmt.Fprintf(w, "  [%s] %-28s expect=%-16s actual=%s", c.Status, c.Name, c.ExpectedAction, c.ActualAction)
		if c.MatchedRule != "" {
			fmt.Fprintf(w, " matched_rule=%s", c.MatchedRule)
		}
		if c.Error != "" && c.Status != "pass" {
			fmt.Fprintf(w, " error=%q", c.Error)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "cases: %d\n", result.Summary.Total)
	fmt.Fprintf(w, "passed: %d\n", result.Summary.Passed)
	fmt.Fprintf(w, "failed: %d\n", result.Summary.Failed)
	fmt.Fprintln(w, "\nWhat this does not prove:")
	for _, line := range result.DoesNotProve {
		fmt.Fprintf(w, "- %s\n", line)
	}
	return nil
}
