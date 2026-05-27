package selftest

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunSelftestPassesFixtureChecks(t *testing.T) {
	result, err := Run(context.Background(), Options{
		SecureGitHubLiveModeCheck: func(context.Context) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Status != StatusPass {
		t.Fatalf("selftest did not pass: %#v", result)
	}
	if result.MutatesLiveSystems || result.RequiresCredentials || result.RequiresNetwork {
		t.Fatalf("selftest must not require mutation, credentials, or network: %#v", result)
	}
	if len(result.Checks) != 10 {
		t.Fatalf("checks = %d, want 10: %#v", len(result.Checks), result.Checks)
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
		check, ok := checkByID(result, id)
		if !ok {
			t.Fatalf("missing check %q: %#v", id, result.Checks)
		}
		if check.Status != StatusPass {
			t.Fatalf("check %q did not pass: %#v", id, check)
		}
	}
}

func TestRunSelftestReportsLiveModeCheckFailure(t *testing.T) {
	result, err := Run(context.Background(), Options{
		SecureGitHubLiveModeCheck: func(context.Context) error {
			return errors.New("live mode unexpectedly allowed")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || result.Status != StatusFail {
		t.Fatalf("selftest passed despite live-mode failure: %#v", result)
	}
	check, ok := checkByID(result, "secure_github_live_mode_fails_closed")
	if !ok {
		t.Fatalf("missing live-mode check: %#v", result.Checks)
	}
	if check.Status != StatusFail || !strings.Contains(check.Detail, "live mode unexpectedly allowed") {
		t.Fatalf("unexpected live-mode failure detail: %#v", check)
	}
	var text bytes.Buffer
	if err := WriteText(&text, result, RenderOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "rerun: boundary secure github serve --fixture=false --dry-run") {
		t.Fatalf("failure output missing rerun command: %s", text.String())
	}
}

func checkByID(result *Result, id string) (CheckResult, bool) {
	for _, check := range result.Checks {
		if check.ID == id {
			return check, true
		}
	}
	return CheckResult{}, false
}
