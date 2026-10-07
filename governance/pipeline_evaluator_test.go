package governance

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// errorEvaluator is a PolicyEvaluator stub that always returns an error.
// It exercises the fail-closed-vs-fail-open branch in Pipeline.Evaluate
// that the stock *policyeval.Evaluator cannot reach.
type errorEvaluator struct{ err error }

func (e *errorEvaluator) Evaluate(_ context.Context, _ *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return nil, e.err
}

// TestPipeline_EvaluatorError_EnforcingTransport_CheckIndeterminate exercises
// the enforcing branch: enforcement is the default, so an empty config must
// return CHECK_INDETERMINATE on evaluator error — ADR-047 requires a
// required-check failure to block without being labeled allow or a
// substantive policy deny. The reason surfaces the stage, category, and a
// fixed-vocabulary cause; the raw error stays operator-side in Check.Detail.
func TestPipeline_EvaluatorError_EnforcingTransport_CheckIndeterminate(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	p := NewPipeline(PipelineConfig{}, nil, ev, nil)

	req := &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
		TenantID:  "t1",
	}
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("expected check_indeterminate on evaluator error for enforcing transport, got %s", d.Action)
	}
	if d.Allowed() {
		t.Fatal("check_indeterminate must not be executable")
	}
	if !strings.Contains(d.Reason, "policy evaluation failed") {
		t.Errorf("expected reason to surface the failed check, got %q", d.Reason)
	}
	// The caller-facing reason/cause carry the fixed-vocabulary description
	// only; the raw evaluator error is operator-side diagnostics.
	if strings.Contains(d.Reason, "evaluator unavailable") {
		t.Errorf("raw evaluator error must not reach the caller-facing reason, got %q", d.Reason)
	}
	if d.Check == nil || d.Check.Category != FailureUnavailable || d.Check.Stage != CheckStagePolicyEval {
		t.Errorf("check context = %+v, want policy_eval/unavailable", d.Check)
	}
	if d.Check != nil {
		if strings.Contains(d.Check.Cause, "evaluator unavailable") {
			t.Errorf("raw evaluator error must not reach the serialized cause, got %q", d.Check.Cause)
		}
		if !strings.Contains(d.Check.Detail, "evaluator unavailable") {
			t.Errorf("check detail = %q, want the raw evaluator error preserved operator-side", d.Check.Detail)
		}
	}
}

// TestPipeline_EvaluatorError_NonEnforcingTransport_Allows verifies the
// declared non-enforcing branch. A transport explicitly named in
// NonEnforcingTransports (ADR-047 can_deny=false) retains the default allow
// action when the evaluator errors, but the decision records the
// would-have-blocked CHECK_INDETERMINATE context rather than looking like a
// clean allow.
func TestPipeline_EvaluatorError_NonEnforcingTransport_Allows(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook, Reason: "informational webhook sink; cannot block upstream"},
	}}
	p := NewPipeline(cfg, nil, ev, nil)

	req := &GovernanceRequest{
		ToolName:  "health_check",
		Transport: TransportWebhook,
		TenantID:  "t1",
	}
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("expected allow on evaluator error for non-enforcing transport, got %s", d.Action)
	}
	if d.Check == nil || d.Check.Category != FailureUnavailable || d.Check.Stage != CheckStagePolicyEval {
		t.Errorf("non-enforcing allow must record the would-have-blocked check, got %+v", d.Check)
	}
}

// TestPipeline_EvaluatorError_AuditEmittedOnBothPaths confirms that both the
// enforcing and non-enforcing branches still emit exactly one audit event —
// the defer hook in Evaluate runs regardless of which branch the evaluator
// error takes — and that both events carry the check context.
func TestPipeline_EvaluatorError_AuditEmittedOnBothPaths(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	auditor := &collectingAuditor{}
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook, Reason: "informational webhook sink; cannot block upstream"},
	}}
	p := NewPipeline(cfg, nil, ev, auditor)

	// Enforcing request (MCP is not declared non-enforcing).
	_, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "a", Transport: TransportMCP, TenantID: "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-enforcing request (Webhook is the declared exception).
	_, err = p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "b", Transport: TransportWebhook, TenantID: "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := auditor.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events (one per Evaluate), got %d", len(events))
	}
	if events[0].Action != ActionCheckIndeterminate {
		t.Errorf("enforcing path: expected audit action check_indeterminate, got %s", events[0].Action)
	}
	if events[0].Check == nil {
		t.Error("enforcing audit event must carry check context")
	}
	if events[1].Action != "allow" {
		t.Errorf("non-enforcing path: expected audit action allow, got %s", events[1].Action)
	}
	if events[1].Check == nil {
		t.Error("non-enforcing audit event must record the would-have-blocked check")
	}
}

// TestPipeline_EvaluatorNil_FallsBackToDefault confirms the NewPipeline
// behavior preserved from before the interface extraction: passing nil
// evaluator installs a default real *policyeval.Evaluator rather than leaving
// the pipeline with a nil evaluator that would panic in Stage 4.
func TestPipeline_EvaluatorNil_FallsBackToDefault(t *testing.T) {
	p := NewPipeline(PipelineConfig{}, nil, nil, nil) // nil evaluator
	req := &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
		TenantID:  "t1",
	}
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Errorf("expected allow with default evaluator and no policies, got %s", d.Action)
	}
}

// TestPipeline_ConcreteEvaluator_StillAccepted verifies the interface change
// is backwards-compatible: a concrete *policyeval.Evaluator passed to
// NewPipeline still works because it satisfies PolicyEvaluator.
func TestPipeline_ConcreteEvaluator_StillAccepted(t *testing.T) {
	ev := policyeval.NewEvaluator(nil) // concrete type
	p := NewPipeline(PipelineConfig{}, nil, ev, nil)
	req := &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportMCP,
		TenantID:  "t1",
	}
	d, err := p.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Errorf("expected allow, got %s", d.Action)
	}
}

// TestPipeline_EvaluatorError_UndeclaredTransport_CheckIndeterminate pins the
// ADR-047 default-enforcing rule: a required-check failure must block on ANY
// transport that is not explicitly declared non-enforcing — including an
// empty transport name, a transport the pipeline does not know, and a custom
// adapter transport. Under the pre-inversion fail-closed LIST these rows all
// failed open (allow) because the map lookup missed; an unknown result must
// never authorize.
func TestPipeline_EvaluatorError_UndeclaredTransport_CheckIndeterminate(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	p := NewPipeline(PipelineConfig{}, nil, ev, nil)

	for _, transport := range []TransportType{"", "bogus", TransportType("claude-code-hook")} {
		t.Run("transport="+string(transport), func(t *testing.T) {
			d, err := p.Evaluate(context.Background(), &GovernanceRequest{
				ToolName:  "read_file",
				Transport: transport,
				TenantID:  "t1",
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Action != ActionCheckIndeterminate {
				t.Fatalf("action = %q, want %q — an undeclared transport must enforce", d.Action, ActionCheckIndeterminate)
			}
			if d.Allowed() {
				t.Fatal("check_indeterminate must not allow execution")
			}
			if d.Check == nil || d.Check.Stage != CheckStagePolicyEval {
				t.Errorf("check = %+v, want policy_eval context recorded", d.Check)
			}
		})
	}
}

// TestPipeline_EvaluatorError_OmissionIsNotNonEnforcing pins the second
// fail-open loophole the inverted model closes: declaring one transport
// non-enforcing must NOT silently disable enforcement on any other. The old
// FailClosedTransports:[mcp] shape allowed a cli request through on an
// evaluator error; now only the named transport is exempt and every other
// transport — cli included — still blocks.
func TestPipeline_EvaluatorError_OmissionIsNotNonEnforcing(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook, Reason: "informational webhook sink; cannot block upstream"},
	}}
	p := NewPipeline(cfg, nil, ev, nil)

	// The declared non-enforcing surface continues and records.
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "notify", Transport: TransportWebhook, TenantID: "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "allow" {
		t.Fatalf("declared non-enforcing transport should continue; got %q", d.Action)
	}
	if d.Check == nil {
		t.Fatal("non-enforcing continuation must record the would-have-blocked check")
	}

	// Every transport NOT declared non-enforcing still enforces — the
	// omission loophole is closed.
	for _, transport := range []TransportType{TransportMCP, TransportCLI, TransportGRPC, "bogus", ""} {
		d, err := p.Evaluate(context.Background(), &GovernanceRequest{
			ToolName: "read_file", Transport: transport, TenantID: "t1",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Action != ActionCheckIndeterminate {
			t.Fatalf("transport %q: action = %q, want %q — omission must not disable enforcement", transport, d.Action, ActionCheckIndeterminate)
		}
		if d.Allowed() {
			t.Fatalf("transport %q: check_indeterminate must not allow execution", transport)
		}
	}
}
