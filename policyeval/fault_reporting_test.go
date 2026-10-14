package policyeval

import (
	"context"
	"testing"
)

// FUL-732: the evaluator reports faults it used to only log, so callers can
// classify them as CHECK_INDETERMINATE instead of trusting the verdict.

func TestEvaluateReportsSkippedRuleFaults(t *testing.T) {
	e := NewEvaluator([]*Policy{badRegexDenyPolicy()})
	d, err := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.EvaluationErrors) == 0 {
		t.Fatalf("EvaluationErrors empty; action %s reason %q", d.Action, d.Reason)
	}
	if d.StrictValidationFailed {
		t.Fatal("non-strict evaluator reported StrictValidationFailed")
	}
}

func TestEvaluateReportsStrictValidationFailure(t *testing.T) {
	e, err := NewEvaluatorStrict([]*Policy{badRegexDenyPolicy()})
	if err == nil {
		t.Fatal("expected strict validation error")
	}
	d, err := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1", InputText: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !d.StrictValidationFailed || d.Action != ActionDeny {
		t.Fatalf("strict refusal = %s StrictValidationFailed=%t, want deny/true", d.Action, d.StrictValidationFailed)
	}
}

func TestEvaluateHealthyHasNoFaults(t *testing.T) {
	e := NewEvaluator(nil)
	d, err := e.Evaluate(context.Background(), &EvaluationRequest{TenantID: "tenant1"})
	if err != nil {
		t.Fatal(err)
	}
	if d.StrictValidationFailed || len(d.EvaluationErrors) != 0 {
		t.Fatalf("healthy evaluation reported faults: %+v", d)
	}
}
