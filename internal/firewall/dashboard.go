package firewall

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const dashboardSchema = "boundary.firewall.dashboard.v1"

type DashboardOptions struct {
	Root                string
	Home                string
	AdditionalConfigs   []string
	IncludeDefaults     bool
	PolicyDir           string
	LockPath            string
	ReceiptsDir         string
	DecisionRecordPaths []string
	RecentDecisionLimit int
	Now                 time.Time
}

type Dashboard struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	LocalOnly     bool                    `json:"local_only"`
	Inventory     Inventory               `json:"inventory"`
	RiskGraph     RiskGraph               `json:"risk_graph"`
	Policies      DashboardPolicyStatus   `json:"policies"`
	Install       DashboardInstallStatus  `json:"install"`
	Lock          DashboardLockStatus     `json:"lock"`
	Decisions     DashboardDecisionStatus `json:"decisions"`
}

type DashboardPolicyStatus struct {
	Path     string   `json:"path,omitempty"`
	Status   string   `json:"status"`
	Files    int      `json:"files"`
	Rules    int      `json:"rules"`
	Warnings []string `json:"warnings,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type DashboardInstallStatus struct {
	ReceiptsDir string                    `json:"receipts_dir,omitempty"`
	Status      string                    `json:"status"`
	Receipts    int                       `json:"receipts"`
	Installed   int                       `json:"installed"`
	Planned     int                       `json:"planned"`
	Errors      []string                  `json:"errors,omitempty"`
	Recent      []DashboardInstallReceipt `json:"recent,omitempty"`
}

type DashboardInstallReceipt struct {
	Path        string   `json:"path"`
	GeneratedAt string   `json:"generated_at,omitempty"`
	ConfigPath  string   `json:"config_path,omitempty"`
	Client      string   `json:"client,omitempty"`
	State       string   `json:"state,omitempty"`
	DryRun      bool     `json:"dry_run"`
	Mutated     bool     `json:"mutated"`
	Servers     []string `json:"servers,omitempty"`
}

type DashboardLockStatus struct {
	Path    string            `json:"path,omitempty"`
	Status  string            `json:"status"`
	Allowed bool              `json:"allowed"`
	Summary LockSummary       `json:"summary,omitempty"`
	Matches []DescriptorMatch `json:"matches,omitempty"`
	Error   string            `json:"error,omitempty"`
}

type DashboardDecisionStatus struct {
	Paths  []string            `json:"paths,omitempty"`
	Status string              `json:"status"`
	Count  int                 `json:"count"`
	Errors []string            `json:"errors,omitempty"`
	Recent []DashboardDecision `json:"recent,omitempty"`
}

type DashboardDecision struct {
	RecordID      string `json:"record_id,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Adapter       string `json:"adapter,omitempty"`
	Tool          string `json:"tool,omitempty"`
	Action        string `json:"action,omitempty"`
	MatchedRule   string `json:"matched_rule,omitempty"`
	DecisionMode  string `json:"decision_mode,omitempty"`
	RequestHash   string `json:"request_hash,omitempty"`
	DecisionHash  string `json:"decision_hash,omitempty"`
	DecisionIndex int    `json:"decision_index"`
}

func BuildDashboard(options DashboardOptions) (Dashboard, error) {
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	inventory, err := BuildInventory(DiscoverOptions{
		Root:                  options.Root,
		Home:                  options.Home,
		AdditionalConfigPaths: options.AdditionalConfigs,
		IncludeDefaults:       options.IncludeDefaults,
	})
	if err != nil {
		return Dashboard{}, err
	}
	graph := BuildRiskGraph(inventory)
	return Dashboard{
		SchemaVersion: dashboardSchema,
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		LocalOnly:     true,
		Inventory:     inventory,
		RiskGraph:     graph,
		Policies:      loadDashboardPolicyStatus(options.PolicyDir),
		Install:       loadDashboardInstallStatus(options.ReceiptsDir),
		Lock:          loadDashboardLockStatus(options.LockPath),
		Decisions:     loadDashboardDecisions(options.DecisionRecordPaths, options.RecentDecisionLimit),
	}, nil
}

func RenderDashboard(dashboard Dashboard, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "", "text":
		return []byte(renderDashboardText(dashboard)), nil
	case "json":
		return json.MarshalIndent(dashboard, "", "  ")
	case "html":
		return renderDashboardHTML(dashboard)
	default:
		return nil, fmt.Errorf("unsupported dashboard format %q", format)
	}
}

func loadDashboardPolicyStatus(dir string) DashboardPolicyStatus {
	if dir == "" {
		return DashboardPolicyStatus{Status: "not_configured"}
	}
	status := DashboardPolicyStatus{Path: dir}
	stat, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			status.Status = "missing"
			return status
		}
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	if !stat.IsDir() {
		status.Status = "error"
		status.Error = "policy path is not a directory"
		return status
	}
	result, err := governance.LoadStaticPolicyFiles(dir)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	status.Status = "ok"
	status.Files = len(result.Files)
	status.Rules = len(result.Rules)
	status.Warnings = append([]string{}, result.Warnings...)
	if len(result.Warnings) > 0 {
		status.Status = "warning"
	}
	return status
}

func loadDashboardInstallStatus(dir string) DashboardInstallStatus {
	if dir == "" {
		return DashboardInstallStatus{Status: "not_configured"}
	}
	status := DashboardInstallStatus{ReceiptsDir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			status.Status = "missing"
			return status
		}
		status.Status = "error"
		status.Errors = append(status.Errors, err.Error())
		return status
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		// #nosec G304 -- dashboard reads operator-selected local Boundary install receipt files.
		body, err := os.ReadFile(path)
		if err != nil {
			status.Errors = append(status.Errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		var receipt InstallResult
		if err := json.Unmarshal(body, &receipt); err != nil {
			status.Errors = append(status.Errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if receipt.SchemaVersion != installReceiptSchema {
			status.Errors = append(status.Errors, fmt.Sprintf("%s: unsupported schema %q", path, receipt.SchemaVersion))
			continue
		}
		summary := DashboardInstallReceipt{
			Path:        path,
			GeneratedAt: receipt.GeneratedAt,
			ConfigPath:  receipt.ConfigPath,
			Client:      string(receipt.Client),
			State:       receipt.State,
			DryRun:      receipt.DryRun,
			Mutated:     receipt.Mutated,
			Servers:     installedServerNames(receipt.Servers),
		}
		status.Recent = append(status.Recent, summary)
		status.Receipts++
		switch receipt.State {
		case "installed":
			status.Installed++
		case "planned":
			status.Planned++
		}
	}
	sort.Slice(status.Recent, func(i, j int) bool {
		return status.Recent[i].GeneratedAt > status.Recent[j].GeneratedAt
	})
	if len(status.Recent) > 10 {
		status.Recent = status.Recent[:10]
	}
	status.Status = "ok"
	if len(status.Errors) > 0 {
		status.Status = "warning"
	}
	return status
}

func loadDashboardLockStatus(path string) DashboardLockStatus {
	if path == "" {
		return DashboardLockStatus{Status: "not_configured"}
	}
	status := DashboardLockStatus{Path: path}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			status.Status = "missing"
			return status
		}
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	result, err := VerifyDescriptorLock(VerifyLockOptions{LockPath: path, OnChange: "warn"})
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	status.Status = result.Status
	status.Allowed = result.Allowed
	status.Summary = result.Summary
	status.Matches = append([]DescriptorMatch{}, result.Matches...)
	return status
}

func loadDashboardDecisions(paths []string, limit int) DashboardDecisionStatus {
	if limit <= 0 {
		limit = 10
	}
	status := DashboardDecisionStatus{Status: "not_configured"}
	if len(paths) == 0 {
		return status
	}
	status.Status = "ok"
	status.Paths = append([]string{}, paths...)
	var all []DashboardDecision
	for _, path := range paths {
		records, err := readDecisionRecordFile(path)
		if err != nil {
			status.Errors = append(status.Errors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		all = append(all, records...)
	}
	for i := range all {
		all[i].DecisionIndex = i + 1
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].Timestamp > all[j].Timestamp
	})
	status.Count = len(all)
	if len(all) > limit {
		all = all[:limit]
	}
	status.Recent = all
	if len(status.Errors) > 0 {
		status.Status = "warning"
	}
	return status
}

func readDecisionRecordFile(path string) ([]DashboardDecision, error) {
	// #nosec G304 -- dashboard reads operator-selected local decision record files.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var decisions []DashboardDecision
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record governance.DecisionRecordV1
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		decisions = append(decisions, decisionSummary(record))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return decisions, nil
}

func decisionSummary(record governance.DecisionRecordV1) DashboardDecision {
	timestamp := ""
	if !record.Timestamp.IsZero() {
		timestamp = record.Timestamp.UTC().Format(time.RFC3339)
	}
	return DashboardDecision{
		RecordID:     record.RecordID,
		Timestamp:    timestamp,
		Adapter:      string(record.Adapter),
		Tool:         record.Tool,
		Action:       record.Action,
		MatchedRule:  record.MatchedRule,
		DecisionMode: string(record.DecisionMode),
		RequestHash:  record.RequestHash,
		DecisionHash: record.DecisionHash,
	}
}

func installedServerNames(servers []InstalledServer) []string {
	names := make([]string, 0, len(servers))
	for _, server := range servers {
		names = append(names, server.Name)
	}
	sort.Strings(names)
	return names
}

func renderDashboardText(dashboard Dashboard) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Boundary Firewall Dashboard")
	fmt.Fprintf(&b, "local-only: %t\n", dashboard.LocalOnly)
	fmt.Fprintf(&b, "generated: %s\n", dashboard.GeneratedAt)
	fmt.Fprintf(&b, "configs: %d\n", dashboard.Inventory.Summary.ConfigFiles)
	fmt.Fprintf(&b, "servers: %d\n", dashboard.Inventory.Summary.Servers)
	fmt.Fprintf(&b, "github servers: %d\n", dashboard.Inventory.Summary.GitHubServers)
	fmt.Fprintf(&b, "high-risk servers: %d\n", dashboard.Inventory.Summary.HighRiskServers)
	fmt.Fprintf(&b, "risk paths: %d\n", dashboard.RiskGraph.Summary.Paths)
	fmt.Fprintf(&b, "high-risk paths: %d\n", dashboard.RiskGraph.Summary.HighRiskPaths)
	fmt.Fprintf(&b, "policy status: %s", dashboard.Policies.Status)
	if dashboard.Policies.Rules > 0 {
		fmt.Fprintf(&b, " (%d rules)", dashboard.Policies.Rules)
	}
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "install receipts: %d\n", dashboard.Install.Receipts)
	fmt.Fprintf(&b, "lock status: %s\n", dashboard.Lock.Status)
	fmt.Fprintf(&b, "recent decisions: %d\n", dashboard.Decisions.Count)
	for _, decision := range dashboard.Decisions.Recent {
		fmt.Fprintf(&b, "- %s %s %s rule=%s\n", decision.Timestamp, decision.Action, decision.Tool, firstNonEmpty(decision.MatchedRule, "none"))
	}
	return b.String()
}

func renderDashboardHTML(dashboard Dashboard) ([]byte, error) {
	tmpl, err := template.New("dashboard").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(dashboardHTMLTemplate)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, dashboard); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

const dashboardHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Boundary Firewall Dashboard</title>
  <style>
    :root {
      color-scheme: light;
      --ink: #172026;
      --muted: #5d6870;
      --line: #d6dee4;
      --bg: #f6f8fa;
      --panel: #ffffff;
      --accent: #0f766e;
      --warn: #9a3412;
      --deny: #991b1b;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      color: var(--ink);
      background: var(--bg);
    }
    main { max-width: 1180px; margin: 0 auto; padding: 32px 20px 48px; }
    header { display: flex; justify-content: space-between; gap: 20px; align-items: flex-start; margin-bottom: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; line-height: 1.15; letter-spacing: 0; }
    h2 { margin: 0 0 12px; font-size: 16px; line-height: 1.2; letter-spacing: 0; }
    p { margin: 0; color: var(--muted); line-height: 1.5; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    .badge {
      display: inline-flex;
      align-items: center;
      min-height: 28px;
      padding: 4px 10px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: var(--panel);
      color: var(--accent);
      font-weight: 700;
      white-space: nowrap;
    }
    .grid { display: grid; gap: 12px; }
    .metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); margin-bottom: 12px; }
    .sections { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    section, .metric {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 16px;
    }
    .metric strong { display: block; font-size: 26px; line-height: 1.1; }
    .metric span { display: block; margin-top: 6px; color: var(--muted); font-size: 13px; }
    table { width: 100%; border-collapse: collapse; font-size: 13px; }
    th, td { border-top: 1px solid var(--line); padding: 8px 6px; text-align: left; vertical-align: top; }
    th { color: var(--muted); font-weight: 700; }
    ul { margin: 0; padding-left: 18px; }
    li { margin: 5px 0; }
    .status-ok { color: var(--accent); font-weight: 700; }
    .status-warning, .status-drift { color: var(--warn); font-weight: 700; }
    .status-error, .action-deny { color: var(--deny); font-weight: 700; }
    .empty { color: var(--muted); }
    @media (max-width: 760px) {
      header { display: block; }
      .badge { margin-top: 14px; }
      .metrics, .sections { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
<main>
  <header>
    <div>
      <h1>Boundary Firewall Dashboard</h1>
      <p>Local-only view generated from Boundary inventory, risk graph, policy, install, lock, and decision-record files.</p>
      <p>Generated <code>{{.GeneratedAt}}</code></p>
    </div>
    <span class="badge">Local only</span>
  </header>

  <div class="grid metrics">
    <div class="metric"><strong>{{.Inventory.Summary.ConfigFiles}}</strong><span>Config files</span></div>
    <div class="metric"><strong>{{.Inventory.Summary.Servers}}</strong><span>MCP servers</span></div>
    <div class="metric"><strong>{{.Inventory.Summary.HighRiskServers}}</strong><span>High-risk servers</span></div>
    <div class="metric"><strong>{{.RiskGraph.Summary.Paths}}</strong><span>Risk paths</span></div>
  </div>

  <div class="grid sections">
    <section>
      <h2>Risk Paths</h2>
      {{if .RiskGraph.Paths}}
      <table>
        <thead><tr><th>Class</th><th>Server</th><th>Path</th></tr></thead>
        <tbody>
        {{range .RiskGraph.Paths}}
          <tr><td><code>{{.RiskClass}}</code></td><td>{{.Server}}</td><td>{{.Category}}</td></tr>
        {{end}}
        </tbody>
      </table>
      {{else}}<p class="empty">No risk paths detected from the selected local configs.</p>{{end}}
    </section>

    <section>
      <h2>Policies</h2>
      <p>Status: <span class="status-{{.Policies.Status}}">{{.Policies.Status}}</span></p>
      <p>Files: <code>{{.Policies.Files}}</code> Rules: <code>{{.Policies.Rules}}</code></p>
      {{if .Policies.Error}}<p class="status-error">{{.Policies.Error}}</p>{{end}}
      {{if .Policies.Warnings}}<ul>{{range .Policies.Warnings}}<li>{{.}}</li>{{end}}</ul>{{end}}
    </section>

    <section>
      <h2>Install Status</h2>
      <p>Status: <span class="status-{{.Install.Status}}">{{.Install.Status}}</span></p>
      <p>Receipts: <code>{{.Install.Receipts}}</code> Installed: <code>{{.Install.Installed}}</code> Planned: <code>{{.Install.Planned}}</code></p>
      {{if .Install.Recent}}
      <table>
        <thead><tr><th>State</th><th>Client</th><th>Servers</th></tr></thead>
        <tbody>
        {{range .Install.Recent}}
          <tr><td>{{.State}}</td><td>{{.Client}}</td><td>{{join .Servers ", "}}</td></tr>
        {{end}}
        </tbody>
      </table>
      {{else}}<p class="empty">No install receipts found.</p>{{end}}
    </section>

    <section>
      <h2>Descriptor Lock</h2>
      <p>Status: <span class="status-{{.Lock.Status}}">{{.Lock.Status}}</span></p>
      <p>Unchanged: <code>{{.Lock.Summary.Unchanged}}</code> Changed: <code>{{.Lock.Summary.Changed}}</code> Missing: <code>{{.Lock.Summary.Missing}}</code> Unexpected: <code>{{.Lock.Summary.Unexpected}}</code></p>
      {{if .Lock.Error}}<p class="status-error">{{.Lock.Error}}</p>{{end}}
    </section>

    <section>
      <h2>Recent Decision Records</h2>
      <p>Status: <span class="status-{{.Decisions.Status}}">{{.Decisions.Status}}</span> Count: <code>{{.Decisions.Count}}</code></p>
      {{if .Decisions.Recent}}
      <table>
        <thead><tr><th>Action</th><th>Tool</th><th>Rule</th></tr></thead>
        <tbody>
        {{range .Decisions.Recent}}
          <tr><td class="action-{{.Action}}">{{.Action}}</td><td>{{.Tool}}</td><td>{{.MatchedRule}}</td></tr>
        {{end}}
        </tbody>
      </table>
      {{else}}<p class="empty">Pass local JSONL decision-record files with <code>--records</code>.</p>{{end}}
    </section>

    <section>
      <h2>Inventory</h2>
      {{if .Inventory.Servers}}
      <table>
        <thead><tr><th>Client</th><th>Server</th><th>Risk</th></tr></thead>
        <tbody>
        {{range .Inventory.Servers}}
          <tr><td>{{.Client}}</td><td>{{.Name}}</td><td><code>{{.HighestRisk}}</code></td></tr>
        {{end}}
        </tbody>
      </table>
      {{else}}<p class="empty">No MCP servers found in selected local configs.</p>{{end}}
    </section>
  </div>
</main>
</body>
</html>
`
