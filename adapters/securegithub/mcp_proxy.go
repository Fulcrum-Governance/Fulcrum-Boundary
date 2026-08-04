package securegithub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const (
	GitHubMCPRemoteURL       = "https://api.githubcopilot.com/mcp/"
	GitHubIssueReadTool      = "issue_read"
	GitHubProtectedWriteTool = "create_or_update_file"

	mcpSessionHeader = "Mcp-Session-Id"
	mcpVersionHeader = "Mcp-Protocol-Version"
	maxMCPBody       = 4 << 20
)

// MCPRouteConfig joins a local, stateful Boundary route to the official GitHub
// MCP endpoint. Source is untrusted by operator configuration; no semantic
// classification of issue prose is performed.
type MCPRouteConfig struct {
	UpstreamURL    string
	Token          string
	SourceOwner    string
	SourceRepo     string
	SourceIssue    int
	TargetOwner    string
	TargetRepo     string
	TargetBranch   string
	AgentID        string
	TenantID       string
	GatewayVersion string
	BuildDigest    string
	Timeout        time.Duration
	HTTPClient     *http.Client
	DecisionSink   MCPDecisionSink
	ForwardSink    MCPForwardEventSink
}

type MCPDecisionSink interface {
	WriteDecision(context.Context, governance.DecisionRecordV1) error
}

// MCPForwardEvent is independent of the decision record. A blocked event is
// emitted at the forwarder boundary before a protected mutation can be sent.
type MCPForwardEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	RequestID string    `json:"request_id"`
	SessionID string    `json:"session_id,omitempty"`
	Tool      string    `json:"tool,omitempty"`
	Outcome   string    `json:"outcome"`
	Forwarded bool      `json:"forwarded"`
	Mutation  bool      `json:"mutation"`
}

type MCPForwardEventSink interface {
	WriteForwardEvent(context.Context, MCPForwardEvent) error
}

type mcpRouteSession struct {
	upstreamID string
	tainted    bool
}

// mcpAuditCapture holds only the in-flight canonical decision event keyed by
// request. governTool consumes the entry immediately after Pipeline.Evaluate,
// so long-lived routes do not accumulate an unbounded audit history.
type mcpAuditCapture struct {
	mu     sync.Mutex
	events map[string]governance.AuditEvent
}

func newMCPAuditCapture() *mcpAuditCapture {
	return &mcpAuditCapture{events: make(map[string]governance.AuditEvent)}
}

func (p *mcpAuditCapture) Publish(_ context.Context, event governance.AuditEvent) {
	if event.EventType != "" && event.EventType != "governance_decision" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events[event.RequestID] = event
}

func (p *mcpAuditCapture) Take(requestID string) (governance.AuditEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	event, ok := p.events[requestID]
	delete(p.events, requestID)
	return event, ok
}

func (p *mcpAuditCapture) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events)
}

// MCPRoute is the stateful Secure GitHub HTTP MCP prerequisite. It forwards
// protocol traffic and the two configured tools, while enforcing the
// read-to-taint/write-deny invariant before the mutation forwarder.
type MCPRoute struct {
	cfg      MCPRouteConfig
	upstream *url.URL
	pipeline *governance.Pipeline
	auditor  *mcpAuditCapture
	mu       sync.RWMutex
	sessions map[string]*mcpRouteSession
}

func NewMCPRoute(cfg MCPRouteConfig) (*MCPRoute, error) {
	if cfg.UpstreamURL == "" {
		cfg.UpstreamURL = GitHubMCPRemoteURL
	}
	upstream, err := url.Parse(cfg.UpstreamURL)
	if err != nil || upstream.Scheme == "" || upstream.Host == "" {
		return nil, fmt.Errorf("secure GitHub MCP: valid upstream URL is required")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("secure GitHub MCP: upstream token is required")
	}
	if cfg.SourceOwner == "" || cfg.SourceRepo == "" || cfg.SourceIssue <= 0 {
		return nil, errors.New("secure GitHub MCP: one source owner/repo/issue is required")
	}
	if cfg.TargetOwner == "" || cfg.TargetRepo == "" || cfg.TargetBranch == "" {
		return nil, errors.New("secure GitHub MCP: one target owner/repo/branch is required")
	}
	if cfg.DecisionSink == nil || cfg.ForwardSink == nil {
		return nil, errors.New("secure GitHub MCP: decision and forward evidence sinks are required")
	}
	if cfg.AgentID == "" {
		cfg.AgentID = "claude-code"
	}
	if cfg.TenantID == "" {
		cfg.TenantID = "local"
	}
	if cfg.GatewayVersion == "" {
		cfg.GatewayVersion = "secure-github-mcp"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{}
	}
	auditor := newMCPAuditCapture()
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies:   secureGitHubMCPPolicyRules(),
		GatewayVersion:   cfg.GatewayVersion,
		BuildDigest:      cfg.BuildDigest,
		PolicyBundleHash: "secure-github-mcp-v1",
		TopologyProfile:  "local-stateful-secure-github",
		RequireAgentID:   true,
	}, nil, nil, auditor)
	return &MCPRoute{cfg: cfg, upstream: upstream, pipeline: pipeline, auditor: auditor, sessions: make(map[string]*mcpRouteSession)}, nil
}

func secureGitHubMCPPolicyRules() []governance.StaticPolicyRule {
	deny := func(name, tool, reason string, conditions ...governance.StaticPolicyMatch) governance.StaticPolicyRule {
		return governance.StaticPolicyRule{
			Name: name, Tool: tool, Action: "deny", Reason: reason,
			Transport: string(governance.TransportMCP), DecisionMode: governance.DecisionModeDeterministic,
			Conditions: conditions, Metadata: map[string]string{"profile": ProfileID},
		}
	}
	equals := func(field, value string) governance.StaticPolicyMatch {
		return governance.StaticPolicyMatch{Type: "equals", Field: field, Value: value}
	}
	return []governance.StaticPolicyRule{
		deny("secure-github-tool-allowlist", "github.*", "tool is not exposed by the Secure GitHub route", equals("arguments.tool_allowed", "false")),
		deny("secure-github-source-allowlist", "github."+GitHubIssueReadTool, "source issue is outside the configured untrusted source", equals("arguments.source_allowed", "false")),
		deny("secure-github-sink-allowlist", "github."+GitHubProtectedWriteTool, "target repository or branch is outside the configured protected sink", equals("arguments.sink_allowed", "false")),
		deny("secure-github-write-after-taint", "github."+GitHubProtectedWriteTool, "protected write denied after configured untrusted source read", equals("arguments.tainted", "true")),
		deny("secure-github-read-before-write", "github."+GitHubProtectedWriteTool, "protected write requires a successful configured source read"),
	}
}

type mcpWireMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type mcpToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (r *MCPRoute) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !localOrigin(req.Header.Get("Origin")) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxMCPBody+1))
	if err != nil || len(body) > maxMCPBody {
		r.writeProtocolError(w, nil, -32600, "invalid or oversized MCP request", nil)
		return
	}
	var msg mcpWireMessage
	if err := json.Unmarshal(body, &msg); err != nil || msg.JSONRPC != "2.0" || msg.Method == "" {
		r.writeProtocolError(w, nil, -32600, "invalid MCP request", nil)
		return
	}
	if !allowedClientMCPMethod(msg.Method) {
		if len(msg.ID) == 0 || string(msg.ID) == "null" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		r.writeProtocolError(w, msg.ID, -32601, "MCP method is not exposed by the Secure GitHub route", nil)
		return
	}

	sessionID := req.Header.Get(mcpSessionHeader)
	if msg.Method != "initialize" && sessionID == "" {
		r.writeProtocolError(w, msg.ID, -32000, "missing Boundary MCP session", nil)
		return
	}
	session, ok := r.session(sessionID)
	if msg.Method != "initialize" && !ok {
		r.writeProtocolError(w, msg.ID, -32000, "unknown Boundary MCP session", nil)
		return
	}

	switch msg.Method {
	case "tools/call":
		r.handleToolCall(w, req, body, msg, sessionID, session)
	case "initialize", "notifications/initialized", "notifications/cancelled", "ping", "tools/list":
		r.forwardProtocol(w, req, body, msg, sessionID, session)
	}
}

func allowedClientMCPMethod(method string) bool {
	switch method {
	case "initialize", "notifications/initialized", "notifications/cancelled", "ping", "tools/list", "tools/call":
		return true
	default:
		return false
	}
}

func (r *MCPRoute) handleToolCall(w http.ResponseWriter, req *http.Request, body []byte, msg mcpWireMessage, sessionID string, session *mcpRouteSession) {
	var params mcpToolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil || params.Name == "" {
		r.writeProtocolError(w, msg.ID, -32602, "invalid tools/call parameters", nil)
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	governed, err := r.governTool(req.Context(), sessionID, params.Name, params.Arguments)
	if err != nil {
		r.writeProtocolError(w, msg.ID, -32003, "Boundary governance evidence unavailable", nil)
		return
	}
	if !governed.decision.Allowed() {
		r.emitDenial(w, req.Context(), msg.ID, sessionID, params.Name, governed, params.Name == GitHubProtectedWriteTool)
		return
	}

	switch params.Name {
	case GitHubIssueReadTool:
		response, status, headers, err := r.forward(req.Context(), body, req.Header, session)
		if err != nil {
			r.writeProtocolError(w, msg.ID, -32002, "GitHub MCP upstream unavailable", nil)
			return
		}
		if successfulMCPResponse(response, headers.Get("Content-Type"), msg.ID) {
			// The validated upstream read is security-relevant state. Commit the
			// taint before attempting evidence persistence so a sink failure can
			// fail the response closed without erasing the transition.
			r.markTainted(sessionID)
			event := MCPForwardEvent{
				Timestamp: time.Now().UTC(), EventType: "mcp_forwarder", RequestID: governed.requestID,
				SessionID: sessionID, Tool: params.Name, Outcome: "forwarded_success", Forwarded: true, Mutation: false,
			}
			if err := r.cfg.ForwardSink.WriteForwardEvent(req.Context(), event); err != nil {
				r.writeProtocolError(w, msg.ID, -32003, "Boundary forwarder evidence unavailable", nil)
				return
			}
		}
		r.writeUpstream(w, response, status, headers, false, sessionID)
	case GitHubProtectedWriteTool:
		// The route-owned canonical policy contains an unconditional final deny
		// for this tool. An allow here is a policy integrity fault, never a path
		// to the mutation forwarder.
		r.writeProtocolError(w, msg.ID, -32003, "Boundary protected-write policy failed closed", nil)
	default:
		r.writeProtocolError(w, msg.ID, -32003, "Boundary tool allowlist policy failed closed", nil)
	}
}

func (r *MCPRoute) forwardProtocol(w http.ResponseWriter, req *http.Request, body []byte, msg mcpWireMessage, sessionID string, session *mcpRouteSession) {
	response, status, headers, err := r.forward(req.Context(), body, req.Header, session)
	if err != nil {
		if len(msg.ID) == 0 || string(msg.ID) == "null" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		r.writeProtocolError(w, msg.ID, -32002, "GitHub MCP upstream unavailable", nil)
		return
	}
	if msg.Method == "initialize" && successfulMCPResponse(response, headers.Get("Content-Type"), msg.ID) {
		sessionID = uuid.NewString()
		r.mu.Lock()
		r.sessions[sessionID] = &mcpRouteSession{upstreamID: headers.Get(mcpSessionHeader)}
		r.mu.Unlock()
		w.Header().Set(mcpSessionHeader, sessionID)
	}
	if msg.Method == "tools/list" {
		response, err = filterGitHubTools(response, headers.Get("Content-Type"))
		if err != nil {
			r.writeProtocolError(w, msg.ID, -32002, "invalid GitHub MCP tools response", nil)
			return
		}
	}
	r.writeUpstream(w, response, status, headers, len(msg.ID) == 0 || string(msg.ID) == "null", sessionID)
}

func (r *MCPRoute) forward(ctx context.Context, body []byte, downstream http.Header, session *mcpRouteSession) ([]byte, int, http.Header, error) {
	forwardCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(forwardCtx, http.MethodPost, r.upstream.String(), bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	req.Header.Set("X-MCP-Tools", GitHubIssueReadTool+","+GitHubProtectedWriteTool)
	if version := downstream.Get(mcpVersionHeader); version != "" {
		req.Header.Set(mcpVersionHeader, version)
	}
	if session != nil && session.upstreamID != "" {
		req.Header.Set(mcpSessionHeader, session.upstreamID)
	}
	resp, err := r.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(io.LimitReader(resp.Body, maxMCPBody+1))
	if err != nil || len(response) > maxMCPBody || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, resp.Header, errors.New("upstream MCP response failed")
	}
	return response, resp.StatusCode, resp.Header, nil
}

type governedMCPTool struct {
	decision  *governance.GovernanceDecision
	record    governance.DecisionRecordV1
	requestID string
}

func (r *MCPRoute) governTool(ctx context.Context, sessionID, tool string, args map[string]any) (governedMCPTool, error) {
	requestID := uuid.NewString()
	policyArgs := make(map[string]any, len(args)+4)
	for key, value := range args {
		policyArgs[key] = value
	}
	policyArgs["tool_allowed"] = tool == GitHubIssueReadTool || tool == GitHubProtectedWriteTool
	policyArgs["source_allowed"] = tool == GitHubIssueReadTool && r.allowedSource(args)
	policyArgs["sink_allowed"] = tool == GitHubProtectedWriteTool && r.allowedTarget(args)
	policyArgs["tainted"] = r.isTainted(sessionID)
	governed := &governance.GovernanceRequest{
		RequestID: requestID, Transport: governance.TransportMCP, AgentID: r.cfg.AgentID,
		TenantID: r.cfg.TenantID, ToolName: "github." + tool, Action: "execute",
		Arguments: policyArgs, EnvelopeID: "mcp-session-" + sessionID,
	}
	decision, err := r.pipeline.Evaluate(ctx, governed)
	if err != nil {
		_, _ = r.auditor.Take(requestID)
		return governedMCPTool{}, err
	}
	event, ok := r.auditor.Take(requestID)
	if !ok {
		return governedMCPTool{}, errors.New("canonical pipeline emitted no decision event")
	}
	event.AdapterID = "secure-github-mcp"
	event.RouteID = "boundary-http-github-mcp"
	event.TopologyProfile = "local-stateful-secure-github"
	event.ExecutionClaim = &governance.ExecutionClaim{UpstreamCalled: false, Executed: false, Source: "secure-github-mcp-forwarder"}
	return governedMCPTool{decision: decision, record: governance.BuildDecisionRecord(event), requestID: requestID}, nil
}

func (r *MCPRoute) emitDenial(w http.ResponseWriter, ctx context.Context, id json.RawMessage, sessionID, tool string, governed governedMCPTool, mutation bool) {
	canary := MCPForwardEvent{Timestamp: time.Now().UTC(), EventType: "mcp_forwarder", RequestID: governed.requestID, SessionID: sessionID, Tool: tool, Outcome: "blocked_before_forward", Forwarded: false, Mutation: mutation}
	if err := r.cfg.DecisionSink.WriteDecision(ctx, governed.record); err != nil {
		r.writeProtocolError(w, id, -32003, "Boundary decision evidence unavailable", nil)
		return
	}
	if err := r.cfg.ForwardSink.WriteForwardEvent(ctx, canary); err != nil {
		r.writeProtocolError(w, id, -32003, "Boundary forwarder evidence unavailable", nil)
		return
	}
	r.writeProtocolError(w, id, -32001, governed.decision.Reason, map[string]any{
		"action": "deny", "matched_rule": governed.decision.MatchedRule, "request_id": governed.requestID,
		"session_id": sessionID, "upstream_called": false, "decision_record": governed.record,
	})
}

func (r *MCPRoute) allowedSource(args map[string]any) bool {
	return stringValue(args["method"]) == "get" && stringValue(args["owner"]) == r.cfg.SourceOwner &&
		stringValue(args["repo"]) == r.cfg.SourceRepo && intValue(args["issue_number"]) == r.cfg.SourceIssue
}

func (r *MCPRoute) allowedTarget(args map[string]any) bool {
	return stringValue(args["owner"]) == r.cfg.TargetOwner && stringValue(args["repo"]) == r.cfg.TargetRepo &&
		stringValue(args["branch"]) == r.cfg.TargetBranch
}

func (r *MCPRoute) session(id string) (*mcpRouteSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

func (r *MCPRoute) markTainted(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session := r.sessions[id]; session != nil {
		session.tainted = true
	}
}

func (r *MCPRoute) isTainted(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session := r.sessions[id]
	return session != nil && session.tainted
}

func (r *MCPRoute) writeProtocolError(w http.ResponseWriter, id json.RawMessage, code int, message string, data map[string]any) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	r.writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message, "data": data}})
}

func (r *MCPRoute) writeUpstream(w http.ResponseWriter, body []byte, status int, headers http.Header, notification bool, sessionID string) {
	if notification || status == http.StatusAccepted {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	contentType := headers.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	for _, name := range []string{"Cache-Control", mcpVersionHeader} {
		if value := headers.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (r *MCPRoute) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func localOrigin(origin string) bool {
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func successfulMCPResponse(body []byte, contentType string, expectedID json.RawMessage) bool {
	messages, err := mcpResponseMessages(body, contentType)
	if err != nil {
		return false
	}
	matched := 0
	successful := false
	for _, raw := range messages {
		var msg mcpWireMessage
		if json.Unmarshal(raw, &msg) != nil || !sameJSONRPCID(msg.ID, expectedID) {
			continue
		}
		matched++
		if matched > 1 {
			return false
		}
		if len(msg.Error) != 0 || len(msg.Result) == 0 {
			successful = false
			continue
		}
		var result struct {
			IsError bool `json:"isError"`
		}
		successful = json.Unmarshal(msg.Result, &result) == nil && !result.IsError
	}
	return matched == 1 && successful
}

func sameJSONRPCID(left, right json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(left), bytes.TrimSpace(right))
}

func mcpResponseMessages(body []byte, contentType string) ([][]byte, error) {
	switch baseMediaType(contentType) {
	case "", "application/json":
		if !json.Valid(body) {
			return nil, errors.New("invalid JSON MCP response")
		}
		return [][]byte{body}, nil
	case "text/event-stream":
		return sseDataMessages(body)
	default:
		return nil, fmt.Errorf("unsupported MCP response content type %q", contentType)
	}
}

func sseDataMessages(body []byte) ([][]byte, error) {
	normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
	events := strings.Split(normalized, "\n\n")
	messages := make([][]byte, 0, len(events))
	for _, event := range events {
		var data []string
		for _, line := range strings.Split(event, "\n") {
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if len(data) == 0 {
			continue
		}
		message := []byte(strings.Join(data, "\n"))
		if !json.Valid(message) {
			return nil, errors.New("invalid JSON-RPC data in SSE response")
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return nil, errors.New("SSE response contained no JSON-RPC data")
	}
	return messages, nil
}

func baseMediaType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return strings.ToLower(mediaType)
}

func filterGitHubTools(body []byte, contentType string) ([]byte, error) {
	if baseMediaType(contentType) == "text/event-stream" {
		return transformSSEData(body, filterGitHubToolsJSON)
	}
	return filterGitHubToolsJSON(body)
}

func filterGitHubToolsJSON(body []byte) ([]byte, error) {
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method,omitempty"`
		Result  struct {
			Tools      []json.RawMessage `json:"tools"`
			NextCursor any               `json:"nextCursor,omitempty"`
		} `json:"result"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.New("invalid tools/list response")
	}
	if response.Method != "" {
		return body, nil
	}
	if len(response.Error) != 0 {
		return nil, errors.New("invalid tools/list response")
	}
	filtered := make([]json.RawMessage, 0, 2)
	for _, raw := range response.Result.Tools {
		var tool struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &tool) == nil && (tool.Name == GitHubIssueReadTool || tool.Name == GitHubProtectedWriteTool) {
			filtered = append(filtered, raw)
		}
	}
	response.Result.Tools = filtered
	return json.Marshal(response)
}

func transformSSEData(body []byte, transform func([]byte) ([]byte, error)) ([]byte, error) {
	normalized := strings.ReplaceAll(string(body), "\r\n", "\n")
	events := strings.Split(normalized, "\n\n")
	transformed := make([]string, 0, len(events))
	for _, event := range events {
		if event == "" {
			continue
		}
		lines := strings.Split(event, "\n")
		var data []string
		kept := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
				continue
			}
			kept = append(kept, line)
		}
		if len(data) > 0 {
			updated, err := transform([]byte(strings.Join(data, "\n")))
			if err != nil {
				return nil, err
			}
			kept = append(kept, "data: "+string(updated))
		}
		transformed = append(transformed, strings.Join(kept, "\n"))
	}
	if len(transformed) == 0 {
		return nil, errors.New("SSE response contained no events")
	}
	return []byte(strings.Join(transformed, "\n\n") + "\n\n"), nil
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

func intValue(value any) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
