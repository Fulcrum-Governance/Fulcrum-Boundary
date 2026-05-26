// Package governance provides Fulcrum Boundary's protocol-agnostic action boundary.
//
// Boundary evaluates tool calls, CLI commands, and code execution requests
// against trust state, static policies, domain interceptors, and the portable
// policy evaluator. Transport adapters convert protocol-specific inputs into
// a canonical GovernanceRequest, invoke the shared Pipeline, and return
// protocol-specific responses.
//
// This package has zero imports from any adapter (internal/adapters/*) or
// transport-specific code (internal/mcpproxy, internal/securemcp).
package governance
