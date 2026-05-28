package boundarycli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func runSecureGitHubConformance(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printSecureGitHubConformanceHelp(stdout)
		return 0
	}
	mode := args[0]
	if mode != "read" && mode != "denied-write" && mode != "all" {
		fmt.Fprintf(stderr, "unknown secure github conformance mode %q\n\n", mode)
		printSecureGitHubConformanceHelp(stderr)
		return 1
	}

	fs := newHelpFlagSet("boundary secure github conformance "+mode, stderr, commandHelp{
		Purpose: "Run Secure GitHub live conformance checks with operator-owned GitHub App credentials.",
		Usage:   "boundary secure github conformance " + mode + " [--json] [--out <dir>]",
		Common: []string{
			"BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance read",
			"BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write",
			"BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance all --out /tmp/boundary-secure-github",
		},
		Notes: []string{
			"without BOUNDARY_GITHUB_CONFORMANCE=true this command skips without network calls",
			"denied-write proves Boundary denies before the mutation client is reached",
			"sanitized transcripts contain hashes and booleans, not raw issue bodies or credentials",
		},
	})
	jsonOut := fs.Bool("json", false, "write JSON conformance output")
	outDir := fs.String("out", "", "directory for sanitized conformance transcripts")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	cfg, err := securegithub.LoadLiveConfigFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "secure github conformance: %v\n", err)
		return 1
	}
	if !cfg.Enabled {
		fmt.Fprintln(stdout, "profile: secure-github")
		fmt.Fprintln(stdout, "status: skipped")
		fmt.Fprintln(stdout, "reason: BOUNDARY_GITHUB_CONFORMANCE is not true")
		return 0
	}
	if *outDir != "" {
		cfg.TranscriptDir = *outDir
	}

	auth := securegithub.NewGitHubAppAuth(cfg)
	client := securegithub.NewRESTGitHubClient(auth, cfg.APIBaseURL)
	results, err := runSecureGitHubConformanceMode(context.Background(), mode, cfg, client)
	if err != nil {
		fmt.Fprintf(stderr, "secure github conformance: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			fmt.Fprintf(stderr, "secure github conformance: %v\n", err)
			return 1
		}
		return 0
	}
	for i, result := range results {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		writeSecureGitHubConformanceResult(stdout, result)
	}
	return 0
}

func runSecureGitHubConformanceMode(ctx context.Context, mode string, cfg securegithub.LiveConfig, client securegithub.GitHubClient) ([]securegithub.LiveConformanceResult, error) {
	switch mode {
	case "read":
		result, err := securegithub.RunLiveReadConformance(ctx, cfg, client)
		return singleResult(result, err)
	case "denied-write":
		result, err := securegithub.RunLiveDeniedWriteConformance(ctx, cfg, client)
		return singleResult(result, err)
	case "all":
		read, err := securegithub.RunLiveReadConformance(ctx, cfg, client)
		if err != nil {
			return nil, err
		}
		denied, err := securegithub.RunLiveDeniedWriteConformance(ctx, cfg, client)
		if err != nil {
			return nil, err
		}
		return []securegithub.LiveConformanceResult{read, denied}, nil
	default:
		return nil, fmt.Errorf("unknown mode %q", mode)
	}
}

func singleResult(result securegithub.LiveConformanceResult, err error) ([]securegithub.LiveConformanceResult, error) {
	if err != nil {
		return nil, err
	}
	return []securegithub.LiveConformanceResult{result}, nil
}

func writeSecureGitHubConformanceResult(w io.Writer, result securegithub.LiveConformanceResult) {
	tr := result.Transcript
	fmt.Fprintf(w, "profile: %s\n", tr.ProfileID)
	fmt.Fprintf(w, "status: %s\n", tr.ProfileStatus)
	fmt.Fprintf(w, "mode: %s\n", tr.Mode)
	fmt.Fprintf(w, "expected action: %s\n", tr.ExpectedAction)
	fmt.Fprintf(w, "actual action: %s\n", tr.ActualAction)
	fmt.Fprintf(w, "reason: %s\n", tr.Reason)
	if tr.MatchedRule != "" {
		fmt.Fprintf(w, "matched rule: %s\n", tr.MatchedRule)
	}
	fmt.Fprintf(w, "read_upstream_called=%t\n", tr.ReadUpstreamCalled)
	fmt.Fprintf(w, "upstream_called=%t\n", tr.UpstreamCalled)
	fmt.Fprintf(w, "github_mutation_called=%t\n", tr.GitHubMutationCalled)
	fmt.Fprintf(w, "raw_content_included=%t\n", tr.RawContentIncluded)
	fmt.Fprintf(w, "credential_data_included=%t\n", tr.CredentialDataIncluded)
	fmt.Fprintf(w, "transcript: %s\n", result.TranscriptPath)
	fmt.Fprintf(w, "transcript_sha256: %s\n", result.TranscriptSHA256)
}

func printSecureGitHubConformanceHelp(w io.Writer) {
	fmt.Fprint(w, `Secure GitHub live conformance preview.

Usage:
  boundary secure github conformance <mode> [flags]

Modes:
  read           Read a real GitHub issue through GitHub App auth and record sanitized taint evidence
  denied-write   After live read taint, deny a protected write before any mutation client call
  all            Run read and denied-write checks

Required environment when enabled:
  BOUNDARY_GITHUB_CONFORMANCE=true
  BOUNDARY_GITHUB_APP_ID
  BOUNDARY_GITHUB_INSTALLATION_ID
  BOUNDARY_GITHUB_PRIVATE_KEY_PATH
  BOUNDARY_GITHUB_OWNER
  BOUNDARY_GITHUB_REPO
  BOUNDARY_GITHUB_ISSUE_NUMBER

Optional:
  BOUNDARY_GITHUB_API_BASE_URL
  BOUNDARY_GITHUB_TRANSCRIPT_DIR
  BOUNDARY_GITHUB_TRANSCRIPT

Notes:
  - Without BOUNDARY_GITHUB_CONFORMANCE=true, this command skips without network calls.
  - Sanitized transcripts contain hashes and booleans, not raw issue bodies or credentials.
  - Secure GitHub remains preview; live conformance does not prove deployment bypass resistance.
`)
}
