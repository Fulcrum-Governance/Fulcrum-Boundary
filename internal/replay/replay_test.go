package replay

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// capturingAuditor records the first AuditEvent the pipeline emits, so a test
// can build the exact decision record the pipeline would have written and then
// replay it.
type capturingAuditor struct{ event governance.AuditEvent }

func (c *capturingAuditor) Publish(_ context.Context, event governance.AuditEvent) {
	if c.event.RequestID == "" {
		c.event = event
	}
}

const denyDropTablePolicy = `name: replay-fixture
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

// emitFixtureRecord builds a faithful record + request + policy-dir triple by
// running a real pipeline evaluation, exactly as production would: it loads the
// policy dir, configures the pipeline with the matching policy_bundle_hash,
// evaluates the request, and builds the decision record from the captured audit
// event. The request is returned AFTER evaluation so it carries the same
// request_id/envelope_id the pipeline hashed into request_hash.
func emitFixtureRecord(t *testing.T, policyDir, sql string) (governance.DecisionRecordV1, *governance.GovernanceRequest) {
	t.Helper()
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
		GatewayVersion:   "replay-fixture",
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
	return record, req
}

// writeJSON marshals v to a temp file under dir and returns the path.
func writeJSON(t *testing.T, dir, name string, v any) string {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestReplayReproducesRecordedDecision is the load-bearing happy path: a record
// emitted by a real pipeline run replays cleanly against the recorded request
// and policy bundle, and EVERY decision-defining field matches — not action
// alone.
func TestReplayReproducesRecordedDecision(t *testing.T) {
	policyDir := t.TempDir()
	record, req := emitFixtureRecord(t, policyDir, "DROP TABLE users")
	if record.Action != "deny" {
		t.Fatalf("fixture record should deny, got %q", record.Action)
	}
	if record.RequestHash == "" || record.PolicyBundleHash == "" {
		t.Fatalf("fixture record must carry request_hash and policy_bundle_hash: %#v", record)
	}

	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", req)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if !result.Matched || result.Status != "ok" {
		t.Fatalf("expected match, got matched=%v status=%q mismatches=%v", result.Matched, result.Status, result.Mismatches)
	}

	// request_hash and policy_bundle_hash gates both ran and matched.
	gateFields := map[string]bool{}
	for _, c := range result.HashChecks {
		gateFields[c.Field] = c.Matched
		if !c.Matched {
			t.Fatalf("hash gate %s did not match: %#v", c.Field, c)
		}
	}
	if !gateFields["request_hash"] || !gateFields["policy_bundle_hash"] {
		t.Fatalf("expected both hash gates, got %v", gateFields)
	}

	// The comparison covers more than action: matched_rule and decision_mode
	// must be among the compared fields.
	checked := map[string]string{}
	for _, c := range result.FieldChecks {
		if !c.Matched {
			t.Fatalf("decision field %s did not match: %#v", c.Field, c)
		}
		checked[c.Field] = c.Reproduced
	}
	for _, want := range []string{"action", "reason", "decision_mode", "matched_rule"} {
		if _, ok := checked[want]; !ok {
			t.Fatalf("replay must compare %q; compared %v", want, checked)
		}
	}
	if checked["matched_rule"] != "block-drop-table" {
		t.Fatalf("reproduced matched_rule = %q, want block-drop-table", checked["matched_rule"])
	}
	if checked["decision_mode"] != "deterministic" {
		t.Fatalf("reproduced decision_mode = %q, want deterministic", checked["decision_mode"])
	}
}

// TestReplayFailsOnDifferentMatchedRuleSameAction proves replay is not
// action-only: a record whose action is "deny" but whose matched_rule is stale
// fails replay even though the reproduced action is also "deny".
func TestReplayFailsOnDifferentMatchedRuleSameAction(t *testing.T) {
	policyDir := t.TempDir()
	record, req := emitFixtureRecord(t, policyDir, "DROP TABLE users")

	// Stale record: same action, different matched_rule. Re-stamp decision_hash
	// so the record is internally consistent and only the rule drifted.
	record.MatchedRule = "some-other-rule"
	record.DecisionHash = governance.ComputeDecisionHash(record)
	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", req)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay errored instead of reporting mismatch: %v", err)
	}
	if result.Matched {
		t.Fatalf("replay matched despite a different matched_rule (action alone agreed): %#v", result)
	}
	if result.Reproduced.Action != "deny" {
		t.Fatalf("reproduced action should still be deny, got %q", result.Reproduced.Action)
	}
	if !containsSubstring(result.Mismatches, "matched_rule mismatch") {
		t.Fatalf("expected matched_rule mismatch reason, got %v", result.Mismatches)
	}
}

// TestReplayFailsOnDifferentReasonSameAction proves a reason drift (same action)
// fails replay.
func TestReplayFailsOnDifferentReasonSameAction(t *testing.T) {
	policyDir := t.TempDir()
	record, req := emitFixtureRecord(t, policyDir, "DROP TABLE users")

	record.Reason = "a different rationale"
	record.DecisionHash = governance.ComputeDecisionHash(record)
	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", req)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay errored: %v", err)
	}
	if result.Matched {
		t.Fatalf("replay matched despite a different reason: %#v", result)
	}
	if !containsSubstring(result.Mismatches, "reason mismatch") {
		t.Fatalf("expected reason mismatch, got %v", result.Mismatches)
	}
}

// TestReplayFailsWhenRecordHadNoRuleButReevalMatchesOne proves the comparison is
// symmetric: a record that carried no matched_rule but whose re-evaluation now
// matches a rule is genuine drift and fails, even though both actions are "deny".
// This is the stale-bundle case where the recorded run matched nothing and the
// current bundle matches a rule.
func TestReplayFailsWhenRecordHadNoRuleButReevalMatchesOne(t *testing.T) {
	policyDir := t.TempDir()
	record, req := emitFixtureRecord(t, policyDir, "DROP TABLE users")

	// Blank the recorded matched_rule/policy_file (as if the recorded run matched
	// no static rule) but keep action=deny, and re-stamp the hash so only those
	// fields drifted. Re-evaluation against the live bundle WILL match the rule.
	record.MatchedRule = ""
	record.PolicyFile = ""
	record.DecisionHash = governance.ComputeDecisionHash(record)
	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", req)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay errored: %v", err)
	}
	if result.Matched {
		t.Fatalf("replay matched despite the record carrying no rule while re-eval matched one: %#v", result)
	}
	if !containsSubstring(result.Mismatches, "matched_rule mismatch") {
		t.Fatalf("expected matched_rule mismatch (empty record vs reproduced rule), got %v", result.Mismatches)
	}
}

// TestReplayFailsOnPolicyBundleHashMismatch proves a stale or different bundle
// is caught by the policy_bundle_hash gate before the decision is trusted.
func TestReplayFailsOnPolicyBundleHashMismatch(t *testing.T) {
	policyDir := t.TempDir()
	record, req := emitFixtureRecord(t, policyDir, "DROP TABLE users")

	// Mutate the policy directory so its recomputed hash no longer matches the
	// record's policy_bundle_hash — exactly the "different bundle" drift case.
	if err := os.WriteFile(filepath.Join(policyDir, "extra.yaml"), []byte("name: extra\nversion: \"1\"\nrules: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", req)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay errored: %v", err)
	}
	if result.Matched {
		t.Fatalf("replay matched despite a changed policy bundle: %#v", result)
	}
	if !containsSubstring(result.Mismatches, "policy_bundle_hash mismatch") {
		t.Fatalf("expected policy_bundle_hash mismatch, got %v", result.Mismatches)
	}
}

// TestReplayFailsOnRequestHashMismatch proves supplying a different request than
// the one recorded fails the request_hash gate.
func TestReplayFailsOnRequestHashMismatch(t *testing.T) {
	policyDir := t.TempDir()
	record, _ := emitFixtureRecord(t, policyDir, "DROP TABLE users")

	// A different request than the one recorded.
	other := &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		AgentID:   "replay-agent",
		TenantID:  "replay-tenant",
		ToolName:  "query",
		Action:    "tools/call",
		Arguments: map[string]any{"sql": "SELECT 1"},
	}
	recordPath := writeJSON(t, policyDir, "record.json", record)
	requestPath := writeJSON(t, policyDir, "request.json", other)

	result, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: policyDir})
	if err != nil {
		t.Fatalf("replay errored: %v", err)
	}
	if result.Matched {
		t.Fatalf("replay matched despite a different request: %#v", result)
	}
	if !containsSubstring(result.Mismatches, "request_hash mismatch") {
		t.Fatalf("expected request_hash mismatch, got %v", result.Mismatches)
	}
}

// TestReplayRequiresRequestAndPolicies proves the reconstruction contract is
// mandatory: replay errors when --request or --policies is missing.
func TestReplayRequiresRequestAndPolicies(t *testing.T) {
	dir := t.TempDir()
	record, _ := emitFixtureRecord(t, dir, "DROP TABLE users")
	recordPath := writeJSON(t, dir, "record.json", record)

	if _, err := Run(Options{RecordPath: recordPath, PolicyDir: dir}); err == nil {
		t.Fatal("replay must require --request")
	}
	if _, err := Run(Options{RecordPath: recordPath, RequestPath: writeJSON(t, dir, "req.json", &governance.GovernanceRequest{})}); err == nil {
		t.Fatal("replay must require --policies")
	}
	if _, err := Run(Options{RequestPath: "x", PolicyDir: dir}); err == nil {
		t.Fatal("replay must require a record path")
	}
}

// TestReplayRejectsUnsupportedSchema proves a record carrying a schema version
// outside {"1","2"} is rejected as malformed input (an error, not a mismatch).
func TestReplayRejectsUnsupportedSchema(t *testing.T) {
	dir := t.TempDir()
	record, req := emitFixtureRecord(t, dir, "DROP TABLE users")
	record.SchemaVersion = "3"
	recordPath := writeJSON(t, dir, "record.json", record)
	requestPath := writeJSON(t, dir, "request.json", req)

	if _, err := Run(Options{RecordPath: recordPath, RequestPath: requestPath, PolicyDir: dir}); err == nil {
		t.Fatal("replay must reject an unsupported schema_version")
	}
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
