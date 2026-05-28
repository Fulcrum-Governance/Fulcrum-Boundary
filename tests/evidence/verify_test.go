package evidence_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

type verifyPayload struct {
	SchemaVersion     string `json:"schema_version"`
	Status            string `json:"status"`
	ManifestSchema    string `json:"manifest_schema"`
	ArtifactCount     int    `json:"artifact_count"`
	VerifiedArtifacts int    `json:"verified_artifacts"`
	ParsedRecords     int    `json:"parsed_records"`
	Checks            []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"checks"`
}

func TestEvidenceVerifyPassesBundle(t *testing.T) {
	out := filepath.Join(t.TempDir(), "evidence")
	bundleEvidence(t, out, false)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "verify", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	result := parseVerify(t, stdout.Bytes())
	if result.SchemaVersion != "boundary.evidence_verify.v1" || result.Status != "pass" || result.ManifestSchema != "boundary.evidence_bundle.v1" {
		t.Fatalf("unexpected verify identity: %#v", result)
	}
	if result.ArtifactCount == 0 || result.VerifiedArtifacts != result.ArtifactCount {
		t.Fatalf("verify did not check every artifact: %#v", result)
	}
	requireVerifyCheck(t, result, "summary_references", "pass")
}

func TestEvidenceVerifyParsesCopiedDecisionRecords(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".boundary")
	if err := os.MkdirAll(filepath.Join(source, "records"), 0o700); err != nil {
		t.Fatalf("mkdir records: %v", err)
	}
	records := "{\"record_id\":\"rec_one\",\"action\":\"deny\"}\n{\"record_id\":\"rec_two\",\"action\":\"allow\"}\n"
	if err := os.WriteFile(filepath.Join(source, "records", "decision-records.jsonl"), []byte(records), 0o600); err != nil {
		t.Fatalf("write records: %v", err)
	}
	out := filepath.Join(root, "evidence")
	var bundleStdout bytes.Buffer
	var bundleStderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "bundle", "--from", source, "--out", out}, &bundleStdout, &bundleStderr)
	if code != 0 {
		t.Fatalf("bundle exit = %d, stdout=%s stderr=%s", code, bundleStdout.String(), bundleStderr.String())
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code = boundarycli.Run([]string{"evidence", "verify", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify records exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	result := parseVerify(t, stdout.Bytes())
	if result.ParsedRecords != 2 {
		t.Fatalf("expected 2 parsed records, got %#v", result)
	}
}

func TestEvidenceVerifyFailsTamperedArtifact(t *testing.T) {
	out := filepath.Join(t.TempDir(), "evidence")
	bundleEvidence(t, out, false)
	if err := os.WriteFile(filepath.Join(out, "version.txt"), []byte("tampered\n"), 0o600); err != nil {
		t.Fatalf("tamper artifact: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "verify", out, "--json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("tampered verify unexpectedly passed, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	result := parseVerify(t, stdout.Bytes())
	if result.Status != "fail" {
		t.Fatalf("tampered verify status = %#v", result)
	}
	found := false
	for _, check := range result.Checks {
		if strings.Contains(check.Name, "version.txt") && check.Status == "fail" && strings.Contains(check.Detail, "expected sha256:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("verify output missing version.txt hash failure: %#v", result.Checks)
	}
}

func TestEvidenceVerifyHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "verify", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify help exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String() + stderr.String()
	for _, want := range []string{
		"Verify a Boundary evidence bundle",
		"boundary evidence verify boundary-evidence --json",
		"SHA-256 hashes",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("verify help missing %q:\n%s", want, output)
		}
	}
}

func bundleEvidence(t *testing.T, out string, includeDemo bool) {
	t.Helper()
	args := []string{"evidence", "bundle", "--out", out}
	if includeDemo {
		args = append(args, "--include-demo")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run(args, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("bundle exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func parseVerify(t *testing.T, data []byte) verifyPayload {
	t.Helper()
	var result verifyPayload
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse verify: %v\n%s", err, string(data))
	}
	return result
}

func requireVerifyCheck(t *testing.T, result verifyPayload, name, status string) {
	t.Helper()
	for _, check := range result.Checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("check %s status = %s, want %s: %#v", name, check.Status, status, check)
			}
			return
		}
	}
	t.Fatalf("check %s not found: %#v", name, result.Checks)
}
