package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fulcrum-governance/boundary/adapters/mcp"
	"github.com/fulcrum-governance/boundary/governance"
	"github.com/fulcrum-governance/boundary/policyeval"
)

func TestMCPGateway_AllowsForwardsOnceAndAddsMetadata(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  map[string]any{"content": "ok"},
		})
	}))
	defer upstream.Close()

	gateway := mcp.NewGateway(governance.NewPipeline(governance.PipelineConfig{GatewayVersion: "test"}, nil, nil, nil), mcp.NewHTTPForwarder(upstream.URL), "tenant-a")
	body := []byte(`{"jsonrpc":"2.0","id":"req-1","method":"tools/call","params":{"name":"safe_tool","arguments":{"x":1}}}`)
	resp := postJSON(t, gateway, body)

	if upstreamCalls != 1 {
		t.Fatalf("upstream calls = %d, want 1", upstreamCalls)
	}
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(governance.HeaderGovernanceAction); got != "allow" {
		t.Fatalf("governance action header = %q", got)
	}
	decoded := decodeObject(t, resp.Body.Bytes())
	if decoded["id"] != "req-1" {
		t.Fatalf("JSON-RPC id was not preserved: %#v", decoded["id"])
	}
	meta := decoded["result"].(map[string]any)["_meta"].(map[string]any)["governance"].(map[string]any)
	if meta["action"] != "allow" || meta["gateway_version"] != "test" {
		t.Fatalf("missing governance metadata: %#v", meta)
	}
}

func TestMCPGateway_DeniedRequestNeverReachesUpstream(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","result":{}}`))
	}))
	defer upstream.Close()

	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "block-danger", Tool: "danger", Action: "deny", Reason: "blocked"},
		},
	}, nil, nil, nil)
	gateway := mcp.NewGateway(pipeline, mcp.NewHTTPForwarder(upstream.URL), "tenant-a")
	resp := postJSON(t, gateway, []byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"danger"}}`))

	if upstreamCalls != 0 {
		t.Fatalf("denied request reached upstream %d times", upstreamCalls)
	}
	decoded := decodeObject(t, resp.Body.Bytes())
	if decoded["id"].(float64) != 7 {
		t.Fatalf("JSON-RPC id was not preserved on denial: %#v", decoded["id"])
	}
	errObj := decoded["error"].(map[string]any)
	if errObj["message"] != "governance denied" {
		t.Fatalf("unexpected error response: %#v", errObj)
	}
}

func TestMCPGateway_ToolsListFiltersDeniedTools(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result": map[string]any{
				"tools": []map[string]any{
					{"name": "safe"},
					{"name": "danger"},
				},
			},
		})
	}))
	defer upstream.Close()

	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{
			{Name: "hide-danger", Tool: "danger", Action: "deny", Reason: "blocked"},
		},
	}, nil, nil, nil)
	gateway := mcp.NewGateway(pipeline, mcp.NewHTTPForwarder(upstream.URL), "tenant-a")
	resp := postJSON(t, gateway, []byte(`{"jsonrpc":"2.0","id":"list-1","method":"tools/list"}`))

	result := decodeObject(t, resp.Body.Bytes())["result"].(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "safe" {
		t.Fatalf("tools/list was not filtered: %#v", tools)
	}
}

func TestMCPGateway_BatchAndNotifications(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req["id"], "result": map[string]any{"ok": true}})
	}))
	defer upstream.Close()

	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{Name: "block-danger", Tool: "danger", Action: "deny", Reason: "blocked"}},
	}, nil, nil, nil)
	gateway := mcp.NewGateway(pipeline, mcp.NewHTTPForwarder(upstream.URL), "tenant-a")
	batch := []byte(`[
		{"jsonrpc":"2.0","id":"a","method":"tools/call","params":{"name":"safe"}},
		{"jsonrpc":"2.0","id":"b","method":"tools/call","params":{"name":"danger"}},
		{"jsonrpc":"2.0","method":"tools/call","params":{"name":"notify"}}
	]`)
	resp := postJSON(t, gateway, batch)
	if upstreamCalls != 2 {
		t.Fatalf("upstream calls = %d, want 2 allowed requests including notification", upstreamCalls)
	}
	var responses []map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &responses); err != nil {
		t.Fatal(err)
	}
	if len(responses) != 2 {
		t.Fatalf("batch response count = %d, want responses only for id-bearing requests", len(responses))
	}
}

func TestMCPGateway_MalformedJSONRPCReturnsProtocolError(t *testing.T) {
	gateway := mcp.NewGateway(governance.NewPipeline(governance.PipelineConfig{}, nil, nil, nil), mcp.NewHTTPForwarder("http://127.0.0.1:1"), "tenant-a")
	resp := postJSON(t, gateway, []byte(`{not-json`))
	decoded := decodeObject(t, resp.Body.Bytes())
	errObj := decoded["error"].(map[string]any)
	if errObj["code"].(float64) != -32700 {
		t.Fatalf("expected parse error, got %#v", errObj)
	}
}

func TestMCPGateway_FailClosedOnPipelineError(t *testing.T) {
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, failingEvaluator{}, nil)
	gateway := mcp.NewGateway(pipeline, mcp.NewHTTPForwarder(upstream.URL), "tenant-a")
	resp := postJSON(t, gateway, []byte(`{"jsonrpc":"2.0","id":"err","method":"tools/call","params":{"name":"safe"}}`))
	if upstreamCalls != 0 {
		t.Fatalf("fail-closed request reached upstream")
	}
	errObj := decodeObject(t, resp.Body.Bytes())["error"].(map[string]any)
	if errObj["message"] != "governance denied" {
		t.Fatalf("unexpected fail-closed response: %#v", errObj)
	}
}

func TestMCPBypassProbeFailsWhenDirectPathIsClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := mcp.BypassProbe(ctx, addr); err != nil {
		t.Fatalf("closed upstream path should pass bypass probe: %v", err)
	}
}

type failingEvaluator struct{}

func (failingEvaluator) Evaluate(context.Context, *policyeval.EvaluationRequest) (*policyeval.Decision, error) {
	return nil, fmt.Errorf("policy engine unavailable")
}

func postJSON(t *testing.T, handler http.Handler, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(governance.HeaderGovernanceAgentID, "agent-a")
	req.Header.Set(governance.HeaderGovernanceTenantID, "tenant-a")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func decodeObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode response: %v body=%s", err, body)
	}
	return decoded
}
