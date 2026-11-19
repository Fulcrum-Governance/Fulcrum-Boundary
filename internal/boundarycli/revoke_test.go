package boundarycli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// FUL-771 Next Action Test, on the `boundary serve` MCP route: an agent is
// allowed, gets revoked mid-task, and its next routed action, a request it had
// queued before the revoke, and a retry of an earlier allowed call are all
// denied without reaching the upstream, each with a signed record that names
// the revocation and the bundle that decided. No restart happens in between.

const allowAllPolicy = `name: base
version: v1
rules:
  - name: allow-tools
    tool: "*"
    action: allow
`

type eventLog struct {
	mu     sync.Mutex
	events []governance.AuditEvent
}

func (l *eventLog) Publish(_ context.Context, e governance.AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, e)
}

func (l *eventLog) last(t *testing.T) governance.AuditEvent {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.events) - 1; i >= 0; i-- {
		if l.events[i].EventType == "" || l.events[i].EventType == "governance_decision" {
			return l.events[i]
		}
	}
	t.Fatal("no decision event recorded")
	return governance.AuditEvent{}
}

type liveServe struct {
	dir      string
	handler  http.Handler
	upstream *atomic.Int64
	events   *eventLog
	pub      ed25519.PublicKey
}

func startLiveServe(t *testing.T) *liveServe {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(allowAllPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := governance.LoadStaticPolicyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := governance.PolicyBundleHashFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := governance.NewEd25519SignerFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize), "")
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`)
	}))
	t.Cleanup(upstream.Close)
	events := &eventLog{}
	pipeline := newServePipeline(governance.PipelineConfig{
		StaticPolicies:   loaded.Rules,
		PolicyBundleHash: hash,
		PolicyDir:        dir,
		ReceiptSigner:    signer,
	}, nil, events)
	handler, _, _, err := serveHandler(upstream.URL, pipeline)
	if err != nil {
		t.Fatal(err)
	}
	return &liveServe{dir: dir, handler: handler, upstream: &calls, events: events, pub: signer.PrivateKey.Public().(ed25519.PublicKey)}
}

func toolCall(agent, id string) *http.Request {
	body := `{"jsonrpc":"2.0","id":` + id + `,"method":"tools/call","params":{"name":"create_issue","arguments":{"title":"t"}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(governance.HeaderGovernanceAgentID, agent)
	return req
}

func (s *liveServe) send(req *http.Request) {
	s.handler.ServeHTTP(httptest.NewRecorder(), req)
}

func (s *liveServe) revoke(t *testing.T, args ...string) {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := Run(append([]string{"revoke", "--policies", s.dir}, args...), &out, &errOut); code != 0 {
		t.Fatalf("revoke %v exit %d: %s", args, code, errOut.String())
	}
}

// assertRevokedDeny checks one routed action after revocation: denied, never
// forwarded, and its record names the revocation rule, verifies against the
// live policy directory, and carries a valid signature.
func (s *liveServe) assertRevokedDeny(t *testing.T, label string, req *http.Request) {
	t.Helper()
	before := s.upstream.Load()
	s.send(req)
	if got := s.upstream.Load(); got != before {
		t.Fatalf("%s: upstream called after revoke (calls %d -> %d)", label, before, got)
	}
	e := s.events.last(t)
	if e.Action != "deny" || e.MatchedRule != RevocationRuleName("agent-7") {
		t.Fatalf("%s: action=%q matched_rule=%q, want deny by %q", label, e.Action, e.MatchedRule, RevocationRuleName("agent-7"))
	}
	if e.ExecutionClaim != nil && e.ExecutionClaim.Executed {
		t.Fatalf("%s: record claims executed=true", label)
	}
	record := governance.BuildDecisionRecord(e)
	if err := governance.VerifyDecisionRecord(record, nil, s.dir, ""); err != nil {
		t.Fatalf("%s: record does not verify against the live bundle: %v", label, err)
	}
	if err := governance.VerifyReceiptSignature(record, s.pub); err != nil {
		t.Fatalf("%s: signature: %v", label, err)
	}
}

func TestNextActionTest_RevokeDeniesNextQueuedAndRetriedActions(t *testing.T) {
	s := startLiveServe(t)

	first := toolCall("agent-7", "1")
	s.send(first)
	if s.upstream.Load() != 1 || s.events.last(t).Action != "allow" {
		t.Fatalf("baseline: want one allowed, forwarded call; upstream=%d action=%q", s.upstream.Load(), s.events.last(t).Action)
	}
	queued := toolCall("agent-7", "2") // built before the revoke, sent after

	s.revoke(t, "--agent", "agent-7", "--reason", "approval withdrawn")

	s.assertRevokedDeny(t, "next action", toolCall("agent-7", "3"))
	s.assertRevokedDeny(t, "queued action", queued)
	s.assertRevokedDeny(t, "retry of allowed call", toolCall("agent-7", "1"))

	// Revocation is scoped to the named agent.
	s.send(toolCall("agent-8", "4"))
	if s.events.last(t).Action != "allow" {
		t.Fatalf("other agent: action=%q, want allow", s.events.last(t).Action)
	}

	s.revoke(t, "--agent", "agent-7", "--lift")
	s.send(toolCall("agent-7", "5"))
	if s.events.last(t).Action != "allow" {
		t.Fatalf("after lift: action=%q, want allow", s.events.last(t).Action)
	}
}

func TestNextActionTest_UnparseableReloadBlocks(t *testing.T) {
	s := startLiveServe(t)
	if err := os.WriteFile(filepath.Join(s.dir, "broken.yaml"), []byte("rules: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.send(toolCall("agent-7", "1"))
	if s.upstream.Load() != 0 {
		t.Fatal("a failed reload must not forward on the previous rules")
	}
	if got := s.events.last(t).Action; got != governance.ActionCheckIndeterminate {
		t.Fatalf("action=%q, want %q", got, governance.ActionCheckIndeterminate)
	}
}

func TestRevokeRejectsPathLikeAgentIDs(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run([]string{"revoke", "--policies", dir, "--agent", "../x"}, &out, &errOut); code == 0 {
		t.Fatal("agent id with a path separator must be rejected")
	}
}
