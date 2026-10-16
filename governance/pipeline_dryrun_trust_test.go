package governance

import (
	"context"
	"testing"
)

// A deny that dry-run lets through was never enforced, so repeating it must
// leave the agent's live trust untouched.
func TestPipeline_DryRunDenyLeavesTrustUnchanged(t *testing.T) {
	trust := NewStandaloneTrustBackend(StandaloneTrustConfig{})
	p := NewPipeline(PipelineConfig{
		DryRun:         true,
		StaticPolicies: []StaticPolicyRule{{Name: "deny-drop", Tool: "drop", Action: "deny", Reason: "no"}},
	}, trust, nil, nil)

	for i := 0; i < 10; i++ {
		d, err := p.Evaluate(context.Background(), &GovernanceRequest{Transport: TransportMCP, ToolName: "drop", AgentID: "agent-a"})
		if err != nil {
			t.Fatal(err)
		}
		if d.Action != "allow" || !d.DryRun {
			t.Fatalf("decision = {%s dry_run=%t}, want allow with dry_run", d.Action, d.DryRun)
		}
	}

	snap, err := trust.GetAgentTrust(context.Background(), "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Known || snap.State != TrustStateTrusted {
		t.Fatalf("trust after 10 dry-run denies = %+v, want an untouched agent", snap)
	}
}
