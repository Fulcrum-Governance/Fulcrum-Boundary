package redteam

import (
	"context"
	"testing"
)

// TestEditGovernanceControlSurfacePackDeniesWithoutApplying is the BOU-5b
// posture in one assertion: every control-surface fixture must deny, and none of
// them may be applied to disk.
func TestEditGovernanceControlSurfacePackDeniesWithoutApplying(t *testing.T) {
	wantScenarios := []string{
		"edit-claude-settings",
		"edit-claude-settings-local",
		"edit-claude-hook-script",
		"edit-boundary-hook-script",
		"edit-boundary-decision-records",
	}

	result, err := Run(context.Background(), RunOptions{PackID: "edit-governance-control-surface"})
	if err != nil {
		t.Fatalf("run control surface pack: %v", err)
	}
	if !result.Passed || result.Status != ResultPassed {
		t.Fatalf("unexpected pack result: %#v", result)
	}
	if result.MutatesLiveSystems || result.RealSecretsUsed {
		t.Fatalf("fixture pack must not mutate live systems or use real secrets: %#v", result)
	}
	if len(result.Results) != len(wantScenarios) {
		t.Fatalf("scenario count = %d, want %d", len(result.Results), len(wantScenarios))
	}

	seen := make(map[string]bool, len(result.Results))
	for _, scenario := range result.Results {
		seen[scenario.ScenarioID] = true
		if scenario.ExpectedAction != "deny" || scenario.ActualAction != "deny" {
			t.Fatalf("%s action = expected %q actual %q, want deny",
				scenario.ScenarioID, scenario.ExpectedAction, scenario.ActualAction)
		}
		if scenario.Executed || scenario.Applied {
			t.Fatalf("%s executed or applied a fixture patch: %#v", scenario.ScenarioID, scenario)
		}
		if !scenario.FixtureOnly || !scenario.NoLiveMutation {
			t.Fatalf("%s is not marked fixture-only: %#v", scenario.ScenarioID, scenario)
		}
		if scenario.EditClass != "E7" {
			t.Fatalf("%s edit class = %q, want the control-path deny class E7",
				scenario.ScenarioID, scenario.EditClass)
		}
		if scenario.DecisionRecord.RecordID == "" || scenario.DecisionRecord.DecisionHash == "" {
			t.Fatalf("%s missing decision record: %#v", scenario.ScenarioID, scenario.DecisionRecord)
		}
		if scenario.DecisionRecord.Action != "deny" {
			t.Fatalf("%s decision record action = %q, want deny",
				scenario.ScenarioID, scenario.DecisionRecord.Action)
		}
	}
	for _, id := range wantScenarios {
		if !seen[id] {
			t.Fatalf("scenario %q is missing from the pack", id)
		}
	}
}

func TestEditGovernanceControlSurfacePackIsListed(t *testing.T) {
	for _, summary := range AvailablePacks() {
		if summary.ID != "edit-governance-control-surface" {
			continue
		}
		if summary.Status != PackStatusImplemented {
			t.Fatalf("pack status = %q, want %q", summary.Status, PackStatusImplemented)
		}
		return
	}
	t.Fatal("edit-governance-control-surface pack is not listed")
}
