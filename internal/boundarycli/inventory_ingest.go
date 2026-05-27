package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

func runFirewallInventoryIngest(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary inventory ingest", stderr, commandHelp{
		Purpose: "Ingest external MCP inventory NDJSON into Boundary firewall inventory records.",
		Usage:   "boundary inventory ingest --file inventory.ndjson [--source external-mcp] [--summary]",
		Common: []string{
			"boundary inventory ingest --file fixtures/external-inventory/external-mcp-inventory.ndjson --source external-mcp --summary",
		},
		Notes: []string{
			"External inventory is input data; Boundary does not depend on or endorse a scanner.",
			"Partial snapshots keep install recommendations disabled unless --allow-partial is set.",
		},
	})
	file := fs.String("file", "", "external MCP inventory NDJSON file to ingest")
	source := fs.String("source", "boundary", "inventory source: boundary, generic, or external-mcp")
	format := fs.String("format", "json", "ingest report format: json")
	out := fs.String("out", "", "write ingest report to a file instead of stdout")
	summary := fs.Bool("summary", false, "print a human-readable ingest summary")
	allowPartial := fs.Bool("allow-partial", false, "enable install recommendations for partial external snapshots")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	if strings.ToLower(strings.TrimSpace(*format)) != "json" {
		fmt.Fprintf(stderr, "inventory ingest: unsupported format %q\n", *format)
		return 1
	}
	result, err := firewall.IngestExternalInventoryFile(firewall.ExternalInventoryIngestOptions{
		File:         *file,
		Source:       *source,
		AllowPartial: *allowPartial,
	})
	if err != nil {
		fmt.Fprintf(stderr, "inventory ingest: %v\n", err)
		return 1
	}
	body, err := firewall.RenderExternalInventoryIngestJSON(result)
	if err != nil {
		fmt.Fprintf(stderr, "inventory ingest: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	if *out != "" {
		if err := os.WriteFile(*out, body, 0o600); err != nil {
			fmt.Fprintf(stderr, "write inventory ingest report: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "inventory ingest report: %s\n", *out)
		if *summary {
			fmt.Fprint(stdout, firewall.RenderExternalInventoryIngestSummary(result))
		}
		return 0
	}
	if *summary {
		fmt.Fprint(stdout, firewall.RenderExternalInventoryIngestSummary(result))
		return 0
	}
	_, _ = stdout.Write(body)
	return 0
}
