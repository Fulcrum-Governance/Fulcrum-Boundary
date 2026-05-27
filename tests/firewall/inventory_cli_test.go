package firewall_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestBoundaryInventoryCLIReportsGitHubRisk(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	configPath := filepath.Join(root, ".mcp.json")
	configBody := `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "env": {"GITHUB_TOKEN": "ghp_secret"}
    }
  }
}`
	writeFile(t, configPath, configBody)

	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "--root", root, "--home", home}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("inventory exit = %d, output=%s", code, stdout.String())
	}
	output := stdout.String()
	for _, want := range []string{`"github_servers": 1`, `"high_risk_servers": 1`, `"name": "merge_pull_request"`, `"class": "W2"`} {
		if !strings.Contains(output, want) {
			t.Fatalf("inventory output missing %q: %s", want, output)
		}
	}
	if strings.Contains(output, "ghp_secret") {
		t.Fatalf("inventory leaked env secret value: %s", output)
	}

	var markdown bytes.Buffer
	code = boundarycli.Run([]string{"inventory", "--root", root, "--home", home, "--format", "markdown"}, &markdown, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("markdown inventory exit = %d", code)
	}
	if !strings.Contains(markdown.String(), "Boundary MCP Inventory") || !strings.Contains(markdown.String(), "merge_pull_request:W2") {
		t.Fatalf("markdown inventory missing expected content: %s", markdown.String())
	}
}

func TestBoundaryInitDoesNotMutateMCPConfig(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	outDir := filepath.Join(root, ".boundary", "firewall")
	configPath := filepath.Join(root, "mcp.json")
	configBody := `{"mcpServers":{"github":{"command":"github-mcp-server"}}}`
	writeFile(t, configPath, configBody)

	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"init", "--root", root, "--home", home, "--out", outDir}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("init exit = %d, output=%s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "mcp config mutation: none") {
		t.Fatalf("init output missing read-only statement: %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(outDir, "boundary-firewall.json")); err != nil {
		t.Fatalf("init did not write Boundary-owned workspace file: %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != configBody {
		t.Fatalf("MCP config was mutated: %s", string(after))
	}
}

func TestBoundaryGraphCLIReportsRiskPaths(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {"command": "github-mcp-server"},
    "slack": {"command": "slack-mcp"}
  }
}`)

	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"graph", "--root", root, "--home", home}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("graph exit = %d, output=%s", code, stdout.String())
	}
	for _, want := range []string{`"schema_version": "boundary.firewall.risk_graph.v1"`, `"category": "repo_write_path"`, `"category": "external_sink"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("graph output missing %q: %s", want, stdout.String())
		}
	}

	stdout.Reset()
	code = boundarycli.Run([]string{"graph", "--root", root, "--home", home, "--format", "mermaid"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("mermaid graph exit = %d", code)
	}
	if !strings.Contains(stdout.String(), "flowchart LR") || !strings.Contains(stdout.String(), "repo_write_path") {
		t.Fatalf("mermaid graph missing expected risk path: %s", stdout.String())
	}
}

func TestBoundaryPolicyGenerateCreatesVerifiablePolicies(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "policies")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"policy", "generate", "--out", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("policy generate exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode: balanced") || !strings.Contains(stdout.String(), "starter policies: 6") {
		t.Fatalf("policy generate output missing count: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"verify", "--policies", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("generated policies did not verify: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "warnings: 0") {
		t.Fatalf("verify output had warnings: %s", stdout.String())
	}
}

func TestBoundaryInstallUninstallAndLockCLI(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	outDir := filepath.Join(root, ".boundary", "firewall")
	configPath := filepath.Join(root, ".mcp.json")
	lockPath := filepath.Join(outDir, "locks", "descriptor-lock.json")
	configBody := `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "get_issue"}, {"name": "create_or_update_file"}]
    }
  }
}`
	writeFile(t, configPath, configBody)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"install", "--root", root, "--home", home, "--client", "repo", "--out", outDir, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("install dry-run exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mcp config mutation: none") {
		t.Fatalf("dry-run output missing no-mutation line: %s", stdout.String())
	}
	assertTestFileEquals(t, configPath, configBody)

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"install", "--root", root, "--home", home, "--client", "repo", "--out", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("install exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "receipt:") || !strings.Contains(stdout.String(), "routed servers: github") {
		t.Fatalf("install output missing receipt/server: %s", stdout.String())
	}
	installed, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(installed), `"command": "boundary"`) || !strings.Contains(string(installed), `"proxy"`) {
		t.Fatalf("config was not routed through Boundary: %s", string(installed))
	}
	receiptPath := firstReceiptPath(t, outDir)

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"uninstall", "--receipt", receiptPath, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("uninstall dry-run exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mcp config mutation: none") {
		t.Fatalf("uninstall dry-run output missing no-mutation line: %s", stdout.String())
	}
	if string(mustReadTestFile(t, configPath)) == configBody {
		t.Fatalf("dry-run uninstall should not restore config before real uninstall")
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"uninstall", "--receipt", receiptPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("uninstall exit = %d, stderr=%s", code, stderr.String())
	}
	assertTestFileEquals(t, configPath, configBody)

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"lock", "--config", configPath, "--out", lockPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("lock exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "servers locked: 1") {
		t.Fatalf("lock output missing count: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"verify-lock", "--lock", lockPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify-lock unchanged exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "lock status: ok") {
		t.Fatalf("verify-lock output missing ok: %s", stdout.String())
	}

	writeFile(t, configPath, `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "get_issue"}, {"name": "merge_pull_request"}]
    }
  }
}`)
	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"verify-lock", "--lock", lockPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("verify-lock default deny should fail on drift, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "lock status: drift") || !strings.Contains(stdout.String(), "github: changed") {
		t.Fatalf("verify-lock drift output missing details: %s", stdout.String())
	}
}

func TestBoundaryDashboardCLIRendersLocalArtifacts(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	configPath := filepath.Join(root, ".mcp.json")
	policyDir := filepath.Join(root, "boundary-firewall-policies")
	lockPath := filepath.Join(root, ".boundary", "firewall", "locks", "descriptor-lock.json")
	receiptsDir := filepath.Join(root, ".boundary", "firewall", "install-receipts")
	recordPath := filepath.Join(root, "decision-records.jsonl")
	writeFile(t, configPath, `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "get_issue"}, {"name": "create_or_update_file"}]
    }
  }
}`)
	writeFile(t, filepath.Join(receiptsDir, "install.json"), `{
  "schema_version": "boundary.firewall.install_receipt.v1",
  "generated_at": "2026-05-27T12:00:00Z",
  "config_path": "`+configPath+`",
  "client": "repo_local",
  "state": "installed",
  "mutated": true,
  "servers": [{"name": "github", "boundary_command": "boundary"}]
}`)
	writeFile(t, recordPath, `{"schema_version":"1","record_id":"rec_dashboard_cli","timestamp":"2026-05-27T12:01:00Z","adapter":"mcp","tool":"github.create_or_update_file","action":"deny","matched_rule":"deny-github-write-after-taint","decision_hash":"sha256:dashboard"}`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"policy", "generate", "--out", policyDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("policy generate exit = %d, stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"lock", "--config", configPath, "--client", "repo", "--out", lockPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("lock exit = %d, stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{
		"dashboard",
		"--root", root,
		"--home", home,
		"--policies", policyDir,
		"--lock", lockPath,
		"--receipts", receiptsDir,
		"--records", recordPath,
		"--format", "text",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dashboard text exit = %d, stderr=%s", code, stderr.String())
	}
	for _, want := range []string{
		"Boundary Firewall Dashboard",
		"local-only: true",
		"policy status: ok",
		"install receipts: 1",
		"lock status: ok",
		"recent decisions: 1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("dashboard text missing %q: %s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{
		"dashboard",
		"--root", root,
		"--home", home,
		"--policies", policyDir,
		"--lock", lockPath,
		"--receipts", receiptsDir,
		"--records", recordPath,
		"--format", "html",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dashboard html exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "<title>Boundary Firewall Dashboard</title>") || !strings.Contains(stdout.String(), "Local only") {
		t.Fatalf("dashboard html missing expected content: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"dashboard", "--serve", "--listen", "0.0.0.0:8942"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("dashboard serve accepted non-loopback listener")
	}
	if !strings.Contains(stderr.String(), "loopback") {
		t.Fatalf("dashboard serve stderr missing loopback guard: %s", stderr.String())
	}
}

func firstReceiptPath(t *testing.T, outDir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(outDir, "install-receipts"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one install receipt, got %d", len(entries))
	}
	return filepath.Join(outDir, "install-receipts", entries[0].Name())
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func assertTestFileEquals(t *testing.T, path, want string) {
	t.Helper()
	body := mustReadTestFile(t, path)
	if string(body) != want {
		t.Fatalf("file %s mismatch:\nwant: %s\n got: %s", path, want, string(body))
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
