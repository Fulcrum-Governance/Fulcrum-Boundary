package boundarycli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// FUL-772 conformance for the Secure GitHub MCP route, mirroring FUL-771's
// serve tests: an operator revocation written by `boundary revoke` stops the
// next tool call, and a policy directory the route cannot trust blocks.

type githubRecords struct {
	mu      sync.Mutex
	records []governance.DecisionRecordV1
}

func (s *githubRecords) WriteDecision(_ context.Context, r governance.DecisionRecordV1) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return nil
}

func (s *githubRecords) WriteForwardEvent(context.Context, securegithub.MCPForwardEvent) error {
	return nil
}

func (s *githubRecords) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (s *githubRecords) last() governance.DecisionRecordV1 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.records[len(s.records)-1]
}

type liveGitHubRoute struct {
	route     http.Handler
	policies  string
	pub       ed25519.PublicKey
	records   *githubRecords
	session   string
	mu        sync.Mutex
	toolCalls int
}

func newLiveGitHubRoute(t *testing.T) *liveGitHubRoute {
	t.Helper()
	l := &liveGitHubRoute{policies: t.TempDir(), records: &githubRecords{}}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		_ = json.Unmarshal(body, &msg)
		w.Header().Set("Content-Type", "application/json")
		switch msg.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "upstream-session")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"github-mcp","version":"test"}}}`))
		case "tools/call":
			l.mu.Lock()
			l.toolCalls++
			l.mu.Unlock()
			_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"content":[{"type":"text","text":"configured issue"}]}}`, msg.ID)
		default:
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":9,"result":{}}`))
		}
	}))
	t.Cleanup(upstream.Close)

	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	signer, err := governance.NewEd25519SignerFromSeed(seed, "")
	if err != nil {
		t.Fatal(err)
	}
	l.pub = ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if err := securegithub.WriteMCPRoutePolicy(l.policies); err != nil {
		t.Fatal(err)
	}
	route, err := securegithub.NewMCPRoute(securegithub.MCPRouteConfig{
		UpstreamURL: upstream.URL, Token: "test-token",
		SourceOwner: "public-org", SourceRepo: "untrusted-issues", SourceIssue: 17,
		TargetOwner: "private-org", TargetRepo: "protected-repo", TargetBranch: "main",
		AgentID: "agent-7", TenantID: "test", DecisionSink: l.records, ForwardSink: l.records,
		PolicyDir: l.policies, ReceiptSigner: signer,
	})
	if err != nil {
		t.Fatal(err)
	}
	l.route = route
	init := l.post(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-code","version":"test"}}}`)
	l.session = init.Header().Get("Mcp-Session-Id")
	if init.Code != http.StatusOK || l.session == "" {
		t.Fatalf("initialize: %d %s", init.Code, init.Body.String())
	}
	return l
}

func (l *liveGitHubRoute) post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if l.session != "" {
		req.Header.Set("Mcp-Session-Id", l.session)
	}
	w := httptest.NewRecorder()
	l.route.ServeHTTP(w, req)
	return w
}

func (l *liveGitHubRoute) upstreamToolCalls() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.toolCalls
}

func issueRead(id int) string {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{
		"name": securegithub.GitHubIssueReadTool, "arguments": map[string]any{"method": "get", "owner": "public-org", "repo": "untrusted-issues", "issue_number": 17},
	}})
	return string(body)
}

func (l *liveGitHubRoute) assertForwarded(t *testing.T, label, call string) {
	t.Helper()
	before := l.upstreamToolCalls()
	got := l.post(t, call)
	if !strings.Contains(got.Body.String(), "configured issue") || l.upstreamToolCalls() != before+1 {
		t.Fatalf("%s: not forwarded: %s", label, got.Body.String())
	}
}

// assertBlocked checks that call never reached GitHub and returns the record
// emitted for it, if any.
func (l *liveGitHubRoute) assertBlocked(t *testing.T, label, call string) *governance.DecisionRecordV1 {
	t.Helper()
	before, recordsBefore := l.upstreamToolCalls(), l.records.count()
	got := l.post(t, call)
	if strings.Contains(got.Body.String(), "configured issue") || l.upstreamToolCalls() != before {
		t.Fatalf("%s: reached upstream: %s", label, got.Body.String())
	}
	if !strings.Contains(got.Body.String(), `"error"`) {
		t.Fatalf("%s: no JSON-RPC error: %s", label, got.Body.String())
	}
	if l.records.count() == recordsBefore {
		return nil
	}
	r := l.records.last()
	return &r
}

func (l *liveGitHubRoute) assertRevokedDeny(t *testing.T, label, call string) {
	t.Helper()
	r := l.assertBlocked(t, label, call)
	if r == nil || r.MatchedRule != RevocationRuleName("agent-7") {
		t.Fatalf("%s: record %+v, want deny by %q", label, r, RevocationRuleName("agent-7"))
	}
	if c := r.ExecutionClaim; c == nil || c.Executed || c.UpstreamCalled {
		t.Fatalf("%s: execution claim %+v, want not executed", label, c)
	}
	if err := governance.VerifyDecisionRecord(*r, nil, l.policies, ""); err != nil {
		t.Fatalf("%s: record does not verify against the live bundle: %v", label, err)
	}
	if err := governance.VerifyReceiptSignature(*r, l.pub); err != nil {
		t.Fatalf("%s: signature: %v", label, err)
	}
}

func TestSecureGitHubNextActionTest_RevokeDeniesNextQueuedAndRetriedCalls(t *testing.T) {
	l := newLiveGitHubRoute(t)
	l.assertForwarded(t, "baseline", issueRead(10))
	queued := issueRead(11) // issued before the revoke, decided after

	if code := Run([]string{"revoke", "--agent", "agent-7", "--policies", l.policies, "--reason", "approval withdrawn"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("revoke exit %d", code)
	}
	l.assertRevokedDeny(t, "next call", issueRead(12))
	l.assertRevokedDeny(t, "queued call", queued)
	l.assertRevokedDeny(t, "retry of allowed call", issueRead(10))

	if code := Run([]string{"revoke", "--agent", "agent-7", "--policies", l.policies, "--lift"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("lift exit %d", code)
	}
	l.assertForwarded(t, "after lift", issueRead(13))
}

func TestSecureGitHubUntrustedPolicyDirBlocks(t *testing.T) {
	builtIn := filepath.Join("", securegithub.MCPRoutePolicyFile)
	cases := map[string]func(t *testing.T, dir string){
		"corrupt operator file": func(t *testing.T, dir string) {
			writeFile(t, filepath.Join(dir, "operator.yaml"), "rules: [")
		},
		"built-in rule removed": func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, builtIn), func(s string) string {
				i := strings.Index(s, "  - name: secure-github-read-before-write")
				if i < 0 {
					t.Fatal("read-before-write rule not found in seeded bundle")
				}
				return s[:i]
			})
		},
		"built-in rule loosened": func(t *testing.T, dir string) {
			editFile(t, filepath.Join(dir, builtIn), func(s string) string {
				return strings.Replace(s, `value: "true"`, `value: "never"`, 1)
			})
		},
		"built-in file deleted": func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, builtIn)); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(dir, "operator.yaml"), "name: op\nversion: v1\nrules: []\n")
		},
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			l := newLiveGitHubRoute(t)
			l.assertForwarded(t, "baseline", issueRead(10))
			break_(t, l.policies)
			if r := l.assertBlocked(t, "after "+name, issueRead(11)); r != nil && r.Action == "allow" {
				t.Fatalf("record says allow: %+v", r)
			}
		})
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func editFile(t *testing.T, path string, edit func(string) string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := edit(string(body))
	if changed == string(body) {
		t.Fatal("edit changed nothing")
	}
	writeFile(t, path, changed)
}
