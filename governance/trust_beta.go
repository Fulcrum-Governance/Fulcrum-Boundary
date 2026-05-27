package governance

import (
	"context"
	"fmt"
	"sync"
	"time"
)

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

func NewStandaloneTrustBackend(cfg StandaloneTrustConfig) *StandaloneTrustBackend {
	cfg = cfg.withDefaults()
	return &StandaloneTrustBackend{
		cfg:    cfg,
		states: make(map[string]*betaTrustState),
	}
}

func (b *StandaloneTrustBackend) CheckAgentState(ctx context.Context, agentID string) (TrustState, error) {
	snapshot, err := b.GetAgentTrust(ctx, agentID)
	if err != nil {
		return TrustStateIsolated, err
	}
	return snapshot.State, nil
}

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

func (b *StandaloneTrustBackend) ResetAgentTrust(_ context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.states, agentID)
	return TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Score: 1, Known: false}, nil
}

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
