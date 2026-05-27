package firewall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestBuildDashboardSummarizesLocalFirewallArtifacts(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	configPath := filepath.Join(root, ".mcp.json")
	writeMCPConfig(t, configPath, `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "get_issue"}, {"name": "create_or_update_file"}]
    }
  }
}`)

	policyDir := filepath.Join(root, "boundary-firewall-policies")
	if _, err := GenerateStarterPolicies(policyDir, false, "balanced"); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, ".boundary", "firewall", "locks", "descriptor-lock.json")
	if _, err := CreateDescriptorLock(LockOptions{ConfigPath: configPath, Client: ClientRepoLocal, OutPath: lockPath}); err != nil {
		t.Fatal(err)
	}
	receiptsDir := filepath.Join(root, ".boundary", "firewall", "install-receipts")
	writeInstallReceipt(t, filepath.Join(receiptsDir, "install.json"), InstallResult{
		SchemaVersion: installReceiptSchema,
		GeneratedAt:   "2026-05-27T12:00:00Z",
		ConfigPath:    configPath,
		Client:        ClientRepoLocal,
		State:         "installed",
		Mutated:       true,
		Servers: []InstalledServer{{
			Name:            "github",
			BoundaryCommand: "boundary",
		}},
	})
	recordPath := filepath.Join(root, "decision-records.jsonl")
	writeDecisionRecord(t, recordPath, governance.DecisionRecordV1{
		SchemaVersion: governance.DecisionRecordSchemaVersion,
		RecordID:      "rec_dashboard",
		Timestamp:     time.Date(2026, 5, 27, 12, 1, 0, 0, time.UTC),
		Adapter:       governance.TransportMCP,
		Tool:          "github.create_or_update_file",
		Action:        "deny",
		MatchedRule:   "deny-github-write-after-taint",
		DecisionHash:  "sha256:dashboard",
	})

	dashboard, err := BuildDashboard(DashboardOptions{
		Root:                root,
		Home:                home,
		IncludeDefaults:     true,
		PolicyDir:           policyDir,
		LockPath:            lockPath,
		ReceiptsDir:         receiptsDir,
		DecisionRecordPaths: []string{recordPath},
		Now:                 time.Date(2026, 5, 27, 12, 2, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.SchemaVersion != dashboardSchema || !dashboard.LocalOnly {
		t.Fatalf("dashboard identity mismatch: %+v", dashboard)
	}
	if dashboard.Inventory.Summary.GitHubServers != 1 || dashboard.RiskGraph.Summary.Paths == 0 {
		t.Fatalf("dashboard missing inventory or risk paths: %+v", dashboard)
	}
	if dashboard.Policies.Status != "ok" || dashboard.Policies.Rules == 0 {
		t.Fatalf("policy status = %+v, want ok with rules", dashboard.Policies)
	}
	if dashboard.Install.Receipts != 1 || dashboard.Install.Installed != 1 {
		t.Fatalf("install status = %+v, want one installed receipt", dashboard.Install)
	}
	if dashboard.Lock.Status != "ok" || !dashboard.Lock.Allowed {
		t.Fatalf("lock status = %+v, want ok allowed", dashboard.Lock)
	}
	if dashboard.Decisions.Count != 1 || dashboard.Decisions.Recent[0].Action != "deny" {
		t.Fatalf("decisions = %+v, want one deny", dashboard.Decisions)
	}

	for _, format := range []string{"text", "json", "html"} {
		body, err := RenderDashboard(dashboard, format)
		if err != nil {
			t.Fatalf("render %s: %v", format, err)
		}
		text := string(body)
		want := "Boundary Firewall Dashboard"
		if format == "json" {
			want = `"schema_version": "boundary.firewall.dashboard.v1"`
		}
		if !strings.Contains(text, want) {
			t.Fatalf("%s render missing %q: %s", format, want, text)
		}
	}
	html, err := RenderDashboard(dashboard, "html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(html), "https://") || strings.Contains(string(html), "http://") {
		t.Fatalf("dashboard HTML should not reference remote assets: %s", string(html))
	}
}

func writeInstallReceipt(t *testing.T, path string, receipt InstallResult) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeDecisionRecord(t *testing.T, path string, record governance.DecisionRecordV1) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
