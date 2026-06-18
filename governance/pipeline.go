package governance

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// DefaultFailClosedTransports enumerates the transports that fail-closed on
// PolicyEval errors out of the box. These are the transports where silently
// allowing an ungoverned action has real security consequences:
//
//   - TransportMCP — model-facing tool surface; the primary governance wedge.
//   - TransportManagedAgents — hosted agent tool confirmations.
//   - TransportCLI — wrapper-owned command execution.
//   - TransportCodeExec — arbitrary code execution.
//   - TransportGRPC — internal service surface for control-plane calls.
//   - TransportA2A — agent-to-agent tasks crossing a governed boundary.
//
// Operators who want different defaults must set
// PipelineConfig.FailClosedTransports explicitly. An explicit (non-nil) empty
// slice opts out of every transport being fail-closed.
var DefaultFailClosedTransports = []TransportType{
	TransportMCP,
	TransportManagedAgents,
	TransportCLI,
	TransportCodeExec,
	TransportGRPC,
	TransportA2A,
}

// PipelineConfig holds configuration for the governance pipeline.
type PipelineConfig struct {
	// StaticPolicies are simple allow/deny rules evaluated before the full engine.
	StaticPolicies []StaticPolicyRule

	// GatewayVersion is copied into decisions and audit records so operators
	// can tie runtime verdicts to the released boundary build.
	GatewayVersion string

	// PolicyBundleHash is copied into receipt-grade decision records.
	PolicyBundleHash string

	// BuildDigest identifies the Boundary binary or image that emitted the record.
	BuildDigest string

	// TopologyProfile is the named deployment posture asserted into every
	// schema_version "2" decision record this pipeline emits. It is asserted,
	// not attested: setting it does not verify that the running deployment
	// matches the named posture. Empty leaves the field unset (records stay
	// schema_version "1" unless other route-context is present).
	TopologyProfile string

	// RequireAgentID denies protected adapter requests that do not carry an
	// agent identity. This is intended for production trust-aware deployments.
	RequireAgentID bool

	// FailClosedTransports are transports that deny on pipeline errors.
	// All other transports fail-open on pipeline errors.
	//
	// Semantics:
	//   - nil (field unset) → DefaultFailClosedTransports is applied.
	//   - non-nil empty slice → all transports fail-open (operator opt-out).
	//   - non-nil populated slice → only the listed transports fail-closed.
	FailClosedTransports []TransportType

	// DryRun enables audit-only mode. When true, any decision that would
	// otherwise deny is converted to allow before Evaluate returns, with
	// GovernanceDecision.DryRun set to true and the original action recorded
	// in the decision reason. The audit event is emitted with the ORIGINAL
	// deny action, so logs reflect what governance would have blocked.
	DryRun bool

	// ReceiptSigner, when non-nil, signs every emitted decision record: the
	// audit event carries signature and signature_key_id populated from this
	// signer over the record's decision_hash. It is opt-in integrity for key
	// holders, not authenticity of the verdict — see ReceiptSigner. Nil (the
	// default) leaves records unsigned and byte-identical to the unsigned path.
	ReceiptSigner ReceiptSigner

	// Escalation, when non-nil, is invoked for PolicyEval ActionEscalate
	// decisions to resolve the escalation out-of-band (kernel mode) and adopt
	// its returned verdict. Nil (the default) preserves the relabel-and-return
	// behavior byte-for-byte: the decision is marked escalate/classified and
	// returned without any await. This is a kernel-mode seam; the standalone
	// path leaves it nil.
	Escalation EscalationHandler
}

// PolicyEvaluator is the abstract dependency the pipeline has on the policy
// evaluation engine. The concrete *policyeval.Evaluator in this repo
// satisfies it; tests and alternate implementations can substitute any type
// with the same method.
//
// Keeping this as an interface (rather than a concrete type) is what makes
// the fail-closed-vs-fail-open branch in Evaluate reachable from tests — a
// stub evaluator can return a synthetic error that the stock evaluator
// cannot produce.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, req *policyeval.EvaluationRequest) (*policyeval.Decision, error)
}

// Pipeline evaluates governance requests against trust state, static policies,
// domain interceptors, and the portable policy evaluator.
//
// This is the shared core of Boundary: all transport adapters call Pipeline.Evaluate().
type Pipeline struct {
	trustChecker     TrustChecker
	interceptors     *InterceptorRegistry
	evaluator        PolicyEvaluator
	auditor          AuditPublisher
	staticPolicies   []StaticPolicyRule
	gatewayVersion   string
	policyBundleHash string
	buildDigest      string
	topologyProfile  string
	requireAgentID   bool
	failClosed       map[TransportType]bool
	dryRun           bool
	signer           ReceiptSigner
	escalation       EscalationHandler
}

// NewPipeline creates a governance pipeline.
// All parameters are optional — pass nil for components that are not available.
func NewPipeline(cfg PipelineConfig, trust TrustChecker, evaluator PolicyEvaluator, auditor AuditPublisher) *Pipeline {
	if auditor == nil {
		auditor = noopAuditPublisher{}
	}
	if evaluator == nil {
		evaluator = policyeval.NewEvaluator(nil)
	}

	// nil FailClosedTransports → apply the kernel's secure-by-default list.
	// Non-nil (including explicit empty slice) is taken verbatim.
	failClosedList := cfg.FailClosedTransports
	if failClosedList == nil {
		failClosedList = DefaultFailClosedTransports
	}
	fc := make(map[TransportType]bool, len(failClosedList))
	for _, t := range failClosedList {
		fc[t] = true
	}

	return &Pipeline{
		trustChecker:     trust,
		interceptors:     NewInterceptorRegistry(),
		evaluator:        evaluator,
		auditor:          auditor,
		staticPolicies:   cfg.StaticPolicies,
		gatewayVersion:   cfg.GatewayVersion,
		policyBundleHash: cfg.PolicyBundleHash,
		buildDigest:      cfg.BuildDigest,
		topologyProfile:  cfg.TopologyProfile,
		requireAgentID:   cfg.RequireAgentID,
		failClosed:       fc,
		dryRun:           cfg.DryRun,
		signer:           cfg.ReceiptSigner,
		escalation:       cfg.Escalation,
	}
}

// RegisterInterceptor adds a domain-specific interceptor for a tool name.
func (p *Pipeline) RegisterInterceptor(toolName string, fn Interceptor) {
	p.interceptors.Register(toolName, fn)
}

// toolMatches reports whether a static policy pattern matches a tool name.
// Empty pattern and "*" match everything; otherwise exact match is tried
// first, then path.Match for glob syntax ("*", "?", "[abc]"). Malformed
// patterns are treated as non-matching rather than crashing the pipeline.
func toolMatches(pattern, toolName string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if pattern == toolName {
		return true
	}
	matched, _ := path.Match(pattern, toolName)
	return matched
}

// Evaluate runs the full governance pipeline for a request. It executes four
// stages in order; each may return a terminal decision (typically a deny),
// otherwise control falls through to the next stage:
//
//  1. Trust Check — the configured TrustChecker (in-process Beta evaluator in
//     standalone mode, or the Redis-backed backend in kernel mode). Skipped
//     when no checker is set or AgentID is empty. Isolated/Terminated → deny;
//     Evaluating → score 0.5; a checker error denies (fail-closed).
//  2. Static Policy Rules — linear scan; the first matching deny terminates.
//  3. Domain Interceptors — per-tool hooks; an interceptor error denies
//     (fail-closed).
//  4. PolicyEval Engine — the portable evaluator. An evaluator error is
//     per-transport: transports in FailClosedTransports deny, others fall
//     through and allow.
//
// Audit is emitted exactly once per call via a deferred hook (plus a
// trust_transition event when the trust state changes). Dry-run conversion
// happens AFTER audit so logs always reflect the real decision.
func (p *Pipeline) Evaluate(ctx context.Context, req *GovernanceRequest) (*GovernanceDecision, error) {
	start := time.Now()

	if req.EnvelopeID == "" {
		req.EnvelopeID = uuid.New().String()
	}
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	decision := &GovernanceDecision{
		RequestID:      req.RequestID,
		Action:         "allow",
		TrustScore:     1.0,
		EnvelopeID:     req.EnvelopeID,
		GatewayVersion: p.gatewayVersion,
		// Deterministic is the correct label for every Boundary pipeline outcome
		// except PolicyEval ActionEscalate (which flips to classified, and —
		// when an EscalationHandler resolves it out-of-band — may further carry
		// the upstream human_approved verdict the handler RELAYS). The PRD-002
		// taxonomy reserves "proved" and "human_approved" for upstream Foundry
		// decisions; the pipeline never mints either from its own logic — it
		// only relays a resolution the EscalationHandler returns.
		DecisionMode: DecisionModeDeterministic,
		TrustState:   TrustStateTrusted.String(),
	}
	trustState := TrustStateTrusted
	var trustUpdate *TrustDecisionUpdate

	defer func() {
		decision.Duration = time.Since(start)
		if update, err := p.recordTrustDecision(ctx, req, decision); err == nil && update != nil {
			trustUpdate = update
			decision.TrustScore = update.After.Score
			decision.TrustState = update.After.State.String()
			if update.After.State == TrustStateIsolated && decision.Allowed() {
				decision.Action = "deny"
				decision.Reason = fmt.Sprintf("agent %s is ISOLATED", req.AgentID)
			} else if update.After.State == TrustStateEvaluating && decision.Allowed() {
				decision.Action = "require_approval"
				decision.Reason = fmt.Sprintf("agent %s is degraded", req.AgentID)
			}
		} else if err != nil && decision.Allowed() && p.failClosed[req.Transport] {
			decision.Action = "deny"
			decision.Reason = fmt.Sprintf("trust update failed: %v", err)
			decision.TrustScore = 0.0
			decision.TrustState = TrustStateIsolated.String()
		}
		p.emitAudit(ctx, req, decision)
		if trustUpdate != nil && trustUpdate.Transition {
			p.emitTrustTransition(ctx, req, decision, *trustUpdate)
		}
		if p.dryRun && decision.Action == "deny" {
			original := decision.Reason
			if original == "" {
				original = "(no reason)"
			}
			decision.DryRun = true
			decision.Reason = "DRY-RUN would deny: " + original
			decision.Action = "allow"
		}
	}()

	// Stage 1: Trust Check
	if p.requireAgentID && p.failClosed[req.Transport] && req.AgentID == "" {
		decision.Action = "deny"
		decision.Reason = "agent identity is required for protected adapter"
		decision.TrustScore = 0.0
		decision.TrustState = TrustStateIsolated.String()
		return decision, nil
	}
	if p.trustChecker != nil && req.AgentID != "" {
		state, err := p.trustChecker.CheckAgentState(ctx, req.AgentID)
		if err != nil {
			decision.Action = "deny"
			decision.Reason = fmt.Sprintf("trust check failed: %v", err)
			decision.TrustScore = 0.0
			return decision, nil
		}
		trustState = state
		decision.TrustState = state.String()
		if state.Blocked() {
			decision.Action = "deny"
			decision.Reason = fmt.Sprintf("agent %s is %s", req.AgentID, state)
			decision.TrustScore = 0.0
			decision.TrustState = state.String()
			return decision, nil
		}
		if state == TrustStateEvaluating {
			decision.TrustScore = 0.5
			decision.TrustState = state.String()
		}
	}

	// Stage 2: Static Policy Rules (glob-aware tool and launch-grade field matches)
	for _, rule := range p.staticPolicies {
		if !rule.matchesRequest(req) {
			continue
		}
		decision.PolicyID = rule.Name
		decision.MatchedRule = rule.Name
		decision.PolicyFile = rule.PolicyFile
		// Use the same adoptable-mode allow-set as the escalation seam so a
		// mis-set or hostile policy mode (e.g. "proved") cannot make the
		// standalone pipeline emit a non-deterministic or proof-grade decision.
		if isAdoptableEscalationMode(rule.DecisionMode) {
			decision.DecisionMode = rule.DecisionMode
		}
		if rule.Action == "deny" {
			decision.Action = "deny"
			decision.Reason = rule.Reason
			if decision.Reason == "" {
				decision.Reason = fmt.Sprintf("denied by policy %q", rule.Name)
			}
			return decision, nil
		}
		if rule.Action == "warn" || rule.Action == "audit" {
			decision.Action = "warn"
			decision.Reason = rule.Reason
			return decision, nil
		}
		if rule.Action == "escalate" {
			decision.Action = "escalate"
			decision.Reason = rule.Reason
			decision.DecisionMode = DecisionModeClassified
			return decision, nil
		}
		if rule.Action == "require_approval" {
			decision.Action = "require_approval"
			decision.Reason = rule.Reason
			return decision, nil
		}
	}

	// Stage 3: Domain Interceptors
	interceptResult, err := p.interceptors.Run(ctx, req)
	if err != nil {
		decision.Action = "deny"
		decision.Reason = fmt.Sprintf("interceptor error: %v", err)
		return decision, nil
	}
	if interceptResult != nil && !interceptResult.Allowed {
		decision.Action = interceptResult.Action
		if decision.Action == "" {
			decision.Action = "deny"
		}
		decision.Reason = interceptResult.Reason
		return decision, nil
	}

	// Stage 4: PolicyEval Engine
	evalReq := ProjectPolicyEvalRequest(req, &decision.TrustScore, trustState, p.gatewayVersion)
	evalDecision, err := p.evaluator.Evaluate(ctx, evalReq)
	if err != nil {
		if p.failClosed[req.Transport] {
			decision.Action = "deny"
			decision.Reason = fmt.Sprintf("policy evaluation failed (fail-closed): %v", err)
		}
		// fail-open transports: allow proceeds with logged warning
		return decision, nil
	}
	if evalDecision != nil {
		switch evalDecision.Action {
		case policyeval.ActionDeny:
			decision.Action = "deny"
			decision.Reason = evalDecision.Reason
		case policyeval.ActionEscalate:
			decision.Action = "escalate"
			decision.Reason = evalDecision.EscalationReason
			// Escalation implies a semantic condition the evaluator could not
			// resolve deterministically (see pipeline_coverage_test.go
			// newSemanticEscalatePolicy). Relabel the decision so downstream
			// sinks know this row needs semantic follow-up.
			decision.DecisionMode = DecisionModeClassified
			// Out-of-band resolution seam. With no handler configured this case
			// is a pure relabel-and-return (byte-identical to the pre-seam
			// behavior). With a handler (kernel await mode) the pipeline RELAYS
			// the handler's resolved verdict (an upstream human review's
			// approve/deny, or a mechanical expiry deny) and never mints that
			// verdict itself. Skipped under dry-run (audit-only must not block
			// on a human).
			if p.escalation != nil && !p.dryRun {
				p.resolveEscalation(ctx, req, decision)
			} else if p.escalation != nil && p.dryRun {
				decision.Reason = decision.Reason + " (dry-run: escalation await skipped)"
			}
		case policyeval.ActionRequireApproval:
			decision.Action = "require_approval"
			decision.Reason = evalDecision.Reason
		case policyeval.ActionWarn:
			decision.Action = "warn"
			decision.Reason = evalDecision.Reason
		}
		if evalDecision.MatchedPolicy != nil {
			decision.PolicyID = evalDecision.MatchedPolicy.PolicyId
		}
	}

	return decision, nil
}

// resolveEscalation invokes the configured EscalationHandler for an escalate
// decision and folds its resolved verdict into decision. A handler error, a
// nil decision, or a decision whose Action is not one of the recognized
// verdicts is a fault and denies fail-closed with the
// "escalation fault (fail-closed):" reason prefix; fault denies carry
// DecisionModeDeterministic, matching the pipeline's other fail-closed fault
// paths (a local fault is a mechanical outcome, not a relayed resolution).
// On success it adopts the handler's Action, Reason, and DecisionMode — and
// only those fields: trust posture stays pipeline-owned. The adopted mode is
// vetted by isAdoptableEscalationMode so a buggy or hostile handler cannot
// stamp "proved" or any non-recognized mode onto a Boundary decision; an
// unadoptable (or empty) returned DecisionMode leaves the relabel's classified
// in place. This mirrors the action guard: the pipeline never originates
// "proved"/"human_approved" itself and only relays a mode the handler is
// permitted to return (see governance/decision_mode.go and
// docs/PROOF_BOUNDARY.md).
func (p *Pipeline) resolveEscalation(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) {
	resolved, err := p.escalation.Escalate(ctx, *req, decision.Reason)
	if err != nil {
		decision.Action = "deny"
		decision.Reason = "escalation fault (fail-closed): " + err.Error()
		decision.DecisionMode = DecisionModeDeterministic
		return
	}
	if resolved == nil {
		decision.Action = "deny"
		decision.Reason = "escalation fault (fail-closed): handler returned no decision"
		decision.DecisionMode = DecisionModeDeterministic
		return
	}
	if !isValidEscalatedAction(resolved.Action) {
		decision.Action = "deny"
		decision.Reason = fmt.Sprintf("escalation fault (fail-closed): handler returned invalid action %q", resolved.Action)
		decision.DecisionMode = DecisionModeDeterministic
		return
	}
	decision.Action = resolved.Action
	decision.Reason = resolved.Reason
	if isAdoptableEscalationMode(resolved.DecisionMode) {
		decision.DecisionMode = resolved.DecisionMode
	}
}

// isValidEscalatedAction reports whether a is one of the decision actions an
// EscalationHandler may resolve to (the GovernanceDecision action vocabulary).
// Anything else — including an empty action — is treated as a handler fault so
// a buggy or hostile handler cannot inject an out-of-vocabulary action into a
// decision record.
func isValidEscalatedAction(a string) bool {
	switch a {
	case "allow", "deny", "warn", "escalate", "require_approval":
		return true
	}
	return false
}

// isAdoptableEscalationMode reports whether the pipeline may adopt a
// DecisionMode an EscalationHandler returns. The escalation seam RELAYS an
// upstream resolution, so it may legitimately carry "human_approved" (a relayed
// human-review verdict); it may also carry the pipeline-native "deterministic"
// or "classified" (the awaiting handler uses these for mechanical expiry,
// timeout, and fault outcomes). It may NOT carry "proved": Boundary never emits
// proved decisions (governance/decision_mode.go header, docs/PROOF_BOUNDARY.md,
// BND-CLAIM-010), and the escalation seam is not a proof channel. An empty mode
// or any value outside this vetted set is not adopted (the relabel's classified
// stays), so a buggy or hostile handler cannot inject "proved" or an unknown
// mode onto a decision record — the same threat model isValidEscalatedAction
// guards for the action channel.
func isAdoptableEscalationMode(m DecisionMode) bool {
	switch m {
	case DecisionModeDeterministic, DecisionModeClassified, DecisionModeHumanApproved:
		return true
	}
	return false
}

// routeID derives the governed route identifier recorded in schema_version "2"
// decision records. It is descriptive only: it names the transport+tool path
// the request traveled, not an attestation that the route is the only path to
// the tool. Returns "" when the transport is unset so the record stays V1.
func routeID(req *GovernanceRequest) string {
	if req == nil || req.Transport == "" {
		return ""
	}
	if req.ToolName == "" {
		return string(req.Transport)
	}
	return string(req.Transport) + ":" + req.ToolName
}

func (p *Pipeline) emitAudit(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) {
	event := AuditEvent{
		RequestID:           req.RequestID,
		Transport:           req.Transport,
		ToolName:            req.ToolName,
		Action:              decision.Action,
		Reason:              decision.Reason,
		TrustScore:          decision.TrustScore,
		EnvelopeID:          decision.EnvelopeID,
		AgentID:             req.AgentID,
		TenantID:            req.TenantID,
		Timestamp:           time.Now(),
		PolicyBundleHash:    p.policyBundleHash,
		BoundaryBuildDigest: p.buildDigest,
		RequestHash:         ComputeRequestHash(req),
		TrustState:          decision.TrustState,
		DecisionMode:        decision.DecisionMode,
		MatchedRule:         decision.MatchedRule,
		PolicyFile:          decision.PolicyFile,
		GatewayVersion:      decision.GatewayVersion,
		TraceID:             req.TraceID,
		// Route-context (schema_version "2"): descriptive only. adapter_id and
		// route_id name the path the request traveled; topology_profile is the
		// asserted (not attested) deployment posture. execution_claim stays nil
		// here — the pipeline decides BEFORE execution, so a pre-execution
		// record makes no execution self-report; adapters that proxy upstream
		// attach it themselves.
		AdapterID:       string(req.Transport),
		RouteID:         routeID(req),
		TopologyProfile: p.topologyProfile,
	}
	p.signAuditEvent(&event)
	p.auditor.Publish(ctx, event)
}

// signAuditEvent populates event.Signature and event.SignatureKeyID when a
// ReceiptSigner is configured, so the decision record BuildDecisionRecord
// renders from this event carries the operator signature. It builds the record
// the publisher will build (with the signature fields still empty), signs over
// its decision_hash, and writes the signature back onto the event. Because
// ComputeDecisionHash blanks the signature fields, carrying the signature does
// not perturb decision_hash: a signed record verifies to the identical
// decision_hash as the unsigned record. With no signer this is a no-op and the
// event (and its record) is byte-identical to the unsigned path. A signing error
// is treated as fail-closed for the signature: the event is published unsigned
// rather than with a partial or bogus signature, so an unsigned record never
// masquerades as signed.
func (p *Pipeline) signAuditEvent(event *AuditEvent) {
	if p.signer == nil {
		return
	}
	record := BuildDecisionRecord(*event)
	signature, err := p.signer.Sign(record)
	if err != nil {
		return
	}
	event.Signature = signature
	event.SignatureKeyID = p.signer.KeyID()
}

func (p *Pipeline) recordTrustDecision(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) (*TrustDecisionUpdate, error) {
	if req == nil || req.AgentID == "" {
		return nil, nil
	}
	backend, ok := p.trustChecker.(TrustBackend)
	if !ok {
		return nil, nil
	}
	if decision == nil || strings.HasPrefix(decision.Reason, "trust check failed") || strings.Contains(decision.Reason, " is ISOLATED") || strings.Contains(decision.Reason, " is TERMINATED") {
		return nil, nil
	}
	update, err := backend.RecordDecision(ctx, req, decision)
	if err != nil {
		return nil, err
	}
	return &update, nil
}

func (p *Pipeline) emitTrustTransition(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision, update TrustDecisionUpdate) {
	event := AuditEvent{
		EventType:           "trust_transition",
		RequestID:           req.RequestID,
		Transport:           req.Transport,
		ToolName:            req.ToolName,
		Action:              decision.Action,
		Reason:              fmt.Sprintf("trust %s -> %s after %s", update.Before.State, update.After.State, update.Outcome),
		TrustScore:          update.After.Score,
		EnvelopeID:          decision.EnvelopeID,
		AgentID:             req.AgentID,
		TenantID:            req.TenantID,
		Timestamp:           time.Now(),
		PolicyBundleHash:    p.policyBundleHash,
		BoundaryBuildDigest: p.buildDigest,
		RequestHash:         ComputeRequestHash(req),
		TrustState:          update.After.State.String(),
		DecisionMode:        DecisionModeDeterministic,
		MatchedRule:         decision.MatchedRule,
		PolicyFile:          decision.PolicyFile,
		GatewayVersion:      decision.GatewayVersion,
		TraceID:             req.TraceID,
		AdapterID:           string(req.Transport),
		RouteID:             routeID(req),
		TopologyProfile:     p.topologyProfile,
		Metadata: map[string]interface{}{
			"trust_before": update.Before.State.String(),
			"trust_after":  update.After.State.String(),
			"outcome":      string(update.Outcome),
		},
	}
	p.signAuditEvent(&event)
	p.auditor.Publish(ctx, event)
}
