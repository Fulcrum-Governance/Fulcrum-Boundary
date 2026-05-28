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

func TestActionBoundaryDemoTextOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "action-boundary"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Action Boundary demo",
		"status: pass",
		"fixture-only: true",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"Surface: MCP / Secure GitHub",
		"actual action: DENY",
		"reason: lethal_trifecta_detected",
		"upstream_called=false",
		"Surface: Command Boundary",
		"command: git push origin main",
		"class: C3",
		"risk: HIGH",
		"recommended action: require_approval",
		"executed=false",
		"Surface: Edit Boundary",
		"class: E4",
		"risk: CRITICAL",
		"applied=false",
		"What this proves:",
		"What this does not prove:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("demo output missing %q:\n%s", want, output)
		}
	}
}

func TestActionBoundaryDemoJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "action-boundary", "--json"}, &stdout, &stderr)
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
		Surfaces            []struct {
			Surface           string `json:"surface"`
			ActualAction      string `json:"actual_action"`
			Reason            string `json:"reason"`
			Command           string `json:"command"`
			Class             string `json:"class"`
			Risk              string `json:"risk"`
			RecommendedAction string `json:"recommended_action"`
			UpstreamCalled    bool   `json:"upstream_called"`
			Executed          bool   `json:"executed"`
			Applied           bool   `json:"applied"`
			Passed            bool   `json:"passed"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse demo json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.demo.action_boundary.v1" || !payload.Passed || !payload.FixtureOnly {
		t.Fatalf("unexpected demo json identity: %#v", payload)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("demo must not need credentials, network, or live mutation: %#v", payload)
	}
	if len(payload.Surfaces) != 3 {
		t.Fatalf("expected 3 surfaces, got %d: %#v", len(payload.Surfaces), payload.Surfaces)
	}
	surfaces := map[string]struct {
		ActualAction      string
		Reason            string
		Command           string
		Class             string
		Risk              string
		RecommendedAction string
		UpstreamCalled    bool
		Executed          bool
		Applied           bool
		Passed            bool
	}{}
	for _, surface := range payload.Surfaces {
		surfaces[surface.Surface] = struct {
			ActualAction      string
			Reason            string
			Command           string
			Class             string
			Risk              string
			RecommendedAction string
			UpstreamCalled    bool
			Executed          bool
			Applied           bool
			Passed            bool
		}{
			ActualAction:      surface.ActualAction,
			Reason:            surface.Reason,
			Command:           surface.Command,
			Class:             surface.Class,
			Risk:              surface.Risk,
			RecommendedAction: surface.RecommendedAction,
			UpstreamCalled:    surface.UpstreamCalled,
			Executed:          surface.Executed,
			Applied:           surface.Applied,
			Passed:            surface.Passed,
		}
	}
	if got := surfaces["mcp_secure_github"]; got.ActualAction != "DENY" || got.Reason != "lethal_trifecta_detected" || got.UpstreamCalled || !got.Passed {
		t.Fatalf("unexpected MCP surface: %#v", got)
	}
	if got := surfaces["command_boundary"]; got.Command != "git push origin main" || got.Class != "C3" || got.Risk != "HIGH" || got.RecommendedAction != "require_approval" || got.Executed || !got.Passed {
		t.Fatalf("unexpected command surface: %#v", got)
	}
	if got := surfaces["edit_boundary"]; got.Class != "E4" || got.Risk != "CRITICAL" || got.ActualAction != "deny" || got.Applied || !got.Passed {
		t.Fatalf("unexpected edit surface: %#v", got)
	}
}

func TestActionBoundaryDemoMarkdownOut(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "demo.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{
		"demo",
		"action-boundary",
		"--markdown",
		"--out",
		report,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo report exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "demo report: "+report) {
		t.Fatalf("stdout missing report path: %s", stdout.String())
	}
	body, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"# Action Boundary Demo",
		"## Surfaces",
		"### Command Boundary",
		"Command: `git push origin main`",
		"### Edit Boundary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestActionBoundaryDemoDashboardOut(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "action-boundary-demo")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{
		"demo",
		"action-boundary",
		"--dashboard",
		"--out",
		dir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo dashboard exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	htmlPath := filepath.Join(dir, "action-boundary-dashboard.html")
	jsonPath := filepath.Join(dir, "action-boundary-demo.json")
	if !strings.Contains(stdout.String(), "dashboard: "+htmlPath) {
		t.Fatalf("stdout missing dashboard path: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "demo json: "+jsonPath) {
		t.Fatalf("stdout missing demo json path: %s", stdout.String())
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("dashboard artifact missing: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("json artifact missing: %v", err)
	}
}

func TestActionBoundaryDemoHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "action-boundary", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String() + stderr.String()
	for _, want := range []string{
		"fixture-only demo across MCP/Secure GitHub, Command Boundary, and Edit Boundary",
		"no credentials, no network, and no live mutation",
		"not global shell control",
		"boundary demo action-boundary --dashboard --out .boundary/action-boundary-demo",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}
