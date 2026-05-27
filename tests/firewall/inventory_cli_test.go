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

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
