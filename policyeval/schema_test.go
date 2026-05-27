package policyeval

import "testing"

func TestValidatePolicyV1YAML_ValidDocument(t *testing.T) {
	doc, err := ValidatePolicyV1YAML("policy.yaml", []byte(`
schema_version: "1"
policy:
  name: postgres-production
  version: "1.0.0"
  transport: mcp
  rules:
    - name: deny-destructive
      tool: query
      action: deny
      conditions:
        - type: ast_class
          value: DESTRUCTIVE
    - name: tenant-regex
      tool: "*"
      action: warn
      conditions:
        - type: regex
          field: tenant_id
          regex: "^tenant-[0-9]+$"
`))
	if err != nil {
		t.Fatalf("validate policy v1: %v", err)
	}
	if doc.Policy.Name != "postgres-production" {
		t.Fatalf("unexpected policy name %q", doc.Policy.Name)
	}
	if len(doc.Policy.Rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(doc.Policy.Rules))
	}
}

func TestValidatePolicyV1YAML_InvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "wrong schema version",
			body: `
schema_version: "2"
policy:
  name: bad
  version: "1"
  rules:
    - name: deny
      tool: query
      action: deny
`,
		},
		{
			name: "unsupported condition",
			body: `
schema_version: "1"
policy:
  name: bad
  version: "1"
  rules:
    - name: deny
      tool: query
      action: deny
      conditions:
        - type: string_search_magic
          field: arguments.sql
          value: DROP
`,
		},
		{
			name: "invalid regex",
			body: `
schema_version: "1"
policy:
  name: bad
  version: "1"
  rules:
    - name: deny
      tool: query
      action: deny
      conditions:
        - type: regex
          field: tenant_id
          regex: "["
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ValidatePolicyV1YAML("bad.yaml", []byte(tt.body)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestEvaluationRequestToProtoContext_IncludesProjectedContext(t *testing.T) {
	trust := 0.75
	req := &EvaluationRequest{
		TenantID:    "tenant-1",
		AgentID:     "agent-1",
		Transport:   "mcp",
		ToolName:    "query",
		Action:      "tools/call",
		Arguments:   map[string]any{"sql_class": "READ", "limit": 10},
		TrustScore:  &trust,
		TrustState:  "TRUSTED",
		RiskClass:   "READ",
		RequestHash: "sha256:test",
		Provenance:  RequestProvenance{Source: "test", Adapter: "mcp", TraceID: "trace-1"},
	}
	ctx := req.ToProtoContext()
	if ctx.Attributes["agent.id"] != "agent-1" {
		t.Fatalf("missing agent id attribute: %#v", ctx.Attributes)
	}
	if ctx.Attributes["argument.limit"] != "10" {
		t.Fatalf("missing serialized argument: %#v", ctx.Attributes)
	}
	if ctx.Attributes["trust.score"] != "0.75" {
		t.Fatalf("missing trust score: %#v", ctx.Attributes)
	}
	if ctx.Attributes["request.hash"] != "sha256:test" {
		t.Fatalf("missing request hash: %#v", ctx.Attributes)
	}
}
