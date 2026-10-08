// Command madriver runs one Managed Agents session through the Boundary
// managedagents adapter and emits conformance evidence (raw event log,
// sanitized transcript, provenance) under --out-dir.
//
//	madriver --mode stub --out-dir /tmp/ma-run
//	madriver --mode live --agent agt_... --environment-id env_... \
//	    --i-understand-this-spends-money
//
// Live mode additionally requires the operator gate file
// ~/.fulcrum-evidence/ma-conformance/LIVE_GO and a non-empty
// BOUNDARY_MA_UPSTREAM_KEY environment variable.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/fulcrum-governance/fulcrum-boundary/tests/conformance/managed_agents/driver/internal/madriver"
)

func main() {
	cfg := madriver.DefaultConfig()
	fs := flag.NewFlagSet("madriver", flag.ContinueOnError)
	fs.StringVar(&cfg.Mode, "mode", cfg.Mode, "stub (offline scripted upstream) or live (real Managed Agents API)")
	fs.StringVar(&cfg.OutDir, "out-dir", cfg.OutDir, "evidence output directory; must be outside the git worktree")
	fs.Float64Var(&cfg.MaxSpendUSD, "max-spend-usd", cfg.MaxSpendUSD, "hard spend ceiling in USD (max 15.00)")
	fs.IntVar(&cfg.MaxTurns, "max-turns", cfg.MaxTurns, "maximum governed tool-use turns before abort")
	fs.IntVar(&cfg.MaxOutputTokens, "max-output-tokens", cfg.MaxOutputTokens, "per-request output token cap enforced driver-side")
	fs.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "wall-clock limit for the whole run")
	fs.BoolVar(&cfg.AckSpend, "i-understand-this-spends-money", false, "required acknowledgement for --mode live")
	fs.StringVar(&cfg.AgentID, "agent", "", "upstream managed agent id (required in live mode)")
	fs.StringVar(&cfg.EnvironmentID, "environment-id", "", "upstream environment id (required in live mode)")
	fs.StringVar(&cfg.APIBase, "api-base", cfg.APIBase, "upstream API base URL")
	fs.StringVar(&cfg.Prompt, "prompt", cfg.Prompt, "user.message sent to start the session's work")
	fs.StringVar(&cfg.DenyTool, "deny-tool", cfg.DenyTool, "tool name the driver policy denies")
	fs.StringVar(&cfg.ErrorTool, "error-tool", cfg.ErrorTool, "tool name that triggers the forced pipeline error case")
	fs.StringVar(&cfg.TenantID, "tenant-id", cfg.TenantID, "tenant id attached to governance requests")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	res, err := madriver.Run(context.Background(), cfg, madriver.Deps{}, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "madriver: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "transcript: %s\nprovenance: %s\nraw log: %s\n", res.TranscriptPath, res.ProvenancePath, res.RawLogPath)
}
