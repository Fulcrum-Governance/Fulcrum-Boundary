package selftest

import (
	"encoding/json"
	"fmt"
	"io"
)

type RenderOptions struct {
	NoColor bool
}

func WriteText(w io.Writer, result *Result, opts RenderOptions) error {
	_ = opts
	fmt.Fprintln(w, "Boundary selftest")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintln(w, "live mutation: none")
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w)
	for _, check := range result.Checks {
		fmt.Fprintf(w, "[%s] %s - %s\n", check.Status, check.ID, check.Detail)
		if check.Status != StatusPass && check.Command != "" {
			fmt.Fprintf(w, "  rerun: %s\n", check.Command)
		}
	}
	fmt.Fprintln(w)
	for _, next := range result.Next {
		fmt.Fprintf(w, "next: %s\n", next)
	}
	return nil
}

func WriteJSON(w io.Writer, result *Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
