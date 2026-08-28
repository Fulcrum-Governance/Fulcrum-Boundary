package redteam

import (
	"context"
	"testing"
)

func TestCommandCompoundSmugglingPackDecomposesWithoutExecuting(t *testing.T) {
	wantActions := map[string]string{
		"command-line-and-chain-tail":         "deny",
		"command-line-semicolon-chain-tail":   "deny",
		"command-line-newline-chain-tail":     "deny",
		"command-line-substitution-tail":      "deny",
		"command-line-backtick-substitution":  "deny",
		"command-line-shell-c-payload":        "deny",
		"command-line-env-prefix-disguise":    "deny",
		"command-line-pipe-to-shell":          "require_approval",
		"command-line-heredoc-undecomposable": "require_approval",
		// Smuggling that does not use a shell operator at all: a command in
		// argument position, and a write the shell itself performs.
		"command-line-find-exec-payload":            "deny",
		"command-line-xargs-payload":                "deny",
		"command-line-redirect-write-behind-a-read": "warn",
		"command-line-redirect-to-secret-path":      "deny",
	}

	result, err := Run(context.Background(), RunOptions{PackID: "command-compound-smuggling"})
	if err != nil {
		t.Fatalf("run compound smuggling pack: %v", err)
	}
	if !result.Passed || result.Status != ResultPassed {
		t.Fatalf("unexpected pack result: %#v", result)
	}
	if result.MutatesLiveSystems || result.RealSecretsUsed {
		t.Fatalf("fixture pack must not mutate live systems or use real secrets: %#v", result)
	}
	if len(result.Results) != len(wantActions) {
		t.Fatalf("scenario count = %d, want %d", len(result.Results), len(wantActions))
	}

	for _, scenario := range result.Results {
		wantAction, ok := wantActions[scenario.ScenarioID]
		if !ok {
			t.Fatalf("unexpected scenario %q", scenario.ScenarioID)
		}
		if scenario.ExpectedAction != wantAction || scenario.ActualAction != wantAction {
			t.Fatalf("%s action = expected %q actual %q, want %q", scenario.ScenarioID, scenario.ExpectedAction, scenario.ActualAction, wantAction)
		}
		if scenario.Executed {
			t.Fatalf("%s executed a command in fixture mode", scenario.ScenarioID)
		}
		if !scenario.FixtureOnly || !scenario.NoLiveMutation {
			t.Fatalf("%s is not marked fixture-only: %#v", scenario.ScenarioID, scenario)
		}
		if scenario.Command == "" || scenario.CommandClass == "" || scenario.CommandRisk == "" {
			t.Fatalf("%s missing command metadata: %#v", scenario.ScenarioID, scenario)
		}
		if scenario.DecisionRecord.RecordID == "" || scenario.DecisionRecord.DecisionHash == "" {
			t.Fatalf("%s missing decision record: %#v", scenario.ScenarioID, scenario.DecisionRecord)
		}
		if scenario.DecisionRecord.Action != wantAction {
			t.Fatalf("%s decision record action = %q, want %q", scenario.ScenarioID, scenario.DecisionRecord.Action, wantAction)
		}
	}
}

func TestCommandCompoundSmugglingPackIsListed(t *testing.T) {
	for _, summary := range AvailablePacks() {
		if summary.ID != "command-compound-smuggling" {
			continue
		}
		if summary.Status != PackStatusImplemented {
			t.Fatalf("pack status = %q, want %q", summary.Status, PackStatusImplemented)
		}
		return
	}
	t.Fatal("command-compound-smuggling pack is not listed")
}
