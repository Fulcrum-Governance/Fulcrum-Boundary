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
	"runtime"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/mcp"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	sqlguard "github.com/fulcrum-governance/fulcrum-boundary/interceptors/sql"
)

var Version = "0.2.0-dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printRootHelp(stdout)
		return 0
	}

	switch args[0] {
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
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], os.Stdin, stdout, stderr)
	case "trust":
		return runTrust(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printRootHelp(stderr)
		return 1
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintf(w, `Fulcrum Boundary

Usage:
  boundary <command> [flags]

Commands:
  init            Initialize a Boundary firewall workspace
  inventory       Discover MCP configs and inventory server capabilities
  graph           Render inventory-derived MCP risk paths
  dashboard       Render a local-only firewall dashboard
  install         Rewrite selected MCP configs through a Boundary route
  uninstall       Restore an MCP config from a Boundary install receipt
  lock            Create a descriptor lockfile for MCP server descriptors
  verify-lock     Verify MCP server descriptors against a lockfile
  redteam         Run safe fixture attacks and report expected deny records
  selftest        Run local no-credential Boundary release checks
  secure          Manage Secure MCP preview profiles
  policy generate Generate starter Boundary firewall policies
  mcp proxy       Fail-closed generic MCP proxy entrypoint for installed routes
  serve           Start the Boundary gateway
  demo postgres   Run the Postgres safety demo against a running gateway
  verify          Validate YAML policy files
  verify-record   Verify a receipt-grade decision record
  doctor          Check local gateway prerequisites
  audit           Pretty-print structured decision records
  trust           Inspect or reset trust state

Use "boundary <command> --help" for command flags.
`)
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func runServe(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary serve", stderr)
	configPath := fs.String("config", "", "Boundary runtime config file")
	listen := fs.String("listen", ":8080", "HTTP listen address")
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	upstream := fs.String("upstream", "postgres://demo:demo@localhost:5432/demo?sslmode=disable", "upstream MCP HTTP URL or Postgres demo DSN")
	trustMode := fs.String("trust-mode", "disabled", "trust mode: disabled, standalone, or kernel")
	trustRedisURL := fs.String("trust-redis-url", "redis://localhost:6379", "Redis URL for kernel trust mode")
	requireAgentID := fs.Bool("require-agent-id", false, "deny protected adapter requests without agent identity")
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
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies:   policyResult.Rules,
		GatewayVersion:   Version,
		PolicyBundleHash: policyHash,
		RequireAgentID:   *requireAgentID,
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
		fmt.Fprintln(stdout, "Usage: boundary demo postgres [--gateway URL] [--bypass-host HOST] [--bypass-port PORT]")
		fmt.Fprintln(stdout, "       boundary demo trust-degradation")
		return 0
	}
	if args[0] == "trust-degradation" {
		return runTrustDegradationDemo(stdout, stderr)
	}
	if args[0] != "postgres" {
		fmt.Fprintf(stderr, "unknown demo %q\n", args[0])
		return 1
	}
	fs := newFlagSet("boundary demo postgres", stderr)
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

func runVerify(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary verify", stderr)
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	result, err := governance.LoadStaticPolicyFiles(*policyDir)
	if err != nil {
		fmt.Fprintf(stderr, "policy parse failed: %v\n", err)
		return 1
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

func runVerifyRecord(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary verify-record", stderr)
	requestPath := fs.String("request", "", "request JSON body used to verify request_hash")
	policyDir := fs.String("policies", "", "policy directory used to verify policy_bundle_hash")
	binaryDigest := fs.String("binary-digest", "", "expected boundary build digest")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: boundary verify-record [--request request.json] [--policies dir] [--binary-digest sha256:...] record.json")
		return 1
	}

	body, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "read record: %v\n", err)
		return 1
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		fmt.Fprintf(stderr, "parse record: %v\n", err)
		return 1
	}

	var rawRequest []byte
	if *requestPath != "" {
		rawRequest, err = os.ReadFile(*requestPath)
		if err != nil {
			fmt.Fprintf(stderr, "read request: %v\n", err)
			return 1
		}
	}

	if err := governance.VerifyDecisionRecord(record, rawRequest, *policyDir, *binaryDigest); err != nil {
		fmt.Fprintf(stderr, "record verification failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "record verification: ok")
	fmt.Fprintf(stdout, "record_id: %s\n", record.RecordID)
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary doctor", stderr)
	listen := fs.String("listen", ":8080", "HTTP listen address")
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	upstream := fs.String("upstream", "postgres://demo:demo@localhost:5432/demo?sslmode=disable", "Postgres upstream DSN")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	failed := false
	check := func(name string, err error) {
		if err != nil {
			failed = true
			fmt.Fprintf(stdout, "FAIL %s: %v\n", name, err)
			return
		}
		fmt.Fprintf(stdout, "PASS %s\n", name)
	}

	check("go version "+runtime.Version(), nil)
	if stat, err := os.Stat(*policyDir); err != nil {
		check("policy directory", err)
	} else if !stat.IsDir() {
		check("policy directory", fmt.Errorf("%s is not a directory", *policyDir))
	} else {
		check("policy directory", nil)
	}
	listener, err := net.Listen("tcp", *listen)
	if err == nil {
		_ = listener.Close()
	}
	check("listen port available", err)
	check("upstream reachable", dialPostgresDSN(*upstream, 2*time.Second))
	if failed {
		return 1
	}
	return 0
}

func runAudit(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := newFlagSet("boundary audit", stderr)
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
		fmt.Fprintln(stdout, "Usage: boundary trust show [--redis-url URL] [--ipc-prefix PREFIX] <agent-id>")
		fmt.Fprintln(stdout, "       boundary trust reset <agent-id>")
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
		fmt.Fprintf(stderr, "unknown trust command %q\n", args[0])
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
	fmt.Fprintf(w, "agent_id: %s\n", snapshot.AgentID)
	fmt.Fprintf(w, "state: %s\n", snapshot.State)
	fmt.Fprintf(w, "score: %.3f\n", snapshot.Score)
	if snapshot.Known {
		fmt.Fprintf(w, "alpha: %.3f\n", snapshot.Alpha)
		fmt.Fprintf(w, "beta: %.3f\n", snapshot.Beta)
		fmt.Fprintf(w, "interactions: %d\n", snapshot.InteractionCount)
	} else {
		fmt.Fprintln(w, "known: false")
	}
}

func runTrustDegradationDemo(stdout, stderr io.Writer) int {
	auditor := governance.NewSlogAuditPublisher(slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
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
		GatewayVersion: Version,
		RequireAgentID: true,
	}, trust, nil, auditor)
	ctx := context.Background()
	agentID := "demo-agent"
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
	for _, sqlText := range queries {
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
		fmt.Fprintf(stdout, "query=%q action=%s trust=%.2f state=%s reason=%s\n", sqlText, decision.Action, snapshot.Score, snapshot.State, decision.Reason)
	}
	snapshot, _ := trust.GetAgentTrust(ctx, agentID)
	fmt.Fprintln(stdout, "\nCurrent trust:")
	printTrustSnapshot(stdout, snapshot)
	return 0
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

func dialPostgresDSN(dsn string, timeout time.Duration) error {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	if port == "" {
		port = "5432"
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return err
	}
	return conn.Close()
}
