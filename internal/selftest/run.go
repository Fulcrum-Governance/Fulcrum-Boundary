package selftest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

func Run(ctx context.Context, opts Options) (*Result, error) {
	started := time.Now().UTC()
	workspace, err := os.MkdirTemp("", "boundary-selftest-*")
	if err != nil {
		return nil, fmt.Errorf("create selftest workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()

	root := filepath.Join(workspace, "root")
	home := filepath.Join(workspace, "home")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create selftest root: %w", err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, fmt.Errorf("create selftest home: %w", err)
	}
	configPath := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(configPath, []byte(fixtureMCPConfig("create_or_update_file")), 0o600); err != nil {
		return nil, fmt.Errorf("write selftest MCP fixture: %w", err)
	}

	result := &Result{
		SchemaVersion:       SchemaVersion,
		Status:              StatusPass,
		Passed:              true,
		StartedAt:           started.Format(time.RFC3339),
		MutatesLiveSystems:  false,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		Next: []string{
			"go test ./claims/... -count=1",
		},
	}

	var inventory firewall.Inventory
	var graph firewall.RiskGraph
	var lockPath string
	var redteamResult *redteam.RunResult
	var redteamScenario *redteam.ScenarioResult

	addCheck := func(id, name, command string, fn func() (string, error)) {
		checkStart := time.Now()
		check := CheckResult{
			ID:      id,
			Name:    name,
			Status:  StatusPass,
			Command: command,
		}
		if err := ctx.Err(); err != nil {
			check.Status = StatusFail
			check.Detail = err.Error()
		} else {
			detail, err := fn()
			if err != nil {
				check.Status = StatusFail
				check.Detail = err.Error()
			} else {
				check.Detail = detail
			}
		}
		check.DurationMS = time.Since(checkStart).Milliseconds()
		if check.Status != StatusPass {
			result.Status = StatusFail
			result.Passed = false
		}
		result.Checks = append(result.Checks, check)
	}

	addCheck("cli_boots", "CLI command boots", "boundary selftest", func() (string, error) {
		return "boundary selftest command dispatcher reached", nil
	})

	addCheck("inventory_fixture_loads", "Inventory fixture loads", "boundary inventory --root <fixture> --home <fixture>", func() (string, error) {
		var err error
		inventory, err = firewall.BuildInventory(firewall.DiscoverOptions{
			Root:                  root,
			Home:                  home,
			AdditionalConfigPaths: []string{configPath},
			IncludeDefaults:       false,
		})
		if err != nil {
			return "", err
		}
		if inventory.Summary.Servers != 1 {
			return "", fmt.Errorf("expected 1 fixture MCP server, got %d", inventory.Summary.Servers)
		}
		if inventory.Summary.GitHubServers != 1 {
			return "", fmt.Errorf("expected 1 GitHub fixture server, got %d", inventory.Summary.GitHubServers)
		}
		if inventory.Summary.HighRiskServers == 0 {
			return "", fmt.Errorf("expected high-risk GitHub fixture capabilities")
		}
		rendered, err := firewall.RenderInventory(inventory, "markdown")
		if err != nil {
			return "", err
		}
		text := string(rendered)
		if !strings.Contains(text, "Boundary MCP Inventory") || !strings.Contains(text, "create_or_update_file:W1") {
			return "", fmt.Errorf("markdown inventory omitted expected GitHub write risk")
		}
		if strings.Contains(text, "fixture-redacted") {
			return "", fmt.Errorf("inventory leaked fixture token value")
		}
		return "loaded 1 GitHub MCP fixture with high-risk capabilities and redacted env values", nil
	})

	addCheck("risk_graph_fixture_renders", "Risk graph fixture renders", "boundary graph --root <fixture> --home <fixture> --format mermaid", func() (string, error) {
		if inventory.SchemaVersion == "" {
			return "", fmt.Errorf("inventory check did not produce a fixture inventory")
		}
		graph = firewall.BuildRiskGraph(inventory)
		if graph.Summary.Paths == 0 {
			return "", fmt.Errorf("expected fixture risk paths, got 0")
		}
		if graph.Summary.RepoWritePaths == 0 {
			return "", fmt.Errorf("expected at least one repo-write risk path")
		}
		rendered, err := firewall.RenderRiskGraph(graph, "mermaid")
		if err != nil {
			return "", err
		}
		text := string(rendered)
		if !strings.Contains(text, "flowchart LR") || !strings.Contains(text, "repo_write_path") {
			return "", fmt.Errorf("mermaid graph omitted expected repo-write path")
		}
		return fmt.Sprintf("rendered %d risk paths including repo-write paths", graph.Summary.Paths), nil
	})

	addCheck("policy_generator_valid", "Policy generator emits valid starter policies", "boundary policy generate --out <fixture> && boundary verify --policies <fixture>", func() (string, error) {
		policyDir := filepath.Join(workspace, "policies")
		generated, err := firewall.GenerateStarterPolicies(policyDir, false, "balanced")
		if err != nil {
			return "", err
		}
		if len(generated.Files) != 6 {
			return "", fmt.Errorf("expected 6 starter policy files, got %d", len(generated.Files))
		}
		loaded, err := governance.LoadStaticPolicyFiles(policyDir)
		if err != nil {
			return "", err
		}
		if len(loaded.Warnings) > 0 {
			return "", fmt.Errorf("starter policies produced warnings: %s", strings.Join(loaded.Warnings, "; "))
		}
		if len(loaded.Rules) == 0 {
			return "", fmt.Errorf("starter policies loaded 0 rules")
		}
		return fmt.Sprintf("generated %d starter policies with %d valid rules", len(generated.Files), len(loaded.Rules)), nil
	})

	addCheck("descriptor_lock_baseline", "Descriptor lock baseline has no drift", "boundary lock --config <fixture> && boundary verify-lock --lock <fixture>", func() (string, error) {
		lockPath = filepath.Join(workspace, "descriptor-lock.json")
		_, err := firewall.CreateDescriptorLock(firewall.LockOptions{
			ConfigPath: configPath,
			Client:     firewall.ClientCustom,
			OutPath:    lockPath,
			Now:        started,
		})
		if err != nil {
			return "", err
		}
		verification, err := firewall.VerifyDescriptorLock(firewall.VerifyLockOptions{LockPath: lockPath})
		if err != nil {
			return "", err
		}
		if verification.Status != "ok" || !verification.Allowed || verification.Summary.Unchanged != 1 {
			return "", fmt.Errorf("expected lock status ok/allowed with 1 unchanged server, got status=%s allowed=%t unchanged=%d", verification.Status, verification.Allowed, verification.Summary.Unchanged)
		}
		return "descriptor lock verified ok against baseline fixture", nil
	})

	addCheck("descriptor_lock_detects_drift", "Descriptor lock detects modified drift", "boundary verify-lock --lock <fixture>", func() (string, error) {
		if lockPath == "" {
			return "", fmt.Errorf("descriptor lock baseline check did not produce a lock path")
		}
		if err := os.WriteFile(configPath, []byte(fixtureMCPConfig("merge_pull_request")), 0o600); err != nil {
			return "", err
		}
		verification, err := firewall.VerifyDescriptorLock(firewall.VerifyLockOptions{LockPath: lockPath})
		if err != nil {
			return "", err
		}
		if verification.Status != "drift" || verification.Allowed {
			return "", fmt.Errorf("expected fail-closed drift, got status=%s allowed=%t", verification.Status, verification.Allowed)
		}
		if verification.Summary.Changed == 0 {
			return "", fmt.Errorf("expected changed descriptor count after fixture mutation")
		}
		return "modified descriptor produced drift and default deny behavior", nil
	})

	addCheck("redteam_github_lethal_trifecta", "GitHub lethal-trifecta redteam fixture denies", "boundary redteam --pack github-lethal-trifecta", func() (string, error) {
		var err error
		redteamResult, err = redteam.Run(ctx, redteam.RunOptions{
			PackID: redteam.DefaultPackID,
			Mode:   redteam.ModeFixture,
		})
		if err != nil {
			return "", err
		}
		if !redteamResult.Passed {
			return "", fmt.Errorf("redteam fixture did not pass")
		}
		if redteamResult.MutatesLiveSystems || redteamResult.RealSecretsUsed {
			return "", fmt.Errorf("redteam fixture must not mutate live systems or use real secrets")
		}
		if len(redteamResult.Results) == 0 {
			return "", fmt.Errorf("redteam fixture produced no scenario results")
		}
		redteamScenario = &redteamResult.Results[0]
		if !redteamScenario.Passed || redteamScenario.ExpectedAction != "deny" || redteamScenario.ActualAction != "deny" {
			return "", fmt.Errorf("expected deny/deny redteam result, got expected=%s actual=%s passed=%t", redteamScenario.ExpectedAction, redteamScenario.ActualAction, redteamScenario.Passed)
		}
		return "GitHub write-after-taint fixture denied before upstream", nil
	})

	addCheck("secure_github_live_mode_fails_closed", "Secure GitHub live mode fails closed", "boundary secure github serve --fixture=false --dry-run", func() (string, error) {
		if opts.SecureGitHubLiveModeCheck == nil {
			return "live-mode check is supplied by the CLI; package selftest stayed fixture-only", nil
		}
		if err := opts.SecureGitHubLiveModeCheck(ctx); err != nil {
			return "", err
		}
		return "live GitHub App mode rejected before serving and without credentials", nil
	})

	addCheck("decision_record_emitted", "Decision record is emitted", "boundary redteam --pack github-lethal-trifecta", func() (string, error) {
		if redteamScenario == nil {
			return "", fmt.Errorf("redteam check did not produce a scenario decision record")
		}
		record := redteamScenario.DecisionRecord
		if !strings.HasPrefix(record.RecordID, "rec_") {
			return "", fmt.Errorf("decision record id missing rec_ prefix")
		}
		if !strings.HasPrefix(record.DecisionHash, "sha256:") {
			return "", fmt.Errorf("decision hash missing sha256 prefix")
		}
		if record.Action != "deny" {
			return "", fmt.Errorf("expected deny decision record, got %s", record.Action)
		}
		return fmt.Sprintf("decision record %s emitted with %s", record.RecordID, record.DecisionHash), nil
	})

	addCheck("claims_validation_pointer", "Claims validation pointer is shown", "go test ./claims/... -count=1", func() (string, error) {
		return "run go test ./claims/... -count=1 for claims and language validation", nil
	})

	result.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func fixtureMCPConfig(writeTool string) string {
	return fmt.Sprintf(`{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "env": {"GITHUB_TOKEN": "fixture-redacted"},
      "tools": [
        {"name": "get_issue"},
        {"name": %q}
      ]
    }
  }
}
`, writeTool)
}
