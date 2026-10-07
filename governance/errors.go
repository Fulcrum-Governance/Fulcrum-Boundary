package governance

import (
	"context"
	"errors"
	"fmt"
)

// ErrEmptyFailClosedList is returned by PipelineConfig.Validate when
// FailClosedTransports is a non-nil empty slice. ADR-047: an empty fail-closed
// transport list is not an acceptable production enforcement policy — it
// would silently opt every execution-capable transport out of required-check
// enforcement. Omit the field to use the enforcing defaults, or list the
// transports that must enforce; a deployment that needs broader emergency
// bypass requires an explicit, time-bounded, identity-attributed, audited
// break-glass mechanism, which this package does not provide.
var ErrEmptyFailClosedList = errors.New("governance: FailClosedTransports must not be an explicit empty list")

// ErrMissingAuditPublisher is the configuration error recorded when a
// pipeline built with PipelineConfig.RequireAudit gets a nil AuditPublisher:
// on a surface that declares its decision records must be delivered, a nil
// auditor cannot satisfy the evidence contract.
var ErrMissingAuditPublisher = errors.New("governance: RequireAudit is set but no AuditPublisher was provided")

// CheckError marks a check failure with an explicit ADR-047 FailureCategory.
// A TrustChecker, TrustBackend, or PolicyEvaluator may return it (wrapping
// the underlying cause) to report a precise category — for example
// FailureStaleSnapshot for a snapshot used past its validity window — instead
// of letting the pipeline infer the category from the raw error.
type CheckError struct {
	// Category is the ADR-047 failure category reported to the pipeline.
	Category FailureCategory
	// Err is the underlying cause.
	Err error
}

// Error formats the wrapped cause.
func (e *CheckError) Error() string {
	if e == nil || e.Err == nil {
		return "check error"
	}
	return e.Err.Error()
}

// Unwrap returns the underlying cause so errors.Is/As traversal works.
func (e *CheckError) Unwrap() error { return e.Err }

// NewCheckError wraps err with an explicit ADR-047 failure category.
func NewCheckError(category FailureCategory, err error) *CheckError {
	return &CheckError{Category: category, Err: err}
}

// classifyFailure maps a check failure to its ADR-047 FailureCategory. An
// explicit CheckError category wins; deadline-exceeded maps to timeout,
// cancellation to canceled, and anything else to unavailable (the ADR's
// catch-all for a required dependency that cannot produce a valid result).
func classifyFailure(err error) FailureCategory {
	var checkErr *CheckError
	if errors.As(err, &checkErr) && checkErr.Category != "" {
		return checkErr.Category
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return FailureTimeout
	case errors.Is(err, context.Canceled):
		return FailureCanceled
	default:
		return FailureUnavailable
	}
}

// ParseError indicates that an adapter could not construct a valid
// GovernanceRequest from its transport-specific input. The pipeline was
// never invoked. Callers should treat ParseError as deny-equivalent:
// no audit event is emitted by the governance pipeline for these
// failures, and the underlying tool call must not proceed.
//
// Use errors.As to detect ParseError returned by adapter ParseRequest
// methods:
//
//	_, err := adapter.ParseRequest(ctx, raw)
//	var pe *governance.ParseError
//	if errors.As(err, &pe) {
//	    // adapter-level parse failure; pe.Transport identifies which transport
//	}
type ParseError struct {
	// Transport identifies which adapter failed to parse its input.
	Transport TransportType
	// Reason is a short, human-readable cause (e.g., "empty command",
	// "unsupported raw type int"). Stable enough to match in tests.
	Reason string
	// Err is the underlying cause (e.g., a json.Unmarshal error). May be nil
	// when the failure is purely a validation mismatch with no wrapped error.
	Err error
}

// Error formats the parse error. Stable format:
//
//	"<transport>: <reason>"           when Err == nil
//	"<transport>: <reason>: <cause>"  when Err != nil
func (e *ParseError) Error() string {
	if e == nil {
		return "<nil *ParseError>"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Transport, e.Reason, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Transport, e.Reason)
}

// Unwrap returns the underlying cause so errors.Is/As traversal works.
func (e *ParseError) Unwrap() error { return e.Err }

// NewParseError constructs a ParseError. Reason should be a short,
// human-readable cause; err may be nil for pure validation failures.
func NewParseError(transport TransportType, reason string, err error) *ParseError {
	return &ParseError{Transport: transport, Reason: reason, Err: err}
}

// IsParseError reports whether err (or any wrapped error) is a *ParseError.
// It is shorthand for:
//
//	var pe *governance.ParseError
//	errors.As(err, &pe)
func IsParseError(err error) bool {
	var pe *ParseError
	return errors.As(err, &pe)
}
