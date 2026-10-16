package governance

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPipeline_LaterStageRaisesButNeverLowersStaticVerdict(t *testing.T) {
	for _, tc := range []struct {
		name, ruleAction, interceptorAction, want string
	}{
		{"interceptor deny beats static warn", "warn", "deny", "deny"},
		{"interceptor deny beats static require_approval", "require_approval", "deny", "deny"},
		{"interceptor warn keeps static require_approval", "require_approval", "warn", "require_approval"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPipeline(PipelineConfig{StaticPolicies: []StaticPolicyRule{{Name: "rule", Tool: "query", Action: tc.ruleAction, Reason: "static"}}}, nil, nil, nil)
			p.RegisterInterceptor("query", func(context.Context, *GovernanceRequest) (*InterceptorResult, error) {
				return &InterceptorResult{Allowed: false, Action: tc.interceptorAction, Reason: "interceptor"}, nil
			})

			d, err := p.Evaluate(context.Background(), &GovernanceRequest{Transport: TransportMCP, ToolName: "query"})
			if err != nil {
				t.Fatal(err)
			}
			if d.Action != tc.want {
				t.Fatalf("action = %q (reason %q), want %q", d.Action, d.Reason, tc.want)
			}
		})
	}
}

func TestPipeline_ApprovedEscalationKeepsEarlierWarn(t *testing.T) {
	cfg := PipelineConfig{
		StaticPolicies: []StaticPolicyRule{{Name: "warn-email", Tool: "send_email", Action: "warn", Reason: "review email"}},
		Escalation:     &stubEscalationHandler{decision: &GovernanceDecision{Action: "allow", Reason: "approved", DecisionMode: DecisionModeHumanApproved}},
	}
	d, err := newEscalatePipeline(cfg, nil, nil).Evaluate(context.Background(), newEscalateRequest())
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != "warn" || d.Reason != "review email" || d.MatchedRule != "warn-email" {
		t.Fatalf("decision = {%s %q rule=%q}, want the earlier warn by warn-email", d.Action, d.Reason, d.MatchedRule)
	}
}

func TestPipeline_CheckFailureAfterWarnDropsTheWarnAttribution(t *testing.T) {
	warn := []StaticPolicyRule{{Name: "warn-query", Tool: "query", Action: "warn", Reason: "review"}}
	for _, tc := range []struct {
		name  string
		build func() *Pipeline
	}{
		{"interceptor error", func() *Pipeline {
			p := NewPipeline(PipelineConfig{StaticPolicies: warn}, nil, nil, nil)
			p.RegisterInterceptor("query", func(context.Context, *GovernanceRequest) (*InterceptorResult, error) {
				return nil, errors.New("parser down")
			})
			return p
		}},
		{"evaluator failure", func() *Pipeline {
			return NewPipeline(PipelineConfig{StaticPolicies: warn}, nil, panicEvaluator{}, nil)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := tc.build().Evaluate(context.Background(), &GovernanceRequest{Transport: TransportMCP, ToolName: "query"})
			if err != nil {
				t.Fatal(err)
			}
			if d.Action != ActionCheckIndeterminate || d.MatchedRule != "" || !reflect.DeepEqual(d.MatchedRules, []string{"warn-query"}) {
				t.Fatalf("decision = {%s rule=%q rules=%v}, want check_indeterminate with no winning rule and matched_rules [warn-query]", d.Action, d.MatchedRule, d.MatchedRules)
			}
		})
	}
}
