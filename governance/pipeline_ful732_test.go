package governance

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// FUL-732 (ADR-047): every policy-evaluation fault the stock evaluator can
// produce returns CHECK_INDETERMINATE (never allow, never a matched deny),
// and the emitted decision record carries the classification.

func badRegexDenyPolicyForTool(id string) *policyeval.Policy {
	return &policyeval.Policy{
		PolicyId: id,
		TenantId: "tenant-1",
		Name:     "bad regex deny " + id,
		Priority: 100,
		Status:   policyeval.PolicyStatus_POLICY_STATUS_ACTIVE,
		Rules: []*policyeval.PolicyRule{{
			RuleId:  "r-bad",
			Name:    "typo'd deny",
			Enabled: true,
			Conditions: []*policyeval.PolicyCondition{{
				ConditionType: policyeval.ConditionType_CONDITION_TYPE_REGEX,
				Field:         "tool.name",
				Operator:      policyeval.ConditionOperator_CONDITION_OPERATOR_MATCHES,
				Value:         &policyeval.PolicyCondition_StringValue{StringValue: "[invalid"},
			}},
			Actions: []*policyeval.PolicyAction{{
				ActionType: policyeval.PolicyActionType_ACTION_TYPE_DENY,
				Message:    "should deny",
			}},
		}},
	}
}

func validDenyPolicyForTool(id, tool string) *policyeval.Policy {
	return &policyeval.Policy{
		PolicyId: id,
		TenantId: "tenant-1",
		Name:     "deny " + tool,
		Priority: 50,
		Status:   policyeval.PolicyStatus_POLICY_STATUS_ACTIVE,
		Rules: []*policyeval.PolicyRule{{
			RuleId:  "r-ok",
			Name:    "deny tool",
			Enabled: true,
			Conditions: []*policyeval.PolicyCondition{{
				ConditionType: policyeval.ConditionType_CONDITION_TYPE_FIELD_MATCH,
				Field:         "tool.name",
				Operator:      policyeval.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				Value:         &policyeval.PolicyCondition_StringValue{StringValue: tool},
			}},
			Actions: []*policyeval.PolicyAction{{
				ActionType: policyeval.PolicyActionType_ACTION_TYPE_DENY,
				Message:    "denied tool",
			}},
		}},
	}
}

func evalOnce(t *testing.T, p *Pipeline, transport TransportType) *GovernanceDecision {
	t.Helper()
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: transport,
		TenantID:  "tenant-1",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return d
}

func assertIndeterminate(t *testing.T, d *GovernanceDecision, auditor *collectingAuditor, category FailureCategory) {
	t.Helper()
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("action = %q (reason %q), want %q", d.Action, d.Reason, ActionCheckIndeterminate)
	}
	if d.Allowed() {
		t.Fatal("check_indeterminate must not allow")
	}
	if d.Check == nil || d.Check.Stage != CheckStagePolicyEval || d.Check.Category != category {
		t.Fatalf("check = %+v, want policy_eval/%s", d.Check, category)
	}
	events := auditor.Events()
	if len(events) != 1 {
		t.Fatalf("audit events = %d, want 1", len(events))
	}
	record := BuildDecisionRecord(events[0])
	if record.Action != ActionCheckIndeterminate || record.Check == nil || record.Check.Category != category {
		t.Fatalf("decision record action=%q check=%+v, want %s/%s", record.Action, record.Check, ActionCheckIndeterminate, category)
	}
}

// Path 1: nil evaluator. The fallback must be strict, so a policy set loaded
// into it that faults cannot be silently skipped into an allow.
func TestFUL732_NilEvaluatorFallbackIsStrict(t *testing.T) {
	auditor := &collectingAuditor{}
	p := NewPipeline(PipelineConfig{}, nil, nil, auditor)
	if p.ConfigError() != nil {
		t.Fatalf("config error: %v", p.ConfigError())
	}
	fallback, ok := p.evaluator.(*policyeval.Evaluator)
	if !ok {
		t.Fatalf("fallback evaluator type %T", p.evaluator)
	}
	fallback.UpdatePolicies([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")})
	d := evalOnce(t, p, TransportMCP)
	assertIndeterminate(t, d, auditor, FailureMissingConfig)
}

// The nil-evaluator fallback with no policies keeps the default allow:
// switching to strict must not change healthy outcomes.
func TestFUL732_NilEvaluatorFallbackHealthyAllows(t *testing.T) {
	p := NewPipeline(PipelineConfig{}, nil, nil, &collectingAuditor{})
	d := evalOnce(t, p, TransportMCP)
	if d.Action != "allow" || d.Check != nil {
		t.Fatalf("healthy fallback = %q check %+v, want allow with no check", d.Action, d.Check)
	}
}

// Path 2: caller-supplied strict evaluator holding an invalid set. Before,
// the pipeline relabeled its deny-shaped refusal as a substantive deny.
func TestFUL732_StrictValidationFailureIsIndeterminateNotDeny(t *testing.T) {
	ev, err := policyeval.NewEvaluatorStrict([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")})
	if err == nil {
		t.Fatal("expected strict validation error for the fixture")
	}
	auditor := &collectingAuditor{}
	p := NewPipeline(PipelineConfig{}, nil, ev, auditor)
	d := evalOnce(t, p, TransportMCP)
	assertIndeterminate(t, d, auditor, FailureMissingConfig)
}

// Path 3: caller-supplied non-strict evaluator whose rule faults at
// evaluation time. Before, the rule was skipped and the request allowed.
func TestFUL732_NonStrictRuleFaultIsIndeterminateNotAllow(t *testing.T) {
	ev := policyeval.NewEvaluator([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")})
	auditor := &collectingAuditor{}
	p := NewPipeline(PipelineConfig{}, nil, ev, auditor)
	d := evalOnce(t, p, TransportMCP)
	assertIndeterminate(t, d, auditor, FailureInvalidResult)
}

// Path 3b: a valid deny still fires alongside a faulting rule. The deny
// stands (it already blocks) and the fault is recorded on Check.
func TestFUL732_RuleFaultAlongsideValidDenyKeepsDenyAndRecordsCheck(t *testing.T) {
	ev := policyeval.NewEvaluator([]*policyeval.Policy{
		badRegexDenyPolicyForTool("bad"),
		validDenyPolicyForTool("ok", "read_file"),
	})
	p := NewPipeline(PipelineConfig{}, nil, ev, &collectingAuditor{})
	d := evalOnce(t, p, TransportMCP)
	if d.Action != "deny" {
		t.Fatalf("action = %q, want deny", d.Action)
	}
	if d.Check == nil || d.Check.Category != FailureInvalidResult {
		t.Fatalf("check = %+v, want the skipped fault recorded", d.Check)
	}
}

// Non-enforcing transports continue and record the would-have-blocked result
// for both new fault paths.
func TestFUL732_NonEnforcingTransportRecordsWouldHaveBlocked(t *testing.T) {
	cfg := PipelineConfig{NonEnforcingTransports: []NonEnforcingTransport{
		{Transport: TransportWebhook, Reason: "informational sink"},
	}}
	strict, _ := policyeval.NewEvaluatorStrict([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")})
	for name, ev := range map[string]PolicyEvaluator{
		"strict validation": strict,
		"rule fault":        policyeval.NewEvaluator([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")}),
	} {
		p := NewPipeline(cfg, nil, ev, &collectingAuditor{})
		d := evalOnce(t, p, TransportWebhook)
		if d.Action != "allow" || d.Check == nil || d.Check.Stage != CheckStagePolicyEval {
			t.Fatalf("%s: non-enforcing = %q check %+v, want allow with recorded check", name, d.Action, d.Check)
		}
		// The same evaluator on an enforcing transport blocks.
		d = evalOnce(t, p, TransportMCP)
		if d.Action != ActionCheckIndeterminate {
			t.Fatalf("%s: enforcing = %q, want check_indeterminate", name, d.Action)
		}
	}
}
