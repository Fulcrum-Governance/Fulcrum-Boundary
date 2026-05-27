package firewall_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

func TestBoundaryInventoryIngestRoundTripsBoundaryNDJSON(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "env": {"GITHUB_TOKEN": "ghp_secret"}
    }
  }
}`)

	var ndjson bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "--root", root, "--home", home, "--format", "ndjson"}, &ndjson, &stderr)
	if code != 0 {
		t.Fatalf("inventory ndjson exit = %d, stderr=%s", code, stderr.String())
	}
	inventoryPath := filepath.Join(t.TempDir(), "boundary-inventory.ndjson")
	if err := os.WriteFile(inventoryPath, ndjson.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	result, output := runInventoryIngest(t, inventoryPath, "boundary")
	if !result.Complete || result.SnapshotStatus != "complete" {
		t.Fatalf("boundary ingest status = complete:%t status:%s warnings:%v", result.Complete, result.SnapshotStatus, result.Warnings)
	}
	if !result.InstallRecommendationsEnabled {
		t.Fatalf("boundary ingest should enable install recommendations for complete snapshots")
	}
	if result.Summary.MCPServers != 1 || result.Inventory.Summary.GitHubServers != 1 {
		t.Fatalf("boundary ingest server summary mismatch: %+v inventory=%+v", result.Summary, result.Inventory.Summary)
	}
	if len(result.Records) == 0 || result.Records[len(result.Records)-1].RecordType != firewall.InventoryRecordScanSummary {
		t.Fatalf("boundary ingest did not emit scan_summary as final record")
	}
	if strings.Contains(output, "ghp_secret") {
		t.Fatalf("boundary ingest leaked secret-bearing fixture value: %s", output)
	}
}

func TestExternalInventoryIngestMapsGenericMCPFixture(t *testing.T) {
	result, output := runInventoryIngest(t, externalInventoryFixture("generic-mcp.ndjson"), "generic")
	if !result.Complete || result.Source != "generic" {
		t.Fatalf("generic ingest status/source mismatch: complete=%t source=%s warnings=%v", result.Complete, result.Source, result.Warnings)
	}
	if result.Summary.MCPConfigs != 2 || result.Summary.MCPServers != 2 {
		t.Fatalf("generic ingest summary mismatch: %+v", result.Summary)
	}
	github := findIngestedServer(t, result, "github")
	if github.Client != firewall.ClientClaudeDesktop {
		t.Fatalf("github client = %s, want claude_desktop", github.Client)
	}
	if github.HighestRisk == "" || github.HighestRisk == "unknown" {
		t.Fatalf("github highest risk = %s, want governed W-class risk", github.HighestRisk)
	}
	filesystem := findIngestedServer(t, result, "filesystem")
	if filesystem.Client != firewall.ClientRepoLocal {
		t.Fatalf("filesystem client = %s, want repo_local", filesystem.Client)
	}
	if !containsString(filesystem.DescriptorTools, "write_file") {
		t.Fatalf("filesystem descriptor tools missing write_file: %+v", filesystem.DescriptorTools)
	}
	if strings.Contains(output, "ghp_secret") {
		t.Fatalf("generic ingest leaked fixture token: %s", output)
	}
}

func TestExternalInventoryIngestMarksMixedEndpointSnapshotPartial(t *testing.T) {
	fixture := externalInventoryFixture("mixed-endpoint.ndjson")
	result, _ := runInventoryIngest(t, fixture, "generic")
	if result.Complete || result.SnapshotStatus != "partial" {
		t.Fatalf("mixed endpoint status = complete:%t status:%s", result.Complete, result.SnapshotStatus)
	}
	if result.InstallRecommendationsEnabled {
		t.Fatalf("partial ingest should disable install recommendations without --allow-partial")
	}
	if result.Summary.ExternalInventoryComponents != 1 || result.Summary.ExternalExposureFindings != 1 {
		t.Fatalf("mixed endpoint report-only counts mismatch: %+v", result.Summary)
	}
	if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, "\n"), "no complete scan_summary") {
		t.Fatalf("mixed endpoint missing partial snapshot warning: %+v", result.Warnings)
	}
	for _, record := range result.Records {
		if record.RecordType == firewall.InventoryRecordPolicyRecommendation {
			t.Fatalf("partial ingest emitted policy recommendation without --allow-partial: %+v", record)
		}
		if record.InstallStatus != nil && !strings.Contains(record.InstallStatus.Recommendation, "disabled for partial") {
			t.Fatalf("partial ingest install recommendation not disabled: %+v", record.InstallStatus)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "ingest", "--file", fixture, "--source", "generic", "--allow-partial", "--summary"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("allow-partial summary exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "snapshot status: partial") || !strings.Contains(stdout.String(), "install recommendations enabled: true") {
		t.Fatalf("allow-partial summary missing expected state: %s", stdout.String())
	}
}

func TestExternalInventoryIngestMapsBumblebeeStyleFixture(t *testing.T) {
	result, _ := runInventoryIngest(t, externalInventoryFixture("bumblebee-style-mcp.ndjson"), "bumblebee")
	if !result.Complete || result.Source != "bumblebee" {
		t.Fatalf("bumblebee-style ingest mismatch: complete=%t source=%s warnings=%v", result.Complete, result.Source, result.Warnings)
	}
	github := findIngestedServer(t, result, "github")
	if github.Command != "npx" {
		t.Fatalf("bumblebee-style github command = %q, want npx", github.Command)
	}
	if !containsString(github.DescriptorTools, "merge_pull_request") {
		t.Fatalf("bumblebee-style tools missing merge_pull_request: %+v", github.DescriptorTools)
	}
	if result.Summary.ExternalInventoryComponents != 1 {
		t.Fatalf("bumblebee-style component count = %d, want 1", result.Summary.ExternalInventoryComponents)
	}
	if len(result.ExternalInventoryComponents) == 0 || result.ExternalInventoryComponents[0].Kind != "external_inventory_component" {
		t.Fatalf("bumblebee-style component not report-only external component: %+v", result.ExternalInventoryComponents)
	}
}

func runInventoryIngest(t *testing.T, file, source string) (firewall.ExternalInventoryIngestResult, string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"inventory", "ingest", "--file", file, "--source", source, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("inventory ingest exit = %d, stderr=%s", code, stderr.String())
	}
	var result firewall.ExternalInventoryIngestResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode ingest result: %v\n%s", err, stdout.String())
	}
	return result, stdout.String()
}

func externalInventoryFixture(name string) string {
	return filepath.Join("..", "..", "fixtures", "external-inventory", name)
}

func findIngestedServer(t *testing.T, result firewall.ExternalInventoryIngestResult, name string) firewall.Server {
	t.Helper()
	for _, server := range result.Inventory.Servers {
		if server.Name == name {
			return server
		}
	}
	t.Fatalf("server %q not found in %+v", name, result.Inventory.Servers)
	return firewall.Server{}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
