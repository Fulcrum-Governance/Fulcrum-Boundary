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

	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

type configTarget struct {
	Path   string
	Client firewall.ClientType
}

func runFirewallInstall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary install", stderr)
	root := fs.String("root", ".", "project root to inspect for repo-local MCP configs")
	home := fs.String("home", "", "home directory to inspect for user MCP configs")
	clientFlag := fs.String("client", "", "client to install into: claude, cursor, vscode, repo, or custom")
	all := fs.Bool("all", false, "install into all discovered existing MCP configs")
	outDir := fs.String("out", ".boundary/firewall", "Boundary-owned firewall workspace directory")
	receipt := fs.String("receipt", "", "explicit install receipt path; only valid with one config")
	mode := fs.String("mode", "balanced", "install policy mode recorded in the Boundary proxy descriptor")
	boundaryCommand := fs.String("boundary-command", "boundary", "Boundary executable path to write into MCP configs")
	dryRun := fs.Bool("dry-run", false, "show install plan without changing configs, backups, or receipts")
	force := fs.Bool("force", false, "rewrite servers already routed through Boundary")
	format := fs.String("format", "text", "summary format: text or json")
	var configs pathListFlag
	var servers pathListFlag
	fs.Var(&configs, "config", "MCP config path; may be repeated or comma-separated")
	fs.Var(&servers, "server", "MCP server name to route; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	targets, err := resolveConfigTargets(*root, *home, *clientFlag, *all, []string(configs))
	if err != nil {
		fmt.Fprintf(stderr, "install: %v\n", err)
		return 1
	}
	if *receipt != "" && len(targets) != 1 {
		fmt.Fprintln(stderr, "install: --receipt requires exactly one config target")
		return 1
	}

	results := make([]firewall.InstallResult, 0, len(targets))
	for _, target := range targets {
		receiptPath := ""
		if len(targets) == 1 {
			receiptPath = *receipt
		}
		result, err := firewall.InstallConfig(firewall.InstallOptions{
			ConfigPath:      target.Path,
			Client:          target.Client,
			OutDir:          *outDir,
			ReceiptPath:     receiptPath,
			BoundaryCommand: *boundaryCommand,
			Mode:            *mode,
			Servers:         []string(servers),
			DryRun:          *dryRun,
			Force:           *force,
		})
		if err != nil {
			fmt.Fprintf(stderr, "install %s: %v\n", target.Path, err)
			return 1
		}
		results = append(results, result)
	}
	return writeInstallResults(results, *format, *dryRun, stdout, stderr)
}

func runFirewallUninstall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary uninstall", stderr)
	receipt := fs.String("receipt", "", "Boundary install receipt path")
	dryRun := fs.Bool("dry-run", false, "show restore plan without changing configs")
	force := fs.Bool("force", false, "restore even when the current config hash no longer matches the install receipt")
	format := fs.String("format", "text", "summary format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *receipt == "" {
		fmt.Fprintln(stderr, "uninstall: --receipt is required")
		return 1
	}
	result, err := firewall.UninstallConfig(firewall.UninstallOptions{ReceiptPath: *receipt, DryRun: *dryRun, Force: *force})
	if err != nil {
		fmt.Fprintf(stderr, "uninstall: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "json":
		body, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "uninstall: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(body, '\n'))
	case "text", "":
		if result.DryRun {
			fmt.Fprintf(stdout, "restore plan: %s <- %s (dry-run)\n", result.ConfigPath, result.BackupPath)
			fmt.Fprintln(stdout, "mcp config mutation: none")
			return 0
		}
		fmt.Fprintf(stdout, "restored config: %s\n", result.ConfigPath)
		fmt.Fprintf(stdout, "backup: %s\n", result.BackupPath)
		fmt.Fprintf(stdout, "restored servers: %s\n", strings.Join(result.Servers, ", "))
	default:
		fmt.Fprintf(stderr, "uninstall: unsupported summary format %q\n", *format)
		return 1
	}
	return 0
}

func writeInstallResults(results []firewall.InstallResult, format string, dryRun bool, stdout, stderr io.Writer) int {
	switch strings.ToLower(format) {
	case "json":
		body, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "install: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(body, '\n'))
	case "text", "":
		if dryRun {
			fmt.Fprintln(stdout, "install plan: dry-run")
			fmt.Fprintln(stdout, "mcp config mutation: none")
		} else {
			fmt.Fprintf(stdout, "installed configs: %d\n", len(results))
		}
		for _, result := range results {
			fmt.Fprintf(stdout, "- config: %s\n", result.ConfigPath)
			if result.BackupPath != "" {
				fmt.Fprintf(stdout, "  backup: %s\n", result.BackupPath)
			}
			if result.ReceiptPath != "" {
				fmt.Fprintf(stdout, "  receipt: %s\n", result.ReceiptPath)
			}
			names := make([]string, 0, len(result.Servers))
			for _, server := range result.Servers {
				names = append(names, server.Name)
			}
			fmt.Fprintf(stdout, "  routed servers: %s\n", strings.Join(names, ", "))
		}
	default:
		fmt.Fprintf(stderr, "install: unsupported summary format %q\n", format)
		return 1
	}
	return 0
}

func resolveConfigTargets(root, home, clientValue string, all bool, explicit []string) ([]configTarget, error) {
	if len(explicit) > 0 {
		targets := make([]configTarget, 0, len(explicit))
		client := parseClientType(clientValue)
		if client == "" {
			client = firewall.ClientCustom
		}
		for _, path := range explicit {
			abs, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			targets = append(targets, configTarget{Path: filepath.Clean(abs), Client: client})
		}
		return targets, nil
	}
	if !all && clientValue == "" {
		return nil, fmt.Errorf("provide --config, --client, or --all")
	}
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = userHome
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	client := parseClientType(clientValue)
	if clientValue != "" && client == "" {
		return nil, fmt.Errorf("unsupported client %q", clientValue)
	}
	var targets []configTarget
	seen := map[string]bool{}
	for _, candidate := range firewall.DefaultCandidates(absRoot, home) {
		if !all && client != "" && candidate.Client != client {
			continue
		}
		if _, err := os.Stat(candidate.Path); err != nil {
			continue
		}
		if seen[candidate.Path] {
			continue
		}
		seen[candidate.Path] = true
		targets = append(targets, configTarget{Path: candidate.Path, Client: candidate.Client})
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no existing MCP configs matched selection")
	}
	return targets, nil
}

func parseClientType(value string) firewall.ClientType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ""
	case "claude", "claude_desktop", "claude-desktop":
		return firewall.ClientClaudeDesktop
	case "cursor":
		return firewall.ClientCursor
	case "vscode", "vs-code", "code":
		return firewall.ClientVSCode
	case "repo", "repo_local", "repo-local":
		return firewall.ClientRepoLocal
	case "custom":
		return firewall.ClientCustom
	default:
		return ""
	}
}
