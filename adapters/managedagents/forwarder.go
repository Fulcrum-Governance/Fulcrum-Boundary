package managedagents

import (
	"context"
	"sync"
)

// ConfirmationForwarder sends policy-resolved tool confirmations upstream.
type ConfirmationForwarder interface {
	SendConfirmation(ctx context.Context, sessionID string, confirmation ToolConfirmation) error
}

// MemoryForwarder is a deterministic test forwarder and example implementation.
type MemoryForwarder struct {
	mu            sync.Mutex
	Confirmations []ToolConfirmation
	SessionIDs    []string
}

func (f *MemoryForwarder) SendConfirmation(_ context.Context, sessionID string, confirmation ToolConfirmation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.SessionIDs = append(f.SessionIDs, sessionID)
	f.Confirmations = append(f.Confirmations, confirmation)
	return nil
}

func (f *MemoryForwarder) Snapshot() []ToolConfirmation {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]ToolConfirmation, len(f.Confirmations))
	copy(out, f.Confirmations)
	return out
}
