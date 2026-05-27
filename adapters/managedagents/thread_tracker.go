package managedagents

import (
	"fmt"
	"sync"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// ThreadState tracks the budget and trust state for a session thread.
type ThreadState struct {
	ID          string                `json:"id"`
	ParentID    string                `json:"parent_id,omitempty"`
	BudgetLimit float64               `json:"budget_limit,omitempty"`
	BudgetUsed  float64               `json:"budget_used,omitempty"`
	TrustState  governance.TrustState `json:"trust_state"`
}

// ThreadTracker keeps standalone budget and trust state for Managed Agents
// sessions. Kernel-connected deployments can replace this with synced state.
type ThreadTracker struct {
	mu           sync.Mutex
	sessionID    string
	rootBudget   float64
	defaultTrust governance.TrustState
	threads      map[string]*ThreadState
}

func NewThreadTracker(sessionID string, rootBudget float64) *ThreadTracker {
	tracker := &ThreadTracker{
		sessionID:    sessionID,
		rootBudget:   rootBudget,
		defaultTrust: governance.TrustStateTrusted,
		threads:      map[string]*ThreadState{},
	}
	tracker.ensureThreadLocked(rootThreadID(sessionID), "", rootBudget)
	return tracker
}

func (t *ThreadTracker) TrackEvent(event Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	switch event.Type {
	case EventThreadCreated:
		limit := floatFromMap(event.Data, "budget_limit")
		if limit == 0 {
			limit = floatFromMap(event.Input, "budget_limit")
		}
		t.ensureThreadLocked(threadID(event), event.ParentThreadID, limit)
	default:
		if event.Usage != nil {
			t.recordUsageLocked(threadID(event), event.Usage.CostUSD)
		}
	}
}

func (t *ThreadTracker) Reserve(sessionID, thread string, cost float64) error {
	if cost <= 0 {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	id := thread
	if id == "" {
		id = rootThreadID(sessionID)
	}
	state := t.ensureThreadLocked(id, "", 0)
	if state.TrustState.Blocked() {
		return fmt.Errorf("thread %s is %s", id, state.TrustState)
	}
	if state.BudgetLimit > 0 && state.BudgetUsed+cost > state.BudgetLimit {
		return fmt.Errorf("thread %s budget exceeded: %.4f + %.4f > %.4f", id, state.BudgetUsed, cost, state.BudgetLimit)
	}
	root := t.ensureThreadLocked(rootThreadID(t.sessionID), "", t.rootBudget)
	if root.BudgetLimit > 0 && root.BudgetUsed+cost > root.BudgetLimit {
		return fmt.Errorf("session budget exceeded: %.4f + %.4f > %.4f", root.BudgetUsed, cost, root.BudgetLimit)
	}
	state.BudgetUsed += cost
	if state.ID != root.ID {
		root.BudgetUsed += cost
	}
	return nil
}

func (t *ThreadTracker) SetTrust(thread string, state governance.TrustState) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ensureThreadLocked(thread, "", 0).TrustState = state
}

func (t *ThreadTracker) Snapshot() map[string]ThreadState {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]ThreadState, len(t.threads))
	for id, state := range t.threads {
		out[id] = *state
	}
	return out
}

func (t *ThreadTracker) recordUsageLocked(id string, cost float64) {
	if cost <= 0 {
		return
	}
	state := t.ensureThreadLocked(id, "", 0)
	state.BudgetUsed += cost
}

func (t *ThreadTracker) ensureThreadLocked(id, parent string, limit float64) *ThreadState {
	if id == "" {
		id = rootThreadID(t.sessionID)
	}
	state := t.threads[id]
	if state == nil {
		state = &ThreadState{ID: id, ParentID: parent, BudgetLimit: limit, TrustState: t.defaultTrust}
		t.threads[id] = state
	}
	if parent != "" {
		state.ParentID = parent
	}
	if limit > 0 {
		state.BudgetLimit = limit
	}
	return state
}

func budgetKey(sessionID, thread string) string {
	if thread == "" {
		return rootThreadID(sessionID)
	}
	return sessionID + "/" + thread
}

func rootThreadID(sessionID string) string {
	if sessionID == "" {
		return "session"
	}
	return sessionID
}

func threadID(event Event) string {
	if event.SessionThreadID != "" {
		return event.SessionThreadID
	}
	return rootThreadID(event.SessionID)
}

func floatFromMap(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
