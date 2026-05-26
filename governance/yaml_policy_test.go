package governance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStaticPolicyFiles_LoadsRulesAndMetadata(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`name: postgres-production
version: "1.0"
rules:
  - name: block-drop-table
    tool: query
    action: deny
    reason: blocked
    match:
      field: arguments.sql
      contains: DROP TABLE
      case_insensitive: true
`)
	if err := os.WriteFile(filepath.Join(dir, "postgres.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadStaticPolicyFiles(dir)
	if err != nil {
		t.Fatalf("load policies: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(result.Files))
	}
	if len(result.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Rules))
	}
	rule := result.Rules[0]
	if rule.Name != "block-drop-table" {
		t.Fatalf("unexpected rule name %q", rule.Name)
	}
	if rule.PolicyFile != "postgres.yaml" {
		t.Fatalf("expected policy file metadata, got %q", rule.PolicyFile)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", result.Warnings)
	}
}

func TestStaticPolicyMatch_ArgumentsSQLCaseInsensitive(t *testing.T) {
	cfg := PipelineConfig{
		StaticPolicies: []StaticPolicyRule{
			{
				Name:   "block-drop-table",
				Tool:   "query",
				Action: "deny",
				Reason: "blocked",
				Match: &StaticPolicyMatch{
					Field:           "arguments.sql",
					Contains:        "DROP TABLE",
					CaseInsensitive: true,
				},
				PolicyFile: "postgres.yaml",
			},
		},
		GatewayVersion: "test-version",
	}
	p := NewPipeline(cfg, nil, nil, nil)

	decision, err := p.Evaluate(context.Background(), &GovernanceRequest{
		Transport: TransportMCP,
		ToolName:  "query",
		Arguments: map[string]any{"sql": "drop table users"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Action != "deny" {
		t.Fatalf("expected deny, got %s", decision.Action)
	}
	if decision.MatchedRule != "block-drop-table" {
		t.Fatalf("expected matched rule, got %q", decision.MatchedRule)
	}
	if decision.PolicyFile != "postgres.yaml" {
		t.Fatalf("expected policy file, got %q", decision.PolicyFile)
	}
	if decision.GatewayVersion != "test-version" {
		t.Fatalf("expected gateway version, got %q", decision.GatewayVersion)
	}
}
