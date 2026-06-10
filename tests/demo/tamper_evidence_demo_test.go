package demo_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestTamperEvidenceDemoTextOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "tamper-evidence"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	// stdout must be plain when captured (no terminal).
	if strings.ContainsRune(output, '\033') {
		t.Fatalf("captured demo output must not contain ANSI escapes:\n%q", output)
	}
	for _, want := range []string{
		"Tamper-evidence demo: forge the receipt (fixture-only)",
		"fixture-only: true",
		"status: pass",
		"emit + verify original",
		"verified",
		`action: "deny" -> "allow"`,
		"tamper DETECTED",
		"decision_hash mismatch",
		"What this proves:",
		"What this does not prove:",
		"not tamper-proof or immutable",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("demo output missing %q:\n%s", want, output)
		}
	}
}

func TestTamperEvidenceDemoJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "tamper-evidence", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("demo json exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion  string `json:"schema_version"`
		Passed         bool   `json:"passed"`
		FixtureOnly    bool   `json:"fixture_only"`
		TamperDetected bool   `json:"tamper_detected"`
		TamperedField  string `json:"tampered_field"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse demo json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.demo.tamper_evidence.v1" || !payload.Passed || !payload.FixtureOnly {
		t.Fatalf("unexpected demo json identity: %#v", payload)
	}
	if !payload.TamperDetected || payload.TamperedField != "action" {
		t.Fatalf("unexpected tamper fields: %#v", payload)
	}
}

func TestTamperEvidenceDemoHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"demo", "tamper-evidence", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String() + stderr.String()
	for _, want := range []string{
		"tamper-evidence",
		"hash-verifiable",
		"not tamper-proof or immutable",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}
