package madriver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	// StopSpendUnknown means a usage-bearing event arrived without spend data
	// or the stream ran too long without any usage signal; the session was
	// stopped fail-closed.
	StopSpendUnknown = "stopped_spend_unknown"
	// StopMaxTurns means the governed-turn cap was reached.
	StopMaxTurns = "aborted_max_turns"
	// StopOutputCap means an observed request exceeded --max-output-tokens.
	StopOutputCap = "aborted_output_token_cap"
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
	// NotObserved lists the conformance criteria the run produced no evidence
	// for; the driver reports them rather than fabricating evidence.
	NotObserved []string
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
	// ErrConfirmationNotAsked means the upstream never paused for this call,
	// so no confirmation could be delivered. The resolution is still recorded
	// — explicitly as NOT delivered — rather than silently succeeding or
	// failing the governed stream.
	if errors.Is(err, ErrConfirmationNotAsked) {
		f.rec.mu.Lock()
		f.rec.confirmations = append(f.rec.confirmations, confirmation)
		f.rec.confirmByID[confirmation.ToolUseID] = confirmation.Result
		f.rec.deliveredByID[confirmation.ToolUseID] = false
		f.rec.mu.Unlock()
		return nil
	}
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
	s.guard.arm()
	if s.guard.blindExceeded() {
		s.err = fmt.Errorf("%w: no usage signal within %s", ErrSpendUnknown, s.guard.blindWindow)
		return managedagents.Event{}, s.err
	}
	// The blind window is a time limit, not only an event limit: the read is
	// raced against the blind deadline so a stream that goes silent trips
	// the limit instead of hanging on the transport's (longer) idle timeout.
	// The channel is buffered so the read goroutine never blocks past an
	// abandoned call; it exits when the stream unblocks or the run context
	// is cancelled.
	type result struct {
		event managedagents.Event
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		event, err := s.inner.Next(ctx)
		ch <- result{event, err}
	}()
	var deadlineC <-chan time.Time
	var timer *time.Timer
	if deadline := s.guard.blindDeadline(); !deadline.IsZero() {
		timer = time.NewTimer(time.Until(deadline))
		deadlineC = timer.C
	}
	if timer != nil {
		defer timer.Stop()
	}
	var event managedagents.Event
	var err error
	select {
	case r := <-ch:
		event, err = r.event, r.err
	case <-deadlineC:
		if s.guard.blindExceeded() {
			s.err = fmt.Errorf("%w: no usage signal within %s", ErrSpendUnknown, s.guard.blindWindow)
			return managedagents.Event{}, s.err
		}
		// The guard's clock has not closed the window yet (an injected
		// clock can lag the real timer); wait on the read alone.
		select {
		case r := <-ch:
			event, err = r.event, r.err
		case <-ctx.Done():
			return managedagents.Event{}, ctx.Err()
		}
	case <-ctx.Done():
		return managedagents.Event{}, ctx.Err()
	}
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
		// The key is read once, held in a variable, registered with the
		// redactor so the exact value (and its edge fragments) can never reach
		// an output file, and passed only to the upstream transport. It is
		// never written to disk, a log, or a transcript.
		key := deps.Getenv(UpstreamKeyEnv)
		registerSecretValue(key)
		upstream = deps.NewLive(cfg.APIBase, key)
	} else {
		upstream = deps.NewStub(cfg)
		if cfg.AgentID == "" {
			cfg.AgentID = "agt_stub_conformance"
		}
	}
	defer upstream.Close()

	// stopSession asks the upstream to halt a session the driver is aborting.
	// It uses its own short deadline because the run context may already be
	// expired (wall-clock timeout path). A failed stop is not a warning: it
	// is returned so the run result reports the paid session may still be
	// running instead of masquerading as a clean abort.
	stopSession := func(sessionID string) error {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer stopCancel()
		if err := upstream.Stop(stopCtx, sessionID); err != nil {
			fmt.Fprintf(logw, "warning: upstream stop for session %s: %s\n", sessionID, RedactString(err.Error()))
			return fmt.Errorf("upstream interrupt failed for session %s; the paid session may still be running: %w",
				pseudonym("sess", sessionID), err)
		}
		return nil
	}

	// The wall-clock deadline covers the WHOLE run — session create, stream
	// open, prompt send, and the proxied stream read — not just Proxy.
	started := time.Now().UTC()
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	session, err := upstream.CreateSession(runCtx, CreateSessionParams{
		AgentID:       cfg.AgentID,
		EnvironmentID: cfg.EnvironmentID,
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
	guard := newSpendGuard(cfg.MaxSpendUSD, cfg.MaxOutputTokens, cfg.UsageBlindEvents, cfg.UsageBlindWindow)
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

	stream, err := upstream.OpenEventStream(runCtx, session.ID)
	if err != nil {
		return Result{}, errors.Join(fmt.Errorf("open session event stream: %w", err), stopSession(session.ID))
	}
	// The prompt is sent exactly once — here, AFTER the event stream is open
	// — so early tool-use and usage events are not missed and governance is
	// not blind to them. Live session creation carries no initial_events.
	if err := upstream.SendEvents(runCtx, session.ID, []map[string]any{{
		"type":    "user.message",
		"content": []map[string]any{{"type": "text", "text": cfg.effectivePrompt()}},
	}}); err != nil {
		return Result{}, errors.Join(fmt.Errorf("send initial user.message: %w", err), stopSession(session.ID))
	}

	proxyErr := proxy.Proxy(runCtx, &guardedSource{inner: stream, guard: guard, maxTurns: cfg.MaxTurns}, &recordingSink{rec: rec})
	stopReason := classifyStop(proxyErr)
	var stopErr error
	if proxyErr != nil {
		// Every abort path — spend guard, turn/output caps, timeout, stream
		// error — tells the upstream to halt rather than leaving a paid
		// session running. A rejected interrupt is not silent: it propagates
		// so the run result cannot claim a clean abort.
		stopErr = stopSession(session.ID)
	}
	fmt.Fprintf(logw, "mode=%s session=%s stop=%s spend=$%.4f\n", cfg.Mode, session.ID, stopReason, guard.used())

	res := Result{Mode: cfg.Mode, StopReason: stopReason, SpendUSD: guard.used()}
	spendUnknown := errors.Is(proxyErr, ErrSpendUnknown) || !guard.observedUsage()
	writeErr := writeOutputs(cfg, deps, rec, session, guard, trust, upstream, started, stopReason, spendUnknown, &res)
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

	if len(res.NotObserved) > 0 {
		fmt.Fprintf(logw, "criteria not observed: %s\n", strings.Join(res.NotObserved, ", "))
	}

	switch {
	case proxyErr == nil:
		return res, nil
	case errors.Is(proxyErr, ErrSpendAbort),
		errors.Is(proxyErr, ErrMaxTurns),
		errors.Is(proxyErr, ErrOutputTokenCap):
		// The guard aborted the run cleanly ONLY when the upstream accepted
		// the interrupt; a failed stop means the paid session may still be
		// running, so the run reports that failure instead.
		return res, stopErr
	default:
		// Fail-closed stops and infrastructure failures surface as errors;
		// the evidence written above still stands. A failed interrupt is
		// joined on rather than hidden inside a warning line.
		return res, errors.Join(proxyErr, stopErr)
	}
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
	case errors.Is(err, ErrOutputTokenCap):
		return StopOutputCap
	default:
		return "error: " + RedactString(err.Error())
	}
}

// writeOutputs emits the three evidence artifacts under --out-dir.
func writeOutputs(cfg Config, deps Deps, rec *recorder, session *SessionInfo, guard *spendGuard, trust *governance.StandaloneTrustBackend, upstream Upstream, started time.Time, stopReason string, spendUnknown bool, res *Result) error {
	rawPath, rawHash, err := writeRawLog(cfg.OutDir, rec, upstream.RequestIDs())
	if err != nil {
		return err
	}
	res.RawLogPath = rawPath

	doc := buildTranscript(cfg, rec, session, guard, trust, spendUnknown, rawHash)
	res.NotObserved = doc.NotObserved
	transcriptPath, err := writeTranscriptFile(cfg.OutDir, doc)
	if err != nil {
		return err
	}
	res.TranscriptPath = transcriptPath

	prov := provenanceDoc{
		Mode:                cfg.Mode,
		SessionID:           session.ID,
		TranscriptSessionID: doc.SessionID,
		AgentID:             session.AgentID,
		StartedUTC:          started.Format(time.RFC3339),
		EndedUTC:            time.Now().UTC().Format(time.RFC3339),
		DriverCommit:        buildRevision(),
		AdapterCommit:       buildRevision(),
		TranscriptSHA256:    doc.TranscriptHash,
		RawLogFile:          filepath.Base(rawPath),
		RawLogSHA256:        rawHash,
		UpstreamRequestIDs:  upstream.RequestIDs(),
		BetaHeader:          BetaHeader,
		StopReason:          stopReason,
		SpendUSD:            guard.used(),
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
// session_created_through_boundary is DERIVED from observed proxy handling —
// at least one stream event for the created session must have flowed through
// the governed path — never set as a constant.
func buildTranscript(cfg Config, rec *recorder, session *SessionInfo, guard *spendGuard, trust *governance.StandaloneTrustBackend, spendUnknown bool, rawLogSHA256 string) *transcriptDoc {
	rec.mu.Lock()
	defer rec.mu.Unlock()

	sessionID := pseudonym("sess", session.ID)
	doc := &transcriptDoc{
		Sanitized:      true,
		Mode:           cfg.Mode,
		SessionCreated: sessionObserved(rec, session.ID),
		SessionID:      sessionID,
		AgentID:        pseudonym("agent", session.AgentID),
		Budget: budgetDoc{
			Ceiling:       cfg.MaxSpendUSD,
			Used:          guard.used(),
			UsageObserved: guard.observedUsage(),
			SpendUnknown:  spendUnknown,
		},
		Provenance: &provenanceLink{
			Mode:         cfg.Mode,
			SessionID:    sessionID,
			RawLogSHA256: rawLogSHA256,
		},
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
	doc.NotObserved = unobservedCriteria(doc)
	return doc
}

// sessionObserved reports whether at least one event for the created session
// actually flowed through the Boundary-governed stream — the observable
// evidence behind session_created_through_boundary.
func sessionObserved(rec *recorder, sessionID string) bool {
	for _, event := range rec.events {
		if event.SessionID == sessionID {
			return true
		}
	}
	return false
}

// unobservedCriteria reports which conformance criteria have no evidence in
// the transcript. The driver reports these as NOT OBSERVED rather than
// synthesizing evidence — for example session.thread_created or
// agent.mcp_tool_use may never arrive if the upstream agent does not create a
// thread or hold an MCP tool, and whether prompting can reliably induce them
// is UNVERIFIED.
func unobservedCriteria(doc *transcriptDoc) []string {
	var missing []string
	if !doc.SessionCreated {
		missing = append(missing, "session_created_through_boundary")
	}
	allow, deny := false, false
	for _, c := range doc.Confirmations {
		switch c.Result {
		case managedagents.ConfirmationAllow:
			allow = true
		case managedagents.ConfirmationDeny:
			deny = true
		}
	}
	if !allow {
		missing = append(missing, "tool_confirmation_allow")
	}
	if !deny {
		missing = append(missing, "tool_confirmation_deny")
	}
	mcp, threadCreated := false, false
	for _, e := range doc.Events {
		if e.Type == managedagents.EventAgentMCPToolUse {
			mcp = true
		}
		if e.Type == managedagents.EventThreadCreated {
			threadCreated = true
		}
	}
	if !mcp {
		missing = append(missing, "mcp_tool_use_event")
	}
	if !threadCreated {
		missing = append(missing, "thread_creation_and_tracking")
	}
	if !doc.Budget.UsageObserved || doc.Budget.SpendUnknown {
		missing = append(missing, "budget_tracking_against_ceiling")
	}
	if !doc.Trust.Tracked {
		missing = append(missing, "trust_tracking_in_decisions")
	}
	metadataOK := len(doc.Decisions) > 0
	for _, d := range doc.Decisions {
		if d.AgentID == "" || d.SessionID == "" || d.ThreadID == "" || d.Tool == "" || d.Action == "" || d.Rule == "" || d.Trust == 0 {
			metadataOK = false
		}
	}
	if !metadataOK {
		missing = append(missing, "decision_metadata")
	}
	if !doc.FailClosed.Observed || doc.FailClosed.Action != managedagents.ConfirmationDeny {
		missing = append(missing, "fail_closed_on_pipeline_error")
	}
	return missing
}

// effectivePrompt returns the user.message the driver sends. In live mode an
// operator-supplied --prompt wins; otherwise the driver runs its conformance
// scenario prompt, which instructs the agent to exercise the tools the
// scenario names (the referenced upstream agent must actually be configured
// with them — see the README driver section).
func (c Config) effectivePrompt() string {
	if c.Mode == ModeLive && c.Prompt == DefaultConfig().Prompt {
		return fmt.Sprintf(
			"Governance conformance probe. Perform these steps in order, then stop: "+
				"(1) call read_file on README.md and summarize it in one sentence; "+
				"(2) call %q once; (3) call %q once; (4) if an MCP tool is available, call it once; "+
				"(5) if the session supports spawning a thread, open one for a one-line note. "+
				"Report the outcome of each step.",
			c.DenyTool, c.ErrorTool)
	}
	return c.Prompt
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
// redacted JSONL to <out-dir>/events.raw.jsonl and returns the file path plus
// the SHA-256 of the exact sanitized bytes written. The file lives outside
// the repo and is never committed; it exists for operator inspection only.
func writeRawLog(dir string, rec *recorder, requestIDs []string) (path string, sha256Hex string, err error) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	path = filepath.Join(dir, "events.raw.jsonl")
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
	content := []byte(b.String())
	sum := sha256.Sum256(content)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", "", fmt.Errorf("write raw event log: %w", err)
	}
	return path, hex.EncodeToString(sum[:]), nil
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
