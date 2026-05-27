package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestBuildRiskGraphDetectsExpectedRiskPaths(t *testing.T) {
	root := t.TempDir()
	writeMCPConfig(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {"command": "github-mcp-server"},
    "filesystem": {"command": "mcp-filesystem"},
    "postgres": {"command": "postgres-mcp"},
    "slack": {"command": "slack-mcp"},
    "shell": {"command": "shell-mcp"}
  }
}`)

	inventory, err := BuildInventory(DiscoverOptions{Root: root, IncludeDefaults: true})
	if err != nil {
		t.Fatal(err)
	}
	graph := BuildRiskGraph(inventory)
	for _, category := range []string{
		"untrusted_input_to_private_data",
		"untrusted_input_to_private_repo_mutation",
		"external_sink",
		"privileged_mutation",
		"descriptor_change",
		"destructive_db_action",
		"filesystem_exfil",
		"repo_write_path",
	} {
		if !hasRiskCategory(graph, category) {
			t.Fatalf("graph missing risk category %q: %+v", category, graph.Paths)
		}
	}
	if graph.Summary.RepoWritePaths == 0 {
		t.Fatalf("repo write paths = 0, want at least one")
	}
	if graph.Summary.ExternalSinkPaths == 0 {
		t.Fatalf("external sink paths = 0, want at least one")
	}

	body, err := RenderRiskGraph(graph, "mermaid")
	if err != nil {
		t.Fatal(err)
	}
	if text := string(body); !strings.Contains(text, "flowchart LR") || !strings.Contains(text, "repo_write_path") {
		t.Fatalf("mermaid graph missing expected content: %s", text)
	}
}

func TestGenerateStarterPoliciesPassBoundaryPolicyLoader(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "policies")
	result, err := GenerateStarterPolicies(outDir, false, "balanced")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 6 {
		t.Fatalf("generated files = %d, want 6: %+v", len(result.Files), result.Files)
	}
	loaded, err := governance.LoadStaticPolicyFiles(outDir)
	if err != nil {
		t.Fatalf("generated policies did not load: %v", err)
	}
	if len(loaded.Warnings) != 0 {
		t.Fatalf("generated policies produced warnings: %+v", loaded.Warnings)
	}
	if len(loaded.Rules) == 0 {
		t.Fatal("generated policies produced no rules")
	}
	if result.Mode != "balanced" {
		t.Fatalf("mode = %q, want balanced", result.Mode)
	}
	if _, err := GenerateStarterPolicies(outDir, false, "balanced"); err == nil {
		t.Fatal("second generation without force succeeded; want overwrite protection")
	}
	if _, err := GenerateStarterPolicies(filepath.Join(t.TempDir(), "policies"), false, "loose"); err == nil {
		t.Fatal("unsupported generation mode succeeded")
	}
}

func hasRiskCategory(graph RiskGraph, category string) bool {
	for _, path := range graph.Paths {
		if path.Category == category {
			return true
		}
	}
	return false
}

func TestGenerateStarterPoliciesCanOverwriteWithForce(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "policies")
	if _, err := GenerateStarterPolicies(outDir, false, "balanced"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(outDir, "github.yaml")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := GenerateStarterPolicies(outDir, true, "balanced"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "stale") {
		t.Fatalf("force generation did not overwrite stale policy: %s", string(body))
	}
}
