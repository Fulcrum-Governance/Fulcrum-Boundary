package securegithub

import (
	"context"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const (
	ProfileID           = "secure-github"
	StatusPreview       = "preview"
	DefaultTenantID     = "fixture-tenant"
	DefaultAgentID      = "secure-github-fixture-agent"
	DefaultSessionID    = "fixture-session"
	DefaultOwner        = "fixture-org"
	DefaultRepo         = "fixture-private-repo"
	DefaultGateway      = "secure-github-preview"
	FixtureCollaborator = "fixture_external_collaborator"
)

type Config struct {
	TenantID          string
	AgentID           string
	SessionID         string
	Owner             string
	Repo              string
	OneRepoPerSession bool
	FixtureMode       bool
	GatewayVersion    string
	BuildDigest       string
}

func DefaultConfig() Config {
	return Config{
		TenantID:          DefaultTenantID,
		AgentID:           DefaultAgentID,
		SessionID:         DefaultSessionID,
		Owner:             DefaultOwner,
		Repo:              DefaultRepo,
		OneRepoPerSession: true,
		FixtureMode:       true,
		GatewayVersion:    DefaultGateway,
		BuildDigest:       "fixture-only",
	}
}

func (c Config) withDefaults() Config {
	defaults := DefaultConfig()
	if c.TenantID == "" {
		c.TenantID = defaults.TenantID
	}
	if c.AgentID == "" {
		c.AgentID = defaults.AgentID
	}
	if c.SessionID == "" {
		c.SessionID = defaults.SessionID
	}
	if c.Owner == "" {
		c.Owner = defaults.Owner
	}
	if c.Repo == "" {
		c.Repo = defaults.Repo
	}
	if c.GatewayVersion == "" {
		c.GatewayVersion = defaults.GatewayVersion
	}
	if c.BuildDigest == "" {
		c.BuildDigest = defaults.BuildDigest
	}
	c.OneRepoPerSession = defaults.OneRepoPerSession
	c.FixtureMode = defaults.FixtureMode
	return c
}

type ToolCall struct {
	JSONRPC   string         `json:"jsonrpc,omitempty"`
	ID        any            `json:"id,omitempty"`
	Method    string         `json:"method,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	AgentID   string         `json:"agent_id,omitempty"`
	TenantID  string         `json:"tenant_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Params    ToolCallParams `json:"params,omitempty"`
}

type ToolCallParams struct {
	Name      string         `json:"name,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type Envelope struct {
	ProfileID          string   `json:"profile_id"`
	Status             string   `json:"status"`
	SessionID          string   `json:"session_id"`
	RequestID          string   `json:"request_id"`
	EnvelopeID         string   `json:"envelope_id"`
	TraceID            string   `json:"trace_id,omitempty"`
	TenantID           string   `json:"tenant_id"`
	AgentID            string   `json:"agent_id"`
	ToolName           string   `json:"tool_name"`
	Action             string   `json:"action"`
	Owner              string   `json:"owner"`
	Repo               string   `json:"repo"`
	ResourceID         string   `json:"resource_id"`
	CapabilityClass    string   `json:"capability_class"`
	RiskClass          string   `json:"risk_class"`
	SourceClass        string   `json:"source_class"`
	TargetSink         string   `json:"target_sink"`
	MutationClass      string   `json:"mutation_class"`
	Tainted            bool     `json:"tainted"`
	TaintSources       []string `json:"taint_sources,omitempty"`
	CollaboratorModel  string   `json:"collaborator_model,omitempty"`
	OneRepoPerSession  bool     `json:"one_repo_per_session"`
	RepoScopeViolation bool     `json:"repo_scope_violation,omitempty"`
	FixtureMode        bool     `json:"fixture_mode"`
}

func (e Envelope) TargetRepo() string {
	return e.Owner + "/" + e.Repo
}

func (e Envelope) Arguments() map[string]any {
	args := map[string]any{
		"profile_id":            e.ProfileID,
		"profile_status":        e.Status,
		"session_id":            e.SessionID,
		"capability_class":      e.CapabilityClass,
		"risk_class":            e.RiskClass,
		"source_class":          e.SourceClass,
		"target_sink":           e.TargetSink,
		"mutation_class":        e.MutationClass,
		"tainted":               e.Tainted,
		"target_repo":           e.TargetRepo(),
		"resource_id":           e.ResourceID,
		"owner":                 e.Owner,
		"repo":                  e.Repo,
		"collaborator_model":    e.CollaboratorModel,
		"one_repo_per_session":  e.OneRepoPerSession,
		"repo_scope_violation":  e.RepoScopeViolation,
		"fixture_mode":          e.FixtureMode,
		"live_github_evidence":  false,
		"live_git_app_required": true,
	}
	if len(e.TaintSources) > 0 {
		args["taint_source"] = strings.Join(e.TaintSources, ",")
		args["taint_sources"] = append([]string{}, e.TaintSources...)
	}
	return args
}

type MCPResponse struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      any        `json:"id,omitempty"`
	Result  *MCPResult `json:"result,omitempty"`
	Error   *MCPError  `json:"error,omitempty"`
}

type MCPResult struct {
	Content           []MCPContent                 `json:"content"`
	StructuredContent map[string]any               `json:"structuredContent,omitempty"`
	Governance        map[string]string            `json:"governance,omitempty"`
	Envelope          *Envelope                    `json:"boundary_envelope,omitempty"`
	DecisionRecord    *governance.DecisionRecordV1 `json:"decision_record,omitempty"`
}

type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type MCPError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

type GovernedResult struct {
	Response       MCPResponse
	Decision       *governance.GovernanceDecision
	DecisionRecord governance.DecisionRecordV1
	Envelope       Envelope
	UpstreamCalled bool
}

type Upstream interface {
	CallGitHub(ctx context.Context, call ToolCall, envelope Envelope) (*MCPResult, error)
}
