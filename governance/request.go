package governance

import "time"

// TransportType identifies the protocol used for the tool invocation.
type TransportType string

const (
	TransportMCP           TransportType = "mcp"
	TransportCLI           TransportType = "cli"
	TransportCodeExec      TransportType = "code_exec"
	TransportGRPC          TransportType = "grpc"
	TransportA2A           TransportType = "a2a"
	TransportWebhook       TransportType = "webhook"
	TransportManagedAgents TransportType = "managed_agents"
)

// GovernanceRequest is the canonical, protocol-independent representation
// of an action that must be governed before execution.
type GovernanceRequest struct {
	// Identity
	RequestID string        `json:"request_id"`
	Transport TransportType `json:"transport"`
	AgentID   string        `json:"agent_id"`
	TenantID  string        `json:"tenant_id"`

	// Action being governed
	ToolName   string         `json:"tool_name"`
	Action     string         `json:"action"`
	Arguments  map[string]any `json:"arguments"`
	RawPayload []byte         `json:"raw_payload,omitempty"`

	// CLI-specific fields
	Command   string        `json:"command,omitempty"`
	Stdin     []byte        `json:"stdin,omitempty"`
	PipeChain []PipeSegment `json:"pipe_chain,omitempty"`

	// Code-exec-specific fields
	Code      string `json:"code,omitempty"`
	Language  string `json:"language,omitempty"`
	SandboxID string `json:"sandbox_id,omitempty"`

	// Governance context
	EnvelopeID  string `json:"envelope_id"`
	ParentEnvID string `json:"parent_envelope_id,omitempty"`
	TraceID     string `json:"trace_id"`
	BudgetKey   string `json:"budget_key"`
}

// PipeSegment represents a single command in a pipe chain.
type PipeSegment struct {
	Command   string   `json:"command"`
	Args      []string `json:"args"`
	RiskLevel string   `json:"risk_level"` // read, write, admin, destructive
}

// HighestRisk returns the highest risk level across all pipe segments.
// Risk ordering: destructive > admin > write > read.
func HighestRisk(segments []PipeSegment) string {
	order := map[string]int{
		"read":        0,
		"write":       1,
		"admin":       2,
		"destructive": 3,
	}
	highest := "read"
	for _, seg := range segments {
		if order[seg.RiskLevel] > order[highest] {
			highest = seg.RiskLevel
		}
	}
	return highest
}

// ActionCheckIndeterminate is the decision action for a required synchronous
// policy, budget, trust, identity, or configuration check that could not
// produce a valid result (ADR-047). It is neither "allow" nor a substantive
// policy deny: Allowed() returns false for it, so enforcing adapters block.
// The machine-readable FailureCategory and the enforcement-stage context are
// carried in GovernanceDecision.Check and mirrored into the audit/decision
// record. Transports may map the classification to a compatible
// deny/unavailable/error response, but the record keeps "check_indeterminate".
const ActionCheckIndeterminate = "check_indeterminate"

// FailureCategory is the machine-readable ADR-047 classification of why a
// required check could not produce a valid result.
type FailureCategory string

const (
	// FailureUnavailable — a required dependency is unavailable or disabled.
	FailureUnavailable FailureCategory = "unavailable"
	// FailureTimeout — the check did not produce a result before its deadline.
	FailureTimeout FailureCategory = "timeout"
	// FailureCanceled — the check was canceled before producing a result.
	FailureCanceled FailureCategory = "canceled"
	// FailurePanic — the check panicked and was recovered at the pipeline
	// boundary.
	FailurePanic FailureCategory = "panic"
	// FailureInvalidResult — the check returned a nil, malformed, or unknown
	// result.
	FailureInvalidResult FailureCategory = "invalid_result"
	// FailureMissingConfig — required configuration is absent or invalid.
	FailureMissingConfig FailureCategory = "missing_config"
	// FailureMissingIdentity — a required agent or tenant identity is absent.
	FailureMissingIdentity FailureCategory = "missing_identity"
	// FailureStaleSnapshot — a cached policy/trust snapshot was used past its
	// explicit validity window.
	FailureStaleSnapshot FailureCategory = "stale_snapshot"
)

// Enforcement stages for CheckFailure.Stage: the pipeline stage whose required
// check failed.
const (
	// CheckStageTrust — Stage 1, the trust/circuit-breaker lookup.
	CheckStageTrust = "trust"
	// CheckStageTrustUpdate — the deferred trust outcome recording.
	CheckStageTrustUpdate = "trust_update"
	// CheckStageInterceptor — Stage 3, a domain interceptor.
	CheckStageInterceptor = "interceptor"
	// CheckStagePolicyEval — Stage 4, the policy evaluator.
	CheckStagePolicyEval = "policy_eval"
	// CheckStageIdentity — the RequireAgentID identity guard.
	CheckStageIdentity = "identity"
	// CheckStageConfig — pipeline construction/configuration validity.
	CheckStageConfig = "config"
)

// Check classes for CheckFailure.Class (the ADR-047 "check class").
const (
	// CheckClassTrust — trust/circuit-breaker checks.
	CheckClassTrust = "trust"
	// CheckClassPolicy — policy and domain-check evaluations.
	CheckClassPolicy = "policy"
	// CheckClassIdentity — required-identity checks.
	CheckClassIdentity = "identity"
	// CheckClassConfig — configuration-validity checks.
	CheckClassConfig = "config"
)

// CheckFailure carries the ADR-047 safe context for a required check that
// could not produce a valid result. It is attached to the decision and copied
// into the audit event and decision record. Its serialized fields never
// contain secrets, credentials, or raw tool arguments: Stage, Class, and
// Category are machine enums and Cause is a short fixed-vocabulary
// description. Detail holds the raw underlying error for operator-side
// diagnostics and is never serialized.
//
// On an enforcing transport the decision Action is ActionCheckIndeterminate
// and the indeterminacy blocked execution. On an explicitly non-enforcing
// transport the Action may remain allow-compatible; Check is then the recorded
// would-have-blocked result.
type CheckFailure struct {
	// Stage is the enforcement stage whose check failed (CheckStage*).
	Stage string `json:"stage"`
	// Class is the class of the failed check (CheckClass*).
	Class string `json:"class"`
	// Category is the machine-readable failure category.
	Category FailureCategory `json:"category"`
	// Cause is a short, fixed-vocabulary description of the failure (for
	// example "policy evaluation failed"). It never interpolates the
	// underlying error text, so it is safe to return to the governed caller.
	Cause string `json:"cause,omitempty"`
	// Detail is the raw underlying error text for operator-side diagnostics.
	// Check errors routinely embed infrastructure internals — dial strings,
	// endpoints, credentials in connection URLs — that must not be disclosed
	// to the governed caller, so Detail is excluded from JSON serialization:
	// it cannot appear in a serialized decision, a webhook response body, or
	// a hashed decision record. In-process audit consumers (for example
	// SlogAuditPublisher) read it directly from the struct.
	Detail string `json:"-"`
}

// GovernanceDecision is the canonical output of the governance pipeline.
type GovernanceDecision struct {
	RequestID      string        `json:"request_id"`
	Action         string        `json:"action"` // allow, deny, warn, escalate, require_approval, check_indeterminate
	Reason         string        `json:"reason"`
	PolicyID       string        `json:"policy_id,omitempty"`
	MatchedRule    string        `json:"matched_rule,omitempty"`
	PolicyFile     string        `json:"policy_file,omitempty"`
	GatewayVersion string        `json:"gateway_version,omitempty"`
	TrustScore     float64       `json:"trust_score"`
	TrustState     string        `json:"trust_state,omitempty"`
	EnvelopeID     string        `json:"envelope_id"`
	DryRun         bool          `json:"dry_run"`
	CostEstimate   float64       `json:"cost_estimate,omitempty"`
	Duration       time.Duration `json:"duration"`
	// DecisionMode labels the epistemic confidence level of this decision:
	// deterministic, classified, proved, or human_approved. Empty string
	// means the producer did not label the mode (backwards compat).
	DecisionMode DecisionMode `json:"decision_mode,omitempty"`
	// Check carries the ADR-047 CHECK_INDETERMINATE context when a required
	// synchronous check could not produce a valid result; nil when every
	// required check produced a valid result. When Action is
	// ActionCheckIndeterminate the failure blocked execution; when Action
	// remains allow-compatible the transport is explicitly non-enforcing and
	// Check is the recorded would-have-blocked result.
	Check *CheckFailure `json:"check,omitempty"`
}

// Allowed returns true if the decision permits execution.
func (d *GovernanceDecision) Allowed() bool {
	return d.Action == "allow" || d.Action == "warn"
}

// ToolResponse wraps the result of executing a governed tool call.
type ToolResponse struct {
	Content     []byte            `json:"content"`
	ContentType string            `json:"content_type"`
	ExitCode    int               `json:"exit_code,omitempty"`
	Duration    time.Duration     `json:"duration"`
	Truncated   bool              `json:"truncated"`
	FilePath    string            `json:"file_path,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

// ResponseInspection holds the results of post-execution response analysis.
type ResponseInspection struct {
	Safe            bool     `json:"safe"`
	Concerns        []string `json:"concerns,omitempty"`
	InjectionRisk   float64  `json:"injection_risk"`
	SensitiveData   bool     `json:"sensitive_data"`
	ComplianceFlags []string `json:"compliance_flags,omitempty"`
}
