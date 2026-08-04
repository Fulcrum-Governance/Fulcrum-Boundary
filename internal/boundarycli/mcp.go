package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func runFirewallMCP(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, `Fulcrum Boundary MCP helpers

Usage:
  boundary mcp <command> [flags]

Commands:
  proxy          Fail-closed generic MCP proxy entrypoint used by boundary install
  secure-github  Run the stateful local Boundary route to official GitHub MCP

Use "boundary mcp <command> --help" for command flags.
`)
		return 0
	}
	switch args[0] {
	case "proxy":
		return runFirewallMCPProxy(args[1:], stdout, stderr)
	case "secure-github":
		return runSecureGitHubMCP(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown mcp command %q\n", args[0])
		return 1
	}
}

func runSecureGitHubMCP(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary mcp secure-github", stderr)
	listen := fs.String("listen", "127.0.0.1:8080", "local HTTP MCP listen address")
	upstream := fs.String("upstream", securegithub.GitHubMCPRemoteURL, "official GitHub MCP endpoint")
	tokenEnv := fs.String("token-env", "GITHUB_PERSONAL_ACCESS_TOKEN", "environment variable containing the upstream token")
	sourceOwner := fs.String("source-owner", "", "configured untrusted source issue owner")
	sourceRepo := fs.String("source-repo", "", "configured untrusted source issue repository")
	sourceIssue := fs.Int("source-issue", 0, "configured untrusted source issue number")
	targetOwner := fs.String("target-owner", "", "protected target repository owner")
	targetRepo := fs.String("target-repo", "", "protected target repository")
	targetBranch := fs.String("target-branch", "", "protected target branch")
	records := fs.String("decision-records", ".boundary/secure-github/decision-records.jsonl", "append-only decision record path")
	forwardEvents := fs.String("forward-events", ".boundary/secure-github/forward-events.jsonl", "independent forwarder event path")
	timeout := fs.Duration("timeout", 30*time.Second, "per-request upstream timeout")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *sourceOwner == "" || *sourceRepo == "" || *sourceIssue <= 0 || *targetOwner == "" || *targetRepo == "" || *targetBranch == "" {
		fmt.Fprintln(stderr, "secure-github: --source-owner, --source-repo, --source-issue, --target-owner, --target-repo, and --target-branch are required")
		return 1
	}
	if !isLoopbackListen(*listen) {
		fmt.Fprintln(stderr, "secure-github: --listen must use a loopback address")
		return 1
	}
	if !isSecureMCPUpstream(*upstream) {
		fmt.Fprintln(stderr, "secure-github: --upstream must use HTTPS (HTTP is allowed only for loopback test endpoints)")
		return 1
	}
	token, ok := os.LookupEnv(*tokenEnv)
	if !ok || strings.TrimSpace(token) == "" {
		fmt.Fprintf(stderr, "NOT READY: SECURE_GITHUB_MCP_TOKEN_MISSING (%s)\n", *tokenEnv)
		return 1
	}
	evidence, err := securegithub.NewMCPJSONLEvidenceSink(*records, *forwardEvents)
	if err != nil {
		fmt.Fprintf(stderr, "secure-github: evidence configuration: %v\n", err)
		return 1
	}
	route, err := securegithub.NewMCPRoute(securegithub.MCPRouteConfig{
		UpstreamURL: *upstream, Token: token, SourceOwner: *sourceOwner, SourceRepo: *sourceRepo, SourceIssue: *sourceIssue,
		TargetOwner: *targetOwner, TargetRepo: *targetRepo, TargetBranch: *targetBranch,
		GatewayVersion: Version, Timeout: *timeout, DecisionSink: evidence, ForwardSink: evidence,
	})
	if err != nil {
		fmt.Fprintf(stderr, "secure-github: route configuration: %v\n", err)
		return 1
	}
	server := &http.Server{
		Addr:              *listen,
		Handler:           route,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	fmt.Fprintf(stdout, "READY: Secure GitHub MCP route listening on http://%s/mcp\n", *listen)
	fmt.Fprintf(stdout, "source=%s/%s#%d target=%s/%s@%s\n", *sourceOwner, *sourceRepo, *sourceIssue, *targetOwner, *targetRepo, *targetBranch)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "secure-github: serve: %v\n", err)
		return 1
	}
	return 0
}

func isLoopbackListen(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isSecureMCPUpstream(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback()))
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
