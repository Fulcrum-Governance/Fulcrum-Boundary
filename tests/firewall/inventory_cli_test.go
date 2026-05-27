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

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
