package governance

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// programmableTrustBackend is a full TrustBackend test double. Unlike the
// CheckAgentState-only mockTrustChecker, it implements the whole TrustBackend
// contract so Pipeline.recordTrustDecision's type assertion
// (p.trustChecker.(TrustBackend)) succeeds and the post-decision trust-update
// logic in Evaluate's defer is exercised.
//
// CheckAgentState returns checkState (default TrustStateTrusted) so a request
// reaches the otherwise-allow outcome; RecordDecision then returns recordErr,
// or a TrustDecisionUpdate whose After.State is recordAfterState. This lets a
// single stub drive every post-allow trust branch: a backend fault
// (recordErr), an in-flight isolation (recordAfterState == Isolated), or an
// in-flight degrade (recordAfterState == Evaluating).
type programmableTrustBackend struct {
	checkState       TrustState
	checkErr         error
	recordErr        error
	recordAfterState TrustState
	recordCalls      int
}

func (b *programmableTrustBackend) CheckAgentState(_ context.Context, _ string) (TrustState, error) {
	if b.checkErr != nil {
		return TrustStateIsolated, b.checkErr
	}
	return b.checkState, nil
}

func (b *programmableTrustBackend) GetAgentTrust(_ context.Context, agentID string) (TrustSnapshot, error) {
	return TrustSnapshot{AgentID: agentID, State: b.checkState, Known: true}, nil
}

func (b *programmableTrustBackend) RecordDecision(_ context.Context, req *GovernanceRequest, _ *GovernanceDecision) (TrustDecisionUpdate, error) {
	b.recordCalls++
	if b.recordErr != nil {
		return TrustDecisionUpdate{}, b.recordErr
	}
	agentID := ""
	if req != nil {
		agentID = req.AgentID
	}
	return TrustDecisionUpdate{
		Before:     TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Score: 1.0},
		After:      TrustSnapshot{AgentID: agentID, State: b.recordAfterState, Score: 0.2},
		Outcome:    TrustOutcomeFailure,
		Transition: b.recordAfterState != TrustStateTrusted,
	}, nil
}

func (b *programmableTrustBackend) ResetAgentTrust(_ context.Context, agentID string) (TrustSnapshot, error) {
	return TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Known: true}, nil
}

func (b *programmableTrustBackend) TerminateAgent(_ context.Context, agentID string) (TrustSnapshot, error) {
	return TrustSnapshot{AgentID: agentID, State: TrustStateTerminated, Known: true}, nil
}

// TestPipeline_RequireAgentID_DeniesWhenMissing covers the Stage-1
// RequireAgentID enforcement body (pipeline.go ~241-247). When RequireAgentID
// is set, the transport is fail-closed, and the request carries no AgentID, the
// pipeline must deny *before* any other stage, with the protected-adapter
// reason and a zeroed/isolated trust posture. The table also pins the negative
// cases that must NOT trip the guard, so a future refactor cannot quietly widen
// or narrow the condition.
func TestPipeline_RequireAgentID_DeniesWhenMissing(t *testing.T) {
	tests := []struct {
		name           string
		requireAgentID bool
		transport      TransportType
		agentID        string
		failClosed     []TransportType
		wantAction     string
		wantReason     string // substring; "" means do not assert reason
	}{
		{
			name:           "missing id on fail-closed transport denies",
			requireAgentID: true,
			transport:      TransportMCP, // in DefaultFailClosedTransports
			agentID:        "",
			wantAction:     "deny",
			wantReason:     "agent identity is required for protected adapter",
		},
		{
			name:           "present id satisfies the guard",
			requireAgentID: true,
			transport:      TransportMCP,
			agentID:        "agent-1",
			wantAction:     "allow",
		},
		{
			name:           "missing id but require disabled allows",
			requireAgentID: false,
			transport:      TransportMCP,
			agentID:        "",
			wantAction:     "allow",
		},
		{
			name:           "missing id on fail-open transport allows",
			requireAgentID: true,
			transport:      TransportWebhook, // not fail-closed by default
			agentID:        "",
			wantAction:     "allow",
		},
		{
			name:           "explicit empty fail-closed list disarms the guard",
			requireAgentID: true,
			transport:      TransportMCP,
			agentID:        "",
			failClosed:     []TransportType{}, // operator opt-out: nothing fail-closed
			wantAction:     "allow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := PipelineConfig{
				RequireAgentID:       tc.requireAgentID,
				FailClosedTransports: tc.failClosed,
			}
			p := NewPipeline(cfg, nil, nil, nil)
			req := &GovernanceRequest{
				ToolName:  "read_file",
				Transport: tc.transport,
				AgentID:   tc.agentID,
			}
			d, err := p.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (reason=%q)", d.Action, tc.wantAction, d.Reason)
			}
			if tc.wantReason != "" && !strings.Contains(d.Reason, tc.wantReason) {
				t.Errorf("reason = %q, want substring %q", d.Reason, tc.wantReason)
			}
			// On the protected-deny path the pipeline also drops trust to the
			// floor and marks the agent ISOLATED; assert that posture so the
			// branch body is verified, not just the verdict.
			if tc.wantAction == "deny" && tc.wantReason == "agent identity is required for protected adapter" {
				if d.TrustScore != 0.0 {
					t.Errorf("trust score = %v, want 0.0 on protected deny", d.TrustScore)
				}
				if d.TrustState != TrustStateIsolated.String() {
					t.Errorf("trust state = %q, want %q on protected deny", d.TrustState, TrustStateIsolated.String())
				}
			}
		})
	}
}

// TestPipeline_TrustRecordError_FailClosedDeny covers the post-decision
// fail-closed branch (pipeline.go ~219-224, reached via recordTrustDecision's
// error return at ~422). An otherwise-allowed decision whose trust backend
// RecordDecision FAILS must be flipped to deny on a fail-closed transport, with
// the "trust update failed" reason and an isolated/zeroed posture — a backend
// fault is treated as fail-closed, not silently allowed. The fail-open
// transport case proves the flip is gated on FailClosedTransports.
func TestPipeline_TrustRecordError_FailClosedDeny(t *testing.T) {
	tests := []struct {
		name        string
		transport   TransportType
		failClosed  []TransportType
		wantAction  string
		wantReason  string
		wantScore   float64
		wantState   string
		assertState bool
	}{
		{
			name:        "record error on fail-closed transport denies",
			transport:   TransportMCP,
			wantAction:  "deny",
			wantReason:  "trust update failed",
			wantScore:   0.0,
			wantState:   TrustStateIsolated.String(),
			assertState: true,
		},
		{
			name:       "record error on fail-open transport still allows",
			transport:  TransportWebhook, // not in DefaultFailClosedTransports
			wantAction: "allow",
		},
		{
			name:       "record error, explicit empty list -> fail-open allows",
			transport:  TransportMCP,
			failClosed: []TransportType{}, // operator opt-out
			wantAction: "allow",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend := &programmableTrustBackend{
				checkState: TrustStateTrusted, // not blocked: reach the allow path
				recordErr:  fmt.Errorf("trust store unreachable"),
			}
			cfg := PipelineConfig{FailClosedTransports: tc.failClosed}
			p := NewPipeline(cfg, backend, nil, nil)
			req := &GovernanceRequest{
				ToolName:  "read_file",
				Transport: tc.transport,
				AgentID:   "agent-1",
				TenantID:  "tenant-1",
			}
			d, err := p.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if backend.recordCalls == 0 {
				t.Fatal("RecordDecision was never called; the error branch is unreached")
			}
			if d.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (reason=%q)", d.Action, tc.wantAction, d.Reason)
			}
			if tc.wantReason != "" {
				if !strings.Contains(d.Reason, tc.wantReason) {
					t.Errorf("reason = %q, want substring %q", d.Reason, tc.wantReason)
				}
				// The underlying cause must be surfaced, not swallowed.
				if !strings.Contains(d.Reason, "trust store unreachable") {
					t.Errorf("reason = %q, want it to wrap the backend error", d.Reason)
				}
			}
			if tc.assertState {
				if d.TrustScore != tc.wantScore {
					t.Errorf("trust score = %v, want %v", d.TrustScore, tc.wantScore)
				}
				if d.TrustState != tc.wantState {
					t.Errorf("trust state = %q, want %q", d.TrustState, tc.wantState)
				}
			}
		})
	}
}

// TestPipeline_TrustUpdate_FlipsAllowToTerminalState covers the two
// in-flight-transition branches (pipeline.go ~212-218). When an otherwise
// allow decision's own trust update moves the agent to ISOLATED, the verdict
// becomes deny; when it degrades the agent to EVALUATING, the allow becomes
// require_approval. Both reasons are pinned. A control row (After == TRUSTED)
// confirms an allow that does not transition is left untouched.
func TestPipeline_TrustUpdate_FlipsAllowToTerminalState(t *testing.T) {
	tests := []struct {
		name        string
		afterState  TrustState
		wantAction  string
		wantReason  string // substring
		wantTrustSt string
	}{
		{
			name:        "in-flight isolation flips allow to deny",
			afterState:  TrustStateIsolated,
			wantAction:  "deny",
			wantReason:  "is ISOLATED",
			wantTrustSt: TrustStateIsolated.String(),
		},
		{
			name:        "in-flight degrade flips allow to require_approval",
			afterState:  TrustStateEvaluating,
			wantAction:  "require_approval",
			wantReason:  "is degraded",
			wantTrustSt: TrustStateEvaluating.String(),
		},
		{
			name:        "no transition leaves allow intact",
			afterState:  TrustStateTrusted,
			wantAction:  "allow",
			wantReason:  "",
			wantTrustSt: TrustStateTrusted.String(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend := &programmableTrustBackend{
				checkState:       TrustStateTrusted, // Stage 1 sees a healthy agent
				recordAfterState: tc.afterState,     // the update transitions it
			}
			// Use a fail-OPEN transport so this exercises the transition
			// branches specifically, not the record-error fail-closed branch.
			p := NewPipeline(PipelineConfig{}, backend, nil, nil)
			req := &GovernanceRequest{
				ToolName:  "read_file",
				Transport: TransportWebhook,
				AgentID:   "agent-7",
				TenantID:  "tenant-1",
			}
			d, err := p.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if backend.recordCalls == 0 {
				t.Fatal("RecordDecision was never called; the transition branch is unreached")
			}
			if d.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (reason=%q)", d.Action, tc.wantAction, d.Reason)
			}
			if tc.wantReason != "" && !strings.Contains(d.Reason, tc.wantReason) {
				t.Errorf("reason = %q, want substring %q", d.Reason, tc.wantReason)
			}
			if d.TrustState != tc.wantTrustSt {
				t.Errorf("trust state = %q, want %q", d.TrustState, tc.wantTrustSt)
			}
		})
	}
}

// TestPipeline_TrustUpdate_FlipReason_IdentifiesAgent locks the agent
// identifier into the flip reasons. The ISOLATED and EVALUATING branches both
// format the reason with req.AgentID; an operator reading the audit row must be
// able to see *which* agent was isolated/degraded mid-decision.
func TestPipeline_TrustUpdate_FlipReason_IdentifiesAgent(t *testing.T) {
	const agentID = "agent-needle-42"
	backend := &programmableTrustBackend{
		checkState:       TrustStateTrusted,
		recordAfterState: TrustStateIsolated,
	}
	p := NewPipeline(PipelineConfig{}, backend, nil, nil)
	d, err := p.Evaluate(context.Background(), &GovernanceRequest{
		ToolName:  "read_file",
		Transport: TransportWebhook,
		AgentID:   agentID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Action != "deny" {
		t.Fatalf("action = %q, want deny", d.Action)
	}
	if !strings.Contains(d.Reason, agentID) {
		t.Errorf("reason = %q, want it to name the agent %q", d.Reason, agentID)
	}
}
