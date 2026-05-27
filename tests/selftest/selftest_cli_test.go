package selftest_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestBoundarySelftestTextOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"selftest"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("selftest exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Boundary selftest",
		"status: pass",
		"live mutation: none",
		"credentials: none",
		"network: none",
		"[pass] cli_boots",
		"[pass] inventory_fixture_loads",
		"[pass] risk_graph_fixture_renders",
		"[pass] policy_generator_valid",
		"[pass] descriptor_lock_baseline",
		"[pass] descriptor_lock_detects_drift",
		"[pass] redteam_github_lethal_trifecta",
		"[pass] secure_github_live_mode_fails_closed",
		"[pass] decision_record_emitted",
		"[pass] claims_validation_pointer",
		"next: go test ./claims/... -count=1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("selftest output missing %q: %s", want, output)
		}
	}
}

func TestBoundarySelftestJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"selftest", "--json"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("selftest json exit = %d, output=%s", code, stdout.String())
	}
	var payload struct {
		SchemaVersion       string `json:"schema_version"`
		Status              string `json:"status"`
		Passed              bool   `json:"passed"`
		MutatesLiveSystems  bool   `json:"mutates_live_systems"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresNetwork     bool   `json:"requires_network"`
		Checks              []struct {
			ID      string `json:"id"`
			Status  string `json:"status"`
			Detail  string `json:"detail"`
			Command string `json:"command"`
		} `json:"checks"`
		Next []string `json:"next"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse selftest json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.selftest.v1" || payload.Status != "pass" || !payload.Passed {
		t.Fatalf("unexpected selftest identity: %#v", payload)
	}
	if payload.MutatesLiveSystems || payload.RequiresCredentials || payload.RequiresNetwork {
		t.Fatalf("selftest must not require mutation, credentials, or network: %#v", payload)
	}
	if len(payload.Checks) != 10 {
		t.Fatalf("checks = %d, want 10: %#v", len(payload.Checks), payload.Checks)
	}
	seen := map[string]bool{}
	for _, check := range payload.Checks {
		seen[check.ID] = true
		if check.Status != "pass" {
			t.Fatalf("check did not pass: %#v", check)
		}
		if check.Detail == "" || check.Command == "" {
			t.Fatalf("check missing detail or rerun command: %#v", check)
		}
	}
	for _, id := range []string{
		"cli_boots",
		"inventory_fixture_loads",
		"risk_graph_fixture_renders",
		"policy_generator_valid",
		"descriptor_lock_baseline",
		"descriptor_lock_detects_drift",
		"redteam_github_lethal_trifecta",
		"secure_github_live_mode_fails_closed",
		"decision_record_emitted",
		"claims_validation_pointer",
	} {
		if !seen[id] {
			t.Fatalf("missing check %q: %#v", id, payload.Checks)
		}
	}
	if len(payload.Next) != 1 || payload.Next[0] != "go test ./claims/... -count=1" {
		t.Fatalf("unexpected next commands: %#v", payload.Next)
	}
}

func TestBoundarySelftestNoColorOutputHasNoANSI(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"selftest", "--no-color"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("selftest --no-color exit = %d", code)
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("selftest --no-color emitted ANSI escapes: %q", stdout.String())
	}
}
