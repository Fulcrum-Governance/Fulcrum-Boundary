package boundarycli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func TestRun_HelpListsCommands(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, want := range []string{"init", "inventory", "graph", "command", "policy generate", "serve", "demo postgres", "verify", "verify-record", "test", "doctor", "audit"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q: %s", want, stdout.String())
		}
	}
}

func TestRun_VersionFlagAliases(t *testing.T) {
	for _, alias := range []string{"--version", "-v"} {
		var stdout bytes.Buffer
		code := Run([]string{alias}, &stdout, &bytes.Buffer{})
		if code != 0 {
			t.Fatalf("%s: expected exit 0, got %d", alias, code)
		}
		if !strings.Contains(stdout.String(), "Fulcrum Boundary ") {
			t.Fatalf("%s: missing version output: %s", alias, stdout.String())
		}
	}
}

func TestRun_HelpTopicRouting(t *testing.T) {
	var stdout, helpErr bytes.Buffer
	code := Run([]string{"help", "version"}, &stdout, &helpErr)
	if code != 0 {
		t.Fatalf("help version: expected exit 0, got %d", code)
	}
	if combined := stdout.String() + helpErr.String(); !strings.Contains(combined, "Print Boundary version and build metadata.") {
		t.Fatalf("help version: missing rich help purpose: %s", combined)
	}

	stdout.Reset()
	code = Run([]string{"help"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("bare help: expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), `Use "boundary <command> --help"`) {
		t.Fatalf("bare help: expected root help: %s", stdout.String())
	}

	stdout.Reset()
	helpErr.Reset()
	code = Run([]string{"help", "demo", "postgres"}, &stdout, &helpErr)
	if code != 0 {
		t.Fatalf("help demo postgres: expected exit 0, got %d", code)
	}
	if combined := stdout.String() + helpErr.String(); !strings.Contains(combined, "Run the Postgres allow, deny, and bypass demo") {
		t.Fatalf("compound help topic must reach the leaf command's help: %s", combined)
	}

	var stderr bytes.Buffer
	code = Run([]string{"help", "no-such-command"}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("help with unknown topic: expected non-zero exit")
	}
}

func TestRun_BareCommandHelpBackfill(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"init", "--help"}, "Inventory the MCP configs"},
		{[]string{"lock", "--help"}, "descriptor lockfile"},
		{[]string{"verify-lock", "--help"}, "report drift"},
		{[]string{"redteam", "--help"}, "synthetic red-team fixture packs"},
		{[]string{"serve", "--help"}, "governs routed tools"},
		{[]string{"verify", "--help"}, "Validate YAML policy files"},
		{[]string{"verify-record", "--help"}, "record.json is required"},
		{[]string{"audit", "--help"}, "Pretty-print structured decision records"},
		{[]string{"trust", "--help"}, "trust state Boundary consults"},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		code := Run(tc.args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v: expected exit 0, got %d", tc.args, code)
		}
		combined := stdout.String() + stderr.String()
		if !strings.Contains(combined, tc.want) {
			t.Fatalf("%v: help missing %q:\n%s", tc.args, tc.want, combined)
		}
		if !strings.Contains(combined, "Usage:") {
			t.Fatalf("%v: help missing Usage section:\n%s", tc.args, combined)
		}
	}
}

func TestRun_VerifyJSONOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)

	var stdout bytes.Buffer
	code := Run([]string{"verify", "--policies", dir, "--json"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
	var payload struct {
		SchemaVersion string   `json:"schema_version"`
		OK            bool     `json:"ok"`
		Error         string   `json:"error"`
		PolicyFiles   int      `json:"policy_files"`
		Rules         int      `json:"rules"`
		Warnings      []string `json:"warnings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("verify --json did not parse: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.verify.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if !payload.OK || payload.PolicyFiles != 1 || payload.Rules != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Warnings == nil {
		t.Fatalf("warnings must encode as an array, not null: %s", stdout.String())
	}

	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "broken.yaml"), []byte(":\tnot yaml"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	code = Run([]string{"verify", "--policies", empty, "--json"}, &stdout, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("expected parse failure to exit non-zero")
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failure JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload.OK || payload.Error == "" {
		t.Fatalf("failure payload must set ok=false with error: %+v", payload)
	}
}

func TestRun_VerifyRecordJSONOutput(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)
	requestBody := []byte(`{"agent_id":"agent-1","arguments":{"sql":"SELECT 1"},"tenant_id":"tenant-1","tool_name":"query"}`)
	requestHash, err := governance.ComputeRawRequestHash(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	policyHash, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:           governance.TransportMCP,
		ToolName:            "query",
		Action:              "allow",
		PolicyBundleHash:    policyHash,
		RequestHash:         requestHash,
		BoundaryBuildDigest: "sha256:test-build",
		TrustScore:          1,
		TrustState:          governance.TrustStateTrusted.String(),
	})
	recordPath := filepath.Join(dir, "record.json")
	writeRecordFile(t, recordPath, record)

	var stdout bytes.Buffer
	code := Run([]string{"verify-record", "--json", recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stdout.String())
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Error         string `json:"error"`
		RecordID      string `json:"record_id"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("verify-record --json did not parse: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.verify_record.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if !payload.OK || payload.RecordID == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	record.Action = "deny"
	writeRecordFile(t, recordPath, record)
	stdout.Reset()
	code = Run([]string{"verify-record", "--json", recordPath}, &stdout, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("expected tampered record to exit non-zero")
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("failure JSON did not parse: %v\n%s", err, stdout.String())
	}
	if payload.OK || !strings.Contains(payload.Error, "decision_hash") {
		t.Fatalf("failure payload must set ok=false with decision_hash error: %+v", payload)
	}
}

func writeTestPolicy(t *testing.T, dir string) {
	t.Helper()
	policy := []byte(`name: test-policy
version: "1.0"
rules:
  - name: block-drop-table
    tool: query
    action: deny
    reason: blocked
    match:
      field: arguments.sql
      contains: DROP TABLE
`)
	if err := os.WriteFile(filepath.Join(dir, "postgres.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRun_SubcommandHelpExitsZero(t *testing.T) {
	code := Run([]string{"serve", "--help"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
}

func TestRun_VerifyPolicyDirectory(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)

	var stdout bytes.Buffer
	code := Run([]string{"verify", "--policies", dir}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "policy files: 1") {
		t.Fatalf("verify output missing file count: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "rules: 1") {
		t.Fatalf("verify output missing rule count: %s", stdout.String())
	}
}

func TestRun_VerifyPolicyDirectoryRejectsInvalidV1(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`schema_version: "1"
policy:
  name: broken
  version: "1.0.0"
  rules:
    - name: invalid
      tool: query
      action: deny
      conditions:
        - type: regex
          field: arguments.sql
          regex: "["
`)
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := Run([]string{"verify", "--policies", dir}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("expected invalid v1 policy to fail verification")
	}
	if !strings.Contains(stderr.String(), "invalid regex") {
		t.Fatalf("expected schema error, got %s", stderr.String())
	}
}

func TestRun_VerifyRecordAcceptsValidAndRejectsTampered(t *testing.T) {
	dir := t.TempDir()
	writeTestPolicy(t, dir)
	requestBody := []byte(`{"agent_id":"agent-1","arguments":{"sql":"SELECT 1"},"tenant_id":"tenant-1","tool_name":"query"}`)
	requestHash, err := governance.ComputeRawRequestHash(requestBody)
	if err != nil {
		t.Fatal(err)
	}
	policyHash, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport:           governance.TransportMCP,
		ToolName:            "query",
		Action:              "allow",
		PolicyBundleHash:    policyHash,
		RequestHash:         requestHash,
		BoundaryBuildDigest: "sha256:test-build",
		TrustScore:          1,
		TrustState:          governance.TrustStateTrusted.String(),
	})

	requestPath := filepath.Join(dir, "request.json")
	recordPath := filepath.Join(dir, "record.json")
	writeJSONFile(t, requestPath, requestBody)
	writeRecordFile(t, recordPath, record)

	var stdout bytes.Buffer
	code := Run([]string{"verify-record", "--request", requestPath, "--policies", dir, "--binary-digest", "sha256:test-build", recordPath}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected valid record to verify")
	}
	if !strings.Contains(stdout.String(), "record verification: ok") {
		t.Fatalf("missing success output: %s", stdout.String())
	}

	record.Action = "deny"
	writeRecordFile(t, recordPath, record)
	var stderr bytes.Buffer
	code = Run([]string{"verify-record", "--request", requestPath, "--policies", dir, "--binary-digest", "sha256:test-build", recordPath}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("expected tampered record to fail verification")
	}
	if !strings.Contains(stderr.String(), "decision_hash mismatch") {
		t.Fatalf("expected decision hash failure, got %s", stderr.String())
	}
}

func TestGatewayMiddleware_AllowsSelectAndBlocksDrop(t *testing.T) {
	rules := []governance.StaticPolicyRule{
		{
			Name:   "block-drop-table",
			Tool:   "query",
			Action: "deny",
			Reason: "blocked",
			Match: &governance.StaticPolicyMatch{
				Field:           "arguments.sql",
				Contains:        "DROP TABLE",
				CaseInsensitive: true,
			},
			PolicyFile: "postgres.yaml",
		},
	}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: rules,
		GatewayVersion: "test-version",
	}, nil, nil, nil)

	var downstreamCalls int
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downstreamCalls++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	middleware := governance.NewMiddleware(pipeline, downstream, governance.MiddlewareConfig{
		TransportType:  governance.TransportMCP,
		RequestBuilder: buildPostgresGovernanceRequest,
	})

	selectReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"tool_name":"query","arguments":{"sql":"SELECT * FROM users"}}`))
	selectRec := httptest.NewRecorder()
	middleware.ServeHTTP(selectRec, selectReq)
	if selectRec.Code != http.StatusOK {
		t.Fatalf("expected SELECT to pass, got %d body=%s", selectRec.Code, selectRec.Body.String())
	}

	dropReq := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"tool_name":"query","arguments":{"sql":"DROP TABLE users"}}`))
	dropRec := httptest.NewRecorder()
	middleware.ServeHTTP(dropRec, dropReq)
	if dropRec.Code != http.StatusForbidden {
		t.Fatalf("expected DROP TABLE to be blocked, got %d body=%s", dropRec.Code, dropRec.Body.String())
	}
	if !strings.Contains(dropRec.Body.String(), "block-drop-table") {
		t.Fatalf("deny body missing matched rule: %s", dropRec.Body.String())
	}
	if downstreamCalls != 1 {
		t.Fatalf("expected downstream to be called once, got %d", downstreamCalls)
	}
}

func writeJSONFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeRecordFile(t *testing.T, path string, record governance.DecisionRecordV1) {
	t.Helper()
	body, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, path, body)
}
