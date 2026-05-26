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

	"github.com/fulcrum-governance/boundary/adapters/mcp"
	"github.com/fulcrum-governance/boundary/governance"
)

var Version = "0.2.0-dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printRootHelp(stdout)
		return 0
	}

	switch args[0] {
	case "serve":
		return runServe(args[1:], stdout, stderr)
	case "demo":
		return runDemo(args[1:], stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "audit":
		return runAudit(args[1:], os.Stdin, stdout, stderr)
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
  serve           Start the Boundary gateway
  demo postgres   Run the Postgres safety demo against a running gateway
  verify          Validate YAML policy files
  doctor          Check local gateway prerequisites
  audit           Pretty-print structured decision records

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
	listen := fs.String("listen", ":8080", "HTTP listen address")
	policyDir := fs.String("policies", "./policies/", "directory containing YAML policy files")
	upstream := fs.String("upstream", "postgres://demo:demo@localhost:5432/demo?sslmode=disable", "upstream MCP HTTP URL or Postgres demo DSN")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	rules, err := governance.LoadStaticPoliciesFromDir(*policyDir)
	if err != nil {
		fmt.Fprintf(stderr, "load policies: %v\n", err)
		return 1
	}

	logger := slog.New(slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: rules,
		GatewayVersion: Version,
	}, nil, nil, governance.NewSlogAuditPublisher(logger))

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

	fmt.Fprintf(stderr, "boundary serve listening on %s in %s mode with %d static policy rules\n", *listen, mode, len(rules))
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

func serveHandler(upstream string, pipeline *governance.Pipeline) (http.Handler, string, func() error, error) {
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
		return 0
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
