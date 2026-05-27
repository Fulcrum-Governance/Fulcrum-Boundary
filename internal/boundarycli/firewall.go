package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

type pathListFlag []string

func (f *pathListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *pathListFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*f = append(*f, part)
		}
	}
	return nil
}

func runFirewallInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary init", stderr)
	root := fs.String("root", ".", "project root to inspect for repo-local MCP configs")
	home := fs.String("home", "", "home directory to inspect for user MCP configs")
	outDir := fs.String("out", ".boundary/firewall", "Boundary-owned firewall workspace directory")
	dryRun := fs.Bool("dry-run", false, "print initialization plan without writing files")
	includeDefaults := fs.Bool("include-defaults", true, "include known Claude Desktop, Cursor, VS Code, and repo-local config paths")
	var configs pathListFlag
	fs.Var(&configs, "config", "additional MCP config path; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	inventory, err := firewall.BuildInventory(firewall.DiscoverOptions{
		Root:                  *root,
		Home:                  *home,
		AdditionalConfigPaths: []string(configs),
		IncludeDefaults:       *includeDefaults,
	})
	if err != nil {
		fmt.Fprintf(stderr, "firewall init: %v\n", err)
		return 1
	}

	absOut, err := filepath.Abs(*outDir)
	if err != nil {
		fmt.Fprintf(stderr, "firewall init: %v\n", err)
		return 1
	}
	if !*dryRun {
		if err := os.MkdirAll(absOut, 0o700); err != nil {
			fmt.Fprintf(stderr, "create firewall workspace: %v\n", err)
			return 1
		}
		initPath := filepath.Join(absOut, "boundary-firewall.json")
		body, err := json.MarshalIndent(map[string]any{
			"schema_version":      "boundary.firewall.init.v1",
			"created_at":          time.Now().UTC().Format(time.RFC3339),
			"root":                inventory.Root,
			"configs_discovered":  inventory.Summary.ConfigFiles,
			"servers_discovered":  inventory.Summary.Servers,
			"github_servers":      inventory.Summary.GitHubServers,
			"high_risk_servers":   inventory.Summary.HighRiskServers,
			"mutates_mcp_configs": false,
		}, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "encode firewall init: %v\n", err)
			return 1
		}
		if err := os.WriteFile(initPath, append(body, '\n'), 0o600); err != nil {
			fmt.Fprintf(stderr, "write firewall init: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "firewall workspace: %s\n", absOut)
	} else {
		fmt.Fprintf(stdout, "firewall workspace: %s (dry-run)\n", absOut)
	}

	fmt.Fprintf(stdout, "configs discovered: %d\n", inventory.Summary.ConfigFiles)
	fmt.Fprintf(stdout, "servers discovered: %d\n", inventory.Summary.Servers)
	fmt.Fprintf(stdout, "github servers: %d\n", inventory.Summary.GitHubServers)
	fmt.Fprintf(stdout, "high-risk servers: %d\n", inventory.Summary.HighRiskServers)
	fmt.Fprintln(stdout, "mcp config mutation: none")
	fmt.Fprintln(stdout, "next: boundary inventory --format markdown")
	return 0
}

func runFirewallInventory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary inventory", stderr)
	root := fs.String("root", ".", "project root to inspect for repo-local MCP configs")
	home := fs.String("home", "", "home directory to inspect for user MCP configs")
	format := fs.String("format", "json", "inventory format: json, ndjson, markdown, or sarif")
	out := fs.String("out", "", "write inventory report to a file instead of stdout")
	includeDefaults := fs.Bool("include-defaults", true, "include known Claude Desktop, Cursor, VS Code, and repo-local config paths")
	var configs pathListFlag
	fs.Var(&configs, "config", "additional MCP config path; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	inventory, err := firewall.BuildInventory(firewall.DiscoverOptions{
		Root:                  *root,
		Home:                  *home,
		AdditionalConfigPaths: []string(configs),
		IncludeDefaults:       *includeDefaults,
	})
	if err != nil {
		fmt.Fprintf(stderr, "inventory: %v\n", err)
		return 1
	}
	body, err := firewall.RenderInventory(inventory, *format)
	if err != nil {
		fmt.Fprintf(stderr, "inventory: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	if *out == "" {
		_, _ = stdout.Write(body)
		return 0
	}
	if err := os.WriteFile(*out, body, 0o600); err != nil {
		fmt.Fprintf(stderr, "write inventory: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "inventory report: %s\n", *out)
	return 0
}

func runFirewallGraph(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary graph", stderr)
	root := fs.String("root", ".", "project root to inspect for repo-local MCP configs")
	home := fs.String("home", "", "home directory to inspect for user MCP configs")
	format := fs.String("format", "json", "graph format: json or mermaid")
	out := fs.String("out", "", "write graph report to a file instead of stdout")
	includeDefaults := fs.Bool("include-defaults", true, "include known Claude Desktop, Cursor, VS Code, and repo-local config paths")
	var configs pathListFlag
	fs.Var(&configs, "config", "additional MCP config path; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	inventory, err := firewall.BuildInventory(firewall.DiscoverOptions{
		Root:                  *root,
		Home:                  *home,
		AdditionalConfigPaths: []string(configs),
		IncludeDefaults:       *includeDefaults,
	})
	if err != nil {
		fmt.Fprintf(stderr, "graph: %v\n", err)
		return 1
	}
	graph := firewall.BuildRiskGraph(inventory)
	body, err := firewall.RenderRiskGraph(graph, *format)
	if err != nil {
		fmt.Fprintf(stderr, "graph: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	if *out == "" {
		_, _ = stdout.Write(body)
		return 0
	}
	if err := os.WriteFile(*out, body, 0o600); err != nil {
		fmt.Fprintf(stderr, "write graph: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "risk graph: %s\n", *out)
	return 0
}

func runFirewallPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, `Fulcrum Boundary policy

Usage:
  boundary policy <command> [flags]

Commands:
  generate   Generate starter Boundary firewall policies

Use "boundary policy <command> --help" for command flags.
`)
		return 0
	}
	switch args[0] {
	case "generate":
		return runFirewallPolicyGenerate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown policy command %q\n", args[0])
		return 1
	}
}

func runFirewallPolicyGenerate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary policy generate", stderr)
	outDir := fs.String("out", "boundary-firewall-policies", "directory to write starter policy YAML files")
	force := fs.Bool("force", false, "overwrite existing starter policy files")
	mode := fs.String("mode", "balanced", "starter policy mode: balanced")
	format := fs.String("format", "text", "summary format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	result, err := firewall.GenerateStarterPolicies(*outDir, *force, *mode)
	if err != nil {
		fmt.Fprintf(stderr, "policy generate: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "text", "":
		fmt.Fprintf(stdout, "policy directory: %s\n", result.Directory)
		fmt.Fprintf(stdout, "mode: %s\n", result.Mode)
		fmt.Fprintf(stdout, "starter policies: %d\n", len(result.Files))
		for _, file := range result.Files {
			fmt.Fprintf(stdout, "- %s\n", file)
		}
		fmt.Fprintf(stdout, "verify: boundary verify --policies %s\n", result.Directory)
	case "json":
		body, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "policy generate: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(body, '\n'))
	default:
		fmt.Fprintf(stderr, "policy generate: unsupported summary format %q\n", *format)
		return 1
	}
	return 0
}
