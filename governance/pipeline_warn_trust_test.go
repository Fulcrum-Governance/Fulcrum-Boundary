package governance

import (
	"context"
	"testing"
)

func warnTrustPipeline(t *testing.T) (*Pipeline, *StandaloneTrustBackend) {
	t.Helper()
	trust := NewStandaloneTrustBackend(StandaloneTrustConfig{})
	p := NewPipeline(PipelineConfig{StaticPolicies: []StaticPolicyRule{{
		Name:   "warn-echo-secret",
		Tool:   "echo",
		Action: "warn",
		Reason: "echo with secret-looking text should be reviewed",
		Match:  &StaticPolicyMatch{Type: "contains", Field: "arguments.text", Contains: "secret"},
	}}}, trust, nil, nil)
	return p, trust
}

func evalEcho(t *testing.T, p *Pipeline, agent, text string) *GovernanceDecision {
	t.Helper()
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		Transport: TransportMCP,
		AgentID:   agent,
		ToolName:  "echo",
		Action:    "tools/call",
		Arguments: map[string]any{"text": text},
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestWarn_FreshAgentFirstWarnExecutes(t *testing.T) {
	p, trust := warnTrustPipeline(t)

	d := evalEcho(t, p, "agent-a", "my secret token")

	if d.Action != "warn" || d.MatchedRule != "warn-echo-secret" {
		t.Fatalf("first warn for a fresh agent: action=%q rule=%q reason=%q, want warn by warn-echo-secret", d.Action, d.MatchedRule, d.Reason)
	}
	snap, _ := trust.GetAgentTrust(context.Background(), "agent-a")
	if snap.State != TrustStateTrusted || snap.Score != 1 || snap.Known {
		t.Fatalf("trust after one warn = %+v, want the untouched default (TRUSTED, score 1, unknown)", snap)
	}
}

func TestWarn_RepeatedWarnsNeverDegradeTheAgent(t *testing.T) {
	p, trust := warnTrustPipeline(t)
	for i := 0; i < 5; i++ {
		evalEcho(t, p, "agent-b", "hello")
	}
	before, _ := trust.GetAgentTrust(context.Background(), "agent-b")

	for i := 1; i <= 25; i++ {
		if d := evalEcho(t, p, "agent-b", "my secret token"); d.Action != "warn" {
			t.Fatalf("warn #%d: action=%q reason=%q, want warn", i, d.Action, d.Reason)
		}
	}
	if d := evalEcho(t, p, "agent-b", "hello"); d.Action != "allow" {
		t.Fatalf("allow after 25 warns: action=%q reason=%q, want allow", d.Action, d.Reason)
	}

	after, _ := trust.GetAgentTrust(context.Background(), "agent-b")
	if before.Alpha != 6 || before.Beta != 1 || after.Alpha != 7 || after.Beta != 1 {
		t.Fatalf("alpha/beta before=%v/%v after=%v/%v, want 6/1 then 7/1 (warns change nothing, the final allow adds 1)", before.Alpha, before.Beta, after.Alpha, after.Beta)
	}
}

func TestWarn_DegradedAgentStillNeedsApproval(t *testing.T) {
	p, _ := warnTrustPipeline(t)
	p.staticPolicies = append(p.staticPolicies, StaticPolicyRule{Name: "deny-rm", Tool: "rm", Action: "deny"})
	if _, err := p.Evaluate(context.Background(), &GovernanceRequest{Transport: TransportMCP, AgentID: "agent-c", ToolName: "rm"}); err != nil {
		t.Fatal(err)
	}

	d := evalEcho(t, p, "agent-c", "my secret token")

	if d.Action != "require_approval" || d.Reason != "agent agent-c is degraded" {
		t.Fatalf("warn from a degraded agent: action=%q reason=%q, want require_approval (agent agent-c is degraded)", d.Action, d.Reason)
	}
}
