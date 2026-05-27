package interceptors_test

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/boundary/governance"
	sqlguard "github.com/fulcrum-governance/boundary/interceptors/sql"
)

func TestPostgresClassifierUsesASTClasses(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want sqlguard.Class
	}{
		{"select with drop in comment stays read", "SELECT /* DROP TABLE users */ id FROM users", sqlguard.ClassRead},
		{"drop table destructive", "DROP TABLE users", sqlguard.ClassDestructive},
		{"mixed statements take highest severity", "SELECT 1; DROP TABLE users", sqlguard.ClassDestructive},
		{"invalid SQL unknown", "DR/**/OP TABLE users", sqlguard.ClassUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqlguard.ClassifyPostgres(tt.sql)
			if got.Class != tt.want {
				t.Fatalf("classify %q = %s (%v), want %s", tt.sql, got.Class, got.ParseError, tt.want)
			}
		})
	}
}

func TestPostgresGuardDeniesUnknownAndDestructiveFailClosed(t *testing.T) {
	guard := sqlguard.NewPostgresInterceptor()
	tests := []struct {
		name       string
		sql        string
		wantAction string
	}{
		{"unknown", "SELECT @@@", "deny"},
		{"destructive", "DROP TABLE users", "deny"},
		{"admin", "ALTER TABLE users ADD COLUMN note text", "escalate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &governance.GovernanceRequest{
				ToolName:  "query",
				Arguments: map[string]any{"sql": tt.sql},
			}
			result, err := guard(context.Background(), req)
			if err != nil {
				t.Fatalf("guard returned error: %v", err)
			}
			if result == nil || result.Action != tt.wantAction {
				t.Fatalf("guard action = %#v, want %s", result, tt.wantAction)
			}
			if req.Arguments["sql_class"] == "" {
				t.Fatalf("guard did not annotate sql_class: %#v", req.Arguments)
			}
		})
	}
}

func TestPostgresGuardAllowsReadAndAnnotatesRequest(t *testing.T) {
	guard := sqlguard.NewPostgresInterceptor()
	req := &governance.GovernanceRequest{
		ToolName:  "query",
		Arguments: map[string]any{"sql": "SELECT id FROM users"},
	}
	result, err := guard(context.Background(), req)
	if err != nil {
		t.Fatalf("guard returned error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected read query to continue pipeline, got %#v", result)
	}
	if req.Arguments["sql_class"] != "READ" {
		t.Fatalf("expected READ annotation, got %#v", req.Arguments)
	}
}
