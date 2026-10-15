package governance

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
)

// Review follow-up (#252): a deny that stood on a cleanly evaluated rule while
// a sibling rule faulted must still feed the agent's trust score.
func TestFUL732_MixedFaultDenyStillUpdatesTrust(t *testing.T) {
	ev := policyeval.NewEvaluator([]*policyeval.Policy{
		badRegexDenyPolicyForTool("bad"),
		validDenyPolicyForTool("ok", "read_file"),
	})
	trust := &programmableTrustBackend{checkState: TrustStateTrusted, recordAfterState: TrustStateTrusted}
	p := NewPipeline(PipelineConfig{}, trust, ev, &collectingAuditor{})
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "read_file", Transport: TransportMCP, TenantID: "tenant-1", AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.Action != "deny" || d.Check == nil || d.Check.Category != FailureInvalidResult {
		t.Fatalf("action=%q check=%+v, want deny with the skipped fault recorded as invalid_result", d.Action, d.Check)
	}
	if trust.recordCalls != 1 {
		t.Fatalf("trust RecordDecision calls = %d, want 1", trust.recordCalls)
	}
}

// A fault that turns the verdict into CHECK_INDETERMINATE is not a policy
// outcome and must still not move trust.
func TestFUL732_IndeterminateFaultDoesNotUpdateTrust(t *testing.T) {
	ev := policyeval.NewEvaluator([]*policyeval.Policy{badRegexDenyPolicyForTool("bad")})
	trust := &programmableTrustBackend{checkState: TrustStateTrusted, recordAfterState: TrustStateTrusted}
	p := NewPipeline(PipelineConfig{}, trust, ev, &collectingAuditor{})
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName: "read_file", Transport: TransportMCP, TenantID: "tenant-1", AgentID: "agent-1",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.Action != ActionCheckIndeterminate {
		t.Fatalf("action=%q, want %q", d.Action, ActionCheckIndeterminate)
	}
	if trust.recordCalls != 0 {
		t.Fatalf("trust RecordDecision calls = %d, want 0", trust.recordCalls)
	}
}

type timedOutEvaluator struct{}

func (timedOutEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return &policyeval.Decision{
		Action:             policyeval.ActionAllow,
		EvaluationErrors:   []string{"policy p rule r: external call failed: context deadline exceeded"},
		EvaluationTimedOut: true,
	}, nil
}

// Review follow-up (#252): a skipped rule that timed out is classified as a
// timeout, not an invalid result.
func TestFUL732_TimedOutRuleIsTimeoutCategory(t *testing.T) {
	auditor := &collectingAuditor{}
	p := NewPipeline(PipelineConfig{}, nil, timedOutEvaluator{}, auditor)
	assertIndeterminate(t, evalOnce(t, p, TransportMCP), auditor, FailureTimeout)
}
