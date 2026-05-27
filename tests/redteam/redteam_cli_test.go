package redteam_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestBoundaryRedteamDefaultShowsGitHubDenyDecisionRecord(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"redteam"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("redteam exit = %d, stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"redteam mode: fixture",
		"pack: github-lethal-trifecta",
		"live mutation: none",
		"real secrets: none",
		"scenario: github-write-after-taint",
		"expected: DENY",
		"actual: DENY",
		"result: pass",
		"decision record: rec_",
		"decision hash: sha256:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("redteam output missing %q: %s", want, output)
		}
	}
}

func TestBoundaryRedteamJSONIncludesDecisionRecord(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"redteam", "--format", "json"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("redteam json exit = %d", code)
	}
	var payload struct {
		SchemaVersion      string `json:"schema_version"`
		Mode               string `json:"mode"`
		PackID             string `json:"pack_id"`
		Passed             bool   `json:"passed"`
		MutatesLiveSystems bool   `json:"mutates_live_systems"`
		RealSecretsUsed    bool   `json:"real_secrets_used"`
		Results            []struct {
			ScenarioID     string `json:"scenario_id"`
			ExpectedAction string `json:"expected_action"`
			ActualAction   string `json:"actual_action"`
			DecisionRecord struct {
				RecordID     string `json:"record_id"`
				DecisionHash string `json:"decision_hash"`
				Action       string `json:"action"`
			} `json:"decision_record"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse redteam json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion == "" || payload.Mode != "fixture" || payload.PackID != "github-lethal-trifecta" || !payload.Passed {
		t.Fatalf("unexpected payload identity: %#v", payload)
	}
	if payload.MutatesLiveSystems || payload.RealSecretsUsed {
		t.Fatalf("fixture payload must not mutate live systems or use real secrets: %#v", payload)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(payload.Results))
	}
	got := payload.Results[0]
	if got.ExpectedAction != "deny" || got.ActualAction != "deny" || got.DecisionRecord.Action != "deny" {
		t.Fatalf("expected deny decision in payload: %#v", got)
	}
	if !strings.HasPrefix(got.DecisionRecord.RecordID, "rec_") || !strings.HasPrefix(got.DecisionRecord.DecisionHash, "sha256:") {
		t.Fatalf("decision record missing hashes: %#v", got.DecisionRecord)
	}
}

func TestBoundaryRedteamListShowsStubs(t *testing.T) {
	var stdout bytes.Buffer
	code := boundarycli.Run([]string{"redteam", "--list"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("redteam list exit = %d", code)
	}
	output := stdout.String()
	for _, want := range []string{
		"github-lethal-trifecta\timplemented",
		"secrets-exfil\tstub",
		"tool-poisoning\tstub",
		"rug-pull\tstub",
		"postgres-destruction\tstub",
		"github-pr-exfil\tstub",
		"filesystem-credential-read\tstub",
		"slack-exfil\tstub",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("redteam list missing %q: %s", want, output)
		}
	}
}

func TestBoundaryRedteamLiveModeFailsClosed(t *testing.T) {
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"redteam", "--mode", "live"}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatal("expected live mode to fail")
	}
	if !strings.Contains(stderr.String(), "only \"fixture\" runs without live system access") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
