package replay_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

const denyDropTablePolicy = `name: replay-cli-fixture
version: "1"
rules:
  - name: block-drop-table
    tool: query
    action: deny
    reason: destructive SQL
    match:
      field: arguments.sql
      contains: "DROP TABLE"
      case_insensitive: true
`

type capturingAuditor struct{ event governance.AuditEvent }

func (c *capturingAuditor) Publish(_ context.Context, event governance.AuditEvent) {
	if c.event.RequestID == "" {
		c.event = event
	}
}

func runReplay(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := boundarycli.Run(append([]string{"replay"}, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

// fixtureTriple writes a deny.yaml policy, evaluates a DROP TABLE request through
// a real pipeline, and writes the resulting record + recorded request to disk.
// It returns the record path, request path, and policy dir — a faithful triple a
// CLI replay can reproduce. The request is written AFTER evaluation so it carries
// the request_id/envelope_id the pipeline hashed into request_hash.
func fixtureTriple(t *testing.T, sql string) (recordPath, requestPath, policyDir string) {
	t.Helper()
	policyDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(policyDir, "deny.yaml"), []byte(denyDropTablePolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	policies, err := governance.LoadStaticPoliciesFromDir(policyDir)
	if err != nil {
		t.Fatal(err)
	}
	bundleHash, err := governance.PolicyBundleHashFromDir(policyDir)
	if err != nil {
		t.Fatal(err)
	}
	auditor := &capturingAuditor{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies:   policies,
		PolicyBundleHash: bundleHash,
		GatewayVersion:   "replay-cli-fixture",
	}, nil, nil, auditor)

	req := &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		AgentID:   "replay-agent",
		TenantID:  "replay-tenant",
		ToolName:  "query",
		Action:    "tools/call",
		Arguments: map[string]any{"sql": sql},
	}
	if _, err := pipeline.Evaluate(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	record := governance.BuildDecisionRecord(auditor.event)

	dir := t.TempDir()
	recordPath = filepath.Join(dir, "record.json")
	requestPath = filepath.Join(dir, "request.json")
	writeJSON(t, recordPath, record)
	writeJSON(t, requestPath, req)
	return recordPath, requestPath, policyDir
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestReplayCLIReproducesFullDecision proves boundary replay, through real CLI
// dispatch, reproduces the FULL recorded decision (every decision-defining
// field, not just action) and exits 0.
func TestReplayCLIReproducesFullDecision(t *testing.T) {
	recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")

	stdout, stderr, code := runReplay(t, recordPath, "--request", requestPath, "--policies", policyDir)
	if code != 0 {
		t.Fatalf("replay exit = %d stderr=%s\nstdout=%s", code, stderr, stdout)
	}
	for _, want := range []string{
		"Boundary replay",
		"status: ok",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"request_hash: match",
		"policy_bundle_hash: match",
		"action: match",
		"matched_rule: match",
		"decision_mode: match",
		"result: MATCH",
		"What this does not prove:",
		"does not prove enforcement",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("replay output missing %q:\n%s", want, stdout)
		}
	}
}

// TestReplayCLIJSONEnvelopeIsStable is the load-bearing schema assertion: the
// --json envelope identifies as boundary.replay.v1, carries the three local-only
// truth flags set false, and reports matched=true with both hash gates.
func TestReplayCLIJSONEnvelopeIsStable(t *testing.T) {
	recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")

	stdout, stderr, code := runReplay(t, "--json", recordPath, "--request", requestPath, "--policies", policyDir)
	if code != 0 {
		t.Fatalf("replay json exit = %d stderr=%s", code, stderr)
	}
	var payload struct {
		SchemaVersion       string `json:"schema_version"`
		Status              string `json:"status"`
		Matched             bool   `json:"matched"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresNetwork     bool   `json:"requires_network"`
		MutatesLiveSystems  bool   `json:"mutates_live_systems"`
		RecordSchemaVersion string `json:"record_schema_version"`
		HashChecks          []struct {
			Field   string `json:"field"`
			Matched bool   `json:"matched"`
		} `json:"hash_checks"`
		FieldChecks []struct {
			Field   string `json:"field"`
			Matched bool   `json:"matched"`
		} `json:"field_checks"`
		DoesNotProve []string `json:"does_not_prove"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse replay json: %v\n%s", err, stdout)
	}
	if payload.SchemaVersion != "boundary.replay.v1" {
		t.Fatalf("schema_version = %q, want boundary.replay.v1", payload.SchemaVersion)
	}
	if payload.Status != "ok" || !payload.Matched {
		t.Fatalf("expected ok/matched, got status=%q matched=%v", payload.Status, payload.Matched)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("replay must not require credentials, network, or live mutation: %#v", payload)
	}
	// A routed MCP request carries adapter_id/route_id, so the pipeline emits a
	// schema_version "2" record. Replay compares the decision, not route-context,
	// so the V2 fields do not affect the match.
	if payload.RecordSchemaVersion != "2" {
		t.Fatalf("record_schema_version = %q, want 2 (routed request carries route-context)", payload.RecordSchemaVersion)
	}
	hashFields := map[string]bool{}
	for _, c := range payload.HashChecks {
		hashFields[c.Field] = c.Matched
	}
	if !hashFields["request_hash"] || !hashFields["policy_bundle_hash"] {
		t.Fatalf("expected both hash gates matched, got %v", hashFields)
	}
	if len(payload.FieldChecks) < 4 {
		t.Fatalf("replay must compare more than action; got %d field checks", len(payload.FieldChecks))
	}
	if len(payload.DoesNotProve) == 0 {
		t.Fatalf("does_not_prove footer must be present")
	}
}

// TestReplayCLIFailsClosedOnDecisionDrift proves drift cases exit non-zero
// through the CLI: same action / different matched_rule, same action / different
// reason, a changed policy bundle, and a different request.
func TestReplayCLIFailsClosedOnDecisionDrift(t *testing.T) {
	t.Run("same_action_different_matched_rule", func(t *testing.T) {
		recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")
		mutateRecord(t, recordPath, func(r *governance.DecisionRecordV1) {
			r.MatchedRule = "stale-rule"
		})
		stdout, _, code := runReplay(t, recordPath, "--request", requestPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected non-zero exit on matched_rule drift:\n%s", stdout)
		}
		if !strings.Contains(stdout, "matched_rule mismatch") || !strings.Contains(stdout, "result: MISMATCH") {
			t.Fatalf("expected matched_rule mismatch in output:\n%s", stdout)
		}
	})

	t.Run("same_action_different_reason", func(t *testing.T) {
		recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")
		mutateRecord(t, recordPath, func(r *governance.DecisionRecordV1) {
			r.Reason = "different rationale"
		})
		stdout, _, code := runReplay(t, recordPath, "--request", requestPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected non-zero exit on reason drift:\n%s", stdout)
		}
		if !strings.Contains(stdout, "reason mismatch") {
			t.Fatalf("expected reason mismatch in output:\n%s", stdout)
		}
	})

	t.Run("policy_bundle_hash_mismatch", func(t *testing.T) {
		recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")
		// Change the bundle so its recomputed hash diverges from the record.
		if err := os.WriteFile(filepath.Join(policyDir, "extra.yaml"), []byte("name: extra\nversion: \"1\"\nrules: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		stdout, _, code := runReplay(t, recordPath, "--request", requestPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected non-zero exit on policy bundle drift:\n%s", stdout)
		}
		if !strings.Contains(stdout, "policy_bundle_hash mismatch") {
			t.Fatalf("expected policy_bundle_hash mismatch in output:\n%s", stdout)
		}
	})

	t.Run("request_hash_mismatch", func(t *testing.T) {
		recordPath, _, policyDir := fixtureTriple(t, "DROP TABLE users")
		// Supply a different request than the one recorded.
		other := &governance.GovernanceRequest{
			Transport: governance.TransportMCP,
			AgentID:   "replay-agent",
			TenantID:  "replay-tenant",
			ToolName:  "query",
			Action:    "tools/call",
			Arguments: map[string]any{"sql": "SELECT 1"},
		}
		otherPath := filepath.Join(t.TempDir(), "other-request.json")
		writeJSON(t, otherPath, other)
		stdout, _, code := runReplay(t, recordPath, "--request", otherPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected non-zero exit on request drift:\n%s", stdout)
		}
		if !strings.Contains(stdout, "request_hash mismatch") {
			t.Fatalf("expected request_hash mismatch in output:\n%s", stdout)
		}
	})
}

// TestReplayCLIRejectsMissingFlags proves the reconstruction contract is
// enforced at the CLI: missing positional record, missing --request, and missing
// --policies each exit non-zero with a stderr message.
func TestReplayCLIRejectsMissingFlags(t *testing.T) {
	recordPath, requestPath, policyDir := fixtureTriple(t, "DROP TABLE users")

	t.Run("missing_record", func(t *testing.T) {
		stdout, stderr, code := runReplay(t, "--request", requestPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected usage failure with no record, stdout=%s", stdout)
		}
		if !strings.Contains(stderr, "usage: boundary replay") {
			t.Fatalf("stderr missing usage line: %s", stderr)
		}
	})

	t.Run("missing_request", func(t *testing.T) {
		stdout, stderr, code := runReplay(t, recordPath, "--policies", policyDir)
		if code == 0 {
			t.Fatalf("expected failure with no --request, stdout=%s", stdout)
		}
		if !strings.Contains(stderr, "replay:") || !strings.Contains(stderr, "--request is required") {
			t.Fatalf("stderr missing --request error: %s", stderr)
		}
	})

	t.Run("missing_policies", func(t *testing.T) {
		stdout, stderr, code := runReplay(t, recordPath, "--request", requestPath)
		if code == 0 {
			t.Fatalf("expected failure with no --policies, stdout=%s", stdout)
		}
		if !strings.Contains(stderr, "--policies is required") {
			t.Fatalf("stderr missing --policies error: %s", stderr)
		}
	})
}

// TestReplayCLIRejectsMissingFile proves a bad record path exits non-zero with a
// stderr message.
func TestReplayCLIRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runReplay(t, filepath.Join(dir, "missing.json"), "--request", filepath.Join(dir, "r.json"), "--policies", dir)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing record, stdout=%s", stdout)
	}
	if !strings.Contains(stderr, "replay:") {
		t.Fatalf("stderr missing replay error: %s", stderr)
	}
}

// TestReplayCLIHelp proves the --help block keeps the local-only, fixture-safe,
// decision-not-enforcement framing the language gate and help-language test
// expect.
func TestReplayCLIHelp(t *testing.T) {
	stdout, stderr, code := runReplay(t, "--help")
	if code != 0 {
		t.Fatalf("replay help exit = %d stderr=%s", code, stderr)
	}
	output := stdout + stderr
	for _, want := range []string{
		"Re-evaluate a recorded request",
		"boundary replay [--json]",
		"local-only and fixture-safe",
		"not action alone",
		"reproduces the decision, not enforcement",
		"does not prove no upstream bytes moved",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("replay help missing %q:\n%s", want, output)
		}
	}
}

// TestReplayCommittedExampleReproducesDecision pins the committed example triple
// (record + recorded request + policy directory) so the documented
// find -> verify -> explain -> replay walkthrough cannot silently drift: the
// committed record replays cleanly against the committed request and policy
// bundle, reproducing every decision-defining field, through real CLI dispatch.
func TestReplayCommittedExampleReproducesDecision(t *testing.T) {
	const (
		record   = "../../docs/examples/decision-record-replay.example.json"
		request  = "../../docs/examples/replay-request.example.json"
		policies = "../../docs/examples/replay-policies"
	)
	for _, p := range []string{record, request} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("committed replay fixture missing: %s: %v", p, err)
		}
	}

	stdout, stderr, code := runReplay(t, record, "--request", request, "--policies", policies)
	if code != 0 {
		t.Fatalf("committed example replay exit = %d stderr=%s\n%s", code, stderr, stdout)
	}
	for _, want := range []string{
		"request_hash: match",
		"policy_bundle_hash: match",
		"action: match",
		"reason: match",
		"decision_mode: match",
		"matched_rule: match",
		"policy_file: match",
		"result: MATCH",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("committed example replay missing %q:\n%s", want, stdout)
		}
	}
}

func mutateRecord(t *testing.T, path string, mutate func(*governance.DecisionRecordV1)) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	mutate(&record)
	// Re-stamp decision_hash so the record stays internally consistent and only
	// the targeted field drifted (replay compares decisions, not the hash).
	record.DecisionHash = governance.ComputeDecisionHash(record)
	writeJSON(t, path, record)
}
