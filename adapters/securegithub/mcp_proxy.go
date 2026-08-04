package securegithub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// MCPRoute is the stateful Secure GitHub HTTP MCP prerequisite. It forwards
// protocol traffic and the two configured tools, while enforcing the
// read-to-taint/write-deny invariant before the mutation forwarder.
type MCPRoute struct {
	cfg      MCPRouteConfig
	upstream *url.URL
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
	return &MCPRoute{cfg: cfg, upstream: upstream, sessions: make(map[string]*mcpRouteSession)}, nil
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
	default:
		r.forwardProtocol(w, req, body, msg, sessionID, session)
	}
}

func (r *MCPRoute) handleToolCall(w http.ResponseWriter, req *http.Request, body []byte, msg mcpWireMessage, sessionID string, session *mcpRouteSession) {
	var params mcpToolParams
	if err := json.Unmarshal(msg.Params, &params); err != nil || params.Name == "" {
		r.deny(w, req.Context(), msg.ID, sessionID, "", nil, "invalid tools/call parameters", "secure-github-invalid-tool-call", false)
		return
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}

	switch params.Name {
	case GitHubIssueReadTool:
		if !r.allowedSource(params.Arguments) {
			r.deny(w, req.Context(), msg.ID, sessionID, params.Name, params.Arguments, "source issue is outside the configured untrusted source", "secure-github-source-allowlist", false)
			return
		}
		response, status, headers, err := r.forward(req.Context(), body, req.Header, session)
		if err != nil {
			r.writeProtocolError(w, msg.ID, -32002, "GitHub MCP upstream unavailable", nil)
			return
		}
		if isSuccessfulMCPResponse(response) {
			r.markTainted(sessionID)
		}
		r.writeUpstream(w, response, status, headers, false, sessionID)
	case GitHubProtectedWriteTool:
		reason, rule := "protected write requires a successful configured source read", "secure-github-read-before-write"
		if !r.allowedTarget(params.Arguments) {
			reason, rule = "target repository or branch is outside the configured protected sink", "secure-github-sink-allowlist"
		} else if r.isTainted(sessionID) {
			reason, rule = "protected write denied after configured untrusted source read", "secure-github-write-after-taint"
		}
		r.deny(w, req.Context(), msg.ID, sessionID, params.Name, params.Arguments, reason, rule, true)
	default:
		r.deny(w, req.Context(), msg.ID, sessionID, params.Name, params.Arguments, "tool is not exposed by the Secure GitHub route", "secure-github-tool-allowlist", false)
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
	if msg.Method == "initialize" && isSuccessfulMCPResponse(response) {
		sessionID = uuid.NewString()
		r.mu.Lock()
		r.sessions[sessionID] = &mcpRouteSession{upstreamID: headers.Get(mcpSessionHeader)}
		r.mu.Unlock()
		w.Header().Set(mcpSessionHeader, sessionID)
	}
	if msg.Method == "tools/list" {
		response, err = filterGitHubTools(response)
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

func (r *MCPRoute) deny(w http.ResponseWriter, ctx context.Context, id json.RawMessage, sessionID, tool string, args map[string]any, reason, rule string, mutation bool) {
	requestID := uuid.NewString()
	governed := &governance.GovernanceRequest{
		RequestID: requestID, Transport: governance.TransportMCP, AgentID: r.cfg.AgentID,
		TenantID: r.cfg.TenantID, ToolName: "github." + tool, Action: "execute",
		Arguments: args, EnvelopeID: "mcp-session-" + sessionID,
	}
	record := governance.BuildDecisionRecord(governance.AuditEvent{
		RequestID: requestID, Transport: governance.TransportMCP, ToolName: governed.ToolName,
		Action: "deny", Reason: reason, MatchedRule: rule, PolicyBundleHash: "secure-github-mcp-v1",
		GatewayVersion: r.cfg.GatewayVersion, BoundaryBuildDigest: r.cfg.BuildDigest,
		RequestHash: governance.ComputeRequestHash(governed), AgentID: r.cfg.AgentID, TenantID: r.cfg.TenantID,
		EnvelopeID: governed.EnvelopeID, DecisionMode: governance.DecisionModeDeterministic,
		AdapterID: "secure-github-mcp", RouteID: "boundary-http-github-mcp",
		TopologyProfile: "local-stateful-secure-github",
		ExecutionClaim:  &governance.ExecutionClaim{UpstreamCalled: false, Executed: false, Source: "secure-github-mcp-forwarder"},
	})
	canary := MCPForwardEvent{Timestamp: time.Now().UTC(), EventType: "mcp_forwarder", RequestID: requestID, SessionID: sessionID, Tool: tool, Outcome: "blocked_before_forward", Forwarded: false, Mutation: mutation}
	if r.cfg.DecisionSink != nil {
		if err := r.cfg.DecisionSink.WriteDecision(ctx, record); err != nil {
			r.writeProtocolError(w, id, -32003, "Boundary decision evidence unavailable", nil)
			return
		}
	}
	if r.cfg.ForwardSink != nil {
		if err := r.cfg.ForwardSink.WriteForwardEvent(ctx, canary); err != nil {
			r.writeProtocolError(w, id, -32003, "Boundary forwarder evidence unavailable", nil)
			return
		}
	}
	r.writeProtocolError(w, id, -32001, reason, map[string]any{
		"action": "deny", "matched_rule": rule, "request_id": requestID,
		"session_id": sessionID, "upstream_called": false, "decision_record": record,
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
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(mcpVersionHeader, headers.Get(mcpVersionHeader))
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

func isSuccessfulMCPResponse(body []byte) bool {
	var msg mcpWireMessage
	if json.Unmarshal(body, &msg) != nil || len(msg.Error) != 0 {
		return false
	}
	if len(msg.Result) == 0 {
		return false
	}
	var result struct {
		IsError bool `json:"isError"`
	}
	return json.Unmarshal(msg.Result, &result) == nil && !result.IsError
}

func filterGitHubTools(body []byte) ([]byte, error) {
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  struct {
			Tools      []json.RawMessage `json:"tools"`
			NextCursor any               `json:"nextCursor,omitempty"`
		} `json:"result"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &response); err != nil || len(response.Error) != 0 {
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
