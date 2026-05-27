package boundarycli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

func runFirewallDashboard(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary dashboard", stderr, commandHelp{
		Purpose: "Render a local-only dashboard from inventory, policies, receipts, and records.",
		Usage:   "boundary dashboard [--format text|html|json] [--serve] [--out PATH]",
		Common: []string{
			"boundary dashboard --format html --out .boundary/firewall/dashboard.html",
			"boundary dashboard --serve --listen 127.0.0.1:8942",
		},
		Notes: []string{
			"The served dashboard is loopback-only and intended for local operator review.",
			"Dashboard output is a presentation surface, not a policy enforcement path.",
		},
	})
	root := fs.String("root", ".", "project root to inspect for repo-local MCP configs")
	home := fs.String("home", "", "home directory to inspect for user MCP configs")
	policyDir := fs.String("policies", "boundary-firewall-policies", "directory containing generated or reviewed Boundary firewall policies")
	lockPath := fs.String("lock", ".boundary/firewall/locks/descriptor-lock.json", "descriptor lockfile path to verify")
	receiptsDir := fs.String("receipts", ".boundary/firewall/install-receipts", "directory containing Boundary install receipts")
	format := fs.String("format", "text", "dashboard format: text, html, or json")
	out := fs.String("out", "", "write dashboard output to a file instead of stdout")
	serve := fs.Bool("serve", false, "serve the dashboard from a loopback-only local HTTP server")
	listen := fs.String("listen", "127.0.0.1:8942", "loopback listen address for --serve")
	includeDefaults := fs.Bool("include-defaults", true, "include known user-level Claude Desktop, Cursor, and VS Code config paths")
	var configs pathListFlag
	var records pathListFlag
	fs.Var(&configs, "config", "additional MCP config path; may be repeated or comma-separated")
	fs.Var(&records, "records", "local decision-record JSONL file; may be repeated or comma-separated")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	options := firewall.DashboardOptions{
		Root:                *root,
		Home:                *home,
		AdditionalConfigs:   []string(configs),
		IncludeDefaults:     *includeDefaults,
		PolicyDir:           *policyDir,
		LockPath:            *lockPath,
		ReceiptsDir:         *receiptsDir,
		DecisionRecordPaths: []string(records),
	}

	if *serve {
		if *out != "" {
			fmt.Fprintln(stderr, "dashboard: --out cannot be used with --serve")
			return 1
		}
		if !loopbackAddress(*listen) {
			fmt.Fprintf(stderr, "dashboard: --serve requires a loopback listen address, got %q\n", *listen)
			return 1
		}
		return serveDashboard(*listen, options, stdout, stderr)
	}

	dashboard, err := firewall.BuildDashboard(options)
	if err != nil {
		fmt.Fprintf(stderr, "dashboard: %v\n", err)
		return 1
	}
	body, err := firewall.RenderDashboard(dashboard, *format)
	if err != nil {
		fmt.Fprintf(stderr, "dashboard: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	if *out == "" {
		_, _ = stdout.Write(body)
		return 0
	}
	if err := os.WriteFile(*out, body, 0o600); err != nil {
		fmt.Fprintf(stderr, "write dashboard: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "dashboard: %s\n", *out)
	return 0
}

func serveDashboard(listen string, options firewall.DashboardOptions, stdout, stderr io.Writer) int {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		dashboard, err := firewall.BuildDashboard(options)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		format := "html"
		if strings.EqualFold(r.URL.Query().Get("format"), "json") {
			format = "json"
		}
		body, err := firewall.RenderDashboard(dashboard, format)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if format == "json" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		_, _ = w.Write(body)
	})
	server := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	fmt.Fprintf(stdout, "dashboard: http://%s\n", listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(stderr, "dashboard server: %v\n", err)
		return 1
	}
	return 0
}

func loopbackAddress(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
