// Package policyeval provides a portable, dependency-free policy evaluation engine.
//
// This package is designed to be embedded in MCP proxies, SDKs, and the main Fulcrum server,
// ensuring consistent policy evaluation behavior across all deployment contexts.
//
// The evaluator operates entirely in-memory with no database, Redis, or NATS dependencies.
// Policies are loaded via UpdatePolicies() and evaluated synchronously.
package policyeval

// ActionType represents the outcome of policy evaluation.
type ActionType int

const (
	// ActionAllow permits the action to proceed.
	ActionAllow ActionType = iota
	// ActionDeny blocks the action.
	ActionDeny
	// ActionEscalate requires a phone-home check (e.g., Semantic Judge).
	ActionEscalate
	// ActionWarn allows but logs a warning.
	ActionWarn
	// ActionRequireApproval requires human approval before proceeding.
	ActionRequireApproval
)

// String returns a human-readable representation of the action type.
func (a ActionType) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionDeny:
		return "deny"
	case ActionEscalate:
		return "escalate"
	case ActionWarn:
		return "warn"
	case ActionRequireApproval:
		return "require_approval"
	default:
		return "unknown"
	}
}

// Decision represents the result of policy evaluation.
type Decision struct {
	// Action is the primary outcome (allow, deny, escalate, warn, require_approval).
	Action ActionType

	// MatchedPolicy is the policy that produced this decision (if any).
	MatchedPolicy *Policy

	// MatchedRules are the rules within the policy that matched.
	MatchedRules []*RuleMatch

	// Actions are the specific policy actions triggered.
	Actions []*PolicyAction

	// Reason provides a human-readable explanation of the decision.
	Reason string

	// EvaluationDurationMs is how long the evaluation took.
	EvaluationDurationMs int64

	// EscalationReason explains why escalation is needed (only set when Action == ActionEscalate).
	EscalationReason string
}

// RuleMatch represents a matched rule within a policy.
type RuleMatch struct {
	RuleID   string
	RuleName string
	Priority int32
}

// EvaluationRequest contains the context for policy evaluation.
type EvaluationRequest struct {
	// TenantID is the tenant making the request.
	TenantID string

	// UserID is the user or agent making the request.
	UserID string

	// UserRoles are the roles assigned to the user.
	UserRoles []string

	// WorkflowID is the workflow context (if any).
	WorkflowID string

	// EnvelopeID is the envelope being evaluated.
	EnvelopeID string

	// Phase is the execution phase (PRE, MID, POST).
	Phase ExecutionPhase

	// ModelID is the LLM model being used.
	ModelID string

	// ToolNames are the tools being invoked.
	ToolNames []string

	// AgentID is the agent identity from the transport adapter, when known.
	AgentID string

	// Transport is the Boundary transport surface that produced the request.
	Transport string

	// ToolName is the primary tool being invoked. It mirrors ToolNames[0] for
	// consumers that need one canonical value.
	ToolName string

	// Action is the transport-level action, for example tools/call.
	Action string

	// Arguments are the structured tool arguments received by Boundary.
	Arguments map[string]any

	// TrustScore is the current trust score, if the caller has one.
	TrustScore *float64

	// TrustState is the current trust/circuit-breaker state, if known.
	TrustState string

	// RiskClass carries adapter or interceptor classification such as SQL AST
	// class or CLI pipe risk.
	RiskClass string

	// ResourceIDs are resource identifiers derived from the request arguments.
	ResourceIDs []string

	// RequestHash is a canonical hash of the request context used for records
	// and later receipt verification.
	RequestHash string

	// PolicyVersion identifies the policy bundle used for this evaluation.
	PolicyVersion string

	// Provenance captures where the evaluation context came from.
	Provenance RequestProvenance

	// InputText is the input being processed (for content policies).
	InputText string

	// OutputText is the output being generated (for content policies).
	OutputText string

	// Attributes are custom key-value pairs for condition matching.
	Attributes map[string]string
}

// ToProtoContext converts an EvaluationRequest to the protobuf EvaluationContext.
func (r *EvaluationRequest) ToProtoContext() *EvaluationContext {
	attributes := cloneAttributes(r.Attributes)
	if attributes == nil {
		attributes = map[string]string{}
	}
	if r.AgentID != "" {
		attributes["agent.id"] = r.AgentID
	}
	if r.Transport != "" {
		attributes["transport"] = r.Transport
	}
	if r.ToolName != "" {
		attributes["tool.name"] = r.ToolName
	}
	if r.Action != "" {
		attributes["action"] = r.Action
	}
	if r.TrustScore != nil {
		attributes["trust.score"] = formatFloat(*r.TrustScore)
	}
	if r.TrustState != "" {
		attributes["trust.state"] = r.TrustState
	}
	if r.RiskClass != "" {
		attributes["risk.class"] = r.RiskClass
	}
	if r.RequestHash != "" {
		attributes["request.hash"] = r.RequestHash
	}
	if r.PolicyVersion != "" {
		attributes["policy.version"] = r.PolicyVersion
	}
	for i, resourceID := range r.ResourceIDs {
		attributes["resource."+itoa(i)+".id"] = resourceID
	}
	for key, value := range r.Arguments {
		attributes["argument."+key] = stringifyArgument(value)
	}
	if r.Provenance.Source != "" {
		attributes["provenance.source"] = r.Provenance.Source
	}
	if r.Provenance.Adapter != "" {
		attributes["provenance.adapter"] = r.Provenance.Adapter
	}
	if r.Provenance.TraceID != "" {
		attributes["trace.id"] = r.Provenance.TraceID
	}
	if len(attributes) == 0 {
		attributes = nil
	}

	return &EvaluationContext{
		TenantId:   r.TenantID,
		UserId:     r.UserID,
		UserRoles:  r.UserRoles,
		WorkflowId: r.WorkflowID,
		EnvelopeId: r.EnvelopeID,
		Phase:      r.Phase,
		ModelId:    r.ModelID,
		ToolNames:  r.ToolNames,
		InputText:  r.InputText,
		OutputText: r.OutputText,
		Attributes: attributes,
	}
}

// FromProtoContext creates an EvaluationRequest from a protobuf EvaluationContext.
func FromProtoContext(ctx *EvaluationContext) *EvaluationRequest {
	if ctx == nil {
		return &EvaluationRequest{}
	}
	return &EvaluationRequest{
		TenantID:   ctx.TenantId,
		UserID:     ctx.UserId,
		UserRoles:  ctx.UserRoles,
		WorkflowID: ctx.WorkflowId,
		EnvelopeID: ctx.EnvelopeId,
		Phase:      ctx.Phase,
		ModelID:    ctx.ModelId,
		ToolNames:  ctx.ToolNames,
		InputText:  ctx.InputText,
		OutputText: ctx.OutputText,
		Attributes: ctx.Attributes,
	}
}
