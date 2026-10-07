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
// the enforcing branch: a transport in the FailClosedTransports set must
// return CHECK_INDETERMINATE on evaluator error — ADR-047 requires a
// required-check failure to block without being labeled allow or a
// substantive policy deny. The reason surfaces the stage, category, and
// underlying cause.
func TestPipeline_EvaluatorError_EnforcingTransport_CheckIndeterminate(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	cfg := PipelineConfig{FailClosedTransports: []TransportType{TransportMCP}}
	p := NewPipeline(cfg, nil, ev, nil)

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
	if !strings.Contains(d.Reason, "evaluator unavailable") {
		t.Errorf("expected reason to wrap the underlying error, got %q", d.Reason)
	}
	if d.Check == nil || d.Check.Category != FailureUnavailable || d.Check.Stage != CheckStagePolicyEval {
		t.Errorf("check context = %+v, want policy_eval/unavailable", d.Check)
	}
}

// TestPipeline_EvaluatorError_NonEnforcingTransport_Allows verifies the
// declared non-enforcing branch. A transport NOT in the FailClosedTransports
// set (ADR-047 can_deny=false) retains the default allow action when the
// evaluator errors, but the decision records the would-have-blocked
// CHECK_INDETERMINATE context rather than looking like a clean allow.
func TestPipeline_EvaluatorError_NonEnforcingTransport_Allows(t *testing.T) {
	ev := &errorEvaluator{err: fmt.Errorf("evaluator unavailable")}
	cfg := PipelineConfig{FailClosedTransports: []TransportType{TransportMCP}}
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
	cfg := PipelineConfig{FailClosedTransports: []TransportType{TransportMCP}}
	p := NewPipeline(cfg, nil, ev, auditor)

	// Enforcing request.
	_, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "a", Transport: TransportMCP, TenantID: "t1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Non-enforcing request (Webhook is not in the explicit enforcing list).
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
