package actions_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMCPAuditActionMetadata(t *testing.T) {
	repoRoot := testRepoRoot(t)
	body := readFile(t, filepath.Join(repoRoot, "actions", "mcp-audit", "action.yml"))
	for _, want := range []string{
		"root:",
		"format:",
		"sarif:",
		"fail-on-critical:",
		"include-defaults:",
		"default: \"false\"",
		"critical-count:",
		"high-count:",
		"report-path:",
		"sarif-path:",
		"github/codeql-action/upload-sarif@v3",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("action.yml missing %q:\n%s", want, body)
		}
	}
}

func TestMCPAuditScriptAuditsRepoLocalConfigsOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mcp-audit.sh is a bash action runner")
	}
	repoRoot := testRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "boundary")
	build := exec.Command("go", "build", "-o", bin, "./cmd/boundary")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build boundary: %v\n%s", err, string(output))
	}

	auditRoot := t.TempDir()
	writeFile(t, filepath.Join(auditRoot, ".mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [{"name": "get_issue"}, {"name": "create_or_update_file"}, {"name": "merge_pull_request"}]
    }
  }
}`)
	home := t.TempDir()
	writeFile(t, filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), `{
  "mcpServers": {
    "slack": {"command": "slack-mcp"}
  }
}`)

	outDir := filepath.Join(t.TempDir(), "audit")
	ghOutput := filepath.Join(t.TempDir(), "github-output.txt")
	stepSummary := filepath.Join(t.TempDir(), "step-summary.md")
	script := filepath.Join(repoRoot, "scripts", "actions", "mcp-audit.sh")
	cmd := exec.Command("bash", script,
		"--root", auditRoot,
		"--format", "sarif",
		"--sarif", "true",
		"--fail-on-critical", "false",
		"--include-defaults", "false",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"BOUNDARY_BIN="+bin,
		"BOUNDARY_ACTION_REPO="+repoRoot,
		"BOUNDARY_MCP_AUDIT_OUT="+outDir,
		"GITHUB_OUTPUT="+ghOutput,
		"GITHUB_STEP_SUMMARY="+stepSummary,
		"HOME="+home,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mcp audit script failed: %v\n%s", err, string(output))
	}
	text := string(output)
	for _, want := range []string{"Fulcrum Boundary MCP Audit", "MCP configs found: 1", "Servers found: 1", "Critical paths:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("script output missing %q:\n%s", want, text)
		}
	}

	inventory := readJSONFile(t, filepath.Join(outDir, "inventory.json"))
	summary := inventory["summary"].(map[string]any)
	if got := int(summary["config_files"].(float64)); got != 1 {
		t.Fatalf("config_files = %d, want repo-local only 1; inventory=%v", got, inventory)
	}
	if strings.Contains(readFile(t, filepath.Join(outDir, "inventory.md")), "slack") {
		t.Fatalf("inventory included HOME default config despite include-defaults=false")
	}
	assertFileContains(t, filepath.Join(outDir, "risk-graph.json"), `"schema_version": "boundary.firewall.risk_graph.v1"`)
	assertFileContains(t, filepath.Join(outDir, "inventory.sarif.json"), `"version": "2.1.0"`)
	assertFileContains(t, filepath.Join(outDir, "summary.md"), "Fulcrum Boundary MCP Audit")
	assertFileContains(t, stepSummary, "Fulcrum Boundary MCP Audit")
	assertFileContains(t, ghOutput, "critical-count=")
	assertFileContains(t, ghOutput, "high-count=")
	assertFileContains(t, ghOutput, "report-path="+filepath.Join(outDir, "inventory.sarif.json"))
	assertFileContains(t, ghOutput, "sarif-path="+filepath.Join(outDir, "inventory.sarif.json"))
	if _, err := os.Stat(filepath.Join(outDir, "starter-policies")); err != nil {
		t.Fatalf("starter policies were not generated as action artifacts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(auditRoot, "boundary-firewall-policies")); !os.IsNotExist(err) {
		t.Fatalf("policy generation wrote into audited repo root")
	}
}

func TestMCPAuditScriptCanFailOnCritical(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mcp-audit.sh is a bash action runner")
	}
	repoRoot := testRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "boundary")
	build := exec.Command("go", "build", "-o", bin, "./cmd/boundary")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build boundary: %v\n%s", err, string(output))
	}
	auditRoot := t.TempDir()
	writeFile(t, filepath.Join(auditRoot, ".mcp.json"), `{"mcpServers":{"github":{"command":"github-mcp-server","tools":[{"name":"merge_pull_request"}]}}}`)

	cmd := exec.Command("bash", filepath.Join(repoRoot, "scripts", "actions", "mcp-audit.sh"),
		"--root", auditRoot,
		"--fail-on-critical", "true",
		"--include-defaults", "false",
	)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"BOUNDARY_BIN="+bin,
		"BOUNDARY_ACTION_REPO="+repoRoot,
		"BOUNDARY_MCP_AUDIT_OUT="+filepath.Join(t.TempDir(), "audit"),
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected fail-on-critical to fail, output=%s", string(output))
	}
	if !strings.Contains(string(output), "critical MCP risk paths found") {
		t.Fatalf("missing fail-on-critical message:\n%s", string(output))
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()
	body := readFile(t, path)
	if !strings.Contains(body, want) {
		t.Fatalf("%s missing %q:\n%s", path, want, body)
	}
}
