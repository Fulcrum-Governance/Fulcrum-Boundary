package boundarycli

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// liveHook drives `boundary hook pretooluse` the way Claude Code does: one
// process per tool call, each reading the operator policy directory fresh.
type liveHook struct {
	policies, records, seed string
	pub                     ed25519.PublicKey
}

func newLiveHook(t *testing.T) *liveHook {
	t.Helper()
	h := &liveHook{policies: t.TempDir(), records: t.TempDir()}
	if err := os.WriteFile(filepath.Join(h.policies, "base.yaml"), []byte(allowAllPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{9}, ed25519.SeedSize)
	h.seed = filepath.Join(t.TempDir(), "receipt.seed")
	if err := os.WriteFile(h.seed, []byte(hex.EncodeToString(seed)), 0o600); err != nil {
		t.Fatal(err)
	}
	h.pub = ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	return h
}

func bashEvent(command string) string {
	body, _ := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"session_id":      "s-1",
		"tool_name":       "Bash",
		"tool_input":      map[string]string{"command": command},
	})
	return string(body)
}

// hookOutcome is what one hook invocation decided and persisted.
type hookOutcome struct {
	verdict string
	record  governance.DecisionRecordV1
}

func (h *liveHook) run(t *testing.T, event string, extra ...string) hookOutcome {
	t.Helper()
	args := append([]string{"--dir", h.records, "--policies", h.policies, "--receipt-seed", h.seed,
		"--agent-id", "agent-7", "--print-record"}, extra...)
	var out, errOut bytes.Buffer
	if code := runHookPreToolUse(args, strings.NewReader(event), &out, &errOut); code != 0 {
		t.Fatalf("hook exit %d: %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	var pointer struct {
		RecordPath string `json:"record_path"`
		Verdict    string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &pointer); err != nil || pointer.RecordPath == "" {
		t.Fatalf("no record pointer in %q (%v)", out.String(), err)
	}
	body, err := os.ReadFile(pointer.RecordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatal(err)
	}
	return hookOutcome{verdict: pointer.Verdict, record: record}
}

func (h *liveHook) assertRevokedDeny(t *testing.T, label, event string) {
	t.Helper()
	got := h.run(t, event)
	if got.verdict != "deny" || got.record.MatchedRule != RevocationRuleName("agent-7") {
		t.Fatalf("%s: verdict=%q matched_rule=%q, want deny by %q", label, got.verdict, got.record.MatchedRule, RevocationRuleName("agent-7"))
	}
	if c := got.record.ExecutionClaim; c == nil || c.Executed || c.UpstreamCalled {
		t.Fatalf("%s: execution claim %+v, want not executed", label, c)
	}
	if err := governance.VerifyDecisionRecord(got.record, nil, h.policies, ""); err != nil {
		t.Fatalf("%s: record does not verify against the live bundle: %v", label, err)
	}
	if err := governance.VerifyReceiptSignature(got.record, h.pub); err != nil {
		t.Fatalf("%s: signature: %v", label, err)
	}
}

func TestHookNextActionTest_RevokeDeniesNextQueuedAndRetriedCalls(t *testing.T) {
	h := newLiveHook(t)
	allowed := bashEvent("git status")
	if got := h.run(t, allowed); got.verdict != "allow" {
		t.Fatalf("baseline: verdict=%q, want allow", got.verdict)
	}
	queued := bashEvent("ls -la") // issued before the revoke, decided after

	if code := Run([]string{"revoke", "--agent", "agent-7", "--policies", h.policies, "--reason", "approval withdrawn"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("revoke exit %d", code)
	}
	h.assertRevokedDeny(t, "next call", bashEvent("go test ./..."))
	h.assertRevokedDeny(t, "queued call", queued)
	h.assertRevokedDeny(t, "retry of allowed call", allowed)
	h.assertRevokedDeny(t, "edit call", `{"hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"notes.md","content":"x"}}`)

	if code := Run([]string{"revoke", "--agent", "agent-7", "--policies", h.policies, "--lift"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("lift exit %d", code)
	}
	if got := h.run(t, allowed); got.verdict != "allow" {
		t.Fatalf("after lift: verdict=%q, want allow", got.verdict)
	}
}

func TestHookOperatorPolicyNeverLoosensCompiledPosture(t *testing.T) {
	h := newLiveHook(t) // the bundle allows every tool
	got := h.run(t, bashEvent("cp evil.json .claude/settings.json"))
	if got.verdict != "deny" || !strings.HasPrefix(got.record.MatchedRule, "command-control-surface:") {
		t.Fatalf("verdict=%q matched_rule=%q, want the compiled-in control-surface deny", got.verdict, got.record.MatchedRule)
	}
}

func TestHookUnloadableBundleDeniesRegardlessOfFailMode(t *testing.T) {
	h := newLiveHook(t)
	if err := os.WriteFile(filepath.Join(h.policies, "broken.yaml"), []byte("rules: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := h.run(t, bashEvent("git status"), "--failmode", "open")
	if got.verdict != "deny" || got.record.MatchedRule != "hook-operator-policy:unavailable" {
		t.Fatalf("verdict=%q matched_rule=%q, want deny by hook-operator-policy:unavailable", got.verdict, got.record.MatchedRule)
	}
}
