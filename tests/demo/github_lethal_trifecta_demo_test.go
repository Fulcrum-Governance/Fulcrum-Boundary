package demo_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestGitHubLethalTrifectaDemoTextOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "github-lethal-trifecta"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"GitHub lethal-trifecta demo",
		"status: pass",
		"fixture-only: true",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"expected action: DENY",
		"actual action: DENY",
		"reason: lethal_trifecta_detected",
		"upstream_called=false",
		"read_upstream_called=true",
		"decision record id: rec_",
		"decision hash: sha256:",
		"What this proves:",
		"What this does not prove:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("demo output missing %q:\n%s", want, output)
		}
	}
}

func TestGitHubLethalTrifectaDemoJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "github-lethal-trifecta", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo json exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion       string `json:"schema_version"`
		Passed              bool   `json:"passed"`
		FixtureOnly         bool   `json:"fixture_only"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresNetwork     bool   `json:"requires_network"`
		MutatesLiveSystems  bool   `json:"mutates_live_systems"`
		Scenario            struct {
			ExpectedAction string `json:"expected_action"`
			ActualAction   string `json:"actual_action"`
			Reason         string `json:"reason"`
			UpstreamCalled bool   `json:"upstream_called"`
		} `json:"scenario"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse demo json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.demo.github_lethal_trifecta.v1" || !payload.Passed || !payload.FixtureOnly {
		t.Fatalf("unexpected demo json identity: %#v", payload)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("demo must not need credentials, network, or live mutation: %#v", payload)
	}
	if payload.Scenario.ExpectedAction != "DENY" ||
		payload.Scenario.ActualAction != "DENY" ||
		payload.Scenario.Reason != "lethal_trifecta_detected" ||
		payload.Scenario.UpstreamCalled {
		t.Fatalf("unexpected demo scenario: %#v", payload.Scenario)
	}
}

func TestGitHubLethalTrifectaDemoMarkdownOutAndDashboard(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "demo-report.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{
		"demo",
		"github-lethal-trifecta",
		"--markdown",
		"--out",
		report,
		"--dashboard",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo report exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "demo report: "+report) {
		t.Fatalf("stdout missing report path: %s", stdout.String())
	}
	dashboard := filepath.Join(dir, "github-lethal-trifecta-dashboard.html")
	if !strings.Contains(stdout.String(), "dashboard: "+dashboard) {
		t.Fatalf("stdout missing dashboard path: %s", stdout.String())
	}
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"# GitHub Lethal-Trifecta Demo",
		"Reason: `lethal_trifecta_detected`",
		"Upstream called: `false`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(dashboard); err != nil {
		t.Fatalf("dashboard artifact missing: %v", err)
	}
}
