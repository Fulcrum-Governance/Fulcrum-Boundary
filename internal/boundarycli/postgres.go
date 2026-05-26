package boundarycli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fulcrum-governance/boundary/governance"
)

type toolCallPayload struct {
	ToolName  string         `json:"tool_name"`
	AgentID   string         `json:"agent_id,omitempty"`
	TenantID  string         `json:"tenant_id,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Arguments map[string]any `json:"arguments,omitempty"`
	Method    string         `json:"method,omitempty"`
	Params    struct {
		Name      string         `json:"name,omitempty"`
		Arguments map[string]any `json:"arguments,omitempty"`
	} `json:"params,omitempty"`
	SQL string `json:"sql,omitempty"`
}

func buildPostgresGovernanceRequest(r *http.Request) (*governance.GovernanceRequest, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	payload, err := parseToolCallPayload(body)
	if err != nil {
		return nil, err
	}

	toolName := payload.ToolName
	if toolName == "" {
		toolName = payload.Params.Name
	}
	if toolName == "" {
		toolName = r.Header.Get(governance.HeaderToolName)
	}
	if toolName == "" {
		toolName = strings.Trim(strings.TrimPrefix(r.URL.Path, "/"), "/")
	}
	if toolName == "" || toolName == "mcp" {
		toolName = "query"
	}

	args := payload.Arguments
	if len(args) == 0 {
		args = payload.Params.Arguments
	}
	if args == nil {
		args = map[string]any{}
	}
	if payload.SQL != "" && args["sql"] == nil {
		args["sql"] = payload.SQL
	}

	action := payload.Method
	if action == "" {
		action = "tools/call"
	}

	return &governance.GovernanceRequest{
		Transport:  governance.TransportMCP,
		ToolName:   toolName,
		Action:     action,
		Arguments:  args,
		RawPayload: body,
		AgentID:    payload.AgentID,
		TenantID:   payload.TenantID,
		TraceID:    payload.TraceID,
	}, nil
}

func parseToolCallPayload(body []byte) (*toolCallPayload, error) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, fmt.Errorf("empty request body")
	}
	var payload toolCallPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func postgresHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload, err := parseToolCallPayload(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sqlText := extractSQL(payload)
		if strings.TrimSpace(sqlText) == "" {
			http.Error(w, "missing arguments.sql", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if queryReturnsRows(sqlText) {
			writeRows(ctx, w, db, sqlText)
			return
		}
		result, err := db.ExecContext(ctx, sqlText)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		affected, _ := result.RowsAffected()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":            true,
			"rows_affected": affected,
		})
	})
}

func extractSQL(payload *toolCallPayload) string {
	if payload == nil {
		return ""
	}
	if payload.SQL != "" {
		return payload.SQL
	}
	args := payload.Arguments
	if len(args) == 0 {
		args = payload.Params.Arguments
	}
	if args == nil {
		return ""
	}
	if value, ok := args["sql"]; ok {
		return fmt.Sprint(value)
	}
	if value, ok := args["query"]; ok {
		return fmt.Sprint(value)
	}
	return ""
}

func queryReturnsRows(sqlText string) bool {
	normalized := strings.ToLower(strings.TrimSpace(sqlText))
	return strings.HasPrefix(normalized, "select") ||
		strings.HasPrefix(normalized, "with") ||
		strings.HasPrefix(normalized, "show")
}

func writeRows(ctx context.Context, w http.ResponseWriter, db *sql.DB, sqlText string) {
	rows, err := db.QueryContext(ctx, sqlText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	outRows := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			switch value := values[i].(type) {
			case []byte:
				row[col] = string(value)
			default:
				row[col] = value
			}
		}
		outRows = append(outRows, row)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   true,
		"rows": outRows,
	})
}
