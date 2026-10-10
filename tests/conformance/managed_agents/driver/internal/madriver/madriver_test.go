package madriver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
	maconf "github.com/fulcrum-governance/fulcrum-boundary/tests/conformance/managed_agents"
)

func testConfig(t *testing.T) Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.OutDir = t.TempDir()
	return cfg
}

func TestRedactorRemovesSecretShapes(t *testing.T) {
	cases := []string{
		"key is sk-ant-api03-FAKEFAKEFAKEFAKEFAKEFAKE here",
		"token sk-abcdefghijklmnopqrstuvwxyz0123456789 embedded",
		"Authorization: Bearer abcdef0123456789token",
		"contact me at operator@example.com",
		`{"session_secret": "abc123secretvalue"}`,
		`api_key="xyz123456789"`,
	}
	for _, in := range cases {
		out := RedactString(in)
		if !strings.Contains(out, redactedPlaceholder) {
			t.Fatalf("redactor left %q untouched", in)
		}
		if LooksSecret(out) {
			t.Fatalf("redacted output still looks secret: %q", out)
		}
	}
	if RedactString("plain tool output") != "plain tool output" {
		t.Fatal("redactor mangled clean text")
	}
}

func TestSpendGuardStopsAtEightyPercent(t *testing.T) {
	g := newSpendGuard(1.00, 1024, 8, time.Minute)
	if err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "79"}}}); err != nil {
		t.Fatalf("79 cents must not abort: %v", err)
	}
	err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "80"}}})
	if !errors.Is(err, ErrSpendAbort) {
		t.Fatalf("80 cents on a $1 ceiling must abort, got %v", err)
	}
}

func TestSpendGuardFailsClosedOnMissingUsage(t *testing.T) {
	g := newSpendGuard(5.00, 1024, 8, time.Minute)
	if err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{}}}); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("session.usage without list_cost must be unknown spend, got %v", err)
	}
	if err := g.observe(managedagents.Event{Type: "span.model_request_end", Data: map[string]any{}}); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("span without model_usage must be unknown spend, got %v", err)
	}
}

// TestSpendGuardFailsClosedWhenBlind covers the no-usage-signal-at-all case:
// after --usage-blind-events consecutive events without a usable usage field,
// the guard stops the session rather than letting spend drift unobserved.
func TestSpendGuardFailsClosedWhenBlind(t *testing.T) {
	g := newSpendGuard(5.00, 1024, 4, time.Minute)
	nonUsage := managedagents.Event{Type: "session.status_running"}
	for i := 0; i < 3; i++ {
		if err := g.observe(nonUsage); err != nil {
			t.Fatalf("event %d of 3 must not trip the blind guard: %v", i+1, err)
		}
	}
	if err := g.observe(nonUsage); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("4 consecutive usage-free events must fail closed, got %v", err)
	}
	if g.observedUsage() {
		t.Fatal("no usage signal may be recorded as observed")
	}
}

// TestSpendGuardFailsClosedOnBlindWindow is the time-based variant: even with
// few events, a stream that goes too long without usage fails closed.
func TestSpendGuardFailsClosedOnBlindWindow(t *testing.T) {
	start := time.Now()
	now := start
	g := newSpendGuard(5.00, 1024, 100, 30*time.Second)
	g.now = func() time.Time { return now }
	nonUsage := managedagents.Event{Type: "session.status_running"}
	if err := g.observe(nonUsage); err != nil {
		t.Fatalf("first event must not trip: %v", err)
	}
	now = start.Add(31 * time.Second)
	if err := g.observe(nonUsage); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("31s without a usage signal must fail closed, got %v", err)
	}
}

// TestSpendGuardListCostKeepsMax verifies cumulative list_cost semantics: a
// later session.usage replaces rather than adds, and a lower figure must not
// decrease the spend estimate.
func TestSpendGuardListCostKeepsMax(t *testing.T) {
	g := newSpendGuard(5.00, 1024, 8, time.Minute)
	for _, cents := range []string{"30", "50", "40"} {
		err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": cents}}})
		if err != nil {
			t.Fatalf("list_cost %s: %v", cents, err)
		}
	}
	if g.used() != 0.50 {
		t.Fatalf("guard must keep the maximum observed list_cost, got %.2f", g.used())
	}
}

// TestSpendGuardAbortsOverOutputCap enforces --max-output-tokens: the driver
// cannot preempt generation, so the session aborts the moment an over-cap
// request is reported.
func TestSpendGuardAbortsOverOutputCap(t *testing.T) {
	g := newSpendGuard(5.00, 100, 8, time.Minute)
	err := g.observe(managedagents.Event{
		Type: "span.model_request_end",
		Data: map[string]any{"model_usage": map[string]any{
			"model": "claude-sonnet-4-5-20250929", "input_tokens": 1000, "output_tokens": 250,
		}},
	})
	if !errors.Is(err, ErrOutputTokenCap) {
		t.Fatalf("250 output tokens over a cap of 100 must abort, got %v", err)
	}
}

func TestSpendGuardPricesTokens(t *testing.T) {
	g := newSpendGuard(5.00, 1024, 8, time.Minute)
	err := g.observe(managedagents.Event{
		Type: "span.model_request_end",
		Data: map[string]any{"model_usage": map[string]any{
			"model": "claude-sonnet-4-5-20250929", "input_tokens": 1000, "output_tokens": 250,
		}},
	})
	if err != nil {
		t.Fatalf("priced span must not error: %v", err)
	}
	want := (1000*3.00 + 250*15.00) / 1_000_000
	if g.used() != want {
		t.Fatalf("token spend = %v, want %v", g.used(), want)
	}
	if !g.observedUsage() {
		t.Fatal("a priced span is observed usage evidence")
	}
	if err := g.observe(managedagents.Event{
		Type: "span.model_request_end",
		Data: map[string]any{"model_usage": map[string]any{
			"model": "unknown-model", "input_tokens": 1, "output_tokens": 1,
		}},
	}); !errors.Is(err, ErrSpendUnknown) {
		t.Fatal("unknown model must be unknown spend")
	}
}

func TestLiveModeGates(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	present := func(string) bool { return true }
	env := func(string) string { return "fake-test-key-not-real" }

	cfg := testConfig(t)
	cfg.Mode = ModeLive
	cfg.AckSpend = true
	cfg.AgentID = "agt_x"
	cfg.EnvironmentID = "env_x"

	if err := cfg.validate(repoRoot, env, present); err != nil {
		t.Fatalf("all gates satisfied must validate, got %v", err)
	}

	cases := []struct {
		name  string
		mut   func(*Config)
		env   func(string) string
		files func(string) bool
	}{
		{"missing ack flag", func(c *Config) { c.AckSpend = false }, env, present},
		{"missing gate file", nil, env, func(string) bool { return false }},
		{"missing api key env", nil, func(string) string { return "" }, present},
		{"empty api key env", nil, func(string) string { return "  " }, present},
		{"missing agent id", func(c *Config) { c.AgentID = "" }, env, present},
		{"missing environment id", func(c *Config) { c.EnvironmentID = "" }, env, present},
	}
	for _, tc := range cases {
		c := cfg
		c.AckSpend = true
		if tc.mut != nil {
			tc.mut(&c)
		}
		if err := c.validate(repoRoot, tc.env, tc.files); err == nil {
			t.Fatalf("%s: live mode must refuse", tc.name)
		}
	}
}

func TestOutDirInsideRepoRejected(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := outDirOutsideRepo(repoRoot, repoRoot); err == nil {
		t.Fatal("repo root as out-dir must be rejected")
	}
	if err := outDirOutsideRepo(filepath.Join(repoRoot, "tests", "conformance"), repoRoot); err == nil {
		t.Fatal("subdir of repo must be rejected")
	}
	if err := outDirOutsideRepo(t.TempDir(), repoRoot); err != nil {
		t.Fatalf("tempdir outside repo must be accepted: %v", err)
	}
}

func TestSecretScanDetectsPlantedKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "clean.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, paths, err := ScanDirForSecrets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("clean dir scanned dirty: %v", paths)
	}
	if err := os.WriteFile(filepath.Join(dir, "leak.txt"), []byte("sk-ant-api03-PLANTEDKEYPLANTEDKEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	hits, _, err = ScanDirForSecrets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("planted key must be detected, hits=%d", hits)
	}
}

func TestStubRunProducesConformanceShapedTranscript(t *testing.T) {
	cfg := testConfig(t)
	var envReads []string
	res, err := Run(context.Background(), cfg, Deps{
		Getenv: func(name string) string {
			envReads = append(envReads, name)
			return ""
		},
	}, io.Discard)
	if err != nil {
		t.Fatalf("stub run: %v", err)
	}
	for _, name := range envReads {
		if name == UpstreamKeyEnv {
			t.Fatal("stub mode must never read the upstream key env var")
		}
	}
	if res.StopReason != StopCompleted {
		t.Fatalf("stub run stop=%q, want completed", res.StopReason)
	}

	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Mode != ModeStub {
		t.Fatalf("transcript mode=%q, want stub", tr.Mode)
	}
	if !tr.Sanitized || !tr.SessionCreated || tr.SessionID == "" || tr.ThreadID == "" || tr.AgentID == "" {
		t.Fatalf("transcript identity fields incomplete: %+v", tr)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(tr.TranscriptHash) {
		t.Fatalf("transcript_sha256 malformed: %q", tr.TranscriptHash)
	}
	if !eventOfType(tr, managedagents.EventAgentMCPToolUse) || !eventOfType(tr, managedagents.EventThreadCreated) {
		t.Fatalf("required events missing: %+v", tr.Events)
	}
	if !confirmationResult(tr, "allow") || !confirmationResult(tr, "deny") {
		t.Fatalf("allow and deny confirmations required: %+v", tr.Confirmations)
	}
	if len(tr.Decisions) == 0 {
		t.Fatal("no decision records")
	}
	for _, d := range tr.Decisions {
		if d.AgentID == "" || d.SessionID == "" || d.ThreadID == "" || d.Tool == "" || d.Action == "" || d.Rule == "" || d.Trust == 0 {
			t.Fatalf("decision missing metadata: %+v", d)
		}
	}
	if tr.Budget.Ceiling <= 0 || tr.Budget.Used < 0 || tr.Budget.Used > tr.Budget.Ceiling {
		t.Fatalf("budget out of bounds: %+v", tr.Budget)
	}
	if !tr.Trust.Tracked || tr.Trust.Score == 0 {
		t.Fatalf("trust tracking missing: %+v", tr.Trust)
	}
	if !tr.FailClosed.Observed || tr.FailClosed.Action != "deny" || tr.FailClosed.Reason == "" {
		t.Fatalf("fail-closed evidence wrong: %+v", tr.FailClosed)
	}
	if strings.Contains(string(data), "stub-evt") || strings.Contains(string(data), "sesn_stub") {
		t.Fatal("sanitized transcript must not contain raw identifiers")
	}
	if LooksSecret(string(data)) {
		t.Fatal("transcript contains secret-shaped data")
	}

	var prov provenanceDoc
	provData, err := os.ReadFile(res.ProvenancePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(provData, &prov); err != nil {
		t.Fatal(err)
	}
	if prov.Mode != ModeStub || prov.TranscriptSHA256 != tr.TranscriptHash || prov.SessionID == "" || prov.BetaHeader != BetaHeader {
		t.Fatalf("provenance incomplete: %+v", prov)
	}
}

// TestStubTranscriptPassesSharedConformanceChecks evaluates the stub run's
// transcript through the exact criterion functions the env-gated live
// conformance harness (tests/conformance/managed_agents) applies — in
// process, with no environment variables. Every criterion must pass except
// the live-mode gate, which must reject a stub transcript.
func TestStubTranscriptPassesSharedConformanceChecks(t *testing.T) {
	cfg := testConfig(t)
	res, err := Run(context.Background(), cfg, Deps{}, io.Discard)
	if err != nil {
		t.Fatalf("stub run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	tr, err := maconf.ParseTranscript(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range maconf.Checks() {
		err := check.Run(tr, data)
		if check.Name == "live_mode_transcript_only" {
			if err == nil {
				t.Fatal("live-mode gate accepted a stub transcript")
			}
			continue
		}
		if err != nil {
			t.Errorf("check %s failed on stub transcript: %v", check.Name, err)
		}
	}
}

func TestStubRunAbortOnSpendCeiling(t *testing.T) {
	cfg := testConfig(t)
	cfg.MaxSpendUSD = 0.15 // 80% = $0.12 < stub list_cost $0.18
	res, err := Run(context.Background(), cfg, Deps{}, io.Discard)
	if err != nil {
		t.Fatalf("spend abort is a clean stop, got %v", err)
	}
	if res.StopReason != StopSpendAbort {
		t.Fatalf("stop=%q, want %q", res.StopReason, StopSpendAbort)
	}
	if _, err := os.Stat(res.TranscriptPath); err != nil {
		t.Fatal("transcript must be written on abort")
	}
}

func TestErrorInjectorFailsPipelineForErrorTool(t *testing.T) {
	eval := &errorInjectEvaluator{inner: policyeval.NewEvaluator(nil), errorTool: "probe_tool"}
	if _, err := eval.Evaluate(context.Background(), &policyeval.EvaluationRequest{ToolName: "probe_tool"}); err == nil {
		t.Fatal("error tool must produce an evaluator error")
	}
	if _, err := eval.Evaluate(context.Background(), &policyeval.EvaluationRequest{ToolName: "other"}); err != nil {
		t.Fatalf("other tools must pass through: %v", err)
	}
}

func eventOfType(tr transcriptDoc, typ string) bool {
	for _, e := range tr.Events {
		if e.Type == typ {
			return true
		}
	}
	return false
}

func confirmationResult(tr transcriptDoc, result string) bool {
	for _, c := range tr.Confirmations {
		if c.Result == result {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fix-up tests: fake upstream, abort paths, provenance linkage, redactor
// ---------------------------------------------------------------------------

// fakeUpstream is an in-process Upstream used for live-mode driver tests. It
// records call order and sent events so tests can assert protocol behavior
// (prompt sent once, stream opened first, stop called on abort) without any
// network access.
type fakeUpstream struct {
	mu           sync.Mutex
	calls        []string
	events       []managedagents.Event
	streamErr    error
	block        bool // source blocks until ctx done (timeout tests)
	confirmErr   error
	stopErr      error
	userMessages int
	stopCalls    int
	closed       bool
}

func (f *fakeUpstream) CreateSession(_ context.Context, params CreateSessionParams) (*SessionInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "create")
	return &SessionInfo{ID: "sess_fake_0001", AgentID: params.AgentID}, nil
}

func (f *fakeUpstream) OpenEventStream(_ context.Context, _ string) (managedagents.EventSource, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "open")
	return &fakeSource{up: f}, nil
}

func (f *fakeUpstream) SendEvents(_ context.Context, _ string, events []map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "send")
	for _, e := range events {
		if e["type"] == "user.message" {
			f.userMessages++
		}
	}
	return nil
}

func (f *fakeUpstream) Stop(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "stop")
	f.stopCalls++
	return f.stopErr
}

func (f *fakeUpstream) Forwarder() managedagents.ConfirmationForwarder { return f }

func (f *fakeUpstream) SendConfirmation(_ context.Context, _ string, _ managedagents.ToolConfirmation) error {
	return f.confirmErr
}

func (f *fakeUpstream) RequestIDs() []string { return nil }

func (f *fakeUpstream) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeUpstream) StopCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopCalls
}

func (f *fakeUpstream) UserMessages() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.userMessages
}

func (f *fakeUpstream) CallOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

type fakeSource struct {
	up *fakeUpstream
	i  int
}

func (s *fakeSource) Next(ctx context.Context) (managedagents.Event, error) {
	if s.up.block {
		<-ctx.Done()
		return managedagents.Event{}, ctx.Err()
	}
	s.up.mu.Lock()
	defer s.up.mu.Unlock()
	if s.up.streamErr != nil {
		return managedagents.Event{}, s.up.streamErr
	}
	if s.i >= len(s.up.events) {
		return managedagents.Event{}, io.EOF
	}
	event := s.up.events[s.i]
	s.i++
	return event, nil
}

// liveTestConfig returns a config whose live-mode gates all pass when Deps
// supplies a non-empty key and an existing gate file.
func liveTestConfig(t *testing.T) Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.Mode = ModeLive
	cfg.AckSpend = true
	cfg.AgentID = "agt_fake"
	cfg.EnvironmentID = "env_fake"
	return cfg
}

// liveDeps injects up as the live upstream with the gate seams satisfied.
func liveDeps(up Upstream) Deps {
	return Deps{
		Getenv:     func(string) string { return "fake-test-key-not-real" },
		FileExists: func(string) bool { return true },
		NewLive:    func(_, _ string) Upstream { return up },
	}
}

// TestLiveRunSendsPromptOnceAfterStreamOpen proves the driver sends exactly
// one user.message, and only after the event stream is open (previously the
// prompt was duplicated between initial_events and a post-open send).
func TestLiveRunSendsPromptOnceAfterStreamOpen(t *testing.T) {
	up := &fakeUpstream{events: []managedagents.Event{
		{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
		{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001", Data: map[string]any{"usage": map[string]any{"list_cost": "5"}}},
		{ID: "e3", Type: managedagents.EventStatusIdle, SessionID: "sess_fake_0001"},
	}}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("live fake run: %v", err)
	}
	if res.StopReason != StopCompleted {
		t.Fatalf("stop=%q, want completed", res.StopReason)
	}
	if got := up.UserMessages(); got != 1 {
		t.Fatalf("user.message sent %d times, want exactly 1", got)
	}
	order := up.CallOrder()
	openIdx, sendIdx := -1, -1
	for i, c := range order {
		if c == "open" && openIdx == -1 {
			openIdx = i
		}
		if c == "send" && sendIdx == -1 {
			sendIdx = i
		}
	}
	if openIdx == -1 || sendIdx == -1 || openIdx > sendIdx {
		t.Fatalf("stream must open before the prompt is sent, call order: %v", order)
	}
	if got := up.StopCalls(); got != 0 {
		t.Fatalf("completed run must not stop the session, stop calls=%d", got)
	}
}

// TestLiveCreateSessionSendsNoInitialEvents asserts at the HTTP layer that
// session creation carries no initial_events (so the prompt can never be
// delivered twice).
func TestLiveCreateSessionSendsNoInitialEvents(t *testing.T) {
	var createBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		if r.Method == http.MethodPost && r.URL.Path == "/v1/sessions" {
			createBody, _ = io.ReadAll(r.Body)
			fmt.Fprint(w, `{"id":"sess_x","agent":"agt_x"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	up := newLiveUpstream(srv.URL, "fake")
	defer up.Close()
	if _, err := up.CreateSession(context.Background(), CreateSessionParams{AgentID: "a", EnvironmentID: "e", BudgetUSD: 1}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if strings.Contains(string(createBody), "initial_events") {
		t.Fatalf("create body must not carry initial_events: %s", createBody)
	}
}

// TestUpstreamStopOnAbortPaths proves every abort path tells the upstream to
// halt the session exactly once.
func TestUpstreamStopOnAbortPaths(t *testing.T) {
	usage := func() managedagents.Event {
		return managedagents.Event{Type: "session.usage", SessionID: "sess_fake_0001", Data: map[string]any{"usage": map[string]any{"list_cost": "80"}}}
	}
	toolUse := func(id string) managedagents.Event {
		return managedagents.Event{
			ID: id, Type: managedagents.EventAgentToolUse, AgentID: "agt_fake", SessionID: "sess_fake_0001",
			ToolName: "safe_tool", Usage: &managedagents.Usage{CostUSD: 0.01},
			Data: map[string]any{"evaluated_permission": "ask"},
		}
	}
	cases := []struct {
		name   string
		mutate func(*Config)
		up     func() *fakeUpstream
	}{
		{"spend abort", func(c *Config) { c.MaxSpendUSD = 0.10 }, func() *fakeUpstream {
			return &fakeUpstream{events: []managedagents.Event{usage()}}
		}},
		{"max turns", func(c *Config) { c.MaxTurns = 1 }, func() *fakeUpstream {
			return &fakeUpstream{events: []managedagents.Event{toolUse("t1"), toolUse("t2"), usage()}}
		}},
		{"output token cap", func(c *Config) { c.MaxOutputTokens = 10 }, func() *fakeUpstream {
			return &fakeUpstream{events: []managedagents.Event{{
				Type: "span.model_request_end", SessionID: "sess_fake_0001",
				Data: map[string]any{"model_usage": map[string]any{"model": "claude-sonnet-4-5-20250929", "input_tokens": 10, "output_tokens": 50}},
			}}}
		}},
		{"spend unknown", nil, func() *fakeUpstream {
			return &fakeUpstream{events: []managedagents.Event{{
				Type: "session.usage", SessionID: "sess_fake_0001", Data: map[string]any{"usage": map[string]any{}},
			}}}
		}},
		{"stream error", nil, func() *fakeUpstream {
			return &fakeUpstream{streamErr: errors.New("upstream went away")}
		}},
		{"wall-clock timeout", func(c *Config) { c.Timeout = 150 * time.Millisecond }, func() *fakeUpstream {
			return &fakeUpstream{block: true}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := tc.up()
			cfg := liveTestConfig(t)
			if tc.mutate != nil {
				tc.mutate(&cfg)
			}
			Run(context.Background(), cfg, liveDeps(up), io.Discard) //nolint:errcheck // abort paths return errors by design
			if got := up.StopCalls(); got != 1 {
				t.Fatalf("%s: upstream Stop called %d times, want exactly 1", tc.name, got)
			}
		})
	}
}

// TestSilentStreamHitsIdleDeadline runs a real HTTP exchange against a local
// server that opens the SSE stream and then goes silent: the idle-read
// deadline must end the read rather than hang.
func TestSilentStreamHitsIdleDeadline(t *testing.T) {
	old := liveStreamIdleTimeout
	liveStreamIdleTimeout = 150 * time.Millisecond
	defer func() { liveStreamIdleTimeout = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/events/stream") {
			w.Header().Set("content-type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
			<-r.Context().Done() // silent stream; client must give up
			return
		}
		w.Header().Set("content-type", "application/json")
		fmt.Fprint(w, `{"id":"sess_silent","agent":"agt"}`)
	}))
	defer srv.Close()

	up := newLiveUpstream(srv.URL, "fake")
	defer up.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sess, err := up.CreateSession(ctx, CreateSessionParams{AgentID: "a", EnvironmentID: "e", BudgetUSD: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stream, err := up.OpenEventStream(ctx, sess.ID)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	start := time.Now()
	_, err = stream.Next(ctx)
	if err == nil {
		t.Fatal("a silent stream must produce an error, not hang")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("idle deadline took %s, want ~150ms", elapsed)
	}
}

// TestSessionCreatedDerivedFromObservedEvents proves
// session_created_through_boundary is derived from observed proxy handling:
// when no stream event for the created session arrives, the flag is false
// and the criterion is reported NOT OBSERVED.
func TestSessionCreatedDerivedFromObservedEvents(t *testing.T) {
	up := &fakeUpstream{events: []managedagents.Event{
		{ID: "e1", Type: "session.status_running", SessionID: "sess_different_9999"},
	}}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.SessionCreated {
		t.Fatal("session_created_through_boundary must not be set without observed proxy events")
	}
	if !contains(res.NotObserved, "session_created_through_boundary") {
		t.Fatalf("criterion must be reported NOT OBSERVED, got %v", res.NotObserved)
	}
}

// TestNotObservedCriteriaReported checks the driver reports criteria with no
// evidence rather than fabricating them (no MCP tool, no thread, no deny, no
// fail-closed probe in the stream).
func TestNotObservedCriteriaReported(t *testing.T) {
	up := &fakeUpstream{events: []managedagents.Event{
		{ID: "e1", Type: "session.status_running", SessionID: "sess_fake_0001"},
		{ID: "e2", Type: "session.usage", SessionID: "sess_fake_0001", Data: map[string]any{"usage": map[string]any{"list_cost": "10"}}},
	}}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, want := range []string{
		"mcp_tool_use_event", "thread_creation_and_tracking",
		"tool_confirmation_deny", "fail_closed_on_pipeline_error",
	} {
		if !contains(res.NotObserved, want) {
			t.Fatalf("expected %q in criteria not observed: %v", want, res.NotObserved)
		}
	}
	if contains(res.NotObserved, "session_created_through_boundary") {
		t.Fatalf("session WAS observed through the proxy; not-observed=%v", res.NotObserved)
	}
}

// TestConfirmationNotAskedRecordedNotDelivered proves a confirmation for a
// call the upstream never paused for is recorded as not delivered instead of
// silently succeeding.
func TestConfirmationNotAskedRecordedNotDelivered(t *testing.T) {
	up := &fakeUpstream{
		confirmErr: ErrConfirmationNotAsked,
		events: []managedagents.Event{
			{ID: "tu1", Type: managedagents.EventAgentToolUse, AgentID: "agt_fake", SessionID: "sess_fake_0001",
				ToolName: "safe_tool", Usage: &managedagents.Usage{CostUSD: 0.01},
				Data: map[string]any{"evaluated_permission": "allow"}},
		},
	}
	cfg := liveTestConfig(t)
	res, err := Run(context.Background(), cfg, liveDeps(up), io.Discard)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if len(tr.Confirmations) == 0 {
		t.Fatal("the resolved confirmation must still be recorded")
	}
	for _, c := range tr.Confirmations {
		if c.Delivered == nil || *c.Delivered {
			t.Fatalf("not-asked confirmation must record delivered=false: %+v", c)
		}
	}
}

// TestKeyReadOnlyAfterEarlierGates proves the live key variable is not read
// when an earlier live gate (flag, gate file, api-base) has already failed.
func TestKeyReadOnlyAfterEarlierGates(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	var reads []string
	recorder := func(string) string {
		reads = append(reads, UpstreamKeyEnv)
		return "fake"
	}
	present := func(string) bool { return true }

	base := liveTestConfig(t)

	missing := base
	missing.AckSpend = false
	reads = nil
	if err := missing.validate(repoRoot, recorder, present); err == nil {
		t.Fatal("missing ack flag must refuse")
	}
	if len(reads) != 0 {
		t.Fatalf("key read %d times with missing ack flag", len(reads))
	}

	reads = nil
	if err := base.validate(repoRoot, recorder, func(string) bool { return false }); err == nil {
		t.Fatal("missing gate file must refuse")
	}
	if len(reads) != 0 {
		t.Fatalf("key read %d times with missing gate file", len(reads))
	}

	badBase := base
	badBase.APIBase = "http://not-anthropic.example.com"
	reads = nil
	if err := badBase.validate(repoRoot, recorder, present); err == nil {
		t.Fatal("insecure api-base must refuse")
	}
	if len(reads) != 0 {
		t.Fatalf("key read %d times with bad api-base", len(reads))
	}

	reads = nil
	if err := base.validate(repoRoot, recorder, present); err != nil {
		t.Fatalf("all gates satisfied must validate, got %v", err)
	}
	if len(reads) != 1 {
		t.Fatalf("key must be read exactly once when gates pass, got %d", len(reads))
	}
}

// TestDefaultLiveSpendCeilingIsFiveUSD pins the first-run live spend ceiling
// at $5.00: raising it requires the explicit --max-spend-usd flag, and any
// value above the hard maximum is refused outright. The 80-percent abort is
// covered by TestSpendGuardStopsAtEightyPercent and
// TestStubRunAbortOnSpendCeiling.
func TestDefaultLiveSpendCeilingIsFiveUSD(t *testing.T) {
	if got := DefaultConfig().MaxSpendUSD; got != 5.00 {
		t.Fatalf("default live spend ceiling = %.2f, want 5.00", got)
	}
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	cfg := liveTestConfig(t)
	cfg.MaxSpendUSD = hardMaxSpendUSD + 0.01
	if err := cfg.validate(repoRoot, func(string) string { return "fake" }, func(string) bool { return true }); err == nil {
		t.Fatal("a ceiling above the hard maximum must be rejected")
	}
}

// TestLiveAPIBaseValidation requires --api-base to be exactly
// https://api.anthropic.com in live mode. There is no bypass on the live
// path: even the test-only loopback seam is refused when Mode is live.
func TestLiveAPIBaseValidation(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	present := func(string) bool { return true }
	env := func(string) string { return "fake" }

	cases := []struct {
		name    string
		base    string
		seam    bool
		wantErr bool
	}{
		{"production base", DefaultAPIBase, false, false},
		{"http scheme", "http://api.anthropic.com", false, true},
		{"other host", "https://evil.example.com", false, true},
		{"suffix host trick", "https://api.anthropic.com.evil.example", false, true},
		{"userinfo host trick", "https://api.anthropic.com@evil.example", false, true},
		{"userinfo on real host", "https://user:pass@api.anthropic.com", false, true},
		{"explicit port", "https://api.anthropic.com:8443", false, true},
		{"default port spelled out", "https://api.anthropic.com:443", false, true},
		{"trailing path", "https://api.anthropic.com/v1", false, true},
		{"trailing slash", "https://api.anthropic.com/", false, true},
		{"empty base", "", false, true},
		{"malformed base", "ht!tp://not a url", false, true},
		{"loopback refused without seam", "http://127.0.0.1:9999", false, true},
		{"seam cannot be enabled in live mode", DefaultAPIBase, true, true},
		{"seam plus loopback refused in live mode", "http://127.0.0.1:9999", true, true},
	}
	for _, tc := range cases {
		cfg := liveTestConfig(t)
		cfg.APIBase = tc.base
		cfg.allowLoopbackAPIBase = tc.seam
		err := cfg.validate(repoRoot, env, present)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: api-base %q (seam=%v) must refuse", tc.name, tc.base, tc.seam)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: api-base %q (seam=%v) must be accepted: %v", tc.name, tc.base, tc.seam, err)
		}
	}
}

// TestMockAPIBaseSeam covers the test-only loopback seam on the stub/mock
// path: loopback hosts are permitted through it, every non-loopback base is
// refused even with the seam set, and without the seam nothing but the exact
// production base passes.
func TestMockAPIBaseSeam(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	// Stub mode must never consult getenv or fileExists; fail loudly if it
	// does so a regression cannot reintroduce a credential read here.
	guards := func(string) string {
		t.Fatal("stub mode must not read env vars")
		return ""
	}
	noFiles := func(string) bool {
		t.Fatal("stub mode must not stat gate files")
		return false
	}

	cases := []struct {
		name    string
		base    string
		seam    bool
		wantErr bool
	}{
		{"production base", DefaultAPIBase, false, false},
		{"production base with seam set", DefaultAPIBase, true, false},
		{"loopback ipv4 through seam", "http://127.0.0.1:9999", true, false},
		{"loopback localhost through seam", "http://localhost:8080", true, false},
		{"loopback ipv6 through seam", "http://[::1]:8080", true, false},
		{"loopback https through seam", "https://127.0.0.1:8443", true, false},
		{"loopback refused without seam", "http://127.0.0.1:9999", false, true},
		{"non-loopback refused with seam", "http://192.168.1.10:8080", true, true},
		{"remote host refused with seam", "https://api.anthropic.com.evil.example", true, true},
		{"userinfo refused with seam", "http://user@127.0.0.1:9999", true, true},
		{"non-http scheme refused with seam", "file:///etc/passwd", true, true},
		{"empty base refused", "", false, true},
	}
	for _, tc := range cases {
		cfg := testConfig(t) // stub mode
		cfg.APIBase = tc.base
		cfg.allowLoopbackAPIBase = tc.seam
		err := cfg.validate(repoRoot, guards, noFiles)
		if tc.wantErr && err == nil {
			t.Fatalf("%s: api-base %q (seam=%v) must refuse", tc.name, tc.base, tc.seam)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s: api-base %q (seam=%v) must be accepted: %v", tc.name, tc.base, tc.seam, err)
		}
	}
}

// TestOutDirSymlinkResolution proves a symlink pointing into the repo cannot
// smuggle --out-dir back inside the worktree, including for a not-yet-created
// directory.
func TestOutDirSymlinkResolution(t *testing.T) {
	repoRoot, err := driverRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	link := filepath.Join(tmp, "repo-link")
	if err := os.Symlink(repoRoot, link); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{link, filepath.Join(link, "sub"), filepath.Join(link, "sub", "newdir")} {
		if err := outDirOutsideRepo(out, repoRoot); err == nil {
			t.Fatalf("out-dir %q resolving inside repo must be rejected", out)
		}
	}
	if err := outDirOutsideRepo(filepath.Join(tmp, "real-out"), repoRoot); err != nil {
		t.Fatalf("outside temp dir must be accepted: %v", err)
	}
}

// TestTranscriptHashCoversWrittenBytes proves transcript_sha256 is computed
// over the redacted bytes (with the hash field absent), not over unredacted
// JSON.
func TestTranscriptHashCoversWrittenBytes(t *testing.T) {
	cfg := testConfig(t)
	res, err := Run(context.Background(), cfg, Deps{}, io.Discard)
	if err != nil {
		t.Fatalf("stub run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	recorded := tr.TranscriptHash
	if recorded == "" {
		t.Fatal("transcript_sha256 missing")
	}
	tr.TranscriptHash = ""
	unsigned, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(RedactString(string(unsigned))))
	if got := hex.EncodeToString(sum[:]); got != recorded {
		t.Fatalf("re-hash of written file = %q, recorded %q", got, recorded)
	}
}

// TestRedactorScrubsRegisteredLiteral proves a secret that matches no pattern
// is still scrubbed — as a whole value and as first/last-8 fragments.
func TestRedactorScrubsRegisteredLiteral(t *testing.T) {
	literal := "qk9-unpatterned-SECRETliteral-7x2wz8"
	registerSecretValue(literal)

	if out := RedactString("prefix " + literal + " suffix"); strings.Contains(out, literal) {
		t.Fatalf("literal value survived redaction: %q", out)
	}
	for _, frag := range []string{literal[:8], literal[len(literal)-8:]} {
		if out := RedactString("leak: " + frag); strings.Contains(out, frag) {
			t.Fatalf("fragment %q survived redaction: %q", frag, out)
		}
	}
	if !LooksSecret("contains " + literal + " inside") {
		t.Fatal("scan must flag the registered literal")
	}
}

// TestStubRunWritesProvenanceLinkage checks the transcript ↔ provenance ↔
// raw-log linkage fields on a stub run.
func TestStubRunWritesProvenanceLinkage(t *testing.T) {
	cfg := testConfig(t)
	res, err := Run(context.Background(), cfg, Deps{}, io.Discard)
	if err != nil {
		t.Fatalf("stub run: %v", err)
	}
	data, err := os.ReadFile(res.TranscriptPath)
	if err != nil {
		t.Fatal(err)
	}
	var tr transcriptDoc
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Provenance == nil {
		t.Fatal("transcript must carry a provenance linkage block")
	}
	if tr.Provenance.SessionID != tr.SessionID {
		t.Fatalf("linkage session %q != transcript session %q", tr.Provenance.SessionID, tr.SessionID)
	}
	raw, err := os.ReadFile(res.RawLogPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if tr.Provenance.RawLogSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("transcript raw_log_sha256 does not match the raw log file")
	}

	provData, err := os.ReadFile(res.ProvenancePath)
	if err != nil {
		t.Fatal(err)
	}
	var prov provenanceDoc
	if err := json.Unmarshal(provData, &prov); err != nil {
		t.Fatal(err)
	}
	if prov.TranscriptSessionID != tr.SessionID {
		t.Fatalf("provenance transcript_session_id %q != %q", prov.TranscriptSessionID, tr.SessionID)
	}
	if prov.RawLogSHA256 != tr.Provenance.RawLogSHA256 || prov.RawLogFile != "events.raw.jsonl" {
		t.Fatalf("provenance raw-log linkage wrong: %+v", prov)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
