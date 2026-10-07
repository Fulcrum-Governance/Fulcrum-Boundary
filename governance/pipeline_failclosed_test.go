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

// TestPipeline_RequireAgentID_CheckIndeterminateWhenMissing covers the
// Stage-1 RequireAgentID enforcement body. When RequireAgentID is set and the
// request carries no AgentID, the missing required identity is a check
// failure (ADR-047 missing_identity): on an enforcing transport the pipeline
// returns CHECK_INDETERMINATE before any other stage; on an explicitly
// non-enforcing transport it records the would-have-blocked check and
// continues. An explicit empty FailClosedTransports list is a configuration
// error that blocks everything rather than disarming the guard. The table
// also pins the negative cases that must NOT trip the guard, so a future
// refactor cannot quietly widen or narrow the condition.
func TestPipeline_RequireAgentID_CheckIndeterminateWhenMissing(t *testing.T) {
	tests := []struct {
		name           string
		requireAgentID bool
		transport      TransportType
		agentID        string
		failClosed     []TransportType
		wantAction     string
		wantReason     string // substring; "" means do not assert reason
		wantStage      string
		wantCategory   FailureCategory
		wantCheck      bool
	}{
		{
			name:           "missing id on enforcing transport blocks check_indeterminate",
			requireAgentID: true,
			transport:      TransportMCP, // in DefaultFailClosedTransports
			agentID:        "",
			wantAction:     ActionCheckIndeterminate,
			wantReason:     "agent identity is required for protected adapter",
			wantStage:      CheckStageIdentity,
			wantCategory:   FailureMissingIdentity,
			wantCheck:      true,
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
			name:           "missing id on non-enforcing transport allows but records the check",
			requireAgentID: true,
			transport:      TransportWebhook,
			failClosed:     []TransportType{TransportMCP}, // webhook declared non-enforcing
			agentID:        "",
			wantAction:     "allow",
			wantStage:      CheckStageIdentity,
			wantCategory:   FailureMissingIdentity,
			wantCheck:      true,
		},
		{
			name:           "explicit empty fail-closed list is a config error and blocks",
			requireAgentID: true,
			transport:      TransportMCP,
			agentID:        "",
			failClosed:     []TransportType{}, // invalid: rejected by Validate
			wantAction:     ActionCheckIndeterminate,
			wantReason:     "FailClosedTransports must not be an explicit empty list",
			wantStage:      CheckStageConfig,
			wantCategory:   FailureMissingConfig,
			wantCheck:      true,
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
			if tc.wantCheck {
				if d.Check == nil || d.Check.Stage != tc.wantStage || d.Check.Category != tc.wantCategory {
					t.Errorf("check = %+v, want %s/%s", d.Check, tc.wantStage, tc.wantCategory)
				}
			}
			// When the identity/config check blocks, no trust posture was
			// obtained — the record must say unknown rather than claim one.
			if tc.wantStage == CheckStageIdentity && tc.wantAction == ActionCheckIndeterminate {
				if d.TrustScore != 0.0 {
					t.Errorf("trust score = %v, want 0.0 on indeterminate block", d.TrustScore)
				}
				if d.TrustState != TrustStateUnknown.String() {
					t.Errorf("trust state = %q, want %q on indeterminate block", d.TrustState, TrustStateUnknown.String())
				}
			}
		})
	}
}

// TestPipeline_TrustRecordError_EnforcingCheckIndeterminate covers the
// post-decision check-failure branch reached via recordTrustDecision's error
// return. An otherwise-allowed decision whose trust backend RecordDecision
// FAILS must be flipped to CHECK_INDETERMINATE on an enforcing transport
// (ADR-047: a trust update that cannot produce a valid result blocks; it is
// neither allow nor a policy deny). The non-enforcing case proves the flip is
// gated on FailClosedTransports and the recorded check travels with the
// allow; the empty-list case proves an invalid configuration fails closed at
// the config stage before RecordDecision is ever reached.
func TestPipeline_TrustRecordError_EnforcingCheckIndeterminate(t *testing.T) {
	tests := []struct {
		name          string
		transport     TransportType
		failClosed    []TransportType
		wantAction    string
		wantReason    string
		wantStage     string
		wantCategory  FailureCategory
		wantScore     float64
		wantState     string
		assertState   bool
		expectReached bool // RecordDecision invoked (false on config error)
	}{
		{
			name:          "record error on enforcing transport blocks check_indeterminate",
			transport:     TransportMCP,
			wantAction:    ActionCheckIndeterminate,
			wantReason:    "trust update failed",
			wantStage:     CheckStageTrustUpdate,
			wantCategory:  FailureUnavailable,
			wantScore:     1.0, // lookup succeeded and posture stands; the update failed
			wantState:     TrustStateTrusted.String(),
			assertState:   true,
			expectReached: true,
		},
		{
			name:          "record error on non-enforcing transport still allows but records the check",
			transport:     TransportWebhook,
			failClosed:    []TransportType{TransportMCP}, // webhook declared non-enforcing
			wantAction:    "allow",
			wantStage:     CheckStageTrustUpdate,
			wantCategory:  FailureUnavailable,
			expectReached: true,
		},
		{
			name:          "record error, explicit empty list -> config error blocks before update",
			transport:     TransportMCP,
			failClosed:    []TransportType{}, // invalid config
			wantAction:    ActionCheckIndeterminate,
			wantReason:    "FailClosedTransports must not be an explicit empty list",
			wantStage:     CheckStageConfig,
			wantCategory:  FailureMissingConfig,
			expectReached: false,
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
			if tc.expectReached && backend.recordCalls == 0 {
				t.Fatal("RecordDecision was never called; the error branch is unreached")
			}
			if !tc.expectReached && backend.recordCalls != 0 {
				t.Fatal("RecordDecision ran despite the configuration error")
			}
			if d.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (reason=%q)", d.Action, tc.wantAction, d.Reason)
			}
			if d.Check == nil || d.Check.Stage != tc.wantStage || d.Check.Category != tc.wantCategory {
				t.Fatalf("check = %+v, want %s/%s", d.Check, tc.wantStage, tc.wantCategory)
			}
			if tc.wantReason != "" {
				if !strings.Contains(d.Reason, tc.wantReason) {
					t.Errorf("reason = %q, want substring %q", d.Reason, tc.wantReason)
				}
			}
			if tc.wantStage == CheckStageTrustUpdate && !strings.Contains(d.Reason, "trust store unreachable") && d.Action == ActionCheckIndeterminate {
				t.Errorf("reason = %q, want it to wrap the backend error", d.Reason)
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
			// The in-flight transition branches apply to every transport;
			// recordErr is nil so the check-failure path is unreached.
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
