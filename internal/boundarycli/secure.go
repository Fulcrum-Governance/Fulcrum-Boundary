package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func runSecure(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, `Fulcrum Boundary secure profiles

Usage:
  boundary secure <profile> <command> [flags]

Profiles:
  github   Secure GitHub MCP preview profile

Use "boundary secure github --help" for profile commands.
`)
		return 0
	}
	switch args[0] {
	case "github":
		return runSecureGitHub(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown secure profile %q\n", args[0])
		return 1
	}
}

func runSecureGitHub(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(stdout, `Secure GitHub MCP preview profile

Usage:
  boundary secure github <command> [flags]

Commands:
  setup   Write a fixture profile and starter policy bundle
  serve   Serve the fixture Secure GitHub MCP profile

Secure GitHub remains preview until live GitHub App conformance evidence exists.
`)
		return 0
	}
	switch args[0] {
	case "setup":
		return runSecureGitHubSetup(args[1:], stdout, stderr)
	case "serve":
		return runSecureGitHubServe(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown secure github command %q\n", args[0])
		return 1
	}
}

func runSecureGitHubSetup(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary secure github setup", stderr)
	out := fs.String("out", ".boundary/secure-github", "directory for generated Secure GitHub fixture profile")
	owner := fs.String("owner", securegithub.DefaultOwner, "fixture GitHub owner")
	repo := fs.String("repo", securegithub.DefaultRepo, "fixture GitHub repository")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	result, err := securegithub.WriteSetup(*out, securegithub.Config{Owner: *owner, Repo: *repo})
	if err != nil {
		fmt.Fprintf(stderr, "secure github setup: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "profile: %s\n", result.Profile)
	fmt.Fprintf(stdout, "policy: %s\n", result.Policy)
	fmt.Fprintf(stdout, "status: preview\n")
	fmt.Fprintf(stdout, "fixture mode: true\n")
	fmt.Fprintf(stdout, "live GitHub mutation: none\n")
	fmt.Fprintf(stdout, "next: boundary secure github serve --fixture --dry-run\n")
	return 0
}

func runSecureGitHubServe(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary secure github serve", stderr)
	listen := fs.String("listen", "127.0.0.1:8940", "HTTP listen address for fixture JSON-RPC")
	owner := fs.String("owner", securegithub.DefaultOwner, "fixture GitHub owner")
	repo := fs.String("repo", securegithub.DefaultRepo, "fixture GitHub repository")
	fixture := fs.Bool("fixture", true, "serve fixture profile without live GitHub credentials")
	dryRun := fs.Bool("dry-run", false, "print serve configuration without starting a listener")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if !*fixture {
		fmt.Fprintln(stderr, "secure github serve: live GitHub App mode is not implemented in this preview profile")
		return 1
	}
	cfg := securegithub.Config{Owner: *owner, Repo: *repo}
	if *dryRun {
		fmt.Fprintf(stdout, "profile: %s\n", securegithub.ProfileID)
		fmt.Fprintf(stdout, "status: %s\n", securegithub.StatusPreview)
		fmt.Fprintf(stdout, "listen: %s\n", *listen)
		fmt.Fprintf(stdout, "fixture mode: true\n")
		fmt.Fprintf(stdout, "one repo per session: true\n")
		fmt.Fprintf(stdout, "target repo: %s/%s\n", *owner, *repo)
		fmt.Fprintf(stdout, "protected writes after taint: W1,W2 deny before upstream\n")
		return 0
	}
	adapter := securegithub.NewFixtureAdapter(cfg)
	srv := &http.Server{
		Addr:              *listen,
		Handler:           securegithub.NewHTTPHandler(adapter),
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(stderr, "secure github fixture profile listening on %s for %s/%s\n", *listen, *owner, *repo)
	if err := srv.ListenAndServe(); err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		fmt.Fprintf(stderr, "secure github serve: %v\n", err)
		return 1
	}
	return 0
}
