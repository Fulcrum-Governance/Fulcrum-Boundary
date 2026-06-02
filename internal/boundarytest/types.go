// Package boundarytest runs local policy-as-code test cases through Boundary's
// existing governance pipeline. It is fixture-only: no credentials, no network,
// and no live mutation.
package boundarytest

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

const SchemaVersion = "boundary.test.v1"

type Options struct {
	Path string
}

type Result struct {
	SchemaVersion       string       `json:"schema_version"`
	Status              string       `json:"status"`
	Path                string       `json:"path"`
	RequiresCredentials bool         `json:"requires_credentials"`
	RequiresNetwork     bool         `json:"requires_network"`
	MutatesLiveSystems  bool         `json:"mutates_live_systems"`
	Summary             Summary      `json:"summary"`
	Cases               []CaseResult `json:"cases"`
	DoesNotProve        []string     `json:"does_not_prove"`
}

type Summary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type CaseResult struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ExpectedAction string `json:"expected_action"`
	ActualAction   string `json:"actual_action"`
	Reason         string `json:"reason,omitempty"`
	MatchedRule    string `json:"matched_rule,omitempty"`
	PolicyFile     string `json:"policy_file,omitempty"`
	Error          string `json:"error,omitempty"`
}

type testCase struct {
	Name     string         `yaml:"name"`
	Policies string         `yaml:"policies"`
	Request  requestFixture `yaml:"request"`
	Expect   caseExpect     `yaml:"expect"`
}

type requestFixture struct {
	RequestID string                   `yaml:"request_id"`
	Transport governance.TransportType `yaml:"transport"`
	AgentID   string                   `yaml:"agent_id"`
	TenantID  string                   `yaml:"tenant_id"`
	ToolName  string                   `yaml:"tool_name"`
	Tool      string                   `yaml:"tool"`
	Action    string                   `yaml:"action"`
	Arguments map[string]any           `yaml:"arguments"`
	Command   string                   `yaml:"command"`
	Code      string                   `yaml:"code"`
	Language  string                   `yaml:"language"`
	TraceID   string                   `yaml:"trace_id"`
	BudgetKey string                   `yaml:"budget_key"`
}

type caseExpect struct {
	Action         string `yaml:"action"`
	ReasonContains string `yaml:"reason_contains"`
}
