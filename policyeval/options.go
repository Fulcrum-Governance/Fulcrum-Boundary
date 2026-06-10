package policyeval

import "time"

// Option configures the Evaluator.
type Option func(*Evaluator)

// WithMaxEvaluationTime sets the maximum time allowed for policy evaluation.
// If evaluation exceeds this duration, a warning is logged (if logger is set).
// Default: 10ms
func WithMaxEvaluationTime(d time.Duration) Option {
	return func(e *Evaluator) {
		e.maxEvaluationTime = d
	}
}

// WithLogger sets a logger for the evaluator.
// The logger interface is minimal to avoid dependencies.
func WithLogger(l Logger) Option {
	return func(e *Evaluator) {
		e.logger = l
	}
}

// WithExternalCallsEnabled enables or disables external HTTP calls for conditions.
// Default: false (disabled for security in proxy/SDK contexts).
func WithExternalCallsEnabled(enabled bool) Option {
	return func(e *Evaluator) {
		e.externalCallsEnabled = enabled
	}
}

// WithStopOnDeny configures whether to stop evaluating policies after the first deny.
// Default: true
func WithStopOnDeny(stop bool) Option {
	return func(e *Evaluator) {
		e.stopOnDeny = stop
	}
}

// WithStrictPolicies makes the evaluator validate policies (including compiling
// every regex pattern) whenever the policy set is loaded or replaced, and fail
// closed on any invalid policy instead of silently skipping it at evaluation
// time.
//
// Semantics:
//   - With NewEvaluatorStrict / UpdatePoliciesStrict, an invalid policy set is
//     reported as an error to the caller.
//   - With the plain NewEvaluator / UpdatePolicies (whose signatures return no
//     error for backward compatibility), an invalid policy set is logged at
//     Warn and arms a fail-closed state: Evaluate then denies every request
//     until a valid policy set is loaded. This prevents a typo'd deny rule from
//     degrading into a silent allow.
//
// Default: off (invalid policies are skipped at evaluation time with a Warn
// log; see NewEvaluator).
func WithStrictPolicies() Option {
	return func(e *Evaluator) {
		e.strictPolicies = true
	}
}

// Logger is a minimal logging interface for the evaluator.
// This allows the evaluator to log without depending on a specific logging library.
//
// The evaluator reports operationally significant events — a skipped faulting
// policy or rule (which can turn a typo'd deny into an allow), strict
// validation failures, and evaluation-time-limit overruns — at Warn. Embedders
// that want visibility into these conditions must supply a Logger (via
// WithLogger) whose Warn output reaches a real sink; the default logger
// discards everything. An *slog.Logger can be adapted to this interface in a
// few lines.
type Logger interface {
	Debug(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
}

// Field represents a structured log field.
type Field struct {
	Key   string
	Value interface{}
}

// noopLogger is a logger that does nothing.
type noopLogger struct{}

func (noopLogger) Debug(string, ...Field) {}
func (noopLogger) Warn(string, ...Field)  {}
