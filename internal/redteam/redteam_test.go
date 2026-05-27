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
