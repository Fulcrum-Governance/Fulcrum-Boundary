package governance

import (
	"context"
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
