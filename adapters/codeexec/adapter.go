package codeexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/google/uuid"
)

// Verify Adapter implements governance.TransportAdapter at compile time.
var _ governance.TransportAdapter = (*Adapter)(nil)

// CodeExecInput is the protocol-specific input for code execution requests.
type CodeExecInput struct {
	Code      string `json:"code"`
	Language  string `json:"language"` // "python", "javascript", "typescript", "bash"
	SandboxID string `json:"sandbox_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
}

// Adapter implements governance.TransportAdapter for code execution requests.
type Adapter struct {
	defaultTenantID string
	analyzers       map[string]Analyzer
	sandboxPolicy   SandboxPolicy
	executor        Executor
	boundary        ExecutionBoundary
}

// NewAdapter creates a code-execution transport adapter. If defaultTenantID is
// non-empty it is used when the incoming request omits a tenant ID.
func NewAdapter(defaultTenantID string) *Adapter {
	return newAdapter(defaultTenantID, DefaultSandboxPolicy(), UnconfiguredExecutor{}, DefaultExecutionBoundary())
}

// NewAdapterWithExecutor creates a code-execution adapter that forwards allowed
// requests to a configured execution boundary.
func NewAdapterWithExecutor(defaultTenantID string, executor Executor, boundary ExecutionBoundary) *Adapter {
	if executor == nil {
		executor = UnconfiguredExecutor{}
	}
	if boundary.Name == "" {
		boundary = DefaultExecutionBoundary()
	}
	return newAdapter(defaultTenantID, DefaultSandboxPolicy(), executor, boundary)
}

// NewAdapterWithPolicy creates a code-execution adapter with a custom sandbox
// policy. Execution remains unconfigured unless NewAdapterWithExecutor is used.
func NewAdapterWithPolicy(defaultTenantID string, policy SandboxPolicy) *Adapter {
	return newAdapter(defaultTenantID, normalizeSandboxPolicy(policy), UnconfiguredExecutor{}, DefaultExecutionBoundary())
}

func newAdapter(defaultTenantID string, policy SandboxPolicy, executor Executor, boundary ExecutionBoundary) *Adapter {
	return &Adapter{
		defaultTenantID: defaultTenantID,
		analyzers: map[string]Analyzer{
			"python":     &PythonAnalyzer{},
			"javascript": &JSAnalyzer{},
			"typescript": &JSAnalyzer{},
		},
		sandboxPolicy: normalizeSandboxPolicy(policy),
		executor:      executor,
		boundary:      boundary,
	}
}

// Type returns TransportCodeExec.
func (a *Adapter) Type() governance.TransportType {
	return governance.TransportCodeExec
}

// ParseRequest converts a code-execution input into a canonical
// GovernanceRequest. The raw parameter must be a *CodeExecInput,
// CodeExecInput, json.RawMessage, or []byte.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	var input *CodeExecInput

	switch v := raw.(type) {
	case *CodeExecInput:
		input = v
	case CodeExecInput:
		input = &v
	case json.RawMessage:
		input = &CodeExecInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportCodeExec, "unmarshal input", err)
		}
	case []byte:
		input = &CodeExecInput{}
		if err := json.Unmarshal(v, input); err != nil {
			return nil, governance.NewParseError(governance.TransportCodeExec, "unmarshal input", err)
		}
	default:
		return nil, governance.NewParseError(governance.TransportCodeExec, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}

	if input.Code == "" {
		return nil, governance.NewParseError(governance.TransportCodeExec, "code field is required", nil)
	}
	if input.Language == "" {
		return nil, governance.NewParseError(governance.TransportCodeExec, "language field is required", nil)
	}

	tenantID := input.TenantID
	if tenantID == "" {
		tenantID = a.defaultTenantID
	}

	// Select the language-specific analyser.
	lang := strings.ToLower(input.Language)
	analyzer, ok := a.analyzers[lang]

	var ops []Operation
	if ok {
		ops = analyzer.Analyze(input.Code)
	}
	_, denied := EnforcePolicy(a.sandboxPolicy, ops)
	if !a.sandboxPolicy.LanguageAllowed(lang) {
		denied = append(denied, fmt.Sprintf("language %q denied: not allowed by code execution policy", lang))
	}

	action := HighestOperationRisk(ops)

	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportCodeExec,
		AgentID:   input.AgentID,
		TenantID:  tenantID,
		ToolName:  "code_exec",
		Action:    action,
		Arguments: codeExecArguments(lang, input.SandboxID, ops, denied),
		Code:      input.Code,
		Language:  lang,
		SandboxID: input.SandboxID,
	}, nil
}

// ForwardGoverned executes governed code only when both the governance decision
// and the adapter sandbox policy allow it. Execution is always delegated to the
// configured executor boundary.
func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("governance request is required")
	}
	if decision == nil || !decision.Allowed() {
		return codeExecDeniedResponse(decision), nil
	}
	if sandboxDecision := a.sandboxPolicyDecision(req); sandboxDecision != nil {
		return codeExecDeniedResponse(sandboxDecision), nil
	}

	resp, err := a.executor.Execute(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	resp.Metadata["codeexec_boundary"] = a.boundary.Name
	resp.Metadata["codeexec_boundary_kind"] = a.boundary.Kind
	resp.Metadata["codeexec_secure_sandbox"] = strconv.FormatBool(a.boundary.SecureSandbox)
	if err := a.EmitGovernanceMetadata(ctx, resp, decision); err != nil {
		return nil, err
	}
	inspection, err := a.InspectResponse(ctx, resp)
	if err != nil {
		return nil, err
	}
	attachCodeExecInspectionMetadata(resp, inspection)
	return resp, nil
}

// maxSafeOutputSize is the threshold above which a response is flagged.
const maxSafeOutputSize = 50 * 1024 // 50 KB

// sensitivePatterns are substrings that, if found in the response content,
// trigger a sensitive-data concern.
var sensitivePatterns = []string{
	"BEGIN RSA PRIVATE KEY",
	"BEGIN PRIVATE KEY",
	"BEGIN EC PRIVATE KEY",
	"AKIA",           // AWS access key prefix
	"password",       // generic
	"secret_key",     // generic
	"api_key",        // generic
	"Authorization:", // HTTP header
	"Bearer ",        // OAuth token prefix
}

// InspectResponse examines code-execution output for governance concerns.
func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}, nil
	}

	inspection := &governance.ResponseInspection{
		Safe: true,
	}

	// Check output size.
	if int64(len(resp.Content)) > maxSafeOutputSize {
		inspection.Concerns = append(inspection.Concerns,
			fmt.Sprintf("output size %d bytes exceeds %d byte limit", len(resp.Content), maxSafeOutputSize))
		inspection.Safe = false
	}

	// Check for sensitive data patterns.
	content := string(resp.Content)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(content, pattern) {
			inspection.SensitiveData = true
			inspection.Safe = false
			inspection.Concerns = append(inspection.Concerns,
				fmt.Sprintf("potential sensitive data detected: %q pattern found", pattern))
			break
		}
	}

	// Non-zero exit code is a concern (might indicate attempted privilege escalation).
	if resp.ExitCode != 0 {
		inspection.Concerns = append(inspection.Concerns,
			fmt.Sprintf("non-zero exit code: %d", resp.ExitCode))
	}

	return inspection, nil
}

// EmitGovernanceMetadata attaches governance and code-exec specific metadata
// to the tool response.
func (a *Adapter) EmitGovernanceMetadata(_ context.Context, resp *governance.ToolResponse, decision *governance.GovernanceDecision) error {
	if resp == nil || decision == nil {
		return nil
	}
	if resp.Metadata == nil {
		resp.Metadata = make(map[string]string)
	}
	resp.Metadata["x-fulcrum-action"] = decision.Action
	resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
	resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
	resp.Metadata["x-fulcrum-transport"] = string(governance.TransportCodeExec)
	if decision.PolicyID != "" {
		resp.Metadata["x-fulcrum-policy-id"] = decision.PolicyID
	}
	if decision.MatchedRule != "" {
		resp.Metadata["x-fulcrum-rule"] = decision.MatchedRule
	}
	return nil
}

// GovernCode runs the complete configured CodeExec lifecycle.
func (a *Adapter) GovernCode(ctx context.Context, raw any, pipeline *governance.Pipeline) (*governance.ToolResponse, error) {
	req, err := a.ParseRequest(ctx, raw)
	if err != nil {
		return nil, err
	}
	if pipeline == nil {
		decision := &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     "governance pipeline is required",
			EnvelopeID: req.EnvelopeID,
		}
		return a.ForwardGoverned(ctx, req, decision)
	}
	decision, err := pipeline.Evaluate(ctx, req)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: req.EnvelopeID,
		}
	}
	return a.ForwardGoverned(ctx, req, decision)
}

func (a *Adapter) sandboxPolicyDecision(req *governance.GovernanceRequest) *governance.GovernanceDecision {
	denials, _ := req.Arguments["sandbox_policy_denials"].(string)
	if strings.TrimSpace(denials) == "" {
		return nil
	}
	return &governance.GovernanceDecision{
		RequestID:    req.RequestID,
		Action:       "deny",
		Reason:       "code execution policy denied: " + denials,
		MatchedRule:  "codeexec-sandbox-policy",
		EnvelopeID:   req.EnvelopeID,
		TrustScore:   1.0,
		DecisionMode: governance.DecisionModeDeterministic,
	}
}

func codeExecArguments(language, sandboxID string, ops []Operation, denials []string) map[string]any {
	args := map[string]any{
		"language":                  language,
		"sandbox_id":                sandboxID,
		"operation_count":           len(ops),
		"operations":                operationSummaries(ops),
		"required_capabilities":     strings.Join(requiredCapabilities(ops), ","),
		"sandbox_policy_denials":    strings.Join(denials, "; "),
		"obfuscation_detected":      strconv.FormatBool(hasObfuscation(ops)),
		"network_behavior":          behaviorForCapability(ops, CapabilityNetwork),
		"filesystem_behavior":       filesystemBehavior(ops),
		"subprocess_behavior":       behaviorForCapability(ops, CapabilitySubprocess),
		"resource_access_violation": strconv.FormatBool(len(denials) > 0),
	}
	return args
}

func operationSummaries(ops []Operation) string {
	if len(ops) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ops))
	for _, op := range ops {
		parts = append(parts, op.Type+":"+op.RiskLevel)
	}
	return strings.Join(parts, ",")
}

func requiredCapabilities(ops []Operation) []string {
	seen := map[Capability]bool{}
	var out []string
	for _, op := range ops {
		capability, ok := operationCapability[op.Type]
		if !ok || seen[capability] {
			continue
		}
		seen[capability] = true
		out = append(out, string(capability))
	}
	return out
}

func hasObfuscation(ops []Operation) bool {
	for _, op := range ops {
		if op.Type == "obfuscated_exec" || strings.Contains(op.Detail, "base64") || strings.Contains(op.Detail, "decode") {
			return true
		}
	}
	return false
}

func behaviorForCapability(ops []Operation, capability Capability) string {
	for _, op := range ops {
		if operationCapability[op.Type] == capability {
			return "detected"
		}
	}
	return "not_detected"
}

func filesystemBehavior(ops []Operation) string {
	read := behaviorForCapability(ops, CapabilityFilesystemRead) == "detected"
	write := behaviorForCapability(ops, CapabilityFilesystemWrite) == "detected"
	switch {
	case read && write:
		return "read_write_detected"
	case write:
		return "write_detected"
	case read:
		return "read_detected"
	default:
		return "not_detected"
	}
}

func codeExecDeniedResponse(decision *governance.GovernanceDecision) *governance.ToolResponse {
	reason := "denied by Boundary"
	if decision != nil && decision.Reason != "" {
		reason = decision.Reason
	}
	resp := &governance.ToolResponse{
		Content:     []byte(reason + "\n"),
		ContentType: "text/plain",
		ExitCode:    126,
		Metadata: map[string]string{
			"x-fulcrum-action": "deny",
			"codeexec_denied":  "true",
		},
	}
	if decision != nil {
		resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
		resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
		if decision.MatchedRule != "" {
			resp.Metadata["x-fulcrum-rule"] = decision.MatchedRule
		}
	}
	return resp
}

func attachCodeExecInspectionMetadata(resp *governance.ToolResponse, inspection *governance.ResponseInspection) {
	if resp == nil || inspection == nil {
		return
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	resp.Metadata["codeexec_output_safe"] = strconv.FormatBool(inspection.Safe)
	resp.Metadata["codeexec_sensitive_data"] = strconv.FormatBool(inspection.SensitiveData)
	if len(inspection.Concerns) > 0 {
		resp.Metadata["codeexec_inspection_concerns"] = strings.Join(inspection.Concerns, "; ")
	}
}
