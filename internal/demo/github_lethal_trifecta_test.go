package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunGitHubLethalTrifectaFixturePasses(t *testing.T) {
	result, err := RunGitHubLethalTrifecta(context.Background(), GitHubLethalTrifectaOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Status != "pass" {
		t.Fatalf("demo did not pass: %#v", result)
	}
	if !result.FixtureOnly || result.RequiresCredentials || result.RequiresNetwork || result.MutatesLiveSystems {
		t.Fatalf("demo must be fixture-only and local: %#v", result)
	}
	if result.WorkspaceRetained {
		t.Fatalf("workspace should not be retained without --out or --dashboard: %#v", result)
	}
	if result.Scenario.ExpectedAction != "DENY" || result.Scenario.ActualAction != "DENY" {
		t.Fatalf("unexpected action pair: %#v", result.Scenario)
	}
	if result.Scenario.Reason != GitHubLethalTrifectaReason {
		t.Fatalf("reason = %q, want %q", result.Scenario.Reason, GitHubLethalTrifectaReason)
	}
	if result.Scenario.UpstreamCalled {
		t.Fatalf("denied write reached upstream: %#v", result.Scenario)
	}
	if !result.Scenario.ReadUpstreamCalled {
		t.Fatalf("read step should reach fixture upstream to establish taint: %#v", result.Scenario)
	}
	if result.Scenario.MatchedRule == "" || result.Scenario.DecisionRecordID == "" || result.Scenario.DecisionHash == "" {
		t.Fatalf("missing governance evidence: %#v", result.Scenario)
	}
	if result.InventorySummary.GitHubServers != 1 || result.RiskSummary.RepoWritePaths == 0 {
		t.Fatalf("inventory/risk evidence missing: inventory=%#v risk=%#v", result.InventorySummary, result.RiskSummary)
	}
	if result.PolicyFiles == 0 || result.PolicyRules == 0 {
		t.Fatalf("policy generation evidence missing: files=%d rules=%d", result.PolicyFiles, result.PolicyRules)
	}
}

func TestGitHubLethalTrifectaDashboardAndMarkdownReport(t *testing.T) {
	out := filepath.Join(t.TempDir(), "demo-report.md")
	result, err := RunGitHubLethalTrifecta(context.Background(), GitHubLethalTrifectaOptions{
		OutPath:   out,
		Dashboard: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.WorkspaceRetained {
		t.Fatalf("workspace should be retained with explicit output path: %#v", result)
	}
	if result.ReportPath != out {
		t.Fatalf("report path = %q, want %q", result.ReportPath, out)
	}
	if result.DashboardPath == "" {
		t.Fatalf("dashboard path not set: %#v", result)
	}
	if _, err := os.Stat(result.DashboardPath); err != nil {
		t.Fatalf("dashboard artifact missing: %v", err)
	}
	var markdown bytes.Buffer
	if err := WriteGitHubLethalTrifectaMarkdown(&markdown, result); err != nil {
		t.Fatal(err)
	}
	text := markdown.String()
	for _, want := range []string{
		"# GitHub Lethal-Trifecta Demo",
		"Reason: `lethal_trifecta_detected`",
		"Upstream called: `false`",
		"## What This Proves",
		"## What This Does Not Prove",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown report missing %q:\n%s", want, text)
		}
	}
}

func TestGitHubLethalTrifectaJSONReport(t *testing.T) {
	result, err := RunGitHubLethalTrifecta(context.Background(), GitHubLethalTrifectaOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteGitHubLethalTrifectaJSON(&out, result); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Passed        bool   `json:"passed"`
		Scenario      struct {
			ExpectedAction string `json:"expected_action"`
			ActualAction   string `json:"actual_action"`
			Reason         string `json:"reason"`
			UpstreamCalled bool   `json:"upstream_called"`
		} `json:"scenario"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("parse json report: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != GitHubLethalTrifectaSchemaVersion || !payload.Passed {
		t.Fatalf("unexpected json identity: %#v", payload)
	}
	if payload.Scenario.ExpectedAction != "DENY" ||
		payload.Scenario.ActualAction != "DENY" ||
		payload.Scenario.Reason != GitHubLethalTrifectaReason ||
		payload.Scenario.UpstreamCalled {
		t.Fatalf("unexpected scenario fields: %#v", payload.Scenario)
	}
}
