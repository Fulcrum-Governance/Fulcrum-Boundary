package selftest

import (
	"context"
)

const (
	SchemaVersion = "boundary.selftest.v1"

	StatusPass = "pass"
	StatusFail = "fail"
)

type Options struct {
	NoColor bool

	// SecureGitHubLiveModeCheck lets the CLI verify the preview profile's
	// live-mode fail-closed behavior without creating an import cycle.
	SecureGitHubLiveModeCheck func(context.Context) error `json:"-"`
}

type Result struct {
	SchemaVersion       string        `json:"schema_version"`
	Status              string        `json:"status"`
	Passed              bool          `json:"passed"`
	StartedAt           string        `json:"started_at"`
	CompletedAt         string        `json:"completed_at"`
	MutatesLiveSystems  bool          `json:"mutates_live_systems"`
	RequiresCredentials bool          `json:"requires_credentials"`
	RequiresNetwork     bool          `json:"requires_network"`
	Checks              []CheckResult `json:"checks"`
	Next                []string      `json:"next"`
}

type CheckResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	Command    string `json:"command,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}
