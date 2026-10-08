package madriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

const (
	// StopCompleted means the upstream stream ended on its own.
	StopCompleted = "completed"
	// StopSpendAbort means the guard aborted at 80 percent of the ceiling.
	StopSpendAbort = "aborted_spend_80_percent"
	// StopSpendUnknown means a usage-bearing event arrived without spend data;
	// the session was stopped fail-closed.
	StopSpendUnknown = "stopped_spend_unknown"
	// StopMaxTurns means the governed-turn cap was reached.
	StopMaxTurns = "aborted_max_turns"
)

// Deps injects the environment seams so tests can exercise live-mode gates and
// the upstream factory without credentials or network access.
type Deps struct {
	Getenv     func(string) string
	FileExists func(string) bool
	// NewLive builds the live upstream; tests never receive a key.
	NewLive func(base, key string) Upstream
	// NewStub builds the stub upstream; tests may substitute a variant.
	NewStub func(cfg Config) Upstream
}

func (d Deps) withDefaults() Deps {
	if d.Getenv == nil {
		d.Getenv = os.Getenv
	}
	if d.FileExists == nil {
		d.FileExists = func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		}
	}
	if d.NewLive == nil {
		d.NewLive = func(base, key string) Upstream { return newLiveUpstream(base, key) }
	}
	if d.NewStub == nil {
		d.NewStub = func(cfg Config) Upstream { return newStubUpstream(cfg) }
	}
	return d
}

// Result reports what the run produced and how it stopped.
type Result struct {
	Mode           string
	StopReason     string
	SpendUSD       float64
	TranscriptPath string
	RawLogPath     string
	ProvenancePath string
	SecretHits     []string
}

// recorder accumulates the evidence the transcript is built from.
type recorder struct {
	mu            sync.Mutex
	events        []managedagents.Event
	confirmations []managedagents.ToolConfirmation
	decisions     []observedDecision
	auditEvents   []governance.AuditEvent
	confirmByID   map[string]string // tool_use_id -> confirmation result
	deliveredByID map[string]bool   // tool_use_id -> posted upstream
}

type observedDecision struct {
	agentID, sessionID, threadID string
	tool, action, rule, reason   string
	trust                        float64
	requestID, envelopeID        string
}

// recordingSink is the governed stream's EventSink; it captures every proxied
// event plus the governance metadata the adapter attached to it.
type recordingSink struct {
	rec *recorder
}

func (s *recordingSink) Emit(_ context.Context, event managedagents.Event) error {
	s.rec.mu.Lock()
	defer s.rec.mu.Unlock()
	s.rec.events = append(s.rec.events, event)
	if event.Governance != nil {
		thread := event.SessionThreadID
		if thread == "" {
			thread = event.SessionID
		}
		s.rec.decisions = append(s.rec.decisions, observedDecision{
			agentID:    event.AgentID,
			sessionID:  event.SessionID,
			threadID:   thread,
			tool:       event.ToolName,
			action:     event.Governance.Action,
			rule:       event.Governance.MatchedRule,
			reason:     event.Governance.Reason,
			trust:      event.Governance.TrustScore,
			requestID:  event.Governance.RequestID,
			envelopeID: event.Governance.EnvelopeID,
		})
	}
	return nil
}

// recordingForwarder wraps the upstream's ConfirmationForwarder so every
// user.tool_confirmation Boundary resolves is captured before it is sent.
type recordingForwarder struct {
	inner managedagents.ConfirmationForwarder
	rec   *recorder
}

func (f *recordingForwarder) SendConfirmation(ctx context.Context, sessionID string, confirmation managedagents.ToolConfirmation) error {
	err := f.inner.SendConfirmation(ctx, sessionID, confirmation)
	if err != nil {
		return err
	}
	delivered := true
	if reporter, ok := f.inner.(interface{ ConfirmationSent(string) bool }); ok {
		delivered = reporter.ConfirmationSent(confirmation.ToolUseID)
	}
	f.rec.mu.Lock()
	f.rec.confirmations = append(f.rec.confirmations, confirmation)
	f.rec.confirmByID[confirmation.ToolUseID] = confirmation.Result
	f.rec.deliveredByID[confirmation.ToolUseID] = delivered
	f.rec.mu.Unlock()
	return nil
}

// auditRecorder collects pipeline audit events into the raw log.
type auditRecorder struct {
	rec *recorder
}

func (a *auditRecorder) Publish(_ context.Context, event governance.AuditEvent) {
	a.rec.mu.Lock()
	a.rec.auditEvents = append(a.rec.auditEvents, event)
	a.rec.mu.Unlock()
}

// guardedSource interposes the spend and turn guards between the upstream
// stream and the SessionProxy.
type guardedSource struct {
	inner    managedagents.EventSource
	guard    *spendGuard
	maxTurns int
	turns    int
	err      error
}

func (s *guardedSource) Next(ctx context.Context) (managedagents.Event, error) {
	if s.err != nil {
		return managedagents.Event{}, s.err
	}
	event, err := s.inner.Next(ctx)
	if err != nil {
		return managedagents.Event{}, err
	}
	if err := s.guard.observe(event); err != nil {
		s.err = err
		return managedagents.Event{}, err
	}
	if event.Type == managedagents.EventAgentToolUse || event.Type == managedagents.EventAgentMCPToolUse {
		s.turns++
		if s.turns > s.maxTurns {
			s.err = ErrMaxTurns
			return managedagents.Event{}, ErrMaxTurns
		}
	}
	return event, nil
}

// errorInjectEvaluator forces a pipeline error for one configured tool name so
// the driver exercises ADR-047 fail-closed behavior deterministically; every
// other request delegates to the real policy evaluator.
type errorInjectEvaluator struct {
	inner     *policyeval.Evaluator
	errorTool string
}

func (e *errorInjectEvaluator) Evaluate(ctx context.Context, req *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	if req.ToolName == e.errorTool {
		return nil, fmt.Errorf("forced driver probe: synthetic evaluator failure for tool %q", e.errorTool)
	}
	return e.inner.Evaluate(ctx, req)
}

// Run executes one driver session end to end and writes the raw event log,
// sanitized transcript, and provenance file under cfg.OutDir. It returns the
// Result and a nil error on a clean or deliberately-aborted run; guard
// fail-closed stops and infrastructure failures return non-nil errors after
// writing whatever evidence exists.
func Run(ctx context.Context, cfg Config, deps Deps, logw io.Writer) (Result, error) {
	deps = deps.withDefaults()
	repoRoot, err := driverRepoRoot()
	if err != nil {
		return Result{}, err
	}
	if err := cfg.validate(repoRoot, deps.Getenv, deps.FileExists); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(cfg.OutDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("create --out-dir: %w", err)
	}
	if logw == nil {
		logw = io.Discard
	}

	var upstream Upstream
	if cfg.Mode == ModeLive {
		// The key is read once, held in a variable, and passed only to the
		// upstream transport. It is never written to disk, a log, or a
		// transcript, and every serialized line still passes the redactor.
		key := deps.Getenv(UpstreamKeyEnv)
		upstream = deps.NewLive(cfg.APIBase, key)
	} else {
		upstream = deps.NewStub(cfg)
		if cfg.AgentID == "" {
			cfg.AgentID = "agt_stub_conformance"
		}
	}
	defer upstream.Close()

	started := time.Now().UTC()
	session, err := upstream.CreateSession(ctx, CreateSessionParams{
		AgentID:       cfg.AgentID,
		EnvironmentID: cfg.EnvironmentID,
		Prompt:        cfg.Prompt,
		BudgetUSD:     cfg.MaxSpendUSD,
	})
	if err != nil {
		return Result{}, fmt.Errorf("create session through boundary front: %w", err)
	}
	if session.AgentID == "" {
		// The create response's agent field is UNVERIFIED (id vs object); the
		// configured id is the same identity either way.
		session.AgentID = cfg.AgentID
	}

	rec := &recorder{confirmByID: map[string]string{}, deliveredByID: map[string]bool{}}
	guard := newSpendGuard(cfg.MaxSpendUSD, cfg.MaxOutputTokens)
	trust := governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{})
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		GatewayVersion: "ma-conformance-driver",
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "driver-deny-tool", Tool: cfg.DenyTool, Action: "deny", Reason: "driver policy denies this tool"},
			// A non-terminal allow rule attaches a rule name to every allowed
			// decision so transcript decision records carry `rule`.
			{Name: "driver-observed-allow", Tool: "*", Action: "allow"},
		},
	}, trust, &errorInjectEvaluator{inner: policyeval.NewEvaluator(nil), errorTool: cfg.ErrorTool}, &auditRecorder{rec: rec})

	forwarder := &recordingForwarder{inner: upstream.Forwarder(), rec: rec}
	tracker := managedagents.NewThreadTracker(session.ID, cfg.MaxSpendUSD)
	adapter := managedagents.NewProxyAdapter(cfg.TenantID, forwarder)
	resolver := &managedagents.ToolResolver{Adapter: adapter, Pipeline: pipeline, Tracker: tracker, Forwarder: forwarder}
	proxy := managedagents.NewSessionProxy(resolver, tracker)

	stream, err := upstream.OpenEventStream(ctx, session.ID)
	if err != nil {
		return Result{}, fmt.Errorf("open session event stream: %w", err)
	}
	// Per the upstream docs the stream must be open before events are sent.
	if err := upstream.SendEvents(ctx, session.ID, []map[string]any{{
		"type":    "user.message",
		"content": []map[string]any{{"type": "text", "text": cfg.Prompt}},
	}}); err != nil {
		return Result{}, fmt.Errorf("send initial user.message: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	proxyErr := proxy.Proxy(runCtx, &guardedSource{inner: stream, guard: guard, maxTurns: cfg.MaxTurns}, &recordingSink{rec: rec})
	stopReason := classifyStop(proxyErr)
	fmt.Fprintf(logw, "mode=%s session=%s stop=%s spend=$%.4f\n", cfg.Mode, session.ID, stopReason, guard.used())

	res := Result{Mode: cfg.Mode, StopReason: stopReason, SpendUSD: guard.used()}
	writeErr := writeOutputs(cfg, deps, rec, session, guard, trust, upstream, started, stopReason, &res)
	if writeErr != nil {
		return res, writeErr
	}

	hits, hitPaths, err := ScanDirForSecrets(cfg.OutDir)
	if err != nil {
		return res, fmt.Errorf("secret scan of output dir: %w", err)
	}
	res.SecretHits = hitPaths
	if hits > 0 {
		fmt.Fprintf(logw, "secret scan FAILED:\n%s", scanReport(hitPaths))
		return res, errors.New("secret-like content detected in output directory")
	}

	if errors.Is(proxyErr, ErrSpendUnknown) {
		return res, proxyErr
	}
	return res, nil
}

func classifyStop(err error) string {
	switch {
	case err == nil:
		return StopCompleted
	case errors.Is(err, ErrSpendAbort):
		return StopSpendAbort
	case errors.Is(err, ErrSpendUnknown):
		return StopSpendUnknown
	case errors.Is(err, ErrMaxTurns):
		return StopMaxTurns
	default:
		return "error: " + RedactString(err.Error())
	}
}

// writeOutputs emits the three evidence artifacts under --out-dir.
func writeOutputs(cfg Config, deps Deps, rec *recorder, session *SessionInfo, guard *spendGuard, trust *governance.StandaloneTrustBackend, upstream Upstream, started time.Time, stopReason string, res *Result) error {
	rawPath, err := writeRawLog(cfg.OutDir, rec, upstream.RequestIDs())
	if err != nil {
		return err
	}
	res.RawLogPath = rawPath

	doc := buildTranscript(cfg, rec, session, guard, trust)
	transcriptPath, err := writeTranscriptFile(cfg.OutDir, doc)
	if err != nil {
		return err
	}
	res.TranscriptPath = transcriptPath

	prov := provenanceDoc{
		Mode:               cfg.Mode,
		SessionID:          session.ID,
		AgentID:            session.AgentID,
		StartedUTC:         started.Format(time.RFC3339),
		EndedUTC:           time.Now().UTC().Format(time.RFC3339),
		DriverCommit:       buildRevision(),
		AdapterCommit:      buildRevision(),
		TranscriptSHA256:   doc.TranscriptHash,
		UpstreamRequestIDs: upstream.RequestIDs(),
		BetaHeader:         BetaHeader,
		StopReason:         stopReason,
		SpendUSD:           guard.used(),
	}
	provPath, err := writeProvenanceFile(cfg.OutDir, prov)
	if err != nil {
		return err
	}
	res.ProvenancePath = provPath
	return nil
}

// buildTranscript converts recorded evidence into the sanitized transcript
// document: every identifier is pseudonymized and every string is redacted.
func buildTranscript(cfg Config, rec *recorder, session *SessionInfo, guard *spendGuard, trust *governance.StandaloneTrustBackend) *transcriptDoc {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	doc := &transcriptDoc{
		Sanitized:      true,
		Mode:           cfg.Mode,
		SessionCreated: true,
		SessionID:      pseudonym("sess", session.ID),
		AgentID:        pseudonym("agent", session.AgentID),
		Budget:         budgetDoc{Ceiling: cfg.MaxSpendUSD, Used: guard.used()},
	}
	for _, event := range rec.events {
		doc.Events = append(doc.Events, transcriptEvent{
			Type:      event.Type,
			SessionID: pseudonym("sess", event.SessionID),
			ThreadID:  pseudonym("thread", event.SessionThreadID),
			Tool:      RedactString(event.ToolName),
		})
		if event.Type == managedagents.EventThreadCreated && doc.ThreadID == "" {
			doc.ThreadID = pseudonym("thread", event.SessionThreadID)
		}
	}
	if doc.ThreadID == "" {
		// Fall back to the root thread: the tracker keys it by session id.
		doc.ThreadID = pseudonym("thread", session.ID)
	}
	for _, c := range rec.confirmations {
		delivered := rec.deliveredByID[c.ToolUseID]
		doc.Confirmations = append(doc.Confirmations, confirmationDoc{
			ToolUseID: pseudonym("tool", c.ToolUseID),
			Result:    c.Result,
			Tool:      RedactString(toolForConfirmation(rec, c.ToolUseID)),
			Delivered: &delivered,
		})
	}
	for _, d := range rec.decisions {
		doc.Decisions = append(doc.Decisions, decisionDoc{
			AgentID:   pseudonym("agent", d.agentID),
			SessionID: pseudonym("sess", d.sessionID),
			ThreadID:  pseudonym("thread", d.threadID),
			Tool:      RedactString(d.tool),
			Action:    d.action,
			Rule:      RedactString(d.rule),
			Trust:     d.trust,
			RequestID: pseudonym("req", d.requestID),
			Envelope:  pseudonym("env", d.envelopeID),
		})
		// ADR-047: a required-check failure surfaces as check_indeterminate on
		// the decision and deny on the wire confirmation. The transcript keeps
		// the confirmation result in Action and the check context in Reason.
		if d.action == governance.ActionCheckIndeterminate && !doc.FailClosed.Observed {
			doc.FailClosed = failClosedDoc{
				Observed: true,
				Action:   rec.confirmByID[decisionToolUseID(rec, d)],
				Reason:   RedactString(d.reason),
			}
		}
	}
	if trust != nil {
		doc.Trust.Tracked = true
		if snapshot, err := trust.GetAgentTrust(context.Background(), session.AgentID); err == nil {
			doc.Trust.Score = snapshot.Score
		} else {
			doc.Trust.Score = 1.0
		}
	}
	return doc
}

// toolForConfirmation finds the tool name recorded for a confirmation's
// tool_use id so the transcript can name it.
func toolForConfirmation(rec *recorder, toolUseID string) string {
	for _, event := range rec.events {
		if event.ID == toolUseID {
			return event.ToolName
		}
	}
	return ""
}

// decisionToolUseID finds the tool-use event id behind a decision record by
// matching session/thread/tool against the recorded events.
func decisionToolUseID(rec *recorder, d observedDecision) string {
	for _, event := range rec.events {
		if event.ToolName == d.tool && event.SessionID == d.sessionID &&
			(event.SessionThreadID == d.threadID || (event.SessionThreadID == "" && event.SessionID == d.threadID)) {
			return event.ID
		}
	}
	return ""
}

// writeRawLog writes every observed event, confirmation, and audit event as
// redacted JSONL to <out-dir>/events.raw.jsonl. The file lives outside the
// repo and is never committed; it exists for operator inspection only.
func writeRawLog(dir string, rec *recorder, requestIDs []string) (string, error) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	path := filepath.Join(dir, "events.raw.jsonl")
	var b strings.Builder
	write := func(kind string, v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			return
		}
		fmt.Fprintf(&b, "{\"kind\":%q,\"data\":%s}\n", kind, RedactString(string(raw)))
	}
	write("session_meta", map[string]any{"upstream_request_ids": requestIDs})
	for _, event := range rec.events {
		write("event", event)
	}
	for _, c := range rec.confirmations {
		write("confirmation", c)
	}
	for _, a := range rec.auditEvents {
		write("audit", a)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return "", fmt.Errorf("write raw event log: %w", err)
	}
	return path, nil
}

// buildRevision returns the VCS revision stamped into the binary by the Go
// toolchain (works for `go run` and `go test` built inside the worktree), or
// "unknown" when unavailable.
func buildRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			return s.Value
		}
	}
	return "unknown"
}
