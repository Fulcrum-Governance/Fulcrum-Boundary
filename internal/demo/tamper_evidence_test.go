package demo

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunTamperEvidenceDetectsForgery(t *testing.T) {
	result, err := RunTamperEvidence(context.Background(), TamperEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Status != "pass" {
		t.Fatalf("tamper-evidence demo did not pass: %#v", result)
	}
	if !result.FixtureOnly || result.RequiresCredentials || result.RequiresNetwork || result.MutatesLiveSystems {
		t.Fatalf("demo must be fixture-only and local: %#v", result)
	}
	if !result.OriginalVerified {
		t.Fatalf("original record should verify: %#v", result)
	}
	if !result.TamperDetected {
		t.Fatalf("forged record should fail verification: %#v", result)
	}
	if result.TamperedField != "action" || result.OriginalValue != "deny" || result.ForgedValue != "allow" {
		t.Fatalf("unexpected tamper target: %#v", result)
	}
	if result.StoredHash == result.RecomputedHash {
		t.Fatalf("forging a field must change the recomputed hash: %#v", result)
	}
	if !strings.Contains(result.VerifyError, "decision_hash mismatch") {
		t.Fatalf("verify error should report a decision_hash mismatch: %q", result.VerifyError)
	}
	if result.RecordID == "" {
		t.Fatalf("record id should be set: %#v", result)
	}
}

func TestTamperEvidenceTextNeverEmitsANSIToBuffer(t *testing.T) {
	result, err := RunTamperEvidence(context.Background(), TamperEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	// A bytes.Buffer is not a terminal, so even via the color entry point with a
	// colorizer built from it, the output must be plain.
	if err := WriteTamperEvidenceTextColor(&out, result, NewColorizer(&out)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.ContainsRune(text, '\033') {
		t.Fatalf("text output must not contain ANSI escapes for a non-terminal writer:\n%q", text)
	}
	for _, want := range []string{
		"Tamper-evidence demo: forge the receipt (fixture-only)",
		"fixture-only: true",
		"tamper DETECTED",
		"decision_hash mismatch",
		"What this proves:",
		"What this does not prove:",
		"not tamper-proof or immutable",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestTamperEvidenceJSONShape(t *testing.T) {
	result, err := RunTamperEvidence(context.Background(), TamperEvidenceOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteTamperEvidenceJSON(&out, result); err != nil {
		t.Fatal(err)
	}
	var payload struct {
		SchemaVersion  string `json:"schema_version"`
		Passed         bool   `json:"passed"`
		TamperDetected bool   `json:"tamper_detected"`
		TamperedField  string `json:"tampered_field"`
		StoredHash     string `json:"stored_hash"`
		RecomputedHash string `json:"recomputed_hash"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("parse json: %v\n%s", err, out.String())
	}
	if payload.SchemaVersion != TamperEvidenceSchemaVersion || !payload.Passed || !payload.TamperDetected {
		t.Fatalf("unexpected json identity: %#v", payload)
	}
	if payload.TamperedField != "action" || payload.StoredHash == payload.RecomputedHash {
		t.Fatalf("unexpected json tamper fields: %#v", payload)
	}
}
