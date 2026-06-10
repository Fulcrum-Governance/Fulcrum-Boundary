package boundarycli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	boundarydemo "github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
)

func runActionBoundaryDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo action-boundary", stderr, commandHelp{
		Purpose: "Run a fixture-only demo across MCP/Secure GitHub, Command Boundary, and Edit Boundary.",
		Usage:   "boundary demo action-boundary [--json|--markdown|--dashboard] [--out PATH]",
		Common: []string{
			"boundary demo action-boundary",
			"boundary demo action-boundary --json",
			"boundary demo action-boundary --markdown --out demo.md",
			"boundary demo action-boundary --dashboard --out .boundary/action-boundary-demo",
		},
		Notes: []string{
			"Fixture mode uses no credentials, no network, and no live mutation.",
			"The demo proves routed fixture denial, not global shell control, direct file-edit control, or live upstream conformance.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	markdownOutput := fs.Bool("markdown", false, "emit Markdown")
	dashboard := fs.Bool("dashboard", false, "write local-only HTML and JSON dashboard artifacts")
	outPath := fs.String("out", "", "write the demo report to a file, or dashboard artifacts to a directory")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	formatFlags := 0
	if *jsonOutput {
		formatFlags++
	}
	if *markdownOutput {
		formatFlags++
	}
	if *dashboard {
		formatFlags++
	}
	if formatFlags > 1 {
		fmt.Fprintln(stderr, "choose only one of --json, --markdown, or --dashboard")
		return 1
	}
	if *dashboard && *outPath == "" {
		fmt.Fprintln(stderr, "action-boundary demo: --dashboard requires --out directory")
		return 1
	}

	result, err := boundarydemo.RunActionBoundary(context.Background(), boundarydemo.ActionBoundaryOptions{})
	if err != nil {
		fmt.Fprintf(stderr, "action-boundary demo: %v\n", err)
		return 1
	}
	if *dashboard {
		htmlPath, jsonPath, err := writeActionBoundaryDashboardArtifacts(*outPath, result)
		if err != nil {
			fmt.Fprintf(stderr, "action-boundary demo: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "dashboard: %s\n", htmlPath)
		fmt.Fprintf(stdout, "demo json: %s\n", jsonPath)
		if !result.Passed {
			return 1
		}
		return 0
	}

	format := demoReportFormat(*outPath, *jsonOutput, *markdownOutput)
	var color *boundarydemo.Colorizer
	if *outPath == "" && format == "text" {
		color = boundarydemo.NewColorizer(stdout)
	}
	var report bytes.Buffer
	if err := writeActionBoundaryDemoReport(&report, result, format, color); err != nil {
		fmt.Fprintf(stderr, "action-boundary demo: %v\n", err)
		return 1
	}
	if *outPath == "" {
		if _, err := io.Copy(stdout, &report); err != nil {
			fmt.Fprintf(stderr, "action-boundary demo: %v\n", err)
			return 1
		}
	} else {
		if err := writeDemoReportFile(*outPath, report.Bytes()); err != nil {
			fmt.Fprintf(stderr, "action-boundary demo: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "demo report: %s\n", *outPath)
	}
	if !result.Passed {
		return 1
	}
	return 0
}

func writeActionBoundaryDemoReport(w io.Writer, result *boundarydemo.ActionBoundaryResult, format string, color *boundarydemo.Colorizer) error {
	switch format {
	case "json":
		return boundarydemo.WriteActionBoundaryJSON(w, result)
	case "markdown":
		return boundarydemo.WriteActionBoundaryMarkdown(w, result)
	case "text", "":
		return boundarydemo.WriteActionBoundaryTextColor(w, result, color)
	default:
		return fmt.Errorf("unsupported report format %q", format)
	}
}

func writeActionBoundaryDashboardArtifacts(outDir string, result *boundarydemo.ActionBoundaryResult) (htmlPath, jsonPath string, err error) {
	absDir, err := filepath.Abs(outDir)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return "", "", err
	}

	htmlPath = filepath.Join(absDir, "action-boundary-dashboard.html")
	jsonPath = filepath.Join(absDir, "action-boundary-demo.json")

	var htmlBody bytes.Buffer
	if err := boundarydemo.WriteActionBoundaryDashboard(&htmlBody, result); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(htmlPath, htmlBody.Bytes(), 0o600); err != nil {
		return "", "", err
	}

	var jsonBody bytes.Buffer
	if err := boundarydemo.WriteActionBoundaryJSON(&jsonBody, result); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(jsonPath, jsonBody.Bytes(), 0o600); err != nil {
		return "", "", err
	}
	return htmlPath, jsonPath, nil
}
