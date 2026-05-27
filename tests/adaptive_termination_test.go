package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/fulcrum-governance/boundary/governance"
)

func TestAdaptiveTerminationIsolatesRepeatedViolations(t *testing.T) {
	auditor := &captureAuditPublisher{}
	trust := governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{
		InitialAlpha: 5,
		InitialBeta:  1,
	})
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "block-danger", Tool: "danger", Action: "deny", Reason: "blocked"},
		},
		RequireAgentID: true,
	}, trust, nil, auditor)

	ctx := context.Background()
	req := &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		AgentID:   "agent-1",
		TenantID:  "tenant-1",
		ToolName:  "danger",
		Action:    "tools/call",
	}

	var decision *governance.GovernanceDecision
	for i := 0; i < 8; i++ {
		var err error
		decision, err = pipeline.Evaluate(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
	}
	if decision.Action != "deny" || decision.TrustState != governance.TrustStateIsolated.String() {
		t.Fatalf("agent was not isolated: %#v", decision)
	}

	req.ToolName = "safe"
	req.Action = "tools/call"
	decision, err := pipeline.Evaluate(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != "deny" || !strings.Contains(decision.Reason, "ISOLATED") {
		t.Fatalf("isolated agent was not denied before execution: %#v", decision)
	}

	var sawTransition bool
	for _, event := range auditor.events {
		if event.EventType == "trust_transition" && event.TrustState == governance.TrustStateIsolated.String() {
			sawTransition = true
		}
	}
	if !sawTransition {
		t.Fatalf("missing auditable isolation transition: %#v", auditor.events)
	}
}
