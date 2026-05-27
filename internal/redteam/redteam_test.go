package redteam

import (
	"context"
	"testing"
)

func TestRunDefaultGitHubLethalTrifectaFixture(t *testing.T) {
	result, err := Run(context.Background(), RunOptions{})
	if err != nil {
		t.Fatalf("run default redteam fixture: %v", err)
	}
	if !result.Passed || result.Status != ResultPassed {
		t.Fatalf("expected pass, got %#v", result)
	}
	if result.Mode != ModeFixture || result.PackID != DefaultPackID {
		t.Fatalf("unexpected run identity: %#v", result)
	}
	if result.MutatesLiveSystems || result.RealSecretsUsed {
		t.Fatalf("fixture run must not mutate live systems or use real secrets: %#v", result)
	}
	if len(result.Results) != 1 {
		t.Fatalf("expected one scenario, got %d", len(result.Results))
	}
	scenario := result.Results[0]
	if scenario.ExpectedAction != "deny" || scenario.ActualAction != "deny" || !scenario.Passed {
		t.Fatalf("expected deny/deny pass, got %#v", scenario)
	}
	if scenario.MatchedRule != "deny-github-write-after-taint-fixture" {
		t.Fatalf("unexpected matched rule %q", scenario.MatchedRule)
	}
	if scenario.DecisionRecord.RecordID == "" || scenario.DecisionRecord.DecisionHash == "" {
		t.Fatalf("decision record missing receipt fields: %#v", scenario.DecisionRecord)
	}
	if scenario.DecisionRecord.Action != "deny" {
		t.Fatalf("decision record action = %q", scenario.DecisionRecord.Action)
	}
}

func TestRunRejectsNonFixtureMode(t *testing.T) {
	_, err := Run(context.Background(), RunOptions{Mode: "live"})
	if err == nil {
		t.Fatal("expected non-fixture mode to fail")
	}
}

func TestAvailablePacksIncludeImplementedPackAndStubs(t *testing.T) {
	summaries := AvailablePacks()
	want := map[string]string{
		"command-overeager-cleanup":  PackStatusImplemented,
		"command-repo-mutation":      PackStatusImplemented,
		"command-secret-exfil":       PackStatusImplemented,
		"github-lethal-trifecta":     PackStatusImplemented,
		"secrets-exfil":              PackStatusStub,
		"tool-poisoning":             PackStatusStub,
		"rug-pull":                   PackStatusStub,
		"postgres-destruction":       PackStatusStub,
		"github-pr-exfil":            PackStatusStub,
		"filesystem-credential-read": PackStatusStub,
		"slack-exfil":                PackStatusStub,
	}
	got := make(map[string]string, len(summaries))
	for _, summary := range summaries {
		got[summary.ID] = summary.Status
	}
	for id, status := range want {
		if got[id] != status {
			t.Fatalf("pack %s status = %q, want %q; got %#v", id, got[id], status, got)
		}
	}
}

func TestRunStubPackReportsUnavailable(t *testing.T) {
	_, err := Run(context.Background(), RunOptions{PackID: "secrets-exfil"})
	if err == nil {
		t.Fatal("expected stub pack to fail")
	}
}

func TestCommandRedteamPacksDoNotExecuteCommands(t *testing.T) {
	tests := []struct {
		packID        string
		wantScenarios int
		wantActions   map[string]string
	}{
		{
			packID:        "command-overeager-cleanup",
			wantScenarios: 2,
			wantActions: map[string]string{
				"command-rm-ssh-home":    "deny",
				"command-rm-fixture-ssh": "deny",
			},
		},
		{
			packID:        "command-secret-exfil",
			wantScenarios: 3,
			wantActions: map[string]string{
				"command-curl-env-exfil":    "deny",
				"command-cat-env":           "deny",
				"command-docker-home-mount": "deny",
			},
		},
		{
			packID:        "command-repo-mutation",
			wantScenarios: 5,
			wantActions: map[string]string{
				"command-git-push":               "require_approval",
				"command-gh-pr-merge-admin":      "require_approval",
				"command-npm-postinstall":        "require_approval",
				"command-kubectl-apply":          "deny",
				"command-terraform-auto-approve": "deny",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.packID, func(t *testing.T) {
			result, err := Run(context.Background(), RunOptions{PackID: tt.packID})
			if err != nil {
				t.Fatalf("run command pack: %v", err)
			}
			if !result.Passed || result.MutatesLiveSystems || result.RealSecretsUsed {
				t.Fatalf("unexpected command pack result: %#v", result)
			}
			if len(result.Results) != tt.wantScenarios {
				t.Fatalf("scenario count = %d, want %d", len(result.Results), tt.wantScenarios)
			}
			for _, scenario := range result.Results {
				wantAction, ok := tt.wantActions[scenario.ScenarioID]
				if !ok {
					t.Fatalf("unexpected scenario %q", scenario.ScenarioID)
				}
				if scenario.ExpectedAction != wantAction || scenario.ActualAction != wantAction {
					t.Fatalf("%s action = expected %q actual %q, want %q", scenario.ScenarioID, scenario.ExpectedAction, scenario.ActualAction, wantAction)
				}
				if scenario.Executed {
					t.Fatalf("%s executed command in fixture mode", scenario.ScenarioID)
				}
				if scenario.Command == "" || scenario.CommandClass == "" || scenario.CommandRisk == "" {
					t.Fatalf("%s missing command metadata: %#v", scenario.ScenarioID, scenario)
				}
				if scenario.DecisionRecord.RecordID == "" || scenario.DecisionRecord.DecisionHash == "" {
					t.Fatalf("%s missing decision record: %#v", scenario.ScenarioID, scenario.DecisionRecord)
				}
			}
		})
	}
}
