// Package securegithub provides a preview Secure MCP profile for GitHub.
//
// The profile is fixture-first: it proves write-after-taint denial before a
// GitHub mutation without requiring live credentials. Live GitHub App
// conformance remains a future production gate.
package securegithub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type Adapter struct {
	cfg      Config
	pipeline *governance.Pipeline
	upstream Upstream
	sessions *SessionStore
	auditor  *CaptureAuditPublisher
}

var _ governance.TransportAdapter = (*Adapter)(nil)

func NewFixtureAdapter(cfg Config) *Adapter {
	cfg = cfg.withDefaults()
	auditor := &CaptureAuditPublisher{}
	pipeline := newPreviewPipeline(cfg, auditor)
	return &Adapter{
		cfg:      cfg,
		pipeline: pipeline,
		upstream: FixtureUpstream{},
		sessions: NewSessionStore(),
		auditor:  auditor,
	}
}

func newPreviewPipeline(cfg Config, auditor *CaptureAuditPublisher) *governance.Pipeline {
	policyHash := "fixture-secure-github"
	if cfg.LiveMode {
		policyHash = "live-secure-github"
	}
	return governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies:   DefaultPolicyRules(),
		GatewayVersion:   cfg.GatewayVersion,
		BuildDigest:      cfg.BuildDigest,
		PolicyBundleHash: policyHash,
	}, nil, nil, auditor)
}

func NewAdapter(cfg Config, pipeline *governance.Pipeline, upstream Upstream) *Adapter {
	cfg = cfg.withDefaults()
	if upstream == nil {
		upstream = FixtureUpstream{}
	}
	auditor := &CaptureAuditPublisher{}
	if pipeline == nil {
		pipeline = newPreviewPipeline(cfg, auditor)
	}
	return &Adapter{
		cfg:      cfg,
		pipeline: pipeline,
		upstream: upstream,
		sessions: NewSessionStore(),
		auditor:  auditor,
	}
}

func (a *Adapter) Type() governance.TransportType {
	return governance.TransportMCP
}

func (a *Adapter) ParseRequest(ctx context.Context, raw any) (*governance.GovernanceRequest, error) {
	call, err := normalizeToolCall(raw)
	if err != nil {
		return nil, governance.NewParseError(governance.TransportMCP, "parse Secure GitHub tool call", err)
	}
	req, _, _, err := a.buildRequest(ctx, call)
	if err != nil {
		return nil, governance.NewParseError(governance.TransportMCP, "build Secure GitHub request", err)
	}
	return req, nil
}

func (a *Adapter) ForwardGoverned(ctx context.Context, req *governance.GovernanceRequest, decision *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("governance request is required")
	}
	if decision == nil || !decision.Allowed() {
		body, _ := json.Marshal(deniedResponse(ToolCall{}, envelopeFromRequest(req), decision, governance.DecisionRecordV1{}))
		return &governance.ToolResponse{Content: body, ContentType: "application/json"}, nil
	}
	return nil, fmt.Errorf("secure GitHub forwarding requires GovernToolCall so envelope state is preserved")
}

func (a *Adapter) InspectResponse(_ context.Context, resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}, nil
	}
	return &governance.ResponseInspection{Safe: true}, nil
}

func (a *Adapter) EmitGovernanceMetadata(_ context.Context, resp *governance.ToolResponse, decision *governance.GovernanceDecision) error {
	if resp == nil || decision == nil {
		return nil
	}
	if resp.Metadata == nil {
		resp.Metadata = map[string]string{}
	}
	resp.Metadata["x-fulcrum-action"] = decision.Action
	resp.Metadata["x-fulcrum-request-id"] = decision.RequestID
	resp.Metadata["x-fulcrum-envelope-id"] = decision.EnvelopeID
	if decision.MatchedRule != "" {
		resp.Metadata["x-fulcrum-rule"] = decision.MatchedRule
	}
	return nil
}

func (a *Adapter) GovernToolCall(ctx context.Context, raw any) (*GovernedResult, error) {
	call, err := normalizeToolCall(raw)
	if err != nil {
		return nil, err
	}
	req, envelope, class, err := a.buildRequest(ctx, call)
	if err != nil {
		unsupported := unsupportedResponse(call, err)
		return &GovernedResult{Response: unsupported, Envelope: Envelope{ProfileID: ProfileID, Status: StatusPreview, FixtureMode: true}}, nil
	}
	decision, err := a.pipeline.Evaluate(ctx, req)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  req.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: req.EnvelopeID,
		}
	}
	record := a.decisionRecord(req.RequestID)
	if record.RecordID == "" {
		record = governance.BuildDecisionRecord(governance.AuditEvent{
			RequestID:      req.RequestID,
			Transport:      governance.TransportMCP,
			ToolName:       req.ToolName,
			Action:         decision.Action,
			Reason:         decision.Reason,
			MatchedRule:    decision.MatchedRule,
			TrustScore:     decision.TrustScore,
			TrustState:     decision.TrustState,
			EnvelopeID:     decision.EnvelopeID,
			AgentID:        req.AgentID,
			TenantID:       req.TenantID,
			RequestHash:    governance.ComputeRequestHash(req),
			DecisionMode:   decision.DecisionMode,
			GatewayVersion: decision.GatewayVersion,
			TraceID:        req.TraceID,
		})
	}
	if decision == nil || !decision.Allowed() {
		return &GovernedResult{
			Response:       deniedResponse(call, envelope, decision, record),
			Decision:       decision,
			DecisionRecord: record,
			Envelope:       envelope,
			UpstreamCalled: false,
		}, nil
	}

	result, err := a.upstream.CallGitHub(ctx, call, envelope)
	if err != nil {
		return nil, err
	}
	if class.TaintsEnvelope {
		a.sessions.MarkTainted(envelope.SessionID, class.TaintSource)
		envelope.Tainted = true
		envelope.TaintSources = appendUnique(envelope.TaintSources, class.TaintSource)
	}
	attachGovernance(result, envelope, decision, record)
	return &GovernedResult{
		Response: MCPResponse{
			JSONRPC: jsonRPCVersion(call),
			ID:      call.ID,
			Result:  result,
		},
		Decision:       decision,
		DecisionRecord: record,
		Envelope:       envelope,
		UpstreamCalled: true,
	}, nil
}

func (a *Adapter) buildRequest(_ context.Context, call ToolCall) (*governance.GovernanceRequest, Envelope, classification, error) {
	tool := call.ToolName
	args := call.Arguments
	if tool == "" {
		tool = call.Params.Name
	}
	if len(args) == 0 {
		args = call.Params.Arguments
	}
	if args == nil {
		args = map[string]any{}
	}
	class, err := classifyTool(tool, args)
	if err != nil {
		return nil, Envelope{}, classification{}, err
	}
	cfg := a.cfg.withDefaults()
	owner, repo := ownerRepo(args, cfg)
	sessionID := firstNonEmpty(call.SessionID, stringArg(args, "session_id"), cfg.SessionID)
	tenantID := firstNonEmpty(call.TenantID, stringArg(args, "tenant_id"), cfg.TenantID)
	agentID := firstNonEmpty(call.AgentID, stringArg(args, "agent_id"), cfg.AgentID)
	traceID := firstNonEmpty(call.TraceID, stringArg(args, "trace_id"))
	repoOK := true
	if cfg.OneRepoPerSession {
		repoOK = a.sessions.BindRepo(sessionID, owner, repo)
	}
	state := a.sessions.Get(sessionID)
	taintSources := append([]string{}, state.TaintSources...)
	if boolArg(args, "tainted") {
		taintSources = appendUnique(taintSources, firstNonEmpty(stringArg(args, "taint_source"), class.TaintSource))
	}
	tainted := state.Tainted || boolArg(args, "tainted")
	envelopeID := firstNonEmpty(stringArg(args, "envelope_id"), "env-"+sessionID)
	requestID := firstNonEmpty(stringArg(args, "request_id"), uuid.New().String())
	envelope := Envelope{
		ProfileID:          ProfileID,
		Status:             StatusPreview,
		SessionID:          sessionID,
		RequestID:          requestID,
		EnvelopeID:         envelopeID,
		TraceID:            traceID,
		TenantID:           tenantID,
		AgentID:            agentID,
		ToolName:           class.ToolName,
		Action:             class.Action,
		Owner:              owner,
		Repo:               repo,
		ResourceID:         owner + "/" + repo,
		CapabilityClass:    class.CapabilityClass,
		RiskClass:          class.CapabilityClass,
		SourceClass:        class.SourceClass,
		TargetSink:         class.TargetSink,
		MutationClass:      class.MutationClass,
		Tainted:            tainted,
		TaintSources:       taintSources,
		CollaboratorModel:  FixtureCollaborator,
		OneRepoPerSession:  cfg.OneRepoPerSession,
		RepoScopeViolation: !repoOK,
		FixtureMode:        cfg.FixtureMode,
	}
	req := &governance.GovernanceRequest{
		RequestID:  requestID,
		Transport:  governance.TransportMCP,
		AgentID:    agentID,
		TenantID:   tenantID,
		ToolName:   "github." + class.ToolName,
		Action:     class.Action,
		Arguments:  envelope.Arguments(),
		EnvelopeID: envelopeID,
		TraceID:    traceID,
	}
	return req, envelope, class, nil
}

func (a *Adapter) decisionRecord(requestID string) governance.DecisionRecordV1 {
	event, ok := a.auditor.EventForRequest(requestID)
	if !ok {
		return governance.DecisionRecordV1{}
	}
	return governance.BuildDecisionRecord(event)
}

type CaptureAuditPublisher struct {
	mu     sync.Mutex
	events []governance.AuditEvent
}

func (p *CaptureAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *CaptureAuditPublisher) EventForRequest(requestID string) (governance.AuditEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := len(p.events) - 1; i >= 0; i-- {
		event := p.events[i]
		if event.RequestID == requestID && (event.EventType == "" || event.EventType == "governance_decision") {
			return event, true
		}
	}
	return governance.AuditEvent{}, false
}

func normalizeToolCall(raw any) (ToolCall, error) {
	switch typed := raw.(type) {
	case ToolCall:
		return normalizeCallFields(typed), nil
	case *ToolCall:
		if typed == nil {
			return ToolCall{}, fmt.Errorf("nil tool call")
		}
		return normalizeCallFields(*typed), nil
	case []byte:
		var call ToolCall
		if err := json.Unmarshal(typed, &call); err != nil {
			return ToolCall{}, err
		}
		return normalizeCallFields(call), nil
	case json.RawMessage:
		var call ToolCall
		if err := json.Unmarshal(typed, &call); err != nil {
			return ToolCall{}, err
		}
		return normalizeCallFields(call), nil
	default:
		return ToolCall{}, fmt.Errorf("unsupported Secure GitHub tool call type %T", raw)
	}
}

func normalizeCallFields(call ToolCall) ToolCall {
	if call.Method == "" {
		call.Method = "tools/call"
	}
	if call.JSONRPC == "" {
		call.JSONRPC = "2.0"
	}
	if call.ToolName == "" {
		call.ToolName = call.Params.Name
	}
	if len(call.Arguments) == 0 {
		call.Arguments = call.Params.Arguments
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	return call
}

func deniedResponse(call ToolCall, envelope Envelope, decision *governance.GovernanceDecision, record governance.DecisionRecordV1) MCPResponse {
	reason := "governance denied GitHub tool call"
	action := "deny"
	matchedRule := ""
	requestID := envelope.RequestID
	envelopeID := envelope.EnvelopeID
	if decision != nil {
		reason = firstNonEmpty(decision.Reason, reason)
		action = decision.Action
		matchedRule = decision.MatchedRule
		requestID = firstNonEmpty(decision.RequestID, requestID)
		envelopeID = firstNonEmpty(decision.EnvelopeID, envelopeID)
	}
	return MCPResponse{
		JSONRPC: jsonRPCVersion(call),
		ID:      call.ID,
		Error: &MCPError{
			Code:    -32001,
			Message: "Boundary denied GitHub tool call",
			Data: map[string]any{
				"profile_id":       ProfileID,
				"profile_status":   StatusPreview,
				"action":           action,
				"reason":           reason,
				"matched_rule":     matchedRule,
				"request_id":       requestID,
				"envelope_id":      envelopeID,
				"tool":             "github." + envelope.ToolName,
				"target_repo":      envelope.TargetRepo(),
				"taint_sources":    envelope.TaintSources,
				"target_sink":      envelope.TargetSink,
				"capability_class": envelope.CapabilityClass,
				"risk_class":       envelope.RiskClass,
				"mutation_class":   envelope.MutationClass,
				"upstream_called":  false,
				"fixture_mode":     envelope.FixtureMode,
				"live_mode":        !envelope.FixtureMode,
				"decision_record":  record,
			},
		},
	}
}

func unsupportedResponse(call ToolCall, err error) MCPResponse {
	return MCPResponse{
		JSONRPC: jsonRPCVersion(call),
		ID:      call.ID,
		Error: &MCPError{
			Code:    -32602,
			Message: "Boundary rejected unsupported Secure GitHub tool call",
			Data: map[string]any{
				"profile_id":      ProfileID,
				"profile_status":  StatusPreview,
				"reason":          err.Error(),
				"upstream_called": false,
				"fixture_mode":    true,
				"live_mode":       false,
			},
		},
	}
}

func attachGovernance(result *MCPResult, envelope Envelope, decision *governance.GovernanceDecision, record governance.DecisionRecordV1) {
	if result == nil || decision == nil {
		return
	}
	if result.Governance == nil {
		result.Governance = map[string]string{}
	}
	result.Governance["action"] = decision.Action
	result.Governance["request_id"] = decision.RequestID
	result.Governance["envelope_id"] = decision.EnvelopeID
	result.Governance["profile_id"] = ProfileID
	result.Governance["profile_status"] = StatusPreview
	result.Governance["target_repo"] = envelope.TargetRepo()
	result.Governance["capability_class"] = envelope.CapabilityClass
	result.Governance["target_sink"] = envelope.TargetSink
	result.Governance["mutation_class"] = envelope.MutationClass
	if decision.MatchedRule != "" {
		result.Governance["matched_rule"] = decision.MatchedRule
	}
	if len(envelope.TaintSources) > 0 {
		result.Governance["taint_source"] = envelope.TaintSources[0]
	}
	result.Envelope = &envelope
	result.DecisionRecord = &record
}

func envelopeFromRequest(req *governance.GovernanceRequest) Envelope {
	if req == nil {
		return Envelope{ProfileID: ProfileID, Status: StatusPreview}
	}
	return Envelope{
		ProfileID:       ProfileID,
		Status:          StatusPreview,
		RequestID:       req.RequestID,
		EnvelopeID:      req.EnvelopeID,
		TenantID:        req.TenantID,
		AgentID:         req.AgentID,
		ToolName:        normalizeToolName(req.ToolName),
		Action:          req.Action,
		CapabilityClass: fmt.Sprint(req.Arguments["capability_class"]),
		RiskClass:       fmt.Sprint(req.Arguments["risk_class"]),
		TargetSink:      fmt.Sprint(req.Arguments["target_sink"]),
		MutationClass:   fmt.Sprint(req.Arguments["mutation_class"]),
		FixtureMode:     !truthy(req.Arguments["live_github_evidence"]),
	}
}

func jsonRPCVersion(call ToolCall) string {
	if call.JSONRPC == "" {
		return "2.0"
	}
	return call.JSONRPC
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
