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
