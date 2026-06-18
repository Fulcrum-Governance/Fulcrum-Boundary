package governance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// These tests pin the PipelineConfig.Escalation seam: the PolicyEval
// ActionEscalate case invokes the configured EscalationHandler (kernel await
// mode) and adopts its resolved verdict, while a nil handler preserves the
// pre-seam relabel-and-return behavior byte-for-byte and dry-run skips the
// handler entirely. Faults (handler error, nil decision, invalid action) deny
// fail-closed with the "escalation fault (fail-closed):" reason prefix and
// DecisionModeDeterministic, matching the pipeline's other fault denies.

// semanticEscalationReason is the exact EscalationReason policyeval produces
// for newSemanticEscalatePolicy (rule id r1). The nil-handler and dry-run
// tests assert exact equality against it to prove the relabel path appends
// and mutates nothing.
const semanticEscalationReason = "rule r1 has semantic condition requiring LLM evaluation"

// stubEscalationHandler is an in-process EscalationHandler test double for
// the pipeline's escalation seam. It records every invocation and returns the
// configured decision/error. The pipeline calls Escalate synchronously from
// the Evaluate goroutine, so plain fields are safe to read after Evaluate
// returns.
type stubEscalationHandler struct {
	calls      int
	lastReq    GovernanceRequest
	lastReason string
	decision   *GovernanceDecision
	err        error
}

func (s *stubEscalationHandler) Escalate(_ context.Context, req GovernanceRequest, reason string) (*GovernanceDecision, error) {
	s.calls++
	s.lastReq = req
	s.lastReason = reason
	if s.err != nil {
		return nil, s.err
	}
	return s.decision, nil
}

// newEscalatePipeline builds a pipeline whose PolicyEval stage escalates every
// request via the shared semantic policy helper (rule r1, policy p-esc).
func newEscalatePipeline(cfg PipelineConfig, trust TrustChecker, auditor AuditPublisher) *Pipeline {
	ev := policyeval.NewEvaluator([]*policyeval.Policy{newSemanticEscalatePolicy("p-esc")})
	return NewPipeline(cfg, trust, ev, auditor)
}

// newEscalateRequest builds the request that trips the semantic escalate
// policy in newEscalatePipeline.
func newEscalateRequest() *GovernanceRequest {
	return &GovernanceRequest{
		ToolName:  "send_email",
		Transport: TransportMCP,
		TenantID:  "tenant-1",
	}
}

// TestPipeline_Escalation_NilHandler_RelabelUnchanged pins today's escalate
// decision shape on the nil-handler path: escalate action, the evaluator's
// EscalationReason verbatim, classified mode, and the triggering policy id.
// This is the byte-identical pass-through the Escalation seam guarantees when
// left unset (standalone mode and the policy-as-code corpus depend on it).
func TestPipeline_Escalation_NilHandler_RelabelUnchanged(t *testing.T) {
	p := newEscalatePipeline(PipelineConfig{}, nil, nil) // Escalation nil
	d, err := p.Evaluate(context.Background(), newEscalateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "escalate" {
		t.Fatalf("action = %q, want escalate", d.Action)
	}
	if d.Reason != semanticEscalationReason {
		t.Errorf("reason = %q, want exactly %q (nil handler must not touch the relabel reason)", d.Reason, semanticEscalationReason)
	}
	if d.DecisionMode != DecisionModeClassified {
		t.Errorf("mode = %q, want %q", d.DecisionMode, DecisionModeClassified)
	}
	if d.PolicyID != "p-esc" {
		t.Errorf("policy id = %q, want p-esc", d.PolicyID)
	}
	if d.DryRun {
		t.Error("DryRun must be false on the plain relabel path")
	}
}

// TestPipeline_Escalation_AdoptsResolvedVerdict covers the success path: the
// pipeline relays the handler's Action/Reason/DecisionMode — and only those
// fields (trust posture stays pipeline-owned) — while the fall-through still
// records the triggering policy and the audit event carries the relayed
// verdict and mode.
func TestPipeline_Escalation_AdoptsResolvedVerdict(t *testing.T) {
	tests := []struct {
		name       string
		resolved   *GovernanceDecision
		wantAction string
		wantReason string
		wantMode   DecisionMode
	}{
		{
			name: "approved resolution allows with human_approved",
			resolved: &GovernanceDecision{
				Action:       "allow",
				Reason:       "escalation approved by reviewer-7: looks safe",
				DecisionMode: DecisionModeHumanApproved,
				TrustScore:   0.123,           // must NOT be adopted
				TrustState:   "HANDLER-OWNED", // must NOT be adopted
			},
			wantAction: "allow",
			wantReason: "escalation approved by reviewer-7: looks safe",
			wantMode:   DecisionModeHumanApproved,
		},
		{
			name: "denied resolution denies with human_approved",
			resolved: &GovernanceDecision{
				Action:       "deny",
				Reason:       "escalation denied by reviewer-7",
				DecisionMode: DecisionModeHumanApproved,
			},
			wantAction: "deny",
			wantReason: "escalation denied by reviewer-7",
			wantMode:   DecisionModeHumanApproved,
		},
		{
			name: "empty returned mode keeps the relabel's classified",
			resolved: &GovernanceDecision{
				Action: "allow",
				Reason: "approved without a mode",
			},
			wantAction: "allow",
			wantReason: "approved without a mode",
			wantMode:   DecisionModeClassified,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubEscalationHandler{decision: tc.resolved}
			auditor := &collectingAuditor{}
			p := newEscalatePipeline(PipelineConfig{Escalation: stub}, nil, auditor)
			d, err := p.Evaluate(context.Background(), newEscalateRequest())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if stub.calls != 1 {
				t.Fatalf("handler calls = %d, want 1", stub.calls)
			}
			if stub.lastReason != semanticEscalationReason {
				t.Errorf("handler received reason %q, want %q", stub.lastReason, semanticEscalationReason)
			}
			if stub.lastReq.RequestID == "" {
				t.Error("handler must receive a non-empty RequestID (uuid-filled before stages; the await correlation key)")
			}
			if stub.lastReq.ToolName != "send_email" {
				t.Errorf("handler received tool %q, want send_email", stub.lastReq.ToolName)
			}
			if d.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (reason=%q)", d.Action, tc.wantAction, d.Reason)
			}
			if d.Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", d.Reason, tc.wantReason)
			}
			if d.DecisionMode != tc.wantMode {
				t.Errorf("mode = %q, want %q", d.DecisionMode, tc.wantMode)
			}
			// Adoption is Action/Reason/DecisionMode only: trust posture stays
			// pipeline-owned even when the handler's decision carries trust
			// fields.
			if d.TrustScore != 1.0 {
				t.Errorf("trust score = %v, want pipeline-owned 1.0", d.TrustScore)
			}
			if d.TrustState != TrustStateTrusted.String() {
				t.Errorf("trust state = %q, want pipeline-owned %q", d.TrustState, TrustStateTrusted.String())
			}
			// The triggering policy is still recorded after the verdict is
			// adopted (the case falls through to the MatchedPolicy block).
			if d.PolicyID != "p-esc" {
				t.Errorf("policy id = %q, want p-esc", d.PolicyID)
			}
			// Audit carries the relayed verdict and mode.
			events := auditor.Events()
			if len(events) != 1 {
				t.Fatalf("expected 1 audit event, got %d", len(events))
			}
			if events[0].Action != tc.wantAction || events[0].DecisionMode != tc.wantMode {
				t.Errorf("audit = %s/%s, want %s/%s", events[0].Action, events[0].DecisionMode, tc.wantAction, tc.wantMode)
			}
		})
	}
}

// TestPipeline_Escalation_UnadoptableMode_KeepsClassified pins the mode guard:
// the pipeline adopts only a vetted DecisionMode from the handler. A handler
// returning "proved" (which Boundary must never emit) or any unrecognized mode
// has its Action/Reason relayed but its mode rejected — the relabel's classified
// stays, so a buggy or hostile handler cannot stamp proved (or an out-of-set
// mode) onto a Boundary decision record or audit event. This mirrors the action
// guard's threat model for the parallel mode channel.
func TestPipeline_Escalation_UnadoptableMode_KeepsClassified(t *testing.T) {
	tests := []struct {
		name string
		mode DecisionMode
	}{
		{name: "proved is never adopted", mode: DecisionModeProved},
		{name: "unknown mode is never adopted", mode: DecisionMode("totally-made-up-mode")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubEscalationHandler{decision: &GovernanceDecision{
				Action:       "allow",
				Reason:       "escalation approved by reviewer-7",
				DecisionMode: tc.mode,
			}}
			auditor := &collectingAuditor{}
			p := newEscalatePipeline(PipelineConfig{Escalation: stub}, nil, auditor)
			d, err := p.Evaluate(context.Background(), newEscalateRequest())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Action and Reason are still relayed (the verdict is valid).
			if d.Action != "allow" {
				t.Fatalf("action = %q, want allow (a valid action is still adopted)", d.Action)
			}
			if d.Reason != "escalation approved by reviewer-7" {
				t.Errorf("reason = %q, want the relayed reason", d.Reason)
			}
			// The mode is rejected: classified (the relabel default) stays, and
			// in particular the decision never carries proved.
			if d.DecisionMode != DecisionModeClassified {
				t.Errorf("mode = %q, want %q (an unadoptable mode must not be relayed)", d.DecisionMode, DecisionModeClassified)
			}
			if d.DecisionMode == DecisionModeProved {
				t.Error("pipeline decision must never carry proved")
			}
			// The audit event must not carry the rejected mode either.
			events := auditor.Events()
			if len(events) != 1 {
				t.Fatalf("expected 1 audit event, got %d", len(events))
			}
			if events[0].DecisionMode != DecisionModeClassified {
				t.Errorf("audit mode = %q, want %q", events[0].DecisionMode, DecisionModeClassified)
			}
		})
	}
}

// TestPipeline_Escalation_HandlerError_DeniesFailClosed pins the fault
// contract: a handler error is a fault, not an approval — the decision denies
// with the exact fault-prefixed reason and the deterministic mode every other
// pipeline fault deny carries.
func TestPipeline_Escalation_HandlerError_DeniesFailClosed(t *testing.T) {
	stub := &stubEscalationHandler{err: errors.New("approver backend unreachable")}
	p := newEscalatePipeline(PipelineConfig{Escalation: stub}, nil, nil)
	d, err := p.Evaluate(context.Background(), newEscalateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("handler calls = %d, want 1", stub.calls)
	}
	if d.Action != "deny" {
		t.Fatalf("action = %q, want deny (a handler error is a fault, not an approval)", d.Action)
	}
	const want = "escalation fault (fail-closed): approver backend unreachable"
	if d.Reason != want {
		t.Errorf("reason = %q, want %q", d.Reason, want)
	}
	if d.DecisionMode != DecisionModeDeterministic {
		t.Errorf("mode = %q, want %q (a local fault is not a relayed resolution)", d.DecisionMode, DecisionModeDeterministic)
	}
}

// TestPipeline_Escalation_NilDecision_DeniesFailClosed: a handler that returns
// (nil, nil) violates its contract; the pipeline treats it as a fault and
// denies fail-closed rather than dereferencing or allowing.
func TestPipeline_Escalation_NilDecision_DeniesFailClosed(t *testing.T) {
	stub := &stubEscalationHandler{decision: nil} // nil decision, nil error
	p := newEscalatePipeline(PipelineConfig{Escalation: stub}, nil, nil)
	d, err := p.Evaluate(context.Background(), newEscalateRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "deny" {
		t.Fatalf("action = %q, want deny", d.Action)
	}
	const want = "escalation fault (fail-closed): handler returned no decision"
	if d.Reason != want {
		t.Errorf("reason = %q, want %q", d.Reason, want)
	}
	if d.DecisionMode != DecisionModeDeterministic {
		t.Errorf("mode = %q, want %q", d.DecisionMode, DecisionModeDeterministic)
	}
}

// TestPipeline_Escalation_InvalidAction_DeniesFailClosed: a handler decision
// whose Action is outside the recognized vocabulary (including empty) is a
// fault — the deny reason carries the fault prefix and names the offending
// action, so a buggy or hostile handler cannot inject an out-of-vocabulary
// action into a decision record.
func TestPipeline_Escalation_InvalidAction_DeniesFailClosed(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{name: "out-of-vocabulary action", action: "approve"},
		{name: "empty action", action: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubEscalationHandler{decision: &GovernanceDecision{
				Action:       tc.action,
				Reason:       "handler reason that must not be adopted",
				DecisionMode: DecisionModeHumanApproved,
			}}
			p := newEscalatePipeline(PipelineConfig{Escalation: stub}, nil, nil)
			d, err := p.Evaluate(context.Background(), newEscalateRequest())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Action != "deny" {
				t.Fatalf("action = %q, want deny", d.Action)
			}
			if !strings.HasPrefix(d.Reason, "escalation fault (fail-closed): ") {
				t.Errorf("reason = %q, want the fault prefix", d.Reason)
			}
			if !strings.Contains(d.Reason, fmt.Sprintf("invalid action %q", tc.action)) {
				t.Errorf("reason = %q, want it to name the invalid action %q", d.Reason, tc.action)
			}
			if d.DecisionMode != DecisionModeDeterministic {
				t.Errorf("mode = %q, want %q", d.DecisionMode, DecisionModeDeterministic)
			}
		})
	}
}

// TestPipeline_Escalation_DryRun_SkipsHandler pins the dry-run contract:
// audit-only mode must never block on an out-of-band await. With a handler
// configured, dry-run does not invoke it (zero calls) and keeps the relabel,
// appending the skip note to the reason; with no handler, the dry-run relabel
// stays byte-identical (no suffix).
func TestPipeline_Escalation_DryRun_SkipsHandler(t *testing.T) {
	t.Run("handler configured: skipped and reason notes the skip", func(t *testing.T) {
		stub := &stubEscalationHandler{decision: &GovernanceDecision{
			Action:       "allow",
			Reason:       "must never be seen under dry-run",
			DecisionMode: DecisionModeHumanApproved,
		}}
		p := newEscalatePipeline(PipelineConfig{DryRun: true, Escalation: stub}, nil, nil)
		d, err := p.Evaluate(context.Background(), newEscalateRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if stub.calls != 0 {
			t.Fatalf("handler calls = %d, want 0 (dry-run must not await)", stub.calls)
		}
		if d.Action != "escalate" {
			t.Fatalf("action = %q, want escalate (dry-run keeps the relabel)", d.Action)
		}
		want := semanticEscalationReason + " (dry-run: escalation await skipped)"
		if d.Reason != want {
			t.Errorf("reason = %q, want %q", d.Reason, want)
		}
		if d.DecisionMode != DecisionModeClassified {
			t.Errorf("mode = %q, want %q", d.DecisionMode, DecisionModeClassified)
		}
		if d.DryRun {
			t.Error("escalate is not a deny: the dry-run converter must not flag it")
		}
	})

	t.Run("no handler: dry-run relabel carries no skip suffix", func(t *testing.T) {
		p := newEscalatePipeline(PipelineConfig{DryRun: true}, nil, nil)
		d, err := p.Evaluate(context.Background(), newEscalateRequest())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Action != "escalate" {
			t.Fatalf("action = %q, want escalate", d.Action)
		}
		if d.Reason != semanticEscalationReason {
			t.Errorf("reason = %q, want exactly %q (nil handler stays byte-identical under dry-run too)", d.Reason, semanticEscalationReason)
		}
	})
}

// TestPipeline_Escalation_ApprovedAllow_DeferredIsolationFlip proves the
// deferred trust-record block still applies to an escalation-approved allow:
// when the decision's own trust update moves the agent to ISOLATED, the
// relayed allow is flipped to deny — acceptable fail-closed compounding.
func TestPipeline_Escalation_ApprovedAllow_DeferredIsolationFlip(t *testing.T) {
	stub := &stubEscalationHandler{decision: &GovernanceDecision{
		Action:       "allow",
		Reason:       "escalation approved by reviewer-7",
		DecisionMode: DecisionModeHumanApproved,
	}}
	backend := &programmableTrustBackend{
		checkState:       TrustStateTrusted,  // Stage 1 passes
		recordAfterState: TrustStateIsolated, // the deferred update isolates
	}
	p := newEscalatePipeline(PipelineConfig{Escalation: stub}, backend, nil)
	req := newEscalateRequest()
	req.AgentID = "agent-esc"
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("handler calls = %d, want 1", stub.calls)
	}
	if backend.recordCalls == 0 {
		t.Fatal("RecordDecision was never called; the deferred flip is unreached")
	}
	if d.Action != "deny" {
		t.Fatalf("action = %q, want deny (isolation flip must outrank the approved allow)", d.Action)
	}
	if !strings.Contains(d.Reason, "is ISOLATED") {
		t.Errorf("reason = %q, want the isolation reason", d.Reason)
	}
	if d.TrustState != TrustStateIsolated.String() {
		t.Errorf("trust state = %q, want %q", d.TrustState, TrustStateIsolated.String())
	}
}

// TestPipeline_StaticPolicy_ProvedMode_IsNotAdopted pins the invariant that the
// standalone pipeline never emits a proved decision. The Stage-2 static-policy
// loop copies a matching rule's decision_mode verbatim; this test confirms a
// rule with decision_mode: proved cannot make the pipeline return a decision
// carrying proved. The deny action must still take effect (the rule is
// enforced); only the mode must be rejected, leaving the safe deterministic
// default. A second case confirms that a legitimately adoptable mode
// (classified) IS propagated — the guard must not suppress valid modes.
func TestPipeline_StaticPolicy_ProvedMode_IsNotAdopted(t *testing.T) {
	tests := []struct {
		name          string
		action        string
		ruleMode      DecisionMode
		wantAction    string
		wantMode      DecisionMode
		wantNotProved bool
	}{
		{
			name:          "deny rule with proved mode must not emit proved",
			action:        "deny",
			ruleMode:      DecisionModeProved,
			wantAction:    "deny",
			wantMode:      DecisionModeDeterministic,
			wantNotProved: true,
		},
		{
			name:       "warn rule with classified mode propagates classified",
			action:     "warn",
			ruleMode:   DecisionModeClassified,
			wantAction: "warn",
			wantMode:   DecisionModeClassified,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := PipelineConfig{
				StaticPolicies: []StaticPolicyRule{
					{
						Name:         "mode-test-rule",
						Tool:         "test-tool",
						Action:       tc.action,
						Reason:       "mode guard test",
						DecisionMode: tc.ruleMode,
					},
				},
			}
			p := NewPipeline(cfg, nil, nil, nil)
			d, err := p.Evaluate(context.Background(), &GovernanceRequest{
				ToolName:  "test-tool",
				Transport: TransportMCP,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Action != tc.wantAction {
				t.Errorf("action = %q, want %q (the rule action must still be enforced)", d.Action, tc.wantAction)
			}
			if d.DecisionMode != tc.wantMode {
				t.Errorf("mode = %q, want %q", d.DecisionMode, tc.wantMode)
			}
			if tc.wantNotProved && d.DecisionMode == DecisionModeProved {
				t.Error("pipeline decision must never carry proved — static-policy mode guard violated")
			}
		})
	}
}
