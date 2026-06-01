package explain_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

const (
	v1Example = "../../docs/examples/decision-record.example.json"
	v2Example = "../../docs/examples/decision-record-v2.example.json"
)

func runExplain(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := boundarycli.Run(append([]string{"explain"}, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

// TestExplainV1TextRendersDecisionFields proves explain on the committed V1
// example renders the decision-defining fields and the limitation footer, and
// stays local-only.
func TestExplainV1TextRendersDecisionFields(t *testing.T) {
	stdout, stderr, code := runExplain(t, v1Example)
	if code != 0 {
		t.Fatalf("explain v1 exit = %d stderr=%s", code, stderr)
	}
	for _, want := range []string{
		"Boundary explain",
		"status: ok",
		"record schema_version: 1",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"action: deny",
		"matched_rule: deny-github-write-after-taint-fixture",
		"decision_hash:",
		"What this does not prove:",
		"does not verify the record's hashes",
		"does not prove enforcement",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("explain v1 output missing %q:\n%s", want, stdout)
		}
	}
	// The committed V1 example has no route-context.
	if strings.Contains(stdout, "Route context") {
		t.Fatalf("V1 example must not render route context:\n%s", stdout)
	}
}

// TestExplainV1JSONEnvelopeIsStable is the load-bearing schema assertion: the
// --json envelope identifies as boundary.explain.v1 and carries the three
// local-only truth flags set false.
func TestExplainV1JSONEnvelopeIsStable(t *testing.T) {
	stdout, stderr, code := runExplain(t, "--json", v1Example)
	if code != 0 {
		t.Fatalf("explain v1 json exit = %d stderr=%s", code, stderr)
	}
	var payload struct {
		SchemaVersion       string `json:"schema_version"`
		Status              string `json:"status"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresNetwork     bool   `json:"requires_network"`
		MutatesLiveSystems  bool   `json:"mutates_live_systems"`
		RecordSchemaVersion string `json:"record_schema_version"`
		Decision            struct {
			Action      string `json:"action"`
			MatchedRule string `json:"matched_rule"`
		} `json:"decision"`
		RouteContext *json.RawMessage `json:"route_context"`
		Hashes       []struct {
			Field   string `json:"field"`
			Present bool   `json:"present"`
		} `json:"hashes"`
		DoesNotProve []string `json:"does_not_prove"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse explain json: %v\n%s", err, stdout)
	}
	if payload.SchemaVersion != "boundary.explain.v1" || payload.Status != "ok" {
		t.Fatalf("unexpected explain identity: %#v", payload)
	}
	if payload.RecordSchemaVersion != "1" {
		t.Fatalf("record_schema_version = %q, want 1", payload.RecordSchemaVersion)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("explain must not require credentials, network, or live mutation: %#v", payload)
	}
	if payload.RouteContext != nil {
		t.Fatalf("V1 example must not carry route_context: %s", string(*payload.RouteContext))
	}
	if payload.Decision.Action != "deny" {
		t.Fatalf("decision action = %q, want deny", payload.Decision.Action)
	}
	if len(payload.DoesNotProve) == 0 {
		t.Fatalf("does_not_prove footer must be present: %#v", payload)
	}
}

// TestExplainV2RendersRouteContextWithCaveats proves the committed V2 example's
// route-context fields render with the asserted-not-attested and
// self-report-not-corroborated caveats, in both text and JSON.
func TestExplainV2RendersRouteContextWithCaveats(t *testing.T) {
	stdout, stderr, code := runExplain(t, v2Example)
	if code != 0 {
		t.Fatalf("explain v2 exit = %d stderr=%s", code, stderr)
	}
	for _, want := range []string{
		"record schema_version: 2",
		"Route context (schema_version 2, descriptive only):",
		"adapter_id: securegithub",
		"route_id: mcp:github.create_or_update_file",
		"topology_profile (asserted, not attested): single-tenant-routed",
		"execution_claim (self-report, not corroborated): upstream_called=false executed=false source=securegithub",
		"asserted, not attested",
		"self-report, not corroborated",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("explain v2 output missing %q:\n%s", want, stdout)
		}
	}

	jsonOut, stderr, code := runExplain(t, "--json", v2Example)
	if code != 0 {
		t.Fatalf("explain v2 json exit = %d stderr=%s", code, stderr)
	}
	var payload struct {
		RecordSchemaVersion string `json:"record_schema_version"`
		RouteContext        struct {
			AdapterID       string `json:"adapter_id"`
			RouteID         string `json:"route_id"`
			TopologyProfile string `json:"topology_profile"`
			ExecutionClaim  struct {
				UpstreamCalled bool   `json:"upstream_called"`
				Executed       bool   `json:"executed"`
				Source         string `json:"source"`
			} `json:"execution_claim"`
		} `json:"route_context"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
		t.Fatalf("parse explain v2 json: %v\n%s", err, jsonOut)
	}
	if payload.RecordSchemaVersion != "2" {
		t.Fatalf("record_schema_version = %q, want 2", payload.RecordSchemaVersion)
	}
	rc := payload.RouteContext
	if rc.AdapterID != "securegithub" || rc.TopologyProfile != "single-tenant-routed" {
		t.Fatalf("route_context not rendered: %#v", rc)
	}
	if rc.ExecutionClaim.UpstreamCalled || rc.ExecutionClaim.Executed || rc.ExecutionClaim.Source != "securegithub" {
		t.Fatalf("execution_claim not rendered: %#v", rc.ExecutionClaim)
	}
}

// TestExplainDoesNotVerifyHashes proves explain renders a record with an
// intentionally corrupted decision_hash without failing — verification is
// verify-record's job, not explain's.
func TestExplainDoesNotVerifyHashes(t *testing.T) {
	body, err := os.ReadFile(v1Example)
	if err != nil {
		t.Fatalf("read v1 example: %v", err)
	}
	var record map[string]any
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatalf("parse v1 example: %v", err)
	}
	record["decision_hash"] = "sha256:deadbeefdeadbeef"
	tampered, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "tampered.json")
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runExplain(t, path)
	if code != 0 {
		t.Fatalf("explain must render a tampered record (it does not verify): code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "sha256:deadbeefdeadbeef") {
		t.Fatalf("explain must surface the stored hash verbatim:\n%s", stdout)
	}
}

// TestExplainRejectsMissingFile proves a bad path exits non-zero with a stderr
// message and nothing on stdout.
func TestExplainRejectsMissingFile(t *testing.T) {
	stdout, stderr, code := runExplain(t, filepath.Join(t.TempDir(), "missing.json"))
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing file, stdout=%s", stdout)
	}
	if !strings.Contains(stderr, "explain:") {
		t.Fatalf("stderr missing explain error: %s", stderr)
	}
}

// TestExplainRejectsWrongArity proves explain requires exactly one positional
// record path.
func TestExplainRejectsWrongArity(t *testing.T) {
	stdout, stderr, code := runExplain(t)
	if code == 0 {
		t.Fatalf("expected usage failure with no record, stdout=%s", stdout)
	}
	if !strings.Contains(stderr, "usage: boundary explain") {
		t.Fatalf("stderr missing usage line: %s", stderr)
	}
}

// TestExplainHelp proves the --help block keeps the local-only, read-only, and
// does-not-verify framing that the language gate and help-language test expect.
func TestExplainHelp(t *testing.T) {
	stdout, stderr, code := runExplain(t, "--help")
	if code != 0 {
		t.Fatalf("explain help exit = %d stderr=%s", code, stderr)
	}
	output := stdout + stderr
	for _, want := range []string{
		"Describe a decision record",
		"boundary explain --json",
		"local-only and read-only",
		"does not verify the record's hashes",
		"does not prove enforcement",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("explain help missing %q:\n%s", want, output)
		}
	}
}
