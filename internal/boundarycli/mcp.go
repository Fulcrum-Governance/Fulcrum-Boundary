package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

func runFirewallMCP(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, `Fulcrum Boundary MCP helpers

Usage:
  boundary mcp <command> [flags]

Commands:
  proxy   Fail-closed generic MCP proxy entrypoint used by boundary install

Use "boundary mcp <command> --help" for command flags.
`)
		return 0
	}
	switch args[0] {
	case "proxy":
		return runFirewallMCPProxy(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown mcp command %q\n", args[0])
		return 1
	}
}

func runFirewallMCPProxy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary mcp proxy", stderr)
	receipt := fs.String("install-receipt", "", "Boundary install receipt path")
	server := fs.String("server", "", "MCP server name from the install receipt")
	mode := fs.String("mode", "balanced", "policy mode recorded during install")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *receipt == "" || *server == "" {
		fmt.Fprintln(stderr, "mcp proxy: --install-receipt and --server are required")
		return 1
	}
	fmt.Fprintf(stderr, "mcp proxy: generic installed route for server %q is fail-closed in %s mode; configure a Secure MCP profile before live forwarding\n", *server, *mode)
	return 1
}
