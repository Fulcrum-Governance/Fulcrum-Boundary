package governance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StandaloneTrustBackend is the in-process trust backend for standalone mode.
// It maintains a Beta(alpha,beta) trust model per agent entirely in memory,
// with the same update semantics as fulcrum-trust: a success increments alpha,
// a failure increments beta, and a partial outcome increments both by a half
// weight; the score is alpha/(alpha+beta). State is derived from the score via
// TrustStateFromScore against the configured thresholds. It implements
// TrustBackend. State is process-local and not persisted across restarts.
type StandaloneTrustBackend struct {
	mu     sync.Mutex
	cfg    StandaloneTrustConfig
	states map[string]*betaTrustState
}

type betaTrustState struct {
	AgentID          string
	Alpha            float64
	Beta             float64
	InteractionCount int
	State            TrustState
	LastUpdated      time.Time
}

// NewStandaloneTrustBackend returns a StandaloneTrustBackend configured with
// cfg; any zero-valued config fields are filled with the documented defaults.
func NewStandaloneTrustBackend(cfg StandaloneTrustConfig) *StandaloneTrustBackend {
	cfg = cfg.withDefaults()
	return &StandaloneTrustBackend{
		cfg:    cfg,
		states: make(map[string]*betaTrustState),
	}
}

// CheckAgentState implements TrustChecker by returning the agent's current
// circuit-breaker state. An unknown agent reports TrustStateTrusted; an empty
// agentID returns an error (fail-closed for the caller).
func (b *StandaloneTrustBackend) CheckAgentState(ctx context.Context, agentID string) (TrustState, error) {
	snapshot, err := b.GetAgentTrust(ctx, agentID)
	if err != nil {
		return TrustStateIsolated, err
	}
	return snapshot.State, nil
}

// GetAgentTrust implements TrustBackend. It returns the agent's current
// snapshot, or a default trusted snapshot (Known == false) for an agent with
// no recorded interactions. An empty agentID returns an error.
func (b *StandaloneTrustBackend) GetAgentTrust(_ context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	state, ok := b.states[agentID]
	if !ok {
		return TrustSnapshot{
			AgentID: agentID,
			State:   TrustStateTrusted,
			Score:   1,
			Known:   false,
		}, nil
	}
	return state.snapshot(true), nil
}

// RecordDecision implements TrustBackend. It maps the decision to a trust
// outcome, folds it into the agent's Beta parameters, recomputes the state from
// the new score, and returns the before/after update. A terminated agent stays
// terminated. A nil request or empty AgentID returns an error.
func (b *StandaloneTrustBackend) RecordDecision(_ context.Context, req *GovernanceRequest, decision *GovernanceDecision) (TrustDecisionUpdate, error) {
	if req == nil || req.AgentID == "" {
		return TrustDecisionUpdate{}, fmt.Errorf("agent_id is required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureLocked(req.AgentID)
	before := state.snapshot(true)
	outcome := TrustOutcomeFromDecision(decision)
	switch outcome {
	case TrustOutcomeSuccess:
		state.Alpha += b.cfg.SuccessWeight
	case TrustOutcomeFailure:
		weight := b.cfg.FailureWeight
		if state.State == TrustStateEvaluating {
			weight *= b.cfg.DegradedFailureMultiplier
		}
		state.Beta += weight
	case TrustOutcomePartial:
		state.Alpha += b.cfg.PartialAlphaWeight
		state.Beta += b.cfg.PartialBetaWeight
	}
	state.InteractionCount++
	state.LastUpdated = time.Now().UTC()
	if state.State != TrustStateTerminated {
		state.State = TrustStateFromScore(state.score(), b.cfg.Theta, b.cfg.DegradedThreshold)
	}
	after := state.snapshot(true)
	return TrustDecisionUpdate{
		Before:     before,
		After:      after,
		Outcome:    outcome,
		Transition: before.State != after.State,
	}, nil
}

// ResetAgentTrust implements TrustBackend by discarding the agent's stored
// Beta state, returning it to the default trusted snapshot. An empty agentID
// returns an error.
func (b *StandaloneTrustBackend) ResetAgentTrust(_ context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.states, agentID)
	return TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Score: 1, Known: false}, nil
}

// TerminateAgent implements TrustBackend by forcing the agent into the
// TERMINATED state, which blocks all further execution until ResetAgentTrust is
// called. An empty agentID returns an error.
func (b *StandaloneTrustBackend) TerminateAgent(_ context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.ensureLocked(agentID)
	state.State = TrustStateTerminated
	state.LastUpdated = time.Now().UTC()
	return state.snapshot(true), nil
}

func (b *StandaloneTrustBackend) ensureLocked(agentID string) *betaTrustState {
	state, ok := b.states[agentID]
	if ok {
		return state
	}
	now := time.Now().UTC()
	state = &betaTrustState{
		AgentID:     agentID,
		Alpha:       b.cfg.InitialAlpha,
		Beta:        b.cfg.InitialBeta,
		State:       TrustStateTrusted,
		LastUpdated: now,
	}
	b.states[agentID] = state
	return state
}

func (s *betaTrustState) score() float64 {
	return TrustScore(s.Alpha, s.Beta)
}

func (s *betaTrustState) snapshot(known bool) TrustSnapshot {
	return TrustSnapshot{
		AgentID:          s.AgentID,
		State:            s.State,
		Score:            s.score(),
		Alpha:            s.Alpha,
		Beta:             s.Beta,
		InteractionCount: s.InteractionCount,
		Known:            known,
		LastUpdated:      s.LastUpdated,
	}
}
