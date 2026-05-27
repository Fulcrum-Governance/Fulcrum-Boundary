package sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type GuardConfig struct {
	SQLArgument string
}

func PostgresGuard(cfg GuardConfig) governance.Interceptor {
	argName := cfg.SQLArgument
	if argName == "" {
		argName = "sql"
	}
	return func(_ context.Context, req *governance.GovernanceRequest) (*governance.InterceptorResult, error) {
		sqlText := sqlArgument(req, argName)
		classification := ClassifyPostgres(sqlText)
		if req.Arguments == nil {
			req.Arguments = map[string]any{}
		}
		req.Arguments["sql_class"] = string(classification.Class)
		req.Arguments["sql_statement_types"] = classification.StatementTypes

		switch classification.Class {
		case ClassUnknown:
			reason := "Postgres AST guard denied unknown SQL"
			if classification.ParseError != "" {
				reason = reason + ": " + classification.ParseError
			}
			return &governance.InterceptorResult{Allowed: false, Action: "deny", Reason: reason}, nil
		case ClassDestructive:
			return &governance.InterceptorResult{
				Allowed: false,
				Action:  "deny",
				Reason:  fmt.Sprintf("Postgres AST guard denied destructive SQL (%s)", strings.Join(classification.StatementTypes, ",")),
			}, nil
		case ClassAdmin:
			return &governance.InterceptorResult{
				Allowed: false,
				Action:  "escalate",
				Reason:  fmt.Sprintf("Postgres AST guard escalated administrative SQL (%s)", strings.Join(classification.StatementTypes, ",")),
			}, nil
		default:
			return nil, nil
		}
	}
}

func sqlArgument(req *governance.GovernanceRequest, name string) string {
	if req == nil || req.Arguments == nil {
		return ""
	}
	if value, ok := req.Arguments[name]; ok {
		return fmt.Sprint(value)
	}
	if value, ok := req.Arguments["query"]; ok {
		return fmt.Sprint(value)
	}
	return ""
}
