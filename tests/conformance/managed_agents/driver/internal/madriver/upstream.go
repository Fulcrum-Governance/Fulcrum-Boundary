package madriver

import (
	"context"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// Upstream is the Managed Agents surface the driver calls in its
// Boundary-proxy role. There are exactly two implementations: stubUpstream
// (offline scripted events) and liveUpstream (the real beta API over HTTP/SSE).
// The interface deliberately mirrors the adapter's own seams: an event stream
// in, confirmations and user events out.
type Upstream interface {
	// CreateSession opens the upstream session and returns its identifiers.
	CreateSession(ctx context.Context, params CreateSessionParams) (*SessionInfo, error)
	// OpenEventStream returns the inbound session event stream the
	// SessionProxy governs.
	OpenEventStream(ctx context.Context, sessionID string) (managedagents.EventSource, error)
	// SendEvents posts user.* events to the session (live) or records them
	// (stub).
	SendEvents(ctx context.Context, sessionID string, events []map[string]any) error
	// Forwarder returns the ConfirmationForwarder the ToolResolver uses to
	// emit user.tool_confirmation events for governed tool calls.
	Forwarder() managedagents.ConfirmationForwarder
	// RequestIDs returns upstream request-id header values captured so far
	// (live mode; always nil in stub mode).
	RequestIDs() []string
	// Close releases any open stream or transport resources.
	Close() error
}

// CreateSessionParams carries what session creation needs. MaxOutputTokens is
// enforced driver-side against observed usage, not sent upstream — no
// per-request max_output_tokens field is documented on the sessions API, so
// sending one would be an unverified guess.
type CreateSessionParams struct {
	AgentID       string
	EnvironmentID string
	Prompt        string
	BudgetUSD     float64
}

// SessionInfo identifies the created upstream session.
type SessionInfo struct {
	ID      string
	AgentID string
}
