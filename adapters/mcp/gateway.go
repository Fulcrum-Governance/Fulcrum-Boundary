package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Gateway is an HTTP JSON-RPC MCP proxy that governs every request before
// forwarding it to the upstream MCP server.
type Gateway struct {
	Pipeline        *governance.Pipeline
	Adapter         *Adapter
	DefaultTenantID string
	UpstreamAddress string
}

// NewGateway creates a governed MCP JSON-RPC proxy.
func NewGateway(pipeline *governance.Pipeline, upstream Forwarder, defaultTenantID string) *Gateway {
	return &Gateway{
		Pipeline:        pipeline,
		Adapter:         NewProxyAdapter(defaultTenantID, upstream),
		DefaultTenantID: defaultTenantID,
	}
}

// ServeHTTP handles single and batch JSON-RPC requests.
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity := ExtractIdentity(r, g.DefaultTenantID)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		g.publishParseRejection(r.Context(), nil, identity, err.Error())
		writeJSONRPCError(w, nil, -32700, "parse error", err.Error())
		return
	}
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		g.publishParseRejection(r.Context(), body, identity, "empty request body")
		writeJSONRPCError(w, nil, -32600, "invalid request", "empty request body")
		return
	}
	if body[0] == '[' {
		g.serveBatch(w, r.Context(), body, identity)
		return
	}
	resp, emit, decision := g.handleOne(r.Context(), body, identity)
	if decision != nil {
		writeHTTPGovernanceHeaders(w, decision)
	}
	if !emit {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
}

func (g *Gateway) serveBatch(w http.ResponseWriter, ctx context.Context, body []byte, identity Identity) {
	var raws []json.RawMessage
	if err := json.Unmarshal(body, &raws); err != nil || len(raws) == 0 {
		g.publishParseRejection(ctx, body, identity, "batch must contain at least one request")
		writeJSONRPCError(w, nil, -32600, "invalid request", "batch must contain at least one request")
		return
	}
	responses := make([]json.RawMessage, 0, len(raws))
	for _, raw := range raws {
		resp, emit, _ := g.handleOne(ctx, raw, identity)
		if emit {
			responses = append(responses, json.RawMessage(resp.Body))
		}
	}
	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	encoded, _ := json.Marshal(responses)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

type gatewayResponse struct {
	Status int
	Body   []byte
}

type jsonrpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (g *Gateway) handleOne(ctx context.Context, raw []byte, identity Identity) (gatewayResponse, bool, *governance.GovernanceDecision) {
	var req jsonrpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		g.publishParseRejection(ctx, raw, identity, err.Error())
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCError(nil, -32700, "parse error", err.Error())}, true, nil
	}
	if req.JSONRPC != "2.0" || req.Method == "" {
		g.publishParseRejection(ctx, raw, identity, "jsonrpc 2.0 and method are required")
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCError(req.ID, -32600, "invalid request", "jsonrpc 2.0 and method are required")}, true, nil
	}
	emitResponse := len(req.ID) > 0

	gReq, err := g.governanceRequest(raw, req, identity)
	if err != nil {
		g.publishParseRejection(ctx, raw, identity, err.Error())
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCError(req.ID, -32602, "invalid params", err.Error())}, emitResponse, nil
	}
	decision, err := g.Pipeline.Evaluate(ctx, gReq)
	if err != nil {
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCError(req.ID, -32000, "governance pipeline error", err.Error())}, emitResponse, nil
	}
	if !decision.Allowed() {
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCErrorWithGovernance(req.ID, -32001, "governance denied", decision.Reason, decision)}, emitResponse, decision
	}

	resp, err := g.Adapter.ForwardGoverned(ctx, gReq, decision)
	if err != nil {
		return gatewayResponse{Status: http.StatusOK, Body: jsonRPCError(req.ID, -32002, "upstream forwarding failed", err.Error())}, emitResponse, decision
	}
	_, _ = g.Adapter.InspectResponse(ctx, resp)
	out := resp.Content
	if req.Method == "tools/list" {
		out = filterToolsList(ctx, out, g.Pipeline, identity)
	}
	out = attachGovernanceMetadata(out, decision)
	return gatewayResponse{Status: http.StatusOK, Body: out}, emitResponse, decision
}

func (g *Gateway) publishParseRejection(ctx context.Context, raw []byte, identity Identity, reason string) {
	if g == nil || g.Pipeline == nil {
		return
	}
	g.Pipeline.PublishParseRejection(ctx, governance.ParseRejectionEvent{
		Adapter:         governance.TransportMCP,
		RawPayload:      raw,
		RejectionReason: reason,
		AgentID:         identity.AgentID,
		TenantID:        identity.TenantID,
		TraceID:         identity.TraceID,
	})
}

func (g *Gateway) governanceRequest(raw []byte, rpc jsonrpcRequest, identity Identity) (*governance.GovernanceRequest, error) {
	input := ToolCallInput{
		Method:   rpc.Method,
		AgentID:  identity.AgentID,
		TenantID: identity.TenantID,
		TraceID:  identity.TraceID,
	}
	if len(rpc.Params) > 0 {
		if err := json.Unmarshal(rpc.Params, &input.Params); err != nil {
			return nil, err
		}
	}
	if rpc.Method == "tools/list" {
		input.ToolName = "tools/list"
	}
	req, err := g.Adapter.ParseRequest(context.Background(), input)
	if err != nil {
		return nil, err
	}
	req.RawPayload = raw
	if len(rpc.ID) > 0 {
		req.TraceID = string(rpc.ID)
	}
	return req, nil
}

func (a *Adapter) InspectJSONRPCResponse(resp *governance.ToolResponse) (*governance.ResponseInspection, error) {
	return InspectJSONRPCResponse(resp)
}

// BypassProbe verifies that an agent-visible direct path to upstream is closed.
func BypassProbe(ctx context.Context, address string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", address)
	if err == nil {
		_ = conn.Close()
		return errors.New("direct upstream connection succeeded")
	}
	return nil
}

func writeJSONRPCError(w http.ResponseWriter, id json.RawMessage, code int, message, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jsonRPCError(id, code, message, detail))
}

func jsonRPCError(id json.RawMessage, code int, message, detail string) []byte {
	return jsonRPCErrorWithGovernance(id, code, message, detail, nil)
}

func jsonRPCErrorWithGovernance(id json.RawMessage, code int, message, detail string, decision *governance.GovernanceDecision) []byte {
	if len(id) == 0 {
		id = []byte("null")
	}
	data := map[string]any{}
	if detail != "" {
		data["detail"] = detail
	}
	if decision != nil {
		data["governance"] = metadataFromDecision(decision)
	}
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
			"data":    data,
		},
	}
	encoded, _ := json.Marshal(resp)
	return encoded
}
