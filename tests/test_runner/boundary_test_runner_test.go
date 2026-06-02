package test_runner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestBoundaryTestGoldenCorpusPasses(t *testing.T) {
	casesPath := filepath.Join(repoRoot(t), "tests", "fixtures", "policy-test", "cases")

	stdout, stderr, code := runBoundaryTest("test", "--path", casesPath)
	if code != 0 {
		t.Fatalf("boundary test exit = %d stderr=%s\nstdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{
		"boundary test: " + casesPath,
		"[pass] allow-readonly-list",
		"[pass] deny-write-after-taint",
		"[pass] warn-large-result",
		"[pass] approve-prod-migration",
		"[pass] escalate-semantic-review",
		"[pass] reject-malformed-policy",
		"status: pass",
		"cases: 6",
		"passed: 6",
		"failed: 0",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("boundary test output missing %q:\n%s", want, stdout)
		}
	}
}

func TestBoundaryTestJSONEnvelopeIsStable(t *testing.T) {
	casesPath := filepath.Join(repoRoot(t), "tests", "fixtures", "policy-test", "cases")

	stdout, stderr, code := runBoundaryTest("test", "--path", casesPath, "--format", "json")
	if code != 0 {
		t.Fatalf("boundary test json exit = %d stderr=%s\nstdout=%s", code, stderr, stdout)
	}
	payload := parseResult(t, stdout)
	if payload.SchemaVersion != "boundary.test.v1" {
		t.Fatalf("schema_version = %q, want boundary.test.v1", payload.SchemaVersion)
	}
	if payload.Status != "pass" || payload.Summary.Total != 6 || payload.Summary.Passed != 6 || payload.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %#v", payload)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("boundary test must be local fixture-only: %#v", payload)
	}
	if len(payload.DoesNotProve) == 0 {
		t.Fatalf("does_not_prove footer must be present")
	}
	if !containsLine(payload.DoesNotProve, "does not prove production route enforcement") {
		t.Fatalf("does_not_prove footer missing routed-only caveat: %v", payload.DoesNotProve)
	}

	actions := map[string]string{}
	for _, c := range payload.Cases {
		if c.Status != "pass" {
			t.Fatalf("case %s did not pass: %#v", c.Name, c)
		}
		actions[c.Name] = c.ActualAction
		if c.ExpectedAction == "" {
			t.Fatalf("case %s missing expected action: %#v", c.Name, c)
		}
	}
	wantActions := map[string]string{
		"allow-readonly-list":      "allow",
		"deny-write-after-taint":   "deny",
		"warn-large-result":        "warn",
		"approve-prod-migration":   "require_approval",
		"escalate-semantic-review": "escalate",
		"reject-malformed-policy":  "parse_rejection",
	}
	for name, want := range wantActions {
		if actions[name] != want {
			t.Fatalf("case %s actual action = %q, want %q (all actions: %v)", name, actions[name], want, actions)
		}
	}
}

func TestBoundaryTestFailsOnVerdictMismatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "policies", "deny.yaml"), `name: mismatch
version: "1"
rules:
  - name: deny-private-write
    tool: github.create_or_update_file
    action: deny
    reason: lethal_trifecta_detected
`)
	writeFile(t, filepath.Join(dir, "cases", "mismatch.yaml"), `name: mismatch-deny-expected-allow
policies: ../policies
request:
  transport: mcp
  tool_name: github.create_or_update_file
  action: tools/call
  agent_id: policy-test-agent
  tenant_id: policy-test-tenant
  arguments:
    target_repo_visibility: private
expect:
  action: allow
`)

	stdout, stderr, code := runBoundaryTest("test", "--path", filepath.Join(dir, "cases"), "--format", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit on verdict mismatch, stdout=%s stderr=%s", stdout, stderr)
	}
	payload := parseResult(t, stdout)
	if payload.Status != "fail" || payload.Summary.Failed != 1 {
		t.Fatalf("expected failed run, got %#v", payload)
	}
	if len(payload.Cases) != 1 || payload.Cases[0].Status != "fail" || payload.Cases[0].ActualAction != "deny" {
		t.Fatalf("unexpected mismatch case result: %#v", payload.Cases)
	}
}

func TestBoundaryTestUnexpectedPolicyParseErrorFails(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "policies", "bad.yaml"), "name: bad\nrules:\n  - name: [unterminated\n")
	writeFile(t, filepath.Join(dir, "cases", "unexpected-parse.yaml"), `name: unexpected-policy-parse-error
policies: ../policies
request:
  transport: mcp
  tool_name: github.create_or_update_file
  action: tools/call
  agent_id: policy-test-agent
expect:
  action: deny
`)

	stdout, stderr, code := runBoundaryTest("test", "--path", filepath.Join(dir, "cases"), "--format", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit on unexpected policy parse error, stdout=%s stderr=%s", stdout, stderr)
	}
	payload := parseResult(t, stdout)
	if payload.Status != "fail" || len(payload.Cases) != 1 {
		t.Fatalf("unexpected parse-error payload: %#v", payload)
	}
	if payload.Cases[0].ActualAction != "policy_load_error" || payload.Cases[0].Error == "" {
		t.Fatalf("expected policy_load_error with detail, got %#v", payload.Cases[0])
	}
}

func TestBoundaryTestExpectedParseRejectionFailsWhenPolicyLoads(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "policies", "valid.yaml"), `name: valid
version: "1"
rules:
  - name: deny-private-write
    tool: github.create_or_update_file
    action: deny
    reason: lethal_trifecta_detected
`)
	writeFile(t, filepath.Join(dir, "cases", "expected-reject.yaml"), `name: expected-parse-rejection
policies: ../policies
expect:
  action: parse_rejection
`)

	stdout, stderr, code := runBoundaryTest("test", "--path", filepath.Join(dir, "cases"), "--format", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit when expected parse rejection does not happen, stdout=%s stderr=%s", stdout, stderr)
	}
	payload := parseResult(t, stdout)
	if payload.Status != "fail" || len(payload.Cases) != 1 {
		t.Fatalf("unexpected expected-reject payload: %#v", payload)
	}
	if payload.Cases[0].ActualAction != "policy_loaded" || !strings.Contains(payload.Cases[0].Error, "expected policy parse rejection") {
		t.Fatalf("expected policy_loaded failure with detail, got %#v", payload.Cases[0])
	}
}

func TestBoundaryTestRejectsMalformedCaseFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "cases", "bad.yaml"), "name: bad\nexpect:\n  action: [unterminated\n")

	stdout, stderr, code := runBoundaryTest("test", "--path", filepath.Join(dir, "cases"), "--format", "json")
	if code == 0 {
		t.Fatalf("expected non-zero exit on malformed case file, stdout=%s stderr=%s", stdout, stderr)
	}
	payload := parseResult(t, stdout)
	if payload.Status != "fail" || len(payload.Cases) != 1 {
		t.Fatalf("unexpected malformed-case payload: %#v", payload)
	}
	if payload.Cases[0].ActualAction != "case_parse_error" || payload.Cases[0].Error == "" {
		t.Fatalf("expected case_parse_error with detail, got %#v", payload.Cases[0])
	}
}

func TestBoundaryTestGoldenCorpusDocumentsRouteBypassCaveat(t *testing.T) {
	readmePath := filepath.Join(repoRoot(t), "tests", "fixtures", "policy-test", "README.md")
	body, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"policy verdicts for routed requests only",
		"does not prove production route enforcement",
		"does not prove a deployment removed direct or unrouted paths",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("golden corpus README missing %q:\n%s", want, string(body))
		}
	}
}

type testResult struct {
	SchemaVersion       string       `json:"schema_version"`
	Status              string       `json:"status"`
	Path                string       `json:"path"`
	RequiresCredentials bool         `json:"requires_credentials"`
	RequiresNetwork     bool         `json:"requires_network"`
	MutatesLiveSystems  bool         `json:"mutates_live_systems"`
	Summary             testSummary  `json:"summary"`
	Cases               []caseResult `json:"cases"`
	DoesNotProve        []string     `json:"does_not_prove"`
}

type testSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type caseResult struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ExpectedAction string `json:"expected_action"`
	ActualAction   string `json:"actual_action"`
	Reason         string `json:"reason,omitempty"`
	MatchedRule    string `json:"matched_rule,omitempty"`
	PolicyFile     string `json:"policy_file,omitempty"`
	Error          string `json:"error,omitempty"`
}

func runBoundaryTest(args ...string) (string, string, int) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func parseResult(t *testing.T, stdout string) testResult {
	t.Helper()
	var payload testResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse boundary test JSON: %v\n%s", err, stdout)
	}
	return payload
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func containsLine(lines []string, needle string) bool {
	for _, line := range lines {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}
