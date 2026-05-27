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
	"strings"

	boundarydemo "github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
)

func runGitHubLethalTrifectaDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo github-lethal-trifecta", stderr, commandHelp{
		Purpose: "Run a fixture-only Secure GitHub denial demo for write-after-taint behavior.",
		Usage:   "boundary demo github-lethal-trifecta [--json|--markdown] [--out PATH] [--dashboard]",
		Common: []string{
			"boundary demo github-lethal-trifecta",
			"boundary demo github-lethal-trifecta --markdown --out demo.md",
		},
		Notes: []string{
			"Fixture mode uses no credentials, no network, and no live GitHub mutation.",
			"The demo proves pre-upstream denial for the fixture route, not live GitHub App conformance.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	markdownOutput := fs.Bool("markdown", false, "emit Markdown")
	outPath := fs.String("out", "", "write the demo report to a file")
	dashboard := fs.Bool("dashboard", false, "write a local-only HTML dashboard artifact")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *jsonOutput && *markdownOutput {
		fmt.Fprintln(stderr, "choose only one of --json or --markdown")
		return 1
	}

	result, err := boundarydemo.RunGitHubLethalTrifecta(context.Background(), boundarydemo.GitHubLethalTrifectaOptions{
		OutPath:   *outPath,
		Dashboard: *dashboard,
	})
	if err != nil {
		fmt.Fprintf(stderr, "github lethal-trifecta demo: %v\n", err)
		return 1
	}

	format := demoReportFormat(*outPath, *jsonOutput, *markdownOutput)
	var report bytes.Buffer
	if err := writeGitHubDemoReport(&report, result, format); err != nil {
		fmt.Fprintf(stderr, "github lethal-trifecta demo: %v\n", err)
		return 1
	}
	if *outPath == "" {
		if _, err := io.Copy(stdout, &report); err != nil {
			fmt.Fprintf(stderr, "github lethal-trifecta demo: %v\n", err)
			return 1
		}
	} else {
		if err := writeDemoReportFile(result.ReportPath, report.Bytes()); err != nil {
			fmt.Fprintf(stderr, "github lethal-trifecta demo: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "demo report: %s\n", result.ReportPath)
		if result.DashboardPath != "" {
			fmt.Fprintf(stdout, "dashboard: %s\n", result.DashboardPath)
		}
	}
	if !result.Passed {
		return 1
	}
	return 0
}

func demoReportFormat(outPath string, jsonOutput, markdownOutput bool) string {
	if jsonOutput {
		return "json"
	}
	if markdownOutput {
		return "markdown"
	}
	switch strings.ToLower(filepath.Ext(outPath)) {
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	default:
		return "text"
	}
}

func writeGitHubDemoReport(w io.Writer, result *boundarydemo.GitHubLethalTrifectaResult, format string) error {
	switch format {
	case "json":
		return boundarydemo.WriteGitHubLethalTrifectaJSON(w, result)
	case "markdown":
		return boundarydemo.WriteGitHubLethalTrifectaMarkdown(w, result)
	case "text", "":
		return boundarydemo.WriteGitHubLethalTrifectaText(w, result)
	default:
		return fmt.Errorf("unsupported report format %q", format)
	}
}

func writeDemoReportFile(path string, body []byte) error {
	if path == "" {
		return fmt.Errorf("report path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}
