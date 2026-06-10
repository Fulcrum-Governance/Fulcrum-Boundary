package governance

import (
	"context"
	"testing"
)

// TestStaticPolicyMatch_AllMatchTypes exercises every supported matcher Type in
// StaticPolicyMatch.matches with both a positive (should hit) and a negative
// (should miss) case. Before this test only the "contains" type had runtime
// coverage; transport_is, agent_in, agent_not_in, ast_class, not_contains,
// equals, not_equals, and regex were never matched at runtime, so the shipped
// starter policy's first deny rule (ast_class / DESTRUCTIVE) had no
// matching-path test. It proves each type's truth table directly.
func TestStaticPolicyMatch_AllMatchTypes(t *testing.T) {
	tests := []struct {
		name  string
		match StaticPolicyMatch
		req   *GovernanceRequest
		want  bool
	}{
		// --- contains -------------------------------------------------------
		{
			name:  "contains hit",
			match: StaticPolicyMatch{Type: "contains", Field: "arguments.sql", Contains: "DROP TABLE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DROP TABLE users"}},
			want:  true,
		},
		{
			name:  "contains miss",
			match: StaticPolicyMatch{Type: "contains", Field: "arguments.sql", Contains: "DROP TABLE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "SELECT 1"}},
			want:  false,
		},
		{
			name:  "contains case_insensitive hit",
			match: StaticPolicyMatch{Type: "contains", Field: "arguments.sql", Contains: "drop table", CaseInsensitive: true},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DROP TABLE users"}},
			want:  true,
		},
		{
			name:  "contains case sensitive miss",
			match: StaticPolicyMatch{Type: "contains", Field: "arguments.sql", Contains: "drop table"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DROP TABLE users"}},
			want:  false,
		},
		// --- empty type defaults to contains --------------------------------
		{
			name:  "empty type defaults to contains hit",
			match: StaticPolicyMatch{Field: "arguments.sql", Contains: "DELETE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DELETE FROM t"}},
			want:  true,
		},
		// --- not_contains ---------------------------------------------------
		{
			name:  "not_contains hit (substring absent)",
			match: StaticPolicyMatch{Type: "not_contains", Field: "arguments.sql", Contains: "WHERE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DELETE FROM t"}},
			want:  true,
		},
		{
			name:  "not_contains miss (substring present)",
			match: StaticPolicyMatch{Type: "not_contains", Field: "arguments.sql", Contains: "WHERE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DELETE FROM t WHERE id=1"}},
			want:  false,
		},
		// --- equals / not_equals --------------------------------------------
		{
			name:  "equals hit",
			match: StaticPolicyMatch{Type: "equals", Field: "action", Value: "delete"},
			req:   &GovernanceRequest{Action: "delete"},
			want:  true,
		},
		{
			name:  "equals miss (substring is not equality)",
			match: StaticPolicyMatch{Type: "equals", Field: "action", Value: "delete"},
			req:   &GovernanceRequest{Action: "delete_all"},
			want:  false,
		},
		{
			name:  "equals case_insensitive hit",
			match: StaticPolicyMatch{Type: "equals", Field: "action", Value: "DELETE", CaseInsensitive: true},
			req:   &GovernanceRequest{Action: "delete"},
			want:  true,
		},
		{
			name:  "not_equals hit",
			match: StaticPolicyMatch{Type: "not_equals", Field: "action", Value: "read"},
			req:   &GovernanceRequest{Action: "delete"},
			want:  true,
		},
		{
			name:  "not_equals miss",
			match: StaticPolicyMatch{Type: "not_equals", Field: "action", Value: "delete"},
			req:   &GovernanceRequest{Action: "delete"},
			want:  false,
		},
		// --- regex (valid patterns) -----------------------------------------
		{
			name:  "regex hit",
			match: StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: `(?i)drop\s+table`},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DROP   TABLE users"}},
			want:  true,
		},
		{
			name:  "regex miss",
			match: StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: `^DROP TABLE`},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "SELECT 1; DROP TABLE x"}},
			want:  false,
		},
		{
			name:  "regex case_insensitive flag hit",
			match: StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: `drop table`, CaseInsensitive: true},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "DROP TABLE users"}},
			want:  true,
		},
		{
			name:  "regex pattern from Value field hit",
			match: StaticPolicyMatch{Type: "regex", Field: "command", Value: `rm\s+-rf`},
			req:   &GovernanceRequest{Command: "rm -rf /"},
			want:  true,
		},
		{
			name:  "regex empty pattern is missing-data miss",
			match: StaticPolicyMatch{Type: "regex", Field: "arguments.sql"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "anything"}},
			want:  false,
		},
		// --- transport_is ---------------------------------------------------
		{
			name:  "transport_is hit (Value)",
			match: StaticPolicyMatch{Type: "transport_is", Value: "mcp"},
			req:   &GovernanceRequest{Transport: TransportMCP},
			want:  true,
		},
		{
			name:  "transport_is hit (Contains fallback, case-insensitive)",
			match: StaticPolicyMatch{Type: "transport_is", Contains: "MCP"},
			req:   &GovernanceRequest{Transport: TransportMCP},
			want:  true,
		},
		{
			name:  "transport_is miss",
			match: StaticPolicyMatch{Type: "transport_is", Value: "mcp"},
			req:   &GovernanceRequest{Transport: TransportCLI},
			want:  false,
		},
		{
			name:  "transport_is empty needle miss",
			match: StaticPolicyMatch{Type: "transport_is"},
			req:   &GovernanceRequest{Transport: TransportMCP},
			want:  false,
		},
		// --- agent_in / agent_not_in ----------------------------------------
		{
			name:  "agent_in hit",
			match: StaticPolicyMatch{Type: "agent_in", Values: []string{"agent-a", "agent-b"}},
			req:   &GovernanceRequest{AgentID: "agent-b"},
			want:  true,
		},
		{
			name:  "agent_in miss",
			match: StaticPolicyMatch{Type: "agent_in", Values: []string{"agent-a", "agent-b"}},
			req:   &GovernanceRequest{AgentID: "agent-z"},
			want:  false,
		},
		{
			name:  "agent_in empty agent miss (stringInList rejects empty)",
			match: StaticPolicyMatch{Type: "agent_in", Values: []string{"agent-a"}},
			req:   &GovernanceRequest{AgentID: ""},
			want:  false,
		},
		{
			name:  "agent_not_in hit (agent not listed)",
			match: StaticPolicyMatch{Type: "agent_not_in", Values: []string{"agent-a"}},
			req:   &GovernanceRequest{AgentID: "agent-z"},
			want:  true,
		},
		{
			name:  "agent_not_in miss (agent listed)",
			match: StaticPolicyMatch{Type: "agent_not_in", Values: []string{"agent-a", "agent-z"}},
			req:   &GovernanceRequest{AgentID: "agent-z"},
			want:  false,
		},
		{
			name:  "agent_not_in empty agent hit (empty is not in list)",
			match: StaticPolicyMatch{Type: "agent_not_in", Values: []string{"agent-a"}},
			req:   &GovernanceRequest{AgentID: ""},
			want:  true,
		},
		// --- ast_class ------------------------------------------------------
		{
			name:  "ast_class hit (default field arguments.sql_class, Value)",
			match: StaticPolicyMatch{Type: "ast_class", Value: "DESTRUCTIVE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql_class": "DESTRUCTIVE"}},
			want:  true,
		},
		{
			name:  "ast_class hit (case-insensitive)",
			match: StaticPolicyMatch{Type: "ast_class", Value: "destructive"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql_class": "DESTRUCTIVE"}},
			want:  true,
		},
		{
			name:  "ast_class hit via Values list",
			match: StaticPolicyMatch{Type: "ast_class", Values: []string{"ADMIN", "DESTRUCTIVE"}},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql_class": "DESTRUCTIVE"}},
			want:  true,
		},
		{
			name:  "ast_class miss (different class)",
			match: StaticPolicyMatch{Type: "ast_class", Value: "DESTRUCTIVE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql_class": "READ"}},
			want:  false,
		},
		{
			name:  "ast_class miss (field absent)",
			match: StaticPolicyMatch{Type: "ast_class", Value: "DESTRUCTIVE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"other": "x"}},
			want:  false,
		},
		{
			name:  "ast_class custom field hit (risk_class alias)",
			match: StaticPolicyMatch{Type: "ast_class", Field: "risk_class", Value: "DESTRUCTIVE"},
			req:   &GovernanceRequest{Arguments: map[string]any{"risk_class": "DESTRUCTIVE"}},
			want:  true,
		},
		// --- unknown type & missing-data guards -----------------------------
		{
			name:  "unknown type misses",
			match: StaticPolicyMatch{Type: "no_such_type", Field: "action", Value: "x"},
			req:   &GovernanceRequest{Action: "x"},
			want:  false,
		},
		{
			name:  "field-comparison matcher with absent field misses",
			match: StaticPolicyMatch{Type: "equals", Field: "arguments.missing", Value: "x"},
			req:   &GovernanceRequest{Arguments: map[string]any{"present": "x"}},
			want:  false,
		},
		{
			name:  "field-comparison matcher with empty needle misses",
			match: StaticPolicyMatch{Type: "contains", Field: "arguments.sql"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "anything"}},
			want:  false,
		},
		{
			name:  "field-comparison matcher with empty field misses",
			match: StaticPolicyMatch{Type: "contains", Contains: "x"},
			req:   &GovernanceRequest{Arguments: map[string]any{"sql": "x"}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.match.matches(tt.req); got != tt.want {
				t.Fatalf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStaticPolicyMatch_NilRequest verifies the matcher and rule both treat a
// nil request as a non-match rather than panicking.
func TestStaticPolicyMatch_NilRequest(t *testing.T) {
	m := StaticPolicyMatch{Type: "contains", Field: "arguments.sql", Contains: "x"}
	if m.matches(nil) {
		t.Fatal("matches(nil) should be false")
	}
	r := StaticPolicyRule{Tool: "*", Action: "deny"}
	if r.matchesRequest(nil) {
		t.Fatal("matchesRequest(nil) should be false")
	}
}

// TestStaticPolicyMatch_MalformedRegexFailsClosed pins the runtime behavior for
// a regex pattern that does not compile. Audit FINDING 2: the old code returned
// false on a compile error, so a deny rule carrying a bad pattern silently did
// NOT match and the request was ALLOWED (fail-open). Programmatically built
// rules bypass the YAML load-time regex validation, so this path is reachable.
// The matcher now fails CLOSED: a non-compiling pattern is treated as a hit so a
// gating rule cannot be silently bypassed by an invalid pattern.
func TestStaticPolicyMatch_MalformedRegexFailsClosed(t *testing.T) {
	// "[" is an unterminated character class -> regexp.Compile fails.
	m := StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: "["}
	req := &GovernanceRequest{Arguments: map[string]any{"sql": "anything at all"}}

	if got := m.matches(req); !got {
		t.Fatalf("malformed regex matcher: matches() = %v, want true (fail-closed)", got)
	}

	// The case-insensitive flag prepends "(?i)" but the body is still invalid;
	// it must also fail closed.
	mi := StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: "(", CaseInsensitive: true}
	if got := mi.matches(req); !got {
		t.Fatalf("malformed case-insensitive regex matcher: matches() = %v, want true (fail-closed)", got)
	}

	// Boundary of the contract: a malformed pattern whose target FIELD is absent
	// is missing data, not a hit -> false (the rule simply has nothing to match
	// against). Fail-closed applies only when the field is present.
	absent := StaticPolicyMatch{Type: "regex", Field: "arguments.not_here", Regex: "["}
	if got := absent.matches(req); got {
		t.Fatalf("malformed regex with absent field: matches() = %v, want false (missing data)", got)
	}
}

// TestStaticPolicyMatch_MalformedRegexDeniesViaPipeline proves the fail-closed
// behavior at the rule/pipeline level: a deny rule whose regex matcher cannot
// compile denies the request instead of leaking it through. This is the
// security-relevant end of FINDING 2 — the bug was that such a rule allowed.
func TestStaticPolicyMatch_MalformedRegexDeniesViaPipeline(t *testing.T) {
	cfg := PipelineConfig{
		StaticPolicies: []StaticPolicyRule{
			{
				Name:   "deny-on-broken-regex",
				Tool:   "query",
				Action: "deny",
				Reason: "blocked",
				Match:  &StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: "(unclosed"},
			},
		},
		GatewayVersion: "test-version",
	}
	p := NewPipeline(cfg, nil, nil, nil)

	decision, err := p.Evaluate(context.Background(), &GovernanceRequest{
		Transport: TransportMCP,
		ToolName:  "query",
		Arguments: map[string]any{"sql": "SELECT 1"},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Action != "deny" {
		t.Fatalf("expected deny for malformed-regex deny rule (fail-closed), got %q", decision.Action)
	}
	if decision.MatchedRule != "deny-on-broken-regex" {
		t.Fatalf("expected matched rule, got %q", decision.MatchedRule)
	}
}

// TestStaticPolicyMatch_ValidRegexUnchanged is the companion no-regression guard
// for FINDING 2's fix: a VALID regex behaves exactly as before (compile +
// MatchString), both on a hit and a miss. The fail-closed change must only
// affect uncompilable patterns.
func TestStaticPolicyMatch_ValidRegexUnchanged(t *testing.T) {
	hit := StaticPolicyMatch{Type: "regex", Field: "arguments.sql", Regex: `DROP\s+TABLE`}
	if !hit.matches(&GovernanceRequest{Arguments: map[string]any{"sql": "DROP TABLE x"}}) {
		t.Fatal("valid regex should still hit a matching haystack")
	}
	if hit.matches(&GovernanceRequest{Arguments: map[string]any{"sql": "SELECT 1"}}) {
		t.Fatal("valid regex should still miss a non-matching haystack")
	}
	// Compiling the same pattern twice exercises the cache path.
	if !hit.matches(&GovernanceRequest{Arguments: map[string]any{"sql": "DROP  TABLE y"}}) {
		t.Fatal("valid regex should hit on a second (cached) evaluation")
	}
}

// TestStaticPolicyRule_TenantScope proves tenant scoping is inclusive AND
// exclusive: a rule scoped to tenant X fires for tenant X and must NOT fire for
// tenant Y, and a request carrying no tenant never satisfies a scoped rule.
func TestStaticPolicyRule_TenantScope(t *testing.T) {
	rule := StaticPolicyRule{
		Name:        "scoped-to-tenant-x",
		Tool:        "query",
		Action:      "deny",
		TenantScope: []string{"tenant-x"},
	}

	if !rule.matchesRequest(&GovernanceRequest{ToolName: "query", TenantID: "tenant-x"}) {
		t.Fatal("rule scoped to tenant-x must fire for tenant-x")
	}
	if rule.matchesRequest(&GovernanceRequest{ToolName: "query", TenantID: "tenant-y"}) {
		t.Fatal("rule scoped to tenant-x must NOT fire for tenant-y")
	}
	if rule.matchesRequest(&GovernanceRequest{ToolName: "query", TenantID: ""}) {
		t.Fatal("rule scoped to tenant-x must NOT fire for a request with no tenant")
	}

	// Case-insensitivity matches the stringInList(..., true) call in matchesRequest.
	if !rule.matchesRequest(&GovernanceRequest{ToolName: "query", TenantID: "TENANT-X"}) {
		t.Fatal("tenant scope comparison is case-insensitive; TENANT-X should match tenant-x")
	}
}

// TestStaticPolicyRule_TenantScopeViaPipeline is the end-to-end form: the same
// scoped deny rule must block tenant X and allow tenant Y through Stage 2.
func TestStaticPolicyRule_TenantScopeViaPipeline(t *testing.T) {
	cfg := PipelineConfig{
		StaticPolicies: []StaticPolicyRule{
			{
				Name:        "scoped-to-tenant-x",
				Tool:        "query",
				Action:      "deny",
				Reason:      "tenant-x is denied",
				TenantScope: []string{"tenant-x"},
			},
		},
		GatewayVersion: "test-version",
	}
	p := NewPipeline(cfg, nil, nil, nil)

	denied, err := p.Evaluate(context.Background(), &GovernanceRequest{
		Transport: TransportMCP, ToolName: "query", TenantID: "tenant-x",
	})
	if err != nil {
		t.Fatalf("evaluate tenant-x: %v", err)
	}
	if denied.Action != "deny" {
		t.Fatalf("tenant-x should be denied, got %q", denied.Action)
	}

	allowed, err := p.Evaluate(context.Background(), &GovernanceRequest{
		Transport: TransportMCP, ToolName: "query", TenantID: "tenant-y",
	})
	if err != nil {
		t.Fatalf("evaluate tenant-y: %v", err)
	}
	if allowed.Action == "deny" {
		t.Fatalf("tenant-y must NOT be denied by a tenant-x-scoped rule, got %q", allowed.Action)
	}
}

// TestStaticPolicyRule_AgentScope proves agent scoping is inclusive AND
// exclusive, mirroring tenant scope.
func TestStaticPolicyRule_AgentScope(t *testing.T) {
	rule := StaticPolicyRule{
		Name:       "scoped-to-agent-a",
		Tool:       "query",
		Action:     "deny",
		AgentScope: []string{"agent-a"},
	}

	if !rule.matchesRequest(&GovernanceRequest{ToolName: "query", AgentID: "agent-a"}) {
		t.Fatal("rule scoped to agent-a must fire for agent-a")
	}
	if rule.matchesRequest(&GovernanceRequest{ToolName: "query", AgentID: "agent-b"}) {
		t.Fatal("rule scoped to agent-a must NOT fire for agent-b")
	}
	if rule.matchesRequest(&GovernanceRequest{ToolName: "query", AgentID: ""}) {
		t.Fatal("rule scoped to agent-a must NOT fire for a request with no agent")
	}
}

// TestStaticPolicyRule_TransportGate proves the rule-level Transport field gates
// matching independently of the transport_is matcher type.
func TestStaticPolicyRule_TransportGate(t *testing.T) {
	rule := StaticPolicyRule{Name: "mcp-only", Tool: "query", Action: "deny", Transport: "mcp"}

	if !rule.matchesRequest(&GovernanceRequest{ToolName: "query", Transport: TransportMCP}) {
		t.Fatal("mcp-scoped rule must fire for an MCP request")
	}
	if rule.matchesRequest(&GovernanceRequest{ToolName: "query", Transport: TransportCLI}) {
		t.Fatal("mcp-scoped rule must NOT fire for a CLI request")
	}
}

// TestStaticPolicyRule_GlobToolPatterns proves the tool selector supports exact
// names, "*"/"" wildcards, and path.Match globs, and that a non-matching glob
// does not fire.
func TestStaticPolicyRule_GlobToolPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		tool    string
		want    bool
	}{
		{"exact match", "query", "query", true},
		{"exact mismatch", "query", "write", false},
		{"star matches all", "*", "anything", true},
		{"empty matches all", "", "anything", true},
		{"glob prefix hit", "db_*", "db_query", true},
		{"glob prefix miss", "db_*", "fs_query", false},
		{"glob suffix hit", "*_file", "write_file", true},
		{"glob suffix miss", "*_file", "write_blob", false},
		{"glob single char class hit", "tool_[0-9]", "tool_7", true},
		{"glob single char class miss", "tool_[0-9]", "tool_x", false},
		{"malformed glob is non-matching", "[", "anything", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := StaticPolicyRule{Name: tt.name, Tool: tt.pattern, Action: "deny"}
			got := rule.matchesRequest(&GovernanceRequest{ToolName: tt.tool})
			if got != tt.want {
				t.Fatalf("tool pattern %q vs %q: matchesRequest() = %v, want %v", tt.pattern, tt.tool, got, tt.want)
			}
		})
	}
}

// TestStaticPolicyRule_AllConditionsMustHold proves multi-matcher rules are
// conjunctive: the rule fires only when Match AND every Conditions entry hold.
func TestStaticPolicyRule_AllConditionsMustHold(t *testing.T) {
	rule := StaticPolicyRule{
		Name:   "destructive-on-prod",
		Tool:   "query",
		Action: "deny",
		Match:  &StaticPolicyMatch{Type: "ast_class", Value: "DESTRUCTIVE"},
		Conditions: []StaticPolicyMatch{
			{Type: "equals", Field: "arguments.env", Value: "prod"},
		},
	}

	// Both hold -> fire.
	if !rule.matchesRequest(&GovernanceRequest{
		ToolName:  "query",
		Arguments: map[string]any{"sql_class": "DESTRUCTIVE", "env": "prod"},
	}) {
		t.Fatal("rule must fire when ast_class AND env condition both hold")
	}
	// Match holds, condition fails -> no fire.
	if rule.matchesRequest(&GovernanceRequest{
		ToolName:  "query",
		Arguments: map[string]any{"sql_class": "DESTRUCTIVE", "env": "staging"},
	}) {
		t.Fatal("rule must NOT fire when the env condition fails")
	}
	// Condition holds, match fails -> no fire.
	if rule.matchesRequest(&GovernanceRequest{
		ToolName:  "query",
		Arguments: map[string]any{"sql_class": "READ", "env": "prod"},
	}) {
		t.Fatal("rule must NOT fire when the ast_class match fails")
	}
}

// TestStaticPolicyHelpers_Direct exercises the small comparison helpers that
// Go's black-box coverage attributed to 0% (equalString, matchValues,
// valueMatchesAny, stringInList). They underpin every scope and multi-value
// matcher, so their truth tables are pinned directly.
func TestStaticPolicyHelpers_Direct(t *testing.T) {
	t.Run("equalString", func(t *testing.T) {
		if !equalString("a", "a", false) {
			t.Fatal("a==a")
		}
		if equalString("a", "A", false) {
			t.Fatal("a!=A case-sensitive")
		}
		if !equalString("a", "A", true) {
			t.Fatal("a==A case-insensitive")
		}
	})

	t.Run("matchValues collects Values then Value then Contains", func(t *testing.T) {
		got := matchValues(StaticPolicyMatch{
			Values:   []string{"v1", "v2"},
			Value:    "scalar",
			Contains: "needle",
		})
		want := []string{"v1", "v2", "scalar", "needle"}
		if len(got) != len(want) {
			t.Fatalf("matchValues len = %d (%v), want %d", len(got), got, len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("matchValues[%d] = %q, want %q", i, got[i], want[i])
			}
		}
		// Empty scalars are omitted.
		if g := matchValues(StaticPolicyMatch{Values: []string{"only"}}); len(g) != 1 || g[0] != "only" {
			t.Fatalf("matchValues with empty scalars = %v, want [only]", g)
		}
	})

	t.Run("valueMatchesAny", func(t *testing.T) {
		if !valueMatchesAny("b", []string{"a", "b"}, false) {
			t.Fatal("b in [a b]")
		}
		if valueMatchesAny("z", []string{"a", "b"}, false) {
			t.Fatal("z not in [a b]")
		}
		if !valueMatchesAny("B", []string{"a", "b"}, true) {
			t.Fatal("B in [a b] case-insensitive")
		}
		if valueMatchesAny("x", nil, false) {
			t.Fatal("nothing matches an empty candidate list")
		}
	})

	t.Run("stringInList rejects empty value", func(t *testing.T) {
		if !stringInList("a", []string{"a"}, false) {
			t.Fatal("a in [a]")
		}
		if stringInList("", []string{"a", ""}, false) {
			t.Fatal("empty value must never be in list (fail-closed scoping)")
		}
		if !stringInList("A", []string{"a"}, true) {
			t.Fatal("A in [a] case-insensitive")
		}
	})

	t.Run("firstNonEmpty", func(t *testing.T) {
		if got := firstNonEmpty("", "  ", "x", "y"); got != "x" {
			t.Fatalf("firstNonEmpty = %q, want x", got)
		}
		if got := firstNonEmpty("", "   "); got != "" {
			t.Fatalf("firstNonEmpty all-empty = %q, want empty", got)
		}
	})

	t.Run("contains", func(t *testing.T) {
		if !contains("DROP TABLE", "TABLE", false) {
			t.Fatal("substring present")
		}
		if contains("DROP TABLE", "table", false) {
			t.Fatal("case-sensitive miss")
		}
		if !contains("DROP TABLE", "table", true) {
			t.Fatal("case-insensitive hit")
		}
	})
}

// TestRequestField_FieldResolution covers the alias and arguments/input prefix
// resolution in requestField, including the absent-field false returns the
// matchers rely on.
func TestRequestField_FieldResolution(t *testing.T) {
	req := &GovernanceRequest{
		ToolName:  "query",
		Action:    "delete",
		AgentID:   "agent-a",
		TenantID:  "tenant-x",
		Transport: TransportMCP,
		Command:   "rm -rf /",
		Code:      "print(1)",
		Arguments: map[string]any{
			"sql":       "DROP TABLE t",
			"sql_class": "DESTRUCTIVE",
			"count":     7,
		},
	}

	tests := []struct {
		field   string
		want    string
		wantOK  bool
		comment string
	}{
		{"tool", "query", true, "tool alias"},
		{"tool_name", "query", true, "tool_name alias"},
		{"action", "delete", true, "action"},
		{"agent_id", "agent-a", true, "agent_id"},
		{"tenant_id", "tenant-x", true, "tenant_id"},
		{"transport", "mcp", true, "transport"},
		{"command", "rm -rf /", true, "command"},
		{"code", "print(1)", true, "code"},
		{"risk_class", "DESTRUCTIVE", true, "risk_class resolves sql_class"},
		{"sql.class", "DESTRUCTIVE", true, "sql.class alias resolves sql_class"},
		{"arguments.sql", "DROP TABLE t", true, "arguments.<name>"},
		{"arguments.count", "7", true, "non-string argument stringified"},
		{"input.sql", "DROP TABLE t", true, "input.<name> prefix maps to arguments"},
		{"arguments.text", "DROP TABLE t", true, "arguments.text special-cases sql"},
		{"arguments.missing", "", false, "absent argument is not ok"},
		{"input.missing", "", false, "absent input is not ok"},
		{"no_such_field", "", false, "unknown field is not ok"},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			got, ok := requestField(req, tt.field)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("requestField(%q) = (%q, %v), want (%q, %v)", tt.field, got, ok, tt.want, tt.wantOK)
			}
		})
	}

	if _, ok := requestField(nil, "tool"); ok {
		t.Fatal("requestField(nil, ...) must be not-ok")
	}

	// risk_class/sql.class with neither sql_class nor risk_class present -> not ok.
	noClass := &GovernanceRequest{Arguments: map[string]any{"other": "x"}}
	if _, ok := requestField(noClass, "risk_class"); ok {
		t.Fatal("risk_class with no sql_class/risk_class argument must be not-ok")
	}
	// And with no Arguments map at all.
	if _, ok := requestField(&GovernanceRequest{}, "sql.class"); ok {
		t.Fatal("sql.class with nil arguments must be not-ok")
	}
}
