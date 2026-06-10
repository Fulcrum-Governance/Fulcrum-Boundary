// Package governance provides Fulcrum Boundary's protocol-agnostic action boundary.
//
// Boundary evaluates tool calls, CLI commands, and code execution requests
// against trust state, static policies, domain interceptors, and the portable
// policy evaluator. Transport adapters convert protocol-specific inputs into
// a canonical GovernanceRequest, invoke the shared Pipeline, and return
// protocol-specific responses.
//
// This package is the dependency root: it has zero imports from any transport
// adapter (the packages under adapters/, e.g. adapters/mcp) or from the CLI
// (internal/boundarycli). Adapters and the CLI depend on governance, never the
// reverse.
package governance
