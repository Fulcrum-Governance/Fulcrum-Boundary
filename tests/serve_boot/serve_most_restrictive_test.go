package serve_boot_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type rankedServe struct {
	url      string
	upstream *atomic.Int64
	stdout   *syncBuffer
}

// startRankedServe runs the real `boundary serve` over policy files written
// to a temp dir (name -> YAML) in front of a stub upstream that counts calls.
func startRankedServe(t *testing.T, bin string, files map[string]string) rankedServe {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": map[string]any{"content": "UPSTREAM_OK"}})
	}))
	t.Cleanup(upstream.Close)

	addr := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin, "serve", "--listen", addr, "--policies", dir, "--upstream", upstream.URL)
	var stdout, stderr syncBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = cmd.Wait() })
	if err := pollTCP(addr, time.Now().Add(10*time.Second)); err != nil {
		t.Fatalf("%v; stderr=%s", err, stderr.String())
	}
	return rankedServe{url: "http://" + addr, upstream: &calls, stdout: &stdout}
}

type governanceMeta struct {
	Action       string   `json:"action"`
	Reason       string   `json:"reason"`
	MatchedRule  string   `json:"matched_rule"`
	MatchedRules []string `json:"matched_rules"`
}

func callTool(t *testing.T, s rankedServe, tool, argName, argValue string) governanceMeta {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": map[string]any{argName: argValue}}})
	var resp struct {
		Result struct {
			Meta struct {
				Governance governanceMeta `json:"governance"`
			} `json:"_meta"`
		} `json:"result"`
		Error struct {
			Data struct {
				Governance governanceMeta `json:"governance"`
			} `json:"data"`
		} `json:"error"`
	}
	raw := postMCP(t, s.url, string(body))
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("response %s: %v", raw, err)
	}
	if resp.Result.Meta.Governance.Action != "" {
		return resp.Result.Meta.Governance
	}
	return resp.Error.Data.Governance
}

// auditActions returns action and matched_rules of each governance_decision
// line serve wrote to stdout.
func auditActions(t *testing.T, s rankedServe) []governanceMeta {
	t.Helper()
	var out []governanceMeta
	sc := bufio.NewScanner(strings.NewReader(s.stdout.String()))
	for sc.Scan() {
		var line struct {
			Msg string `json:"msg"`
			governanceMeta
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.Msg == "governance_decision" {
			out = append(out, line.governanceMeta)
		}
	}
	return out
}

const (
	warnEchoPolicy = `name: warn-echo
rules:
  - name: warn-echo-secret
    tool: echo
    action: warn
    reason: review secret-looking echo
    match: {type: contains, field: arguments.text, contains: secret}
`
	denyEchoPolicy = `name: deny-echo
rules:
  - name: deny-echo-secret
    tool: echo
    action: deny
    reason: secret-looking echo is blocked
    match: {type: contains, field: arguments.text, contains: secret}
`
	warnQueryDropPolicy = `name: warn-query
rules:
  - name: warn-query-drop
    tool: query
    action: warn
    reason: DROP should be reviewed
    match: {type: contains, field: arguments.sql, contains: DROP}
`
)

func TestServe_DenyFileBeatsWarnFileThatSortsFirst(t *testing.T) {
	s := startRankedServe(t, buildBoundary(t), map[string]string{"10-warn.yaml": warnEchoPolicy, "20-deny.yaml": denyEchoPolicy})

	got := callTool(t, s, "echo", "text", "my secret token")

	want := governanceMeta{Action: "deny", Reason: "secret-looking echo is blocked", MatchedRule: "deny-echo-secret", MatchedRules: []string{"warn-echo-secret", "deny-echo-secret"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("governance = %+v, want %+v", got, want)
	}
	if n := s.upstream.Load(); n != 0 {
		t.Fatalf("upstream calls = %d, want 0", n)
	}
	if a := auditActions(t, s); len(a) != 1 || !reflect.DeepEqual(a[0].MatchedRules, want.MatchedRules) || a[0].Action != "deny" {
		t.Fatalf("audit lines = %+v, want one deny listing %v", a, want.MatchedRules)
	}
}

func TestServe_WarnRuleDoesNotSkipTheBuiltInDropGuard(t *testing.T) {
	s := startRankedServe(t, buildBoundary(t), map[string]string{"warn.yaml": warnQueryDropPolicy})

	got := callTool(t, s, "query", "sql", "DROP TABLE users")

	if got.Action != "deny" || !strings.HasPrefix(got.Reason, "Postgres AST guard denied") || got.MatchedRule != "" || !reflect.DeepEqual(got.MatchedRules, []string{"warn-query-drop"}) {
		t.Fatalf("governance = %+v, want deny by the Postgres guard with matched_rules [warn-query-drop]", got)
	}
	if n := s.upstream.Load(); n != 0 {
		t.Fatalf("upstream calls = %d, want 0", n)
	}
}

func TestServe_WarnAloneExecutesAndIsRecorded(t *testing.T) {
	s := startRankedServe(t, buildBoundary(t), map[string]string{"warn.yaml": warnEchoPolicy})

	got := callTool(t, s, "echo", "text", "my secret token")

	want := governanceMeta{Action: "warn", Reason: "review secret-looking echo", MatchedRule: "warn-echo-secret", MatchedRules: []string{"warn-echo-secret"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("governance = %+v, want %+v", got, want)
	}
	if n := s.upstream.Load(); n != 1 {
		t.Fatalf("upstream calls = %d, want 1", n)
	}
	if a := auditActions(t, s); len(a) != 1 || a[0].Action != "warn" || a[0].MatchedRule != "warn-echo-secret" {
		t.Fatalf("audit lines = %+v, want one warn by warn-echo-secret", a)
	}
}
