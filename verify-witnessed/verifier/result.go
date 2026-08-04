package verifier

// Status is a stable per-check verification outcome.
type Status string

const (
	StatusPass       Status = "pass"
	StatusFail       Status = "fail"
	StatusMissing    Status = "missing"
	StatusNotPresent Status = "not_present"
)

// Check is one independently reported verification result.
type Check struct {
	ID     string `json:"id"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Results is the deterministic machine-readable result envelope.
type Results struct {
	SchemaVersion string  `json:"schema_version"`
	Checks        []Check `json:"checks"`
}

// HasFailures reports whether any supplied artifact failed verification.
// Missing and not-present artifacts remain visible but are not failures.
func (r Results) HasFailures() bool {
	for _, check := range r.Checks {
		if check.Status == StatusFail {
			return true
		}
	}
	return false
}
