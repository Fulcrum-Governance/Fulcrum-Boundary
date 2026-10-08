package madriver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
	g := newSpendGuard(1.00, 1024)
	if err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "79"}}}); err != nil {
		t.Fatalf("79 cents must not abort: %v", err)
	}
	err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{"list_cost": "80"}}})
	if !errors.Is(err, ErrSpendAbort) {
		t.Fatalf("80 cents on a $1 ceiling must abort, got %v", err)
	}
}

func TestSpendGuardFailsClosedOnMissingUsage(t *testing.T) {
	g := newSpendGuard(5.00, 1024)
	if err := g.observe(managedagents.Event{Type: "session.usage", Data: map[string]any{"usage": map[string]any{}}}); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("session.usage without list_cost must be unknown spend, got %v", err)
	}
	if err := g.observe(managedagents.Event{Type: "span.model_request_end", Data: map[string]any{}}); !errors.Is(err, ErrSpendUnknown) {
		t.Fatalf("span without model_usage must be unknown spend, got %v", err)
	}
}

func TestSpendGuardPricesTokensAndFlagsOutputCap(t *testing.T) {
	g := newSpendGuard(5.00, 100)
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
	if !g.overOutputLimit() {
		t.Fatal("250 output tokens over a cap of 100 must be flagged")
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
