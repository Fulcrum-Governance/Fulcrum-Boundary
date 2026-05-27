package standalone

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type Bundle struct {
	Policies    governance.PolicyProvider
	Trust       governance.TrustBackend
	Cost        governance.CostPredictor
	Budget      governance.BudgetEnforcer
	Escalation  governance.EscalationHandler
	Envelope    governance.EnvelopeManager
	Proofs      governance.ProofCorrespondence
	PolicyRules []governance.StaticPolicyRule
}

func NewBundle(ctx context.Context, policyDir string) (*Bundle, error) {
	policies := FilePolicyProvider{Dir: policyDir}
	rules, err := policies.LoadPolicies(ctx)
	if err != nil {
		return nil, err
	}
	return &Bundle{
		Policies:    policies,
		Trust:       governance.NewStandaloneTrustBackend(governance.StandaloneTrustConfig{}),
		Cost:        StaticCostPredictor{},
		Budget:      NewInMemoryBudgetEnforcer(0),
		Escalation:  RequireApprovalEscalationHandler{},
		Envelope:    LocalEnvelopeManager{},
		Proofs:      StaticProofCorrespondence(),
		PolicyRules: rules,
	}, nil
}

type FilePolicyProvider struct {
	Dir string
}

func (p FilePolicyProvider) LoadPolicies(context.Context) ([]governance.StaticPolicyRule, error) {
	if p.Dir == "" {
		return nil, fmt.Errorf("policy directory is required")
	}
	return governance.LoadStaticPoliciesFromDir(p.Dir)
}

func (p FilePolicyProvider) WatchPolicyUpdates(ctx context.Context) (<-chan governance.PolicyUpdate, error) {
	ch := make(chan governance.PolicyUpdate)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

type StaticCostPredictor struct {
	Estimate governance.CostEstimate
}

func (p StaticCostPredictor) PredictCost(context.Context, governance.GovernanceRequest) (governance.CostEstimate, error) {
	if p.Estimate.Unit == "" {
		return governance.CostEstimate{Unit: "unit", Confidence: 1, Source: "standalone"}, nil
	}
	return p.Estimate, nil
}

type InMemoryBudgetEnforcer struct {
	mu        sync.Mutex
	limit     int64
	spendByID map[string]int64
}

func NewInMemoryBudgetEnforcer(limit int64) *InMemoryBudgetEnforcer {
	return &InMemoryBudgetEnforcer{limit: limit, spendByID: map[string]int64{}}
}

func (b *InMemoryBudgetEnforcer) CheckBudget(_ context.Context, tenantID, agentID string, cost governance.CostEstimate) (bool, error) {
	if b.limit <= 0 {
		return true, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.spendByID[budgetKey(tenantID, agentID)]+cost.Amount <= b.limit, nil
}

func (b *InMemoryBudgetEnforcer) RecordSpend(_ context.Context, tenantID, agentID string, amount int64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.spendByID[budgetKey(tenantID, agentID)] += amount
	return nil
}

func budgetKey(tenantID, agentID string) string {
	return tenantID + "/" + agentID
}

type RequireApprovalEscalationHandler struct{}

func (RequireApprovalEscalationHandler) Escalate(_ context.Context, req governance.GovernanceRequest, reason string) (*governance.GovernanceDecision, error) {
	return &governance.GovernanceDecision{
		RequestID:      req.RequestID,
		Action:         "require_approval",
		Reason:         reason,
		TrustScore:     1,
		TrustState:     governance.TrustStateTrusted.String(),
		EnvelopeID:     req.EnvelopeID,
		DecisionMode:   governance.DecisionModeClassified,
		CostEstimate:   0,
		GatewayVersion: "",
	}, nil
}

type LocalEnvelopeManager struct{}

func (LocalEnvelopeManager) CreateEnvelope(_ context.Context, req governance.GovernanceRequest) (governance.EnvelopeID, error) {
	if req.EnvelopeID != "" {
		return governance.EnvelopeID(req.EnvelopeID), nil
	}
	return governance.EnvelopeID(uuid.New().String()), nil
}

func (LocalEnvelopeManager) TransitionEnvelope(context.Context, governance.EnvelopeID, governance.EnvelopeState) error {
	return nil
}

type ProofMap map[string]governance.ProofRef

func StaticProofCorrespondence() ProofMap {
	return ProofMap{
		"budget_safety": {
			Invariant:      "budget_safety",
			Theorem:        "Fulcrum.budget_safety_guarantee",
			Repo:           "Fulcrum-Proofs",
			Path:           "proofs/lean/Proofs/BasicInvariants.lean",
			Correspondence: "design",
		},
		"trust_termination": {
			Invariant:      "trust_termination",
			Theorem:        "Fulcrum.trust_guaranteed_termination",
			Repo:           "Fulcrum-Proofs",
			Path:           "proofs/lean/Proofs/TrustTermination.lean",
			Correspondence: "design",
		},
		"privilege_subset": {
			Invariant:      "privilege_subset",
			Theorem:        "Fulcrum.thm_privilege_static",
			Repo:           "Fulcrum-Proofs",
			Path:           "proofs/lean/Proofs/BasicInvariants.lean",
			Correspondence: "design",
		},
	}
}

func (m ProofMap) GetCorrespondence(invariant string) (governance.ProofRef, error) {
	ref, ok := m[invariant]
	if !ok {
		return governance.ProofRef{}, fmt.Errorf("unknown proof correspondence %q", invariant)
	}
	return ref, nil
}
