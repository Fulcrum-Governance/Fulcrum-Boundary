package governance

import (
	"context"
	"testing"
)

func TestNilPipelineEvaluateIsCheckIndeterminate(t *testing.T) {
	var p *Pipeline
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{ToolName: "deploy", Transport: TransportMCP})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.Action != ActionCheckIndeterminate || d.Allowed() {
		t.Fatalf("action = %q, want check_indeterminate", d.Action)
	}
	if d.Check == nil || d.Check.Stage != CheckStageConfig || d.Check.Category != FailureMissingConfig {
		t.Fatalf("check = %+v, want config/missing_config", d.Check)
	}
	if d.Reason != "check indeterminate (config/missing_config): governance pipeline is not configured" {
		t.Fatalf("reason = %q", d.Reason)
	}
	if d.RequestID == "" || d.EnvelopeID == "" {
		t.Fatalf("decision carries no request or envelope id: %+v", d)
	}
}
