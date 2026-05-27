package securegithub

import (
	"encoding/json"
	"net/http"
)

func NewHTTPHandler(adapter *Adapter) http.Handler {
	if adapter == nil {
		adapter = NewFixtureAdapter(Config{})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var call ToolCall
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(MCPResponse{
				JSONRPC: "2.0",
				Error: &MCPError{
					Code:    -32700,
					Message: "invalid JSON-RPC request",
					Data:    map[string]any{"reason": err.Error(), "upstream_called": false},
				},
			})
			return
		}
		result, err := adapter.GovernToolCall(r.Context(), call)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(MCPResponse{
				JSONRPC: jsonRPCVersion(call),
				ID:      call.ID,
				Error: &MCPError{
					Code:    -32000,
					Message: "Secure GitHub handler error",
					Data:    map[string]any{"reason": err.Error(), "upstream_called": false},
				},
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if result.Response.Error != nil {
			w.WriteHeader(http.StatusForbidden)
		}
		_ = json.NewEncoder(w).Encode(result.Response)
	})
}
