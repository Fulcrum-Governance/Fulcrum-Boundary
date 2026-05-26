package boundarycli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/boundary/governance"
)

func TestRun_HelpListsCommands(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	for _, want := range []string{"serve", "demo postgres", "verify", "doctor", "audit"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help output missing %q: %s", want, stdout.String())
		}
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
