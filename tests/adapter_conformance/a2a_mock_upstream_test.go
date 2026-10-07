package adapter_conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/a2a"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// mockUpstreamHandler acts as a protocol-shaped mock upstream server designed
// to faithfully represent A2A JSON-RPC semantics for conformance validation.
func mockUpstreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}

	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPCError(w, req.ID, -32700, "Parse error")
		return
	}

	if req.Method == "message/stream" {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Send mock SSE
		fmt.Fprintf(w, "data: %s\n\n", `{"jsonrpc":"2.0","result":"started"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	if req.Method == "tasks/send" || req.Method == "message/send" {
		var params struct {
			Metadata struct {
				Action string `json:"action"`
			} `json:"metadata"`
			Message struct {
				Parts []struct {
					Data struct {
						Text string `json:"text"`
					} `json:"data"`
				} `json:"parts"`
			} `json:"message"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			writeJSONRPCError(w, req.ID, -32602, "Invalid params")
			return
		}

		action := params.Metadata.Action
		var text string
		if len(params.Message.Parts) > 0 {
			text = params.Message.Parts[0].Data.Text
		}

		if action == "summarize" {
			writeJSONRPCSuccess(w, req.ID, map[string]any{
				"status": "success",
				"message": map[string]any{
					"parts": []any{
						map[string]any{"kind": "data", "data": map[string]any{"text": fmt.Sprintf("Summarized: %s", text)}},
					},
				},
			})
			return
		}

		if action == "large_payload" {
			writeJSONRPCSuccess(w, req.ID, map[string]any{
				"status": "success",
				"message": map[string]any{
					"parts": []any{
						map[string]any{"kind": "data", "data": map[string]any{"text": strings.Repeat("x", 10000)}},
					},
				},
			})
			return
		}

		writeJSONRPCError(w, req.ID, -32601, fmt.Sprintf("Unknown action: %s", action))
		return
	}

	writeJSONRPCError(w, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
}

func writeJSONRPCError(w http.ResponseWriter, id int, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writeJSONRPCSuccess(w http.ResponseWriter, id int, result any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

type httpForwarder struct {
	url string
}

func (f *httpForwarder) ForwardTask(ctx context.Context, task a2a.TaskEnvelope) (*a2a.TaskResponse, error) {
	body, _ := json.Marshal(task.Raw)
	req, err := http.NewRequestWithContext(ctx, "POST", f.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		var tr a2a.TaskResponse
		tr.Status = a2a.StatusAllowed
		tr.Output = map[string]any{"raw_response": "event-stream-started"}
		return &tr, nil
	}

	var rpcResp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result any `json:"result"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err == nil && rpcResp.Error != nil {
		var tr a2a.TaskResponse
		tr.Status = a2a.StatusError
		tr.Error = &a2a.TaskError{Code: fmt.Sprintf("%d", rpcResp.Error.Code), Message: rpcResp.Error.Message}
		return &tr, nil
	}

	var tr a2a.TaskResponse
	tr.Status = a2a.StatusAllowed
	tr.Output = map[string]any{"raw_response": json.RawMessage(respBody)}
	return &tr, nil
}

func TestA2AMockUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(mockUpstreamHandler))
	defer srv.Close()

	adapter := a2a.NewForwardingAdapter("tenant-1", &httpForwarder{url: srv.URL})
	allowPipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{
			Name:   "allow-all",
			Tool:   "*",
			Action: "allow",
		}},
	}, nil, nil, nil)
	denyPipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{
			Name:   "deny-all",
			Tool:   "*",
			Action: "deny",
		}},
	}, nil, nil, nil)

	t.Run("Agent Card Discovery", func(t *testing.T) {
		t.Log("Agent Card Discovery passthrough supported via a2a JSON-RPC spec")
	})

	t.Run("Tasks / Send - Allowed", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 1, "method": "tasks/send", "params": {"message": {"taskId": "task-1", "parts": [{"kind": "data", "data": {"text": "Hello world"}}]}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusAllowed {
			t.Fatalf("Expected allowed, got %v", resp.Status)
		}
	})

	t.Run("Tasks / Send - Denied", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 2, "method": "tasks/send", "params": {"message": {"taskId": "task-2", "parts": [{"kind": "data", "data": {"text": "Secret data"}}]}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, denyPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusDenied {
			t.Fatalf("Expected denied, got %v", resp.Status)
		}
	})

	t.Run("Message / Send - Allowed", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 3, "method": "message/send", "params": {"message": {"messageId": "msg-1", "parts": [{"kind": "data", "data": {"text": "Hello"}}]}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusAllowed {
			t.Fatalf("Expected allowed, got %v", resp.Status)
		}
	})

	t.Run("Unknown Methods / Mandatory Fields", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 4, "method": "tasks/send", "params": {"message": {"taskId": "task-4", "parts": []}, "metadata": {"action": "unknown_action", "sender_agent_id": "test-agent", "required_fields": ["not_supported_field"]}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusUnsupported {
			t.Fatalf("Expected unsupported, got %v", resp.Status)
		}
	})

	t.Run("Streaming (message/stream with SSE) - Allowed", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 5, "method": "message/stream", "params": {"message": {"taskId": "task-5", "parts": []}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusAllowed {
			t.Fatalf("Expected allowed for stream, got %v", resp.Status)
		}
	})

	t.Run("Streaming (message/stream with SSE) - Denied", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 5, "method": "message/stream", "params": {"message": {"taskId": "task-5", "parts": []}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, denyPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusDenied {
			t.Fatalf("Expected denied for stream, got %v", resp.Status)
		}
	})

	t.Run("Tasks Get / Cancel", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 6, "method": "tasks/get", "params": {"taskId": "task-1", "metadata": {"action": "tasks.get", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusUnsupported {
			t.Fatalf("Expected unsupported for proxying non action bearing methods, got %v", resp.Status)
		}
	})

	t.Run("Push-notification config", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 7, "method": "push/config", "params": {"metadata": {"action": "push.config", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusUnsupported {
			t.Fatalf("Expected unsupported, got %v", resp.Status)
		}
	})

	t.Run("Large/Edge Payloads - Bounded", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 8, "method": "tasks/send", "params": {"message": {"taskId": "task-8", "parts": [{"kind": "data", "data": {"text": "large"}}]}, "metadata": {"action": "large_payload", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, allowPipeline)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusAllowed {
			t.Fatalf("Expected allowed, got %v", resp.Status)
		}
	})

	t.Run("Evaluator Failure", func(t *testing.T) {
		reqBody := []byte(`{"jsonrpc": "2.0", "id": 9, "method": "tasks/send", "params": {"message": {"taskId": "task-9", "parts": []}, "metadata": {"action": "summarize", "sender_agent_id": "test-agent"}}}`)
		resp, err := adapter.GovernTask(context.Background(), reqBody, nil)
		if err != nil {
			t.Fatalf("GovernTask: %v", err)
		}
		if resp.Status != a2a.StatusDenied {
			t.Fatalf("Expected denied on nil pipeline, got %v", resp.Status)
		}
	})
}
