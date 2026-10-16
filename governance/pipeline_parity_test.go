package governance_test

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/adapterparity"
)

// TestEvaluateAlwaysDecides runs the parity table against Pipeline.Evaluate
// itself. Adapters ignore Evaluate's error result and act on the decision, so
// every case must come back as a non-nil decision with a nil error.
func TestEvaluateAlwaysDecides(t *testing.T) {
	adapterparity.Run(t, adapterparity.Adapter{
		Name: "pipeline",
		Tool: "parity_tool",
		NotApplicable: map[string]string{
			"malformed_input": "input parsing happens in the adapter, before Evaluate",
		},
		Call: func(ctx context.Context, t *testing.T, p *governance.Pipeline, _ adapterparity.Input) adapterparity.Outcome {
			decision, err := p.Evaluate(ctx, &governance.GovernanceRequest{
				RequestID: "parity-request", Transport: governance.TransportMCP,
				AgentID: "parity-agent", TenantID: "parity-tenant", ToolName: "parity_tool", Action: "tools/call",
			})
			if err != nil || decision == nil {
				t.Fatalf("Evaluate = (%v, %v), want a decision and no error", decision, err)
			}
			out := adapterparity.Outcome{Action: decision.Action}
			if decision.Allowed() {
				out.Executed = 1
			}
			return out
		},
	})
}
