package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/doctor"
)

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary doctor", stderr, commandHelp{
		Purpose: "Check local routed-surface diagnostics without credentials, network calls, or live mutation.",
		Usage:   "boundary doctor [--surface all|mcp|command|edit] [--json]",
		Common: []string{
			"boundary doctor",
			"boundary doctor --surface mcp",
			"boundary doctor --surface command",
			"boundary doctor --surface edit",
			"boundary doctor --json",
		},
		Notes: []string{
			"Doctor reports local readiness and bypass caveats; it does not prove production deployment protection.",
			"Direct upstream MCP access, direct shell commands, and direct file edits remain bypasses unless operators remove those paths.",
		},
	})
	surface := fs.String("surface", "all", "surface to diagnose: all, mcp, command, or edit")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	result, err := doctor.Run(doctor.Options{Surface: *surface})
	if err != nil {
		fmt.Fprintf(stderr, "doctor: %v\n", err)
		return 1
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "doctor: %v\n", err)
			return 1
		}
		return 0
	}
	if err := writeDoctorText(stdout, result); err != nil {
		fmt.Fprintf(stderr, "doctor: %v\n", err)
		return 1
	}
	return 0
}

func writeDoctorText(w io.Writer, result *doctor.Result) error {
	if result == nil {
		return fmt.Errorf("doctor result is required")
	}
	fmt.Fprintln(w, "Boundary doctor")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "project: %s\n", result.ProjectRoot)
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w, "live mutation: none")
	for _, surface := range result.Surfaces {
		fmt.Fprintf(w, "\nSurface: %s\n", surface.Label)
		fmt.Fprintf(w, "status: %s\n", surface.Status)
		for _, check := range surface.Checks {
			fmt.Fprintf(w, "- %s %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Detail)
		}
		fmt.Fprintln(w, "Bypass caveats:")
		for _, caveat := range surface.BypassCaveats {
			fmt.Fprintf(w, "- %s\n", caveat)
		}
	}
	return nil
}
