package governance

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadStaticPolicyFiles_LoadsPolicyV1Document(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`schema_version: "1"
policy:
  name: postgres-production
  version: "1.0.0"
  transport: mcp
  rules:
    - name: deny-destructive
      tool: query
      action: deny
      decision_mode: classified
      tenant_scope: ["tenant-1"]
      conditions:
        - type: ast_class
          value: DESTRUCTIVE
`)
	if err := os.WriteFile(filepath.Join(dir, "postgres.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := LoadStaticPolicyFiles(dir)
	if err != nil {
		t.Fatalf("load policies: %v", err)
	}
	if len(result.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(result.Rules))
	}
	rule := result.Rules[0]
	if rule.Transport != "mcp" {
		t.Fatalf("expected transport projection, got %q", rule.Transport)
	}
	if rule.DecisionMode != DecisionModeClassified {
		t.Fatalf("expected classified decision mode, got %q", rule.DecisionMode)
	}
	if len(rule.Conditions) != 1 || rule.Conditions[0].Type != "ast_class" {
		t.Fatalf("unexpected conditions: %#v", rule.Conditions)
	}
}

func TestLoadStaticPolicyFiles_InvalidPolicyV1ReturnsError(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`schema_version: "1"
policy:
  name: broken
  version: "1.0.0"
  rules:
    - name: invalid
      tool: query
      action: deny
      conditions:
        - type: regex
          field: arguments.sql
          regex: "["
`)
	if err := os.WriteFile(filepath.Join(dir, "broken.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadStaticPolicyFiles(dir); err == nil {
		t.Fatal("expected v1 schema validation error")
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

func TestLoadStaticPolicyFiles_LegacyActionIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`name: mixed-case
version: "1"
rules:
  - name: deny-drop
    tool: query
    action: " Deny "
`)
	if err := os.WriteFile(filepath.Join(dir, "deny.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := LoadStaticPolicyFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPipeline(PipelineConfig{StaticPolicies: result.Rules}, nil, nil, nil)

	d, err := p.Evaluate(context.Background(), &GovernanceRequest{Transport: TransportMCP, ToolName: "query"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Action != "deny" || d.MatchedRule != "deny-drop" {
		t.Fatalf("legacy action %q: decision action=%q rule=%q, want deny by deny-drop", " Deny ", d.Action, d.MatchedRule)
	}
}

func TestLoadStaticPolicyFiles_LegacyEmptyActionIsRejected(t *testing.T) {
	dir := t.TempDir()
	policy := []byte(`name: no-action
version: "1"
rules:
  - name: forgot-action
    tool: query
`)
	if err := os.WriteFile(filepath.Join(dir, "empty.yaml"), policy, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadStaticPolicyFiles(dir)
	want := `empty.yaml: rules[0] "forgot-action": unsupported action "" (want one of: allow, deny, warn, audit, escalate, require_approval)`
	if err == nil || !strings.HasSuffix(err.Error(), want) {
		t.Fatalf("load error = %v, want suffix %q", err, want)
	}
}
