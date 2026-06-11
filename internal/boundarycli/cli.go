package boundarycli

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/mcp"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	sqlguard "github.com/fulcrum-governance/fulcrum-boundary/interceptors/sql"
	boundarydemo "github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
)

var Version = "unknown"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printRootHelp(stdout)
		return 0
	}
	switch args[0] {
	case "--help", "-h":
		printRootHelp(stdout)
		return 0
	case "help":
		// `boundary help` prints the root help; `boundary help <topic...>` routes
		// to the topic's own --help so a single help surface backs both
		// spellings. --help is appended after every topic word so compound
		// topics (`help policy generate`, `help demo postgres`) reach the leaf
		// command's help rather than stopping at the parent dispatcher.
		if len(args) == 1 {
			printRootHelp(stdout)
			return 0
		}
		topic := append([]string{}, args[1:]...)
		return Run(append(topic, "--help"), stdout, stderr)
	case "--version", "-v":
		// `--version`/`-v` are aliases for the version command so the standard CLI
		// idiom reports the same build metadata (and the same JSON with --json).
		return runVersion(args[1:], stdout, stderr)
	}

	switch args[0] {
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "init":
		return runFirewallInit(args[1:], stdout, stderr)
	case "inventory":
		return runFirewallInventory(args[1:], stdout, stderr)
	case "graph":
		return runFirewallGraph(args[1:], stdout, stderr)
	case "dashboard":
		return runFirewallDashboard(args[1:], stdout, stderr)
	case "install":
		return runFirewallInstall(args[1:], stdout, stderr)
	case "uninstall":
		return runFirewallUninstall(args[1:], stdout, stderr)
	case "lock":
		return runFirewallLock(args[1:], stdout, stderr)
	case "verify-lock":
		return runFirewallVerifyLock(args[1:], stdout, stderr)
	case "redteam":
		return runRedteam(args[1:], stdout, stderr)
	case "selftest":
		return runSelftest(args[1:], stdout, stderr)
	case "secure":
		return runSecure(args[1:], stdout, stderr)
	case "command":
		return runCommand(args[1:], stdout, stderr)
	case "edit":
		return runEdit(args[1:], stdout, stderr)
	case "shell":
		return runShell(args[1:], stdout, stderr)
	case "policy":
		return runFirewallPolicy(args[1:], stdout, stderr)
	case "mcp":
		return runFirewallMCP(args[1:], stdout, stderr)
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "demo":
		return runDemo(args[1:], stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "verify-record":
		return runVerifyRecord(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "replay":
		return runReplay(args[1:], stdout, stderr)
	case "test":
		return runTest(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "evidence":
		return runEvidence(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], os.Stdin, stdout, stderr)
	case "trust":
		return runTrust(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printRootHelp(stderr)
		return 1
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintf(w, `Fulcrum Boundary

Purpose:
  Govern routed tools before privileged execution and record the verdict.

Usage:
  boundary <command> [flags]

Commands:
  version         Print Boundary version and build metadata
  init            Initialize a Boundary firewall workspace
  inventory       Discover MCP configs or ingest inventory records
  graph           Render inventory-derived MCP risk paths
  dashboard       Render a local-only firewall dashboard
  install         Rewrite selected MCP configs through a Boundary route
  uninstall       Restore an MCP config from a Boundary install receipt
  lock            Create a descriptor lockfile for MCP server descriptors
  verify-lock     Verify MCP server descriptors against a lockfile
  redteam         Run safe fixture attacks and report expected deny records
  selftest        Run local no-credential Boundary release checks
  secure          Manage Secure MCP preview profiles
  command         Classify and govern project-local command paths
  edit            Classify proposed file mutations without applying them
  shell           Launch a project-local Command Boundary subshell
  policy generate Generate starter Boundary firewall policies
  mcp proxy       Fail-closed generic MCP proxy entrypoint for installed routes
  serve           Start the Boundary gateway
  demo action-boundary
                  Run a fixture-only cross-surface action-boundary demo
  demo postgres   Run the Postgres safety demo against a running gateway
  demo github-lethal-trifecta
                  Run a fixture-only Secure GitHub denial demo
  verify          Validate YAML policy files
  verify-record   Verify a receipt-grade decision record
  explain         Describe a decision record without verifying it
  replay          Re-evaluate a recorded request and compare the decision
  test            Run local policy-as-code tests against policy bundles
  doctor          Check local routed-surface diagnostics
  evidence        Bundle and verify local Boundary evidence artifacts
  audit           Pretty-print structured decision records
  trust           Inspect or reset trust state
  completion      Print a bash, zsh, or fish completion script

Use "boundary <command> --help" for command flags.
`)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

// parseInterspersed parses args into fs while allowing positional (non-flag)
// arguments to appear in any position, not only after all flags. Go's flag
// package stops at the first non-flag token, so by default a command with a
// positional argument cannot accept flags that follow it. parseInterspersed
// works around that by parsing in a loop: it parses, captures the first leftover
// non-flag argument as a positional, and resumes parsing the remainder. It
// returns the collected positionals in order. A flag.ErrHelp (from -h/--help) is
// returned to the caller verbatim so it can exit 0.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	remaining := args
	for {
		if err := fs.Parse(remaining); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, rest[0])
		remaining = rest[1:]
	}
}

type commandHelp struct {
	Purpose string
	Usage   string
	Common  []string
	Notes   []string
}

func newHelpFlagSet(name string, stderr io.Writer, help commandHelp) *flag.FlagSet {
	fs := newFlagSet(name, stderr)
	fs.Usage = func() {
		out := fs.Output()
		if help.Purpose != "" {
			fmt.Fprintf(out, "%s\n\n", help.Purpose)
		}
		if help.Usage != "" {
			fmt.Fprintf(out, "Usage:\n  %s\n", help.Usage)
		}
		if len(help.Common) > 0 {
			fmt.Fprintln(out, "\nCommon usage:")
			for _, line := range help.Common {
				fmt.Fprintf(out, "  %s\n", line)
			}
		}
		if len(help.Notes) > 0 {
			fmt.Fprintln(out, "\nNotes:")
			for _, line := range help.Notes {
				fmt.Fprintf(out, "  - %s\n", line)
			}
		}
		fmt.Fprintln(out, "\nFlags:")
		fs.PrintDefaults()
	}
	return fs
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary serve", stderr, commandHelp{
		Purpose: "Start the Boundary gateway that governs routed tools before privileged execution.",
		Usage:   "boundary serve [--config FILE] [--listen ADDR] [--policies DIR] [--upstream URL|DSN] [--trust-mode MODE] [--require-agent-id] [--receipt-seed FILE]",
		Common: []string{
			"boundary serve --policies ./policies/ --listen :8080",
			"boundary serve --upstream http://localhost:9000/mcp",
			"boundary serve --config boundary.yaml",
			"boundary serve --policies ./policies/ --receipt-seed ./boundary-receipt.seed",
		},
		Notes: []string{
			"Boundary governs only routes forced through it; direct access to the same tool is a bypass unless deployment topology removes that path.",
			"MCP is the only production route; other transports remain preview.",
			"--trust-mode kernel connects only the trust seam to Fulcrum services; the policy seam still loads the local policy dir.",
			"Evaluator faults fail closed for the configured transports and otherwise fall through; a policy deny is a decision, a backend fault is not.",
			"--receipt-seed signs every emitted decision record with the Ed25519 seed in FILE (64 hex chars); signing is off by default, and a startup error here fails closed (exit 1) rather than serving unsigned.",
			"A signature proves who signed the record, not the verdict or that execution happened; key custody is the operator's (see docs/SIGNING.md).",
		},
	})
	configPath := fs.String("config", "", "Boundary runtime config file")
	listen := fs.String("listen", ":8080", "HTTP listen address")
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	upstream := fs.String("upstream", "postgres://demo:demo@localhost:5432/demo?sslmode=disable", "upstream MCP HTTP URL or Postgres demo DSN")
	trustMode := fs.String("trust-mode", "disabled", "trust mode: disabled, standalone, or kernel")
	trustRedisURL := fs.String("trust-redis-url", "redis://localhost:6379", "Redis URL for kernel trust mode")
	requireAgentID := fs.Bool("require-agent-id", false, "deny protected adapter requests without agent identity")
	receiptSeed := fs.String("receipt-seed", "", "path to a 64-hex Ed25519 seed; when set, signs every emitted decision record (off by default)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if *configPath != "" {
		cfg, err := LoadRuntimeConfig(*configPath)
		if err != nil {
			fmt.Fprintf(stderr, "config: %v\n", err)
			return 1
		}
		if cfg.Server.Listen != "" {
			*listen = cfg.Server.Listen
		}
		if cfg.Server.Upstream != "" {
			*upstream = cfg.Server.Upstream
		}
		switch cfg.Mode {
		case "standalone":
			*trustMode = string(governance.TrustModeStandalone)
			*policyDir = cfg.Standalone.PolicyDir
		case "kernel":
			*trustMode = string(governance.TrustModeKernel)
			*trustRedisURL = cfg.Kernel.Trust.RedisURL
			*policyDir = "./policies/"
			fmt.Fprintf(stderr, "boundary serve: kernel mode — policy seam not yet wired; using local policy dir %q (only trust seam connects to Fulcrum services)\n", *policyDir)
		}
		if cfg.Security.RequireAgentID {
			*requireAgentID = true
		}
	}

	policyResult, err := governance.LoadStaticPolicyFiles(*policyDir)
	if err != nil {
		fmt.Fprintf(stderr, "load policies: %v\n", err)
		return 1
	}
	policyHash, err := governance.PolicyBundleHashFromDir(*policyDir)
	if err != nil {
		fmt.Fprintf(stderr, "hash policies: %v\n", err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	trustBackend, err := governance.NewProductionTrustBackend(governance.ProductionTrustConfig{
		Mode: governance.TrustMode(*trustMode),
		Kernel: governance.KernelTrustConfig{
			RedisURL:   *trustRedisURL,
			IPCPrefix:  "agent:",
			TimeoutMS:  100,
			FailClosed: true,
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "trust backend: %v\n", err)
		return 1
	}

	// Optional receipt signing. When --receipt-seed is given we MUST fail closed
	// on a missing/short/non-hex seed rather than serve unsigned: requesting
	// signing and silently emitting unsigned records would misrepresent record
	// authorship. The signature attests who signed the record, not the verdict
	// or that execution happened; key custody is the operator's (docs/SIGNING.md).
	var receiptSigner governance.ReceiptSigner
	if *receiptSeed != "" {
		signer, err := governance.NewEd25519SignerFromSeedFile(*receiptSeed, "")
		if err != nil {
			fmt.Fprintf(stderr, "receipt signing requested but seed could not be loaded: %v\n", err)
			return 1
		}
		receiptSigner = signer
	}

	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies:   policyResult.Rules,
		GatewayVersion:   currentGatewayVersion(),
		PolicyBundleHash: policyHash,
		RequireAgentID:   *requireAgentID,
		ReceiptSigner:    receiptSigner,
	}, trustBackend, nil, governance.NewSlogAuditPublisher(logger))
	pipeline.RegisterInterceptor("query", sqlguard.NewPostgresInterceptor())

	handler, mode, closeUpstream, err := serveHandler(*upstream, pipeline)
	if err != nil {
		fmt.Fprintf(stderr, "upstream setup failed: %v\n", err)
		return 1
	}
	if closeUpstream != nil {
		defer func() {
			if err := closeUpstream(); err != nil {
				fmt.Fprintf(stderr, "upstream close failed: %v\n", err)
			}
		}()
	}

	fmt.Fprintf(stderr, "boundary serve listening on %s in %s mode with %d static policy rules\n", *listen, mode, len(policyResult.Rules))
	srv := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "server error: %v\n", err)
		return 1
	}
	return 0
}

func serveHandler(upstream string, pipeline *governance.Pipeline) (handler http.Handler, mode string, closeFn func() error, err error) {
	parsed, err := url.Parse(upstream)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return mcp.NewGateway(pipeline, mcp.NewHTTPForwarder(upstream), "default"), "mcp-proxy", nil, nil
	}

	db, err := sql.Open("pgx", upstream)
	if err != nil {
		return nil, "", nil, fmt.Errorf("open postgres demo upstream: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", nil, fmt.Errorf("postgres demo ping: %w", err)
	}

	downstream := postgresHandler(db)
	middleware := governance.NewMiddleware(pipeline, downstream, governance.MiddlewareConfig{
		TransportType:    governance.TransportMCP,
		RequestBuilder:   buildPostgresGovernanceRequest,
		ToolNameHeader:   governance.HeaderToolName,
		AgentIDHeader:    governance.HeaderGovernanceAgentID,
		TenantIDHeader:   governance.HeaderGovernanceTenantID,
		ToolNameFromPath: true,
	})
	return middleware, "postgres-demo", db.Close, nil
}

func runDemo(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, `Run Boundary demos that show governed routes and explicit fixture/live boundaries.

Usage:
  boundary demo <name> [flags]

Common usage:
  boundary demo action-boundary
  boundary demo action-boundary --markdown --out demo.md
  boundary demo github-lethal-trifecta
  boundary demo github-lethal-trifecta --markdown --out demo.md
  boundary demo command-secret-exfil
  boundary demo tamper-evidence
  boundary demo postgres --gateway http://localhost:8080/mcp
  boundary demo trust-degradation
  boundary demo trust-degradation --show-records

Demos:
  action-boundary          Fixture-only cross-surface action-boundary demo
  postgres                 Exercise allow, deny, and direct-bypass checks against a running gateway
  github-lethal-trifecta   Fixture-only Secure GitHub denial demo
  command-secret-exfil     Fixture-only Command Boundary secret-exfil denial demo
  tamper-evidence          Fixture-only forge-the-receipt hash-verification demo
  trust-degradation        Local adaptive-trust degradation demo (--show-records streams audit JSON to stderr)

Notes:
  - Fixture demos use no credentials, no network, and no live mutation.
  - Postgres requires a running gateway and explicit bypass host/port checks.
`)
		return 0
	}
	if args[0] == "trust-degradation" {
		return runTrustDegradationDemo(args[1:], stdout, stderr)
	}
	if args[0] == "github-lethal-trifecta" {
		return runGitHubLethalTrifectaDemo(args[1:], stdout, stderr)
	}
	if args[0] == "command-secret-exfil" {
		return runCommandSecretExfilDemo(args[1:], stdout, stderr)
	}
	if args[0] == "tamper-evidence" {
		return runTamperEvidenceDemo(args[1:], stdout, stderr)
	}
	if args[0] == "action-boundary" {
		return runActionBoundaryDemo(args[1:], stdout, stderr)
	}
	if args[0] != "postgres" {
		fmt.Fprintf(stderr, "unknown demo %q; expected action-boundary, postgres, github-lethal-trifecta, command-secret-exfil, or trust-degradation\n", args[0])
		return 1
	}
	fs := newHelpFlagSet("boundary demo postgres", stderr, commandHelp{
		Purpose: "Run the Postgres allow, deny, and bypass demo against a running Boundary gateway.",
		Usage:   "boundary demo postgres [--gateway URL] [--bypass-host HOST] [--bypass-port PORT]",
		Common: []string{
			"boundary demo postgres --gateway http://localhost:8080/mcp",
		},
		Notes: []string{
			"This demo requires a running gateway and checks direct Postgres bypass separately.",
			"Direct database access must be blocked by deployment topology, not by CLI output.",
		},
	})
	gateway := fs.String("gateway", "http://localhost:8080/mcp", "gateway endpoint URL")
	bypassHost := fs.String("bypass-host", "localhost", "host to test for direct Postgres bypass")
	bypassPort := fs.String("bypass-port", "5432", "port to test for direct Postgres bypass")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	fmt.Fprintln(stdout, "1. Safe SELECT through Boundary")
	if err := demoRequest(*gateway, "SELECT id, email, plan FROM users ORDER BY id LIMIT 3", http.StatusOK, stdout); err != nil {
		fmt.Fprintf(stderr, "safe SELECT failed: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "\n2. Destructive DROP TABLE through Boundary")
	if err := demoRequest(*gateway, "DROP TABLE users", http.StatusForbidden, stdout); err != nil {
		fmt.Fprintf(stderr, "destructive query was not blocked as expected: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "\n3. Direct bypass attempt to Postgres")
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(*bypassHost, *bypassPort), 2*time.Second)
	if err == nil {
		_ = conn.Close()
		fmt.Fprintf(stderr, "BYPASS FAILED: direct connection to %s:%s succeeded\n", *bypassHost, *bypassPort)
		return 1
	}
	fmt.Fprintf(stdout, "BYPASS BLOCKED: direct connection failed as expected (%v)\n", err)
	return 0
}

func demoRequest(endpoint, sqlText string, wantStatus int, stdout io.Writer) error {
	body := map[string]any{
		"tool_name": "query",
		"agent_id":  "demo-agent",
		"tenant_id": "demo",
		"arguments": map[string]any{
			"sql": sqlText,
		},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("status %d, want %d: %s", resp.StatusCode, wantStatus, strings.TrimSpace(string(respBody)))
	}

	label := "ALLOW"
	if wantStatus == http.StatusForbidden {
		label = "DENY"
	}
	fmt.Fprintf(stdout, "%s status=%d body=%s\n", label, resp.StatusCode, strings.TrimSpace(string(respBody)))
	return nil
}

// verifyResultJSON is the versioned-schema payload emitted by `boundary verify
// --json`. ok reports whether the policy bundle parsed; error carries the parse
// failure message when ok is false. Counts and warnings mirror the text output.
type verifyResultJSON struct {
	SchemaVersion string   `json:"schema_version"`
	OK            bool     `json:"ok"`
	Error         string   `json:"error,omitempty"`
	PolicyFiles   int      `json:"policy_files"`
	Rules         int      `json:"rules"`
	Warnings      []string `json:"warnings"`
}

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary verify", stderr, commandHelp{
		Purpose: "Validate YAML policy files and report rule counts and parse warnings.",
		Usage:   "boundary verify [--policies DIR] [--json]",
		Common: []string{
			"boundary verify --policies ./policies/",
			"boundary verify --policies ./policies/ --json",
		},
		Notes: []string{
			"Verify checks that the policy bundle parses; it does not prove the policies are correct or that a route enforces them.",
			"--json emits a versioned boundary.verify.v1 object with ok/error fields; the exit code is non-zero on a parse failure.",
		},
	})
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	jsonOutput := fs.Bool("json", false, "emit machine-readable boundary.verify.v1 JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	result, err := governance.LoadStaticPolicyFiles(*policyDir)
	if err != nil {
		if *jsonOutput {
			if encErr := writeIndentedJSON(stdout, verifyResultJSON{
				SchemaVersion: "boundary.verify.v1",
				OK:            false,
				Error:         err.Error(),
				Warnings:      []string{},
			}); encErr != nil {
				fmt.Fprintf(stderr, "verify: %v\n", encErr)
			}
			return 1
		}
		fmt.Fprintf(stderr, "policy parse failed: %v\n", err)
		return 1
	}
	if *jsonOutput {
		warnings := result.Warnings
		if warnings == nil {
			warnings = []string{}
		}
		if err := writeIndentedJSON(stdout, verifyResultJSON{
			SchemaVersion: "boundary.verify.v1",
			OK:            true,
			PolicyFiles:   len(result.Files),
			Rules:         len(result.Rules),
			Warnings:      warnings,
		}); err != nil {
			fmt.Fprintf(stderr, "verify: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "policy files: %d\n", len(result.Files))
	fmt.Fprintf(stdout, "rules: %d\n", len(result.Rules))
	if len(result.Warnings) == 0 {
		fmt.Fprintln(stdout, "warnings: 0")
		return 0
	}
	fmt.Fprintf(stdout, "warnings: %d\n", len(result.Warnings))
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "- %s\n", warning)
	}
	return 0
}

// verifyRecordResultJSON is the versioned-schema payload emitted by `boundary
// verify-record --json`. ok reports whether the record passed integrity
// verification over the covered inputs; error carries the first failing check
// when ok is false. record_id echoes the record's identifier when present. A
// passing check is hash-verifiable integrity over the covered inputs, not proof
// the action was executed, prevented, or that the verdict was correct.
type verifyRecordResultJSON struct {
	SchemaVersion string `json:"schema_version"`
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	RecordID      string `json:"record_id,omitempty"`
}

func runVerifyRecord(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary verify-record", stderr, commandHelp{
		Purpose: "Verify a receipt-grade decision record's integrity over its covered inputs.",
		Usage:   "boundary verify-record [--request request.json] [--policies DIR] [--binary-digest sha256:...] [--verify-signature --public-key HEX|FILE] [--json] <record.json>",
		Common: []string{
			"boundary verify-record record.json",
			"boundary verify-record --request request.json --policies ./policies/ record.json",
			"boundary verify-record --verify-signature --public-key ./boundary-receipt.pub record.json",
			"boundary verify-record --json record.json",
		},
		Notes: []string{
			"record.json is required: a single-record decision-record JSON object (not a multi-record .jsonl log).",
			"Verification recomputes decision_hash always, request_hash when --request is given, and policy_bundle_hash when --policies is given.",
			"This is hash-verifiable integrity over the covered inputs, not proof the action was executed, prevented, or that the verdict was correct.",
			"--verify-signature additionally checks the record's optional signature over the recomputed decision_hash against --public-key (64-hex key or a file holding one); it fails closed on a mismatch or a missing signature.",
			"A valid signature proves the record was signed by the holder of that key — it does not prove the verdict was correct, that the action executed or was prevented, or solve key custody. Without --verify-signature the signature fields are ignored for integrity and unsigned records remain the default.",
			"--json emits a versioned boundary.verify_record.v1 object with ok/error fields; the exit code is non-zero on a verification failure.",
		},
	})
	requestPath := fs.String("request", "", "request JSON body used to verify request_hash")
	policyDir := fs.String("policies", "", "policy directory used to verify policy_bundle_hash")
	binaryDigest := fs.String("binary-digest", "", "expected boundary build digest")
	verifySignature := fs.Bool("verify-signature", false, "additionally verify the record's ed25519 signature over decision_hash (requires --public-key)")
	publicKey := fs.String("public-key", "", "ed25519 public key (64 hex chars) or path to a file holding one, used with --verify-signature")
	jsonOutput := fs.Bool("json", false, "emit machine-readable boundary.verify_record.v1 JSON")
	// Allow the positional record path in any position (flags may follow it) by
	// collecting positionals in one pass, mirroring `boundary replay`.
	positionals, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if len(positionals) != 1 {
		fmt.Fprintln(stderr, "usage: boundary verify-record [--request request.json] [--policies dir] [--binary-digest sha256:...] [--verify-signature --public-key hex|file] [--json] record.json")
		return 1
	}

	body, err := os.ReadFile(positionals[0])
	if err != nil {
		return failVerifyRecord(stdout, stderr, *jsonOutput, "", fmt.Sprintf("read record: %v", err))
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		return failVerifyRecord(stdout, stderr, *jsonOutput, "", fmt.Sprintf("parse record: %v", err))
	}

	var rawRequest []byte
	if *requestPath != "" {
		rawRequest, err = os.ReadFile(*requestPath)
		if err != nil {
			return failVerifyRecord(stdout, stderr, *jsonOutput, record.RecordID, fmt.Sprintf("read request: %v", err))
		}
	}

	if err := governance.VerifyDecisionRecord(record, rawRequest, *policyDir, *binaryDigest); err != nil {
		return failVerifyRecord(stdout, stderr, *jsonOutput, record.RecordID, fmt.Sprintf("record verification failed: %v", err))
	}
	if *verifySignature {
		if *publicKey == "" {
			return failVerifyRecord(stdout, stderr, *jsonOutput, record.RecordID, "signature verification failed: --verify-signature requires --public-key")
		}
		pub, err := governance.ParseEd25519PublicKey(*publicKey)
		if err != nil {
			return failVerifyRecord(stdout, stderr, *jsonOutput, record.RecordID, fmt.Sprintf("signature verification failed: %v", err))
		}
		if err := governance.VerifyReceiptSignature(record, pub); err != nil {
			return failVerifyRecord(stdout, stderr, *jsonOutput, record.RecordID, fmt.Sprintf("signature verification failed: %v", err))
		}
	}
	if *jsonOutput {
		if err := writeIndentedJSON(stdout, verifyRecordResultJSON{
			SchemaVersion: "boundary.verify_record.v1",
			OK:            true,
			RecordID:      record.RecordID,
		}); err != nil {
			fmt.Fprintf(stderr, "verify-record: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintln(stdout, "record verification: ok")
	fmt.Fprintf(stdout, "record_id: %s\n", record.RecordID)
	return 0
}

// failVerifyRecord renders a verify-record failure as either the versioned JSON
// object (ok=false with the message in error) or the legacy stderr line, then
// returns exit code 1 so the JSON and text paths share one failure shape.
func failVerifyRecord(stdout, stderr io.Writer, jsonOutput bool, recordID, message string) int {
	if jsonOutput {
		if err := writeIndentedJSON(stdout, verifyRecordResultJSON{
			SchemaVersion: "boundary.verify_record.v1",
			OK:            false,
			Error:         message,
			RecordID:      recordID,
		}); err != nil {
			fmt.Fprintf(stderr, "verify-record: %v\n", err)
		}
		return 1
	}
	fmt.Fprintln(stderr, message)
	return 1
}

func runAudit(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary audit", stderr, commandHelp{
		Purpose: "Pretty-print structured decision records from a log file or stdin.",
		Usage:   "boundary audit [--file LOG] [--filter-agent ID] [--filter-tool NAME] [--filter-action ACTION]",
		Common: []string{
			"boundary audit --file .boundary/decisions.jsonl",
			"cat decisions.jsonl | boundary audit --filter-action deny",
			"boundary audit --file decisions.jsonl --filter-tool query",
		},
		Notes: []string{
			"Audit reads a multi-record decision-record log (one JSON record per line); lines that do not parse are skipped.",
			"Audit displays recorded verdicts; it does not re-verify record hashes (use verify-record) or prove the action was executed or prevented.",
		},
	})
	filePath := fs.String("file", "", "decision record log file; stdin is used when empty")
	filterAgent := fs.String("filter-agent", "", "only show records for this agent_id")
	filterTool := fs.String("filter-tool", "", "only show records for this tool_name")
	filterAction := fs.String("filter-action", "", "only show records for this action")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	input := stdin
	var file *os.File
	if *filePath != "" {
		var err error
		file, err = os.Open(*filePath)
		if err != nil {
			fmt.Fprintf(stderr, "open log file: %v\n", err)
			return 1
		}
		defer file.Close()
		input = file
	}

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		var record map[string]any
		line := scanner.Bytes()
		if err := json.Unmarshal(line, &record); err != nil {
			continue
		}
		if !recordMatches(record, "agent_id", *filterAgent) ||
			!recordMatches(record, "tool_name", *filterTool) ||
			!recordMatches(record, "action", *filterAction) {
			continue
		}
		printAuditRecord(stdout, record)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "read log: %v\n", err)
		return 1
	}
	return 0
}

func runTrust(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, `Inspect or reset the trust state Boundary consults during stage-1 evaluation.

Usage:
  boundary trust show [--redis-url URL] [--ipc-prefix PREFIX] <agent-id>
  boundary trust reset <agent-id>

Common usage:
  boundary trust show demo-agent
  boundary trust show --redis-url redis://localhost:6379 demo-agent
  boundary trust reset demo-agent

Notes:
  - With --redis-url, show reads kernel trust state over Redis IPC; otherwise an in-process standalone backend is used.
  - reset operates on the in-process standalone backend only.
  - An absent trust record is not an error; unknown agents report known: false.
`)
		return 0
	}
	switch args[0] {
	case "show":
		return runTrustShow(args[1:], stdout, stderr)
	case "reset":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: boundary trust reset <agent-id>")
			return 1
		}
		backend := governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{})
		snapshot, err := backend.ResetAgentTrust(context.Background(), args[1])
		if err != nil {
			fmt.Fprintf(stderr, "trust reset failed: %v\n", err)
			return 1
		}
		printTrustSnapshot(stdout, snapshot)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown trust command %q (valid: show, reset)\n", args[0])
		return 1
	}
}

func runTrustShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary trust show", stderr)
	redisURL := fs.String("redis-url", "", "Redis URL for kernel-connected trust state")
	ipcPrefix := fs.String("ipc-prefix", "agent:", "Redis IPC key prefix")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boundary trust show [--redis-url URL] [--ipc-prefix PREFIX] <agent-id>")
		return 1
	}
	var backend governance.TrustBackend
	var err error
	if *redisURL != "" {
		backend, err = governance.NewProductionTrustBackend(governance.ProductionTrustConfig{
			Mode: governance.TrustModeKernel,
			Kernel: governance.KernelTrustConfig{
				RedisURL:   *redisURL,
				IPCPrefix:  *ipcPrefix,
				TimeoutMS:  100,
				FailClosed: true,
			},
		})
		if err != nil {
			fmt.Fprintf(stderr, "trust backend failed: %v\n", err)
			return 1
		}
	} else {
		backend = governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{})
	}
	snapshot, err := backend.GetAgentTrust(context.Background(), fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "trust show failed: %v\n", err)
		return 1
	}
	printTrustSnapshot(stdout, snapshot)
	return 0
}

func printTrustSnapshot(w io.Writer, snapshot governance.TrustSnapshot) {
	printTrustSnapshotColor(w, snapshot, nil)
}

// printTrustSnapshotColor prints a trust snapshot, styling the state line
// through color (a nil colorizer renders plain, preserving the exact output the
// `trust show` command and its tests expect).
func printTrustSnapshotColor(w io.Writer, snapshot governance.TrustSnapshot, color *boundarydemo.Colorizer) {
	fmt.Fprintf(w, "agent_id: %s\n", snapshot.AgentID)
	fmt.Fprintf(w, "state: %s\n", colorTrustState(color, snapshot.State.String()))
	fmt.Fprintf(w, "score: %.3f\n", snapshot.Score)
	if snapshot.Known {
		fmt.Fprintf(w, "alpha: %.3f\n", snapshot.Alpha)
		fmt.Fprintf(w, "beta: %.3f\n", snapshot.Beta)
		fmt.Fprintf(w, "interactions: %d\n", snapshot.InteractionCount)
	} else {
		fmt.Fprintln(w, "known: false")
	}
}

func runTrustDegradationDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo trust-degradation", stderr, commandHelp{
		Purpose: "Run a local adaptive-trust degradation demo: repeated denied actions drive an agent from TRUSTED to ISOLATED.",
		Usage:   "boundary demo trust-degradation [--show-records]",
		Common: []string{
			"boundary demo trust-degradation",
			"boundary demo trust-degradation --show-records",
		},
		Notes: []string{
			"Local-only: no credentials, no network, no live mutation.",
			"By default the raw governance_decision audit records are suppressed so the narrative reads clean; --show-records streams them (JSON) to stderr.",
		},
	})
	showRecords := fs.Bool("show-records", false, "stream the raw governance_decision audit records (JSON) to stderr")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	// The audit stream is the per-decision governance_decision JSON. It used to
	// be wired to stdout, where ~1KB blobs buried every narrative line. Default
	// it to io.Discard so the narrative table is the demo's face; --show-records
	// sends it to stderr (still off stdout) for operators who want the records.
	auditSink := io.Discard
	if *showRecords {
		auditSink = stderr
	}
	auditor := governance.NewSlogAuditPublisher(slog.New(slog.NewJSONHandler(auditSink, &slog.HandlerOptions{Level: slog.LevelInfo})))
	trust := governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{
		InitialAlpha: 5,
		InitialBeta:  1,
	})
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{
			{
				Name:   "block-drop-table",
				Tool:   "query",
				Action: "deny",
				Reason: "destructive SQL",
				Match: &governance.StaticPolicyMatch{
					Field:           "arguments.sql",
					Contains:        "DROP TABLE",
					CaseInsensitive: true,
				},
			},
		},
		GatewayVersion: currentGatewayVersion(),
		RequireAgentID: true,
	}, trust, nil, auditor)
	ctx := context.Background()
	agentID := "demo-agent"
	color := boundarydemo.NewColorizer(stdout)
	queries := []string{
		"SELECT 1",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"DROP TABLE users",
		"SELECT 2",
	}

	fmt.Fprintln(stdout, color.Bold("Adaptive-trust degradation demo (local-only)"))
	fmt.Fprintln(stdout, "fixture-only: true   credentials: none   network: none   live mutation: none")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Repeated denied actions degrade the agent's trust until it is isolated.")
	fmt.Fprintln(stdout)
	// A tidy fixed-width table reads far better than the prior key=value soup
	// (each line used to also carry a ~1KB JSON audit blob). Columns are padded
	// against the plain token width and then colored, so ANSI bytes never throw
	// off alignment and a piped run is column-clean.
	fmt.Fprintf(stdout, "%-3s  %-20s  %-7s  %-6s  %s\n", "#", "query", "action", "trust", "state")
	for i, sqlText := range queries {
		req := &governance.GovernanceRequest{
			Transport: governance.TransportMCP,
			AgentID:   agentID,
			TenantID:  "demo",
			ToolName:  "query",
			Action:    "tools/call",
			Arguments: map[string]any{"sql": sqlText},
		}
		decision, err := pipeline.Evaluate(ctx, req)
		if err != nil {
			fmt.Fprintf(stderr, "trust demo failed: %v\n", err)
			return 1
		}
		snapshot, _ := trust.GetAgentTrust(ctx, agentID)
		fmt.Fprintf(stdout, "%-3d  %-20s  %s  %-6.2f  %s\n",
			i+1,
			truncateTrustQuery(sqlText, 20),
			padColored(colorTrustAction(color, decision.Action), decision.Action, 7),
			snapshot.Score,
			colorTrustState(color, snapshot.State.String()),
		)
	}
	snapshot, _ := trust.GetAgentTrust(ctx, agentID)
	fmt.Fprintln(stdout, "\n"+color.Bold("Final trust:"))
	printTrustSnapshotColor(stdout, snapshot, color)
	return 0
}

// truncateTrustQuery clamps a query string to width columns for the trust
// table, appending an ellipsis marker when it overflows so the table stays
// aligned without hiding that the value was cut.
func truncateTrustQuery(s string, width int) string {
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// padColored left-aligns a (possibly ANSI-wrapped) token to width visible
// columns, computing the pad from plain (the unstyled token) so color escape
// bytes never count toward the column width. It is the small piece that keeps
// the trust table aligned whether or not color is enabled.
func padColored(colored, plain string, width int) string {
	if pad := width - len(plain); pad > 0 {
		return colored + strings.Repeat(" ", pad)
	}
	return colored
}

// colorTrustAction styles a pipeline action token in the trust table: deny in
// red, allow in green, other verdicts uncolored. A disabled colorizer returns
// the plain token.
func colorTrustAction(c *boundarydemo.Colorizer, action string) string {
	switch action {
	case "deny":
		return c.Deny(action)
	case "allow":
		return c.Pass(action)
	default:
		return action
	}
}

// colorTrustState styles a trust-state token: ISOLATED/TERMINATED in red (the
// agent has lost trust), TRUSTED in green, EVALUATING and others uncolored.
func colorTrustState(c *boundarydemo.Colorizer, state string) string {
	switch state {
	case "ISOLATED", "TERMINATED":
		return c.Deny(state)
	case "TRUSTED":
		return c.Pass(state)
	default:
		return state
	}
}

func recordMatches(record map[string]any, key, want string) bool {
	if want == "" {
		return true
	}
	got, _ := record[key].(string)
	return got == want
}

func printAuditRecord(w io.Writer, record map[string]any) {
	action, _ := record["action"].(string)
	color := "\033[32m"
	if action == "deny" {
		color = "\033[31m"
	}
	reset := "\033[0m"
	fmt.Fprintf(w, "%s%-5s%s tool=%s agent=%s rule=%s reason=%s request=%s\n",
		color,
		strings.ToUpper(action),
		reset,
		valueString(record, "tool_name"),
		valueString(record, "agent_id"),
		valueString(record, "matched_rule"),
		valueString(record, "reason"),
		valueString(record, "request_id"),
	)
}

func valueString(record map[string]any, key string) string {
	if value, ok := record[key].(string); ok {
		return value
	}
	return ""
}
