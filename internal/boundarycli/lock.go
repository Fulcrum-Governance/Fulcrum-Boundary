package boundarycli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

func runFirewallLock(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary lock", stderr)
	configPath := fs.String("config", "", "MCP config path to lock")
	clientFlag := fs.String("client", "custom", "client type recorded in the lock: claude, cursor, vscode, repo, or custom")
	out := fs.String("out", ".boundary/firewall/locks/descriptor-lock.json", "descriptor lockfile path")
	dryRun := fs.Bool("dry-run", false, "show lockfile content without writing it")
	format := fs.String("format", "text", "summary format: text or json")
	var servers pathListFlag
	fs.Var(&servers, "server", "MCP server name to lock; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *configPath == "" {
		fmt.Fprintln(stderr, "lock: --config is required")
		return 1
	}
	client := parseClientType(*clientFlag)
	if client == "" {
		fmt.Fprintf(stderr, "lock: unsupported client %q\n", *clientFlag)
		return 1
	}
	result, err := firewall.CreateDescriptorLock(firewall.LockOptions{
		ConfigPath: *configPath,
		Client:     client,
		OutPath:    *out,
		Servers:    []string(servers),
		DryRun:     *dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "lock: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "json":
		body, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "lock: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(body, '\n'))
	case "text", "":
		if result.DryRun {
			fmt.Fprintf(stdout, "descriptor lock: %d servers (dry-run)\n", len(result.LockFile.Servers))
			fmt.Fprintln(stdout, "lockfile mutation: none")
			return 0
		}
		fmt.Fprintf(stdout, "descriptor lock: %s\n", result.Path)
		fmt.Fprintf(stdout, "servers locked: %d\n", len(result.LockFile.Servers))
	default:
		fmt.Fprintf(stderr, "lock: unsupported summary format %q\n", *format)
		return 1
	}
	return 0
}

func runFirewallVerifyLock(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary verify-lock", stderr)
	lockPath := fs.String("lock", ".boundary/firewall/locks/descriptor-lock.json", "descriptor lockfile path")
	configPath := fs.String("config", "", "override MCP config path to verify against")
	onChange := fs.String("on-change", "deny", "descriptor drift behavior: warn, require_approval, or deny")
	format := fs.String("format", "text", "summary format: text or json")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	result, err := firewall.VerifyDescriptorLock(firewall.VerifyLockOptions{
		LockPath:   *lockPath,
		ConfigPath: *configPath,
		OnChange:   *onChange,
	})
	if err != nil {
		fmt.Fprintf(stderr, "verify-lock: %v\n", err)
		return 1
	}
	switch strings.ToLower(*format) {
	case "json":
		body, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "verify-lock: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(append(body, '\n'))
	case "text", "":
		fmt.Fprintf(stdout, "lock status: %s\n", result.Status)
		fmt.Fprintf(stdout, "policy behavior: %s\n", result.OnChange)
		fmt.Fprintf(stdout, "unchanged: %d changed: %d missing: %d unexpected: %d\n",
			result.Summary.Unchanged, result.Summary.Changed, result.Summary.Missing, result.Summary.Unexpected)
		for _, match := range result.Matches {
			if match.Status != "unchanged" {
				fmt.Fprintf(stdout, "- %s: %s\n", match.Name, match.Status)
			}
		}
	default:
		fmt.Fprintf(stderr, "verify-lock: unsupported summary format %q\n", *format)
		return 1
	}
	if !result.Allowed {
		return 1
	}
	return 0
}
