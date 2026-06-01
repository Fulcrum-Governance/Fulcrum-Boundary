package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

const (
	GitHubLethalTrifectaSchemaVersion = "boundary.demo.github_lethal_trifecta.v1"
	GitHubLethalTrifectaReason        = "lethal_trifecta_detected"
)

type GitHubLethalTrifectaOptions struct {
	OutPath   string
	Dashboard bool
	Now       time.Time
}

type GitHubLethalTrifectaResult struct {
	SchemaVersion       string `json:"schema_version"`
	Status              string `json:"status"`
	Passed              bool   `json:"passed"`
	FixtureOnly         bool   `json:"fixture_only"`
	RequiresCredentials bool   `json:"requires_credentials"`
	RequiresNetwork     bool   `json:"requires_network"`
	MutatesLiveSystems  bool   `json:"mutates_live_systems"`
	Workspace           string `json:"workspace"`
	WorkspaceRetained   bool   `json:"workspace_retained"`
	ReportPath          string `json:"report_path,omitempty"`
	DashboardPath       string `json:"dashboard_path,omitempty"`
	ConfigPath          string `json:"config_path"`
	PolicyDir           string `json:"policy_dir"`
	SecureGitHubProfile string `json:"secure_github_profile"`
	SecureGitHubPolicy  string `json:"secure_github_policy"`
	DecisionRecordPath  string `json:"decision_record_path"`
	// DecisionRecordObjectPath is the single-record JSON object (the headline
	// write-denial record) that `boundary verify-record` consumes directly. It
	// is what the uniform `decision record path:` line points at. The multi-
	// record JSONL log lives at DecisionRecordPath and is a separate dashboard/
	// audit artifact, not a verify-record input.
	DecisionRecordObjectPath string                      `json:"decision_record_object_path"`
	InventorySummary         firewall.Summary            `json:"inventory_summary"`
	RiskSummary              firewall.RiskSummary        `json:"risk_summary"`
	PolicyFiles              int                         `json:"policy_files"`
	PolicyRules              int                         `json:"policy_rules"`
	RedteamPack              string                      `json:"redteam_pack"`
	Scenario                 GitHubDemoScenario          `json:"scenario"`
	Proof                    []string                    `json:"proof"`
	Limitations              []string                    `json:"limitations"`
	Checks                   []GitHubDemoCheck           `json:"checks"`
	DecisionRecord           governance.DecisionRecordV1 `json:"decision_record"`
}

type GitHubDemoScenario struct {
	ID                 string `json:"id"`
	ExpectedAction     string `json:"expected_action"`
	ActualAction       string `json:"actual_action"`
	Reason             string `json:"reason"`
	PolicyReason       string `json:"policy_reason"`
	MatchedRule        string `json:"matched_rule"`
	UpstreamCalled     bool   `json:"upstream_called"`
	ReadUpstreamCalled bool   `json:"read_upstream_called"`
	TargetRepo         string `json:"target_repo"`
	TaintSource        string `json:"taint_source"`
	MutationClass      string `json:"mutation_class"`
	DecisionRecordID   string `json:"decision_record_id"`
	DecisionHash       string `json:"decision_hash"`
}

type GitHubDemoCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type workspacePlan struct {
	path     string
	report   string
	retained bool
	cleanup  func() error
}

type secureGitHubProof struct {
	read  *securegithub.GovernedResult
	write *securegithub.GovernedResult
}

func RunGitHubLethalTrifecta(ctx context.Context, opts GitHubLethalTrifectaOptions) (*GitHubLethalTrifectaResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workspace, err := prepareWorkspace(opts)
	if err != nil {
		return nil, err
	}
	if !workspace.retained {
		defer func() { _ = workspace.cleanup() }()
	}

	root := filepath.Join(workspace.path, "workspace")
	home := filepath.Join(workspace.path, "home")
	for _, dir := range []string{root, home} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create demo directory %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(configPath, []byte(githubDemoMCPConfig()), 0o600); err != nil {
		return nil, fmt.Errorf("write demo MCP config: %w", err)
	}
	inventory, err := firewall.BuildInventory(firewall.DiscoverOptions{
		Root:            root,
		Home:            home,
		IncludeDefaults: true,
	})
	if err != nil {
		return nil, fmt.Errorf("build inventory: %w", err)
	}
	graph := firewall.BuildRiskGraph(inventory)
	graphBody, err := firewall.RenderRiskGraph(graph, "mermaid")
	if err != nil {
		return nil, fmt.Errorf("render risk graph: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workspace.path, "risk-graph.mmd"), graphBody, 0o600); err != nil {
		return nil, fmt.Errorf("write risk graph artifact: %w", err)
	}

	policyDir := filepath.Join(workspace.path, "policies")
	policies, err := firewall.GenerateStarterPolicies(policyDir, true, "balanced")
	if err != nil {
		return nil, fmt.Errorf("generate starter policies: %w", err)
	}
	loadedPolicies, err := governance.LoadStaticPolicyFiles(policyDir)
	if err != nil {
		return nil, fmt.Errorf("verify starter policies: %w", err)
	}
	if len(loadedPolicies.Warnings) > 0 {
		return nil, fmt.Errorf("verify starter policies: %d warnings", len(loadedPolicies.Warnings))
	}

	setup, err := securegithub.WriteSetup(filepath.Join(workspace.path, "secure-github"), securegithub.Config{})
	if err != nil {
		return nil, fmt.Errorf("write Secure GitHub fixture setup: %w", err)
	}
	redteamResult, err := redteam.Run(ctx, redteam.RunOptions{PackID: redteam.DefaultPackID, Mode: redteam.ModeFixture})
	if err != nil {
		return nil, fmt.Errorf("run redteam pack: %w", err)
	}
	if len(redteamResult.Results) == 0 {
		return nil, fmt.Errorf("redteam pack %q returned no scenarios", redteam.DefaultPackID)
	}
	redteamScenario := redteamResult.Results[0]

	secureProof, err := runSecureGitHubFixture(ctx)
	if err != nil {
		return nil, err
	}
	recordsPath := filepath.Join(workspace.path, DefaultDecisionRecordFilename)
	if err := WriteDecisionRecordsJSONL(recordsPath, []governance.DecisionRecordV1{
		redteamScenario.DecisionRecord,
		secureProof.write.DecisionRecord,
	}); err != nil {
		return nil, fmt.Errorf("write decision-record log artifact: %w", err)
	}
	// Land the single headline write-denial record as a standalone JSON object
	// so the uniform `decision record path:` line points at a file that
	// `boundary verify-record` consumes directly. The multi-record JSONL log
	// above remains a separate dashboard/audit artifact.
	recordObjectPath := filepath.Join(workspace.path, DefaultDecisionRecordObjectFilename)
	if err := WriteDecisionRecordJSON(recordObjectPath, secureProof.write.DecisionRecord); err != nil {
		return nil, fmt.Errorf("write decision-record object artifact: %w", err)
	}

	dashboardPath := ""
	if opts.Dashboard {
		dashboardPath = dashboardArtifactPath(workspace, opts)
		dashboard, err := firewall.BuildDashboard(firewall.DashboardOptions{
			Root:                root,
			Home:                home,
			AdditionalConfigs:   []string{configPath},
			IncludeDefaults:     false,
			PolicyDir:           policyDir,
			DecisionRecordPaths: []string{recordsPath},
			RecentDecisionLimit: 10,
			Now:                 now,
		})
		if err != nil {
			return nil, fmt.Errorf("build dashboard artifact: %w", err)
		}
		body, err := firewall.RenderDashboard(dashboard, "html")
		if err != nil {
			return nil, fmt.Errorf("render dashboard artifact: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(dashboardPath), 0o700); err != nil {
			return nil, fmt.Errorf("create dashboard directory: %w", err)
		}
		if err := os.WriteFile(dashboardPath, body, 0o600); err != nil {
			return nil, fmt.Errorf("write dashboard artifact: %w", err)
		}
	}

	actualAction := upperAction(secureProof.write.Decision.Action)
	expectedAction := upperAction("deny")
	passed := redteamResult.Passed &&
		strings.EqualFold(redteamScenario.ExpectedAction, "deny") &&
		strings.EqualFold(redteamScenario.ActualAction, "deny") &&
		actualAction == expectedAction &&
		!secureProof.write.UpstreamCalled &&
		secureProof.read.UpstreamCalled

	status := "pass"
	if !passed {
		status = "fail"
	}
	result := &GitHubLethalTrifectaResult{
		SchemaVersion:            GitHubLethalTrifectaSchemaVersion,
		Status:                   status,
		Passed:                   passed,
		FixtureOnly:              true,
		RequiresCredentials:      false,
		RequiresNetwork:          false,
		MutatesLiveSystems:       false,
		Workspace:                workspace.path,
		WorkspaceRetained:        workspace.retained,
		ReportPath:               workspace.report,
		DashboardPath:            dashboardPath,
		ConfigPath:               configPath,
		PolicyDir:                policyDir,
		SecureGitHubProfile:      setup.Profile,
		SecureGitHubPolicy:       setup.Policy,
		DecisionRecordPath:       recordsPath,
		DecisionRecordObjectPath: recordObjectPath,
		InventorySummary:         inventory.Summary,
		RiskSummary:              graph.Summary,
		PolicyFiles:              len(policies.Files),
		PolicyRules:              len(loadedPolicies.Rules),
		RedteamPack:              redteamResult.PackID,
		Scenario: GitHubDemoScenario{
			ID:                 redteamScenario.ScenarioID,
			ExpectedAction:     expectedAction,
			ActualAction:       actualAction,
			Reason:             GitHubLethalTrifectaReason,
			PolicyReason:       secureProof.write.Decision.Reason,
			MatchedRule:        secureProof.write.Decision.MatchedRule,
			UpstreamCalled:     secureProof.write.UpstreamCalled,
			ReadUpstreamCalled: secureProof.read.UpstreamCalled,
			TargetRepo:         secureProof.write.Envelope.TargetRepo(),
			TaintSource:        firstString(secureProof.write.Envelope.TaintSources),
			MutationClass:      secureProof.write.Envelope.MutationClass,
			DecisionRecordID:   secureProof.write.DecisionRecord.RecordID,
			DecisionHash:       secureProof.write.DecisionRecord.DecisionHash,
		},
		Proof: []string{
			"Inventory identifies the fixture GitHub MCP server and private-repo mutation tools.",
			"Risk graph includes untrusted GitHub context to private repository mutation paths.",
			"Starter policies are generated and parsed by Boundary's policy loader.",
			"Secure GitHub fixture setup is written without credentials.",
			"Redteam fixture evaluates the lethal-trifecta scenario as DENY.",
			"Secure GitHub adapter denies the write-after-taint mutation before upstream execution.",
			"Decision records are emitted for the denial path.",
		},
		Limitations: []string{
			"Fixture mode does not prove live GitHub App conformance.",
			"Fixture mode does not mutate GitHub, call the network, or validate deployment topology.",
			"Direct GitHub API or upstream MCP access remains a bypass unless operators remove those paths.",
		},
		DecisionRecord: secureProof.write.DecisionRecord,
	}
	result.Checks = githubDemoChecks(result)
	return result, nil
}

func WriteGitHubLethalTrifectaJSON(w io.Writer, result *GitHubLethalTrifectaResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteGitHubLethalTrifectaText(w io.Writer, result *GitHubLethalTrifectaResult) error {
	if result == nil {
		return fmt.Errorf("demo result is required")
	}
	fmt.Fprintln(w, "GitHub lethal-trifecta demo")
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "fixture-only: %t\n", result.FixtureOnly)
	fmt.Fprintf(w, "credentials: none\n")
	fmt.Fprintf(w, "network: none\n")
	fmt.Fprintf(w, "live mutation: none\n")
	fmt.Fprintf(w, "expected action: %s\n", result.Scenario.ExpectedAction)
	fmt.Fprintf(w, "actual action: %s\n", result.Scenario.ActualAction)
	fmt.Fprintf(w, "reason: %s\n", result.Scenario.Reason)
	fmt.Fprintf(w, "matched rule: %s\n", result.Scenario.MatchedRule)
	fmt.Fprintf(w, "upstream_called=%t\n", result.Scenario.UpstreamCalled)
	fmt.Fprintf(w, "read_upstream_called=%t\n", result.Scenario.ReadUpstreamCalled)
	fmt.Fprintf(w, "decision record id: %s\n", result.Scenario.DecisionRecordID)
	fmt.Fprintf(w, "decision hash: %s\n", result.Scenario.DecisionHash)
	// Only advertise the record path when the workspace is retained (--out or
	// --dashboard). Without retention the workspace is a temp directory that is
	// deleted on return, so printing its path would point at a file that no
	// longer exists by the time the operator reads it. The `decision record
	// path:` line points at the single-record JSON object (verify-record
	// consumes it directly); the `decision record log:` line points at the
	// separate multi-record JSONL dashboard/audit log.
	if result.WorkspaceRetained && result.DecisionRecordObjectPath != "" {
		fmt.Fprintf(w, "decision record path: %s\n", result.DecisionRecordObjectPath)
	}
	if result.WorkspaceRetained && result.DecisionRecordPath != "" {
		fmt.Fprintf(w, "decision record log: %s\n", result.DecisionRecordPath)
	}
	fmt.Fprintf(w, "inventory: configs=%d servers=%d github_servers=%d high_risk_servers=%d\n",
		result.InventorySummary.ConfigFiles,
		result.InventorySummary.Servers,
		result.InventorySummary.GitHubServers,
		result.InventorySummary.HighRiskServers,
	)
	fmt.Fprintf(w, "risk paths: total=%d high_risk=%d repo_write=%d\n",
		result.RiskSummary.Paths,
		result.RiskSummary.HighRiskPaths,
		result.RiskSummary.RepoWritePaths,
	)
	fmt.Fprintf(w, "policies: files=%d rules=%d\n", result.PolicyFiles, result.PolicyRules)
	fmt.Fprintf(w, "workspace retained: %t\n", result.WorkspaceRetained)
	if result.WorkspaceRetained {
		fmt.Fprintf(w, "workspace: %s\n", result.Workspace)
	}
	if result.ReportPath != "" {
		fmt.Fprintf(w, "report: %s\n", result.ReportPath)
	}
	if result.DashboardPath != "" {
		fmt.Fprintf(w, "dashboard: %s\n", result.DashboardPath)
	}
	fmt.Fprintln(w, "\nChecks:")
	for _, check := range result.Checks {
		fmt.Fprintf(w, "- [%s] %s: %s\n", check.Status, check.ID, check.Detail)
	}
	fmt.Fprintln(w, "\nWhat this proves:")
	for _, proof := range result.Proof {
		fmt.Fprintf(w, "- %s\n", proof)
	}
	fmt.Fprintln(w, "\nWhat this does not prove:")
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "- %s\n", limitation)
	}
	return nil
}

func WriteGitHubLethalTrifectaMarkdown(w io.Writer, result *GitHubLethalTrifectaResult) error {
	if result == nil {
		return fmt.Errorf("demo result is required")
	}
	fmt.Fprintln(w, "# GitHub Lethal-Trifecta Demo")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Status: `%s`\n", result.Status)
	fmt.Fprintf(w, "- Fixture only: `%t`\n", result.FixtureOnly)
	fmt.Fprintln(w, "- Credentials: `none`")
	fmt.Fprintln(w, "- Network: `none`")
	fmt.Fprintln(w, "- Live mutation: `none`")
	fmt.Fprintf(w, "- Expected action: `%s`\n", result.Scenario.ExpectedAction)
	fmt.Fprintf(w, "- Actual action: `%s`\n", result.Scenario.ActualAction)
	fmt.Fprintf(w, "- Reason: `%s`\n", result.Scenario.Reason)
	fmt.Fprintf(w, "- Matched rule: `%s`\n", result.Scenario.MatchedRule)
	fmt.Fprintf(w, "- Upstream called: `%t`\n", result.Scenario.UpstreamCalled)
	fmt.Fprintf(w, "- Decision record id: `%s`\n", result.Scenario.DecisionRecordID)
	fmt.Fprintf(w, "- Decision hash: `%s`\n", result.Scenario.DecisionHash)
	if result.DashboardPath != "" {
		fmt.Fprintf(w, "- Local dashboard artifact: `%s`\n", result.DashboardPath)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Evidence")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Inventory: `%d` config file, `%d` server, `%d` GitHub server, `%d` high-risk server\n",
		result.InventorySummary.ConfigFiles,
		result.InventorySummary.Servers,
		result.InventorySummary.GitHubServers,
		result.InventorySummary.HighRiskServers,
	)
	fmt.Fprintf(w, "- Risk graph: `%d` paths, `%d` high-risk paths, `%d` repo-write paths\n",
		result.RiskSummary.Paths,
		result.RiskSummary.HighRiskPaths,
		result.RiskSummary.RepoWritePaths,
	)
	fmt.Fprintf(w, "- Starter policies: `%d` files, `%d` rules\n", result.PolicyFiles, result.PolicyRules)
	if result.WorkspaceRetained && result.DecisionRecordObjectPath != "" {
		fmt.Fprintf(w, "- Decision record path: `%s`\n", result.DecisionRecordObjectPath)
	}
	if result.WorkspaceRetained && result.DecisionRecordPath != "" {
		fmt.Fprintf(w, "- Decision record log: `%s`\n", result.DecisionRecordPath)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## What This Proves")
	fmt.Fprintln(w)
	for _, proof := range result.Proof {
		fmt.Fprintf(w, "- %s\n", proof)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## What This Does Not Prove")
	fmt.Fprintln(w)
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "- %s\n", limitation)
	}
	return nil
}

func prepareWorkspace(opts GitHubLethalTrifectaOptions) (workspacePlan, error) {
	if opts.OutPath == "" {
		dir, err := os.MkdirTemp("", "boundary-github-lethal-trifecta-*")
		if err != nil {
			return workspacePlan{}, err
		}
		retained := opts.Dashboard
		return workspacePlan{
			path:     dir,
			retained: retained,
			cleanup: func() error {
				return os.RemoveAll(dir)
			},
		}, nil
	}
	report, err := filepath.Abs(opts.OutPath)
	if err != nil {
		return workspacePlan{}, err
	}
	dir, err := ArtifactDir(opts.OutPath, "github-lethal-trifecta")
	if err != nil {
		return workspacePlan{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return workspacePlan{}, fmt.Errorf("create demo artifact workspace: %w", err)
	}
	return workspacePlan{
		path:     dir,
		report:   report,
		retained: true,
		cleanup:  func() error { return nil },
	}, nil
}

func dashboardArtifactPath(workspace workspacePlan, opts GitHubLethalTrifectaOptions) string {
	if opts.OutPath != "" {
		report, err := filepath.Abs(opts.OutPath)
		if err == nil {
			return filepath.Join(filepath.Dir(report), "github-lethal-trifecta-dashboard.html")
		}
	}
	return filepath.Join(workspace.path, "github-lethal-trifecta-dashboard.html")
}

func githubDemoMCPConfig() string {
	return `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [
        {"name": "get_issue"},
        {"name": "create_or_update_file"},
        {"name": "merge_pull_request"}
      ]
    }
  }
}
`
}

func runSecureGitHubFixture(ctx context.Context) (secureGitHubProof, error) {
	cfg := securegithub.DefaultConfig()
	cfg.SessionID = "demo-github-lethal-trifecta-session"
	adapter := securegithub.NewFixtureAdapter(cfg)

	read, err := adapter.GovernToolCall(ctx, securegithub.ToolCall{
		ID:        "demo-read",
		ToolName:  "get_issue",
		AgentID:   cfg.AgentID,
		TenantID:  cfg.TenantID,
		SessionID: cfg.SessionID,
		TraceID:   "trace-demo-github-lethal-trifecta",
		Arguments: map[string]any{
			"owner":              cfg.Owner,
			"repo":               cfg.Repo,
			"issue_number":       1,
			"source_class":       "external_collaborator",
			"author_association": "CONTRIBUTOR",
			"request_id":         "demo-github-lethal-trifecta-read",
			"envelope_id":        "env-demo-github-lethal-trifecta",
		},
	})
	if err != nil {
		return secureGitHubProof{}, fmt.Errorf("run Secure GitHub read fixture: %w", err)
	}
	if read == nil || !read.UpstreamCalled || read.Decision == nil || !read.Decision.Allowed() {
		return secureGitHubProof{}, fmt.Errorf("secure GitHub read fixture did not reach upstream as expected")
	}

	write, err := adapter.GovernToolCall(ctx, securegithub.ToolCall{
		ID:        "demo-write",
		ToolName:  "create_or_update_file",
		AgentID:   cfg.AgentID,
		TenantID:  cfg.TenantID,
		SessionID: cfg.SessionID,
		TraceID:   "trace-demo-github-lethal-trifecta",
		Arguments: map[string]any{
			"owner":       cfg.Owner,
			"repo":        cfg.Repo,
			"path":        "README.md",
			"branch":      "main",
			"content":     "fixture-only private repository mutation attempt",
			"target_sink": "private_repo",
			"private":     true,
			"request_id":  "demo-github-lethal-trifecta-write",
			"envelope_id": "env-demo-github-lethal-trifecta",
		},
	})
	if err != nil {
		return secureGitHubProof{}, fmt.Errorf("run Secure GitHub write fixture: %w", err)
	}
	if write == nil || write.Decision == nil {
		return secureGitHubProof{}, fmt.Errorf("secure GitHub write fixture produced no governance decision")
	}
	return secureGitHubProof{read: read, write: write}, nil
}

func githubDemoChecks(result *GitHubLethalTrifectaResult) []GitHubDemoCheck {
	check := func(id string, ok bool, detail string) GitHubDemoCheck {
		status := "pass"
		if !ok {
			status = "fail"
		}
		return GitHubDemoCheck{ID: id, Status: status, Detail: detail}
	}
	return []GitHubDemoCheck{
		check("inventory_fixture_loads", result.InventorySummary.GitHubServers == 1, fmt.Sprintf("github_servers=%d", result.InventorySummary.GitHubServers)),
		check("risk_graph_detects_path", result.RiskSummary.RepoWritePaths > 0, fmt.Sprintf("repo_write_paths=%d", result.RiskSummary.RepoWritePaths)),
		check("starter_policies_verify", result.PolicyFiles > 0 && result.PolicyRules > 0, fmt.Sprintf("files=%d rules=%d", result.PolicyFiles, result.PolicyRules)),
		check("secure_github_fixture_setup", result.SecureGitHubProfile != "" && result.SecureGitHubPolicy != "", "fixture profile and policy artifacts written"),
		check("redteam_denies_scenario", result.Scenario.ActualAction == "DENY", fmt.Sprintf("actual_action=%s", result.Scenario.ActualAction)),
		check("write_denied_before_upstream", !result.Scenario.UpstreamCalled, fmt.Sprintf("upstream_called=%t", result.Scenario.UpstreamCalled)),
		check("decision_record_emitted", result.Scenario.DecisionRecordID != "" && result.Scenario.DecisionHash != "", result.Scenario.DecisionRecordID),
	}
}

func upperAction(action string) string {
	return strings.ToUpper(strings.TrimSpace(action))
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
