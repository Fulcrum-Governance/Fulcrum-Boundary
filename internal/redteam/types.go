package redteam

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

const (
	SchemaVersion = "boundary.redteam.run.v1"
	DefaultPackID = "github-lethal-trifecta"

	ModeFixture = "fixture"

	PackStatusImplemented = "implemented"
	PackStatusStub        = "stub"

	ResultPassed  = "passed"
	ResultFailed  = "failed"
	ResultSkipped = "skipped"
)

type RunOptions struct {
	PackID string
	Mode   string
}

type Pack struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Description string     `json:"description"`
	StubReason  string     `json:"stub_reason,omitempty"`
	Scenarios   []Scenario `json:"scenarios,omitempty"`
}

type Scenario struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	Description    string                        `json:"description"`
	FixtureOnly    bool                          `json:"fixture_only"`
	NoLiveMutation bool                          `json:"no_live_mutation"`
	ExpectedAction string                        `json:"expected_action"`
	Request        governance.GovernanceRequest  `json:"request"`
	Policies       []governance.StaticPolicyRule `json:"policies"`
}

type PackSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type RunResult struct {
	SchemaVersion      string           `json:"schema_version"`
	Mode               string           `json:"mode"`
	PackID             string           `json:"pack_id"`
	PackName           string           `json:"pack_name"`
	Status             string           `json:"status"`
	Passed             bool             `json:"passed"`
	MutatesLiveSystems bool             `json:"mutates_live_systems"`
	RealSecretsUsed    bool             `json:"real_secrets_used"`
	Results            []ScenarioResult `json:"results"`
}

type ScenarioResult struct {
	PackID         string                      `json:"pack_id"`
	ScenarioID     string                      `json:"scenario_id"`
	Name           string                      `json:"name"`
	Mode           string                      `json:"mode"`
	Status         string                      `json:"status"`
	FixtureOnly    bool                        `json:"fixture_only"`
	NoLiveMutation bool                        `json:"no_live_mutation"`
	ExpectedAction string                      `json:"expected_action"`
	ActualAction   string                      `json:"actual_action"`
	Passed         bool                        `json:"passed"`
	Reason         string                      `json:"reason"`
	MatchedRule    string                      `json:"matched_rule,omitempty"`
	DecisionRecord governance.DecisionRecordV1 `json:"decision_record"`
}
