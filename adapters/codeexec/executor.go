package codeexec

import (
	"context"
	"errors"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// ErrExecutorNotConfigured is returned when code execution is requested before
// an operator wires the adapter to a named execution boundary.
var ErrExecutorNotConfigured = errors.New("code execution executor is not configured")

// Executor runs an allowed code-execution request inside a configured runtime
// boundary such as a container, WASM runtime, microVM, or documented local
// process boundary.
type Executor interface {
	Execute(ctx context.Context, req *governance.GovernanceRequest) (*governance.ToolResponse, error)
}

// ExecutionBoundary describes the isolation boundary provided by the executor.
// SecureSandbox must remain false unless the named boundary is implemented,
// tested, and documented by the embedding runtime.
type ExecutionBoundary struct {
	Name          string
	Kind          string
	Description   string
	SecureSandbox bool
}

// DefaultExecutionBoundary describes the unconfigured adapter state.
func DefaultExecutionBoundary() ExecutionBoundary {
	return ExecutionBoundary{
		Name:          "unconfigured",
		Kind:          "none",
		Description:   "No code execution runtime is configured; allowed requests cannot execute until an executor boundary is provided.",
		SecureSandbox: false,
	}
}

// LocalProcessBoundary documents a local-process executor. This is policy-gated
// execution, not secure sandboxing.
func LocalProcessBoundary(name string) ExecutionBoundary {
	if name == "" {
		name = "local-process"
	}
	return ExecutionBoundary{
		Name:          name,
		Kind:          "local_process",
		Description:   "Code executes in an operator-provided local process boundary. This is not a secure sandbox.",
		SecureSandbox: false,
	}
}

// UnconfiguredExecutor refuses to execute code until the host application wires
// a real execution boundary.
type UnconfiguredExecutor struct{}

func (UnconfiguredExecutor) Execute(context.Context, *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	return nil, ErrExecutorNotConfigured
}
