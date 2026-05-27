package sql

import "github.com/fulcrum-governance/fulcrum-boundary/governance"

// NewPostgresInterceptor returns Boundary's default Postgres AST guard.
func NewPostgresInterceptor() governance.Interceptor {
	return PostgresGuard(GuardConfig{})
}
