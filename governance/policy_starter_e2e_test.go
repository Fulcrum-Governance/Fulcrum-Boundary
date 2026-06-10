package governance_test

// This file lives in the external governance_test package (not package
// governance) on purpose: it imports internal/firewall to load the *real*
// shipped starter policy body, and internal/firewall imports governance, so an
// in-package test would create an import cycle. Loading the canonical starter
// YAML (rather than a hand-copied rule) proves the rule that ships to operators
// — internal/firewall/policies.go, postgres template, FIRST deny rule
// (deny-destructive-sql-class, type ast_class / value DESTRUCTIVE) — denies on
// the static rule's OWN matching, with no Stage-3 SQL interceptor registered.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/firewall"
)

// starterPostgresBody returns the canonical postgres starter policy YAML body
// shipped by internal/firewall, failing the test if the template is missing.
func starterPostgresBody(t *testing.T) string {
	t.Helper()
	for _, tmpl := range firewall.StarterPolicyTemplates() {
		if tmpl.Name == "postgres" {
			return tmpl.Body
		}
	}
	t.Fatal("postgres starter template not found in firewall.StarterPolicyTemplates()")
	return ""
}

// TestStarterPostgresPolicy_AstClassDestructiveDeniesViaStaticRule loads the
// real shipped starter postgres policy from disk and proves its first deny rule
// (ast_class / DESTRUCTIVE) denies a request carrying arguments.sql_class =
// DESTRUCTIVE. Critically, NO SQL interceptor is registered on the pipeline, so
// the deny is produced by the static rule's own ast_class matching in Stage 2 —
// not by the Stage-3 SQL AST classifier. This closes the audit gap that the
// starter policy's first deny rule had no runtime-matching test.
func TestStarterPostgresPolicy_AstClassDestructiveDeniesViaStaticRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "postgres.yaml")
	if err := os.WriteFile(path, []byte(starterPostgresBody(t)), 0o600); err != nil {
		t.Fatalf("write starter policy: %v", err)
	}

	rules, err := governance.LoadStaticPoliciesFromDir(dir)
	if err != nil {
		t.Fatalf("load starter policy: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected starter postgres rules to load")
	}
	// Sanity-anchor the rule under test so this stays meaningful if the starter
	// template is ever reordered: the first rule must be the ast_class deny.
	first := rules[0]
	if first.Name != "deny-destructive-sql-class" {
		t.Fatalf("expected first starter rule to be deny-destructive-sql-class, got %q", first.Name)
	}
	if first.Action != "deny" || first.Match == nil || first.Match.Type != "ast_class" {
		t.Fatalf("expected an ast_class deny rule, got action=%q match=%#v", first.Action, first.Match)
	}

	// Build a pipeline from ONLY the loaded static rules. No TrustChecker, no
	// PolicyEval engine, and (the point of the test) no SQL interceptor — so the
	// only thing that can deny is the static ast_class rule itself.
	cfg := governance.PipelineConfig{
		StaticPolicies: rules,
		GatewayVersion: "starter-e2e",
	}
	p := governance.NewPipeline(cfg, nil, nil, nil)

	decision, err := p.Evaluate(context.Background(), &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		ToolName:  "query",
		// The classification result is supplied directly as an argument; the
		// static rule matches arguments.sql_class without invoking any classifier.
		Arguments: map[string]any{"sql_class": "DESTRUCTIVE"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Action != "deny" {
		t.Fatalf("ast_class DESTRUCTIVE must be denied by the static starter rule, got %q (reason=%q)", decision.Action, decision.Reason)
	}
	if decision.MatchedRule != "deny-destructive-sql-class" {
		t.Fatalf("expected matched rule deny-destructive-sql-class, got %q", decision.MatchedRule)
	}
	if !strings.Contains(decision.Reason, "Destructive SQL classes") {
		t.Fatalf("expected the starter rule reason, got %q", decision.Reason)
	}
}

// TestStarterPostgresPolicy_NonDestructiveClassAllowsViaStaticRule is the
// negative companion: a non-destructive class must NOT be denied by the
// ast_class rule (and, with no other matching rule, no SQL text, no interceptor,
// and no PolicyEval engine, the request is allowed). This proves the deny is the
// rule's ast_class matching specifically, not a blanket deny on the tool.
func TestStarterPostgresPolicy_NonDestructiveClassAllowsViaStaticRule(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "postgres.yaml")
	if err := os.WriteFile(path, []byte(starterPostgresBody(t)), 0o600); err != nil {
		t.Fatalf("write starter policy: %v", err)
	}
	rules, err := governance.LoadStaticPoliciesFromDir(dir)
	if err != nil {
		t.Fatalf("load starter policy: %v", err)
	}

	cfg := governance.PipelineConfig{StaticPolicies: rules, GatewayVersion: "starter-e2e"}
	p := governance.NewPipeline(cfg, nil, nil, nil)

	decision, err := p.Evaluate(context.Background(), &governance.GovernanceRequest{
		Transport: governance.TransportMCP,
		ToolName:  "query",
		// READ class, and a benign SQL body so the text-match deny rules
		// (DROP TABLE / TRUNCATE) also stay silent.
		Arguments: map[string]any{"sql_class": "READ", "sql": "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Action == "deny" {
		t.Fatalf("a READ class with benign SQL must not be denied, got deny (matched=%q reason=%q)", decision.MatchedRule, decision.Reason)
	}
}
