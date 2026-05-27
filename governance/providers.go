package governance

import (
	"context"
	"time"
)

// PolicyUpdate is emitted by providers that can stream policy-bundle changes.
type PolicyUpdate struct {
	Version          string
	Rules            []StaticPolicyRule
	PolicyBundleHash string
	ReceivedAt       time.Time
}

// PolicyProvider supplies the static policy rules used by Boundary.
type PolicyProvider interface {
	LoadPolicies(ctx context.Context) ([]StaticPolicyRule, error)
	WatchPolicyUpdates(ctx context.Context) (<-chan PolicyUpdate, error)
}

// CostEstimate is the normalized cost signal used by kernel budget bridges.
type CostEstimate struct {
	Amount     int64
	Unit       string
	Confidence float64
	Source     string
}

type CostPredictor interface {
	PredictCost(ctx context.Context, req GovernanceRequest) (CostEstimate, error)
}

type BudgetEnforcer interface {
	CheckBudget(ctx context.Context, tenantID, agentID string, cost CostEstimate) (bool, error)
	RecordSpend(ctx context.Context, tenantID, agentID string, amount int64) error
}

type EscalationHandler interface {
	Escalate(ctx context.Context, req GovernanceRequest, reason string) (*GovernanceDecision, error)
}

type EnvelopeID string

type EnvelopeState string

const (
	EnvelopeStateCreated EnvelopeState = "CREATED"
	EnvelopeStateRunning EnvelopeState = "RUNNING"
	EnvelopeStateAllowed EnvelopeState = "ALLOWED"
	EnvelopeStateDenied  EnvelopeState = "DENIED"
	EnvelopeStateClosed  EnvelopeState = "CLOSED"
)

type EnvelopeManager interface {
	CreateEnvelope(ctx context.Context, req GovernanceRequest) (EnvelopeID, error)
	TransitionEnvelope(ctx context.Context, id EnvelopeID, state EnvelopeState) error
}

type ProofRef struct {
	Invariant      string
	Theorem        string
	Repo           string
	Path           string
	Correspondence string
	Notes          string
}

// ProofCorrespondence is documentation metadata. It does not certify runtime
// decisions and must not be used to emit DecisionModeProved from Boundary.
type ProofCorrespondence interface {
	GetCorrespondence(invariant string) (ProofRef, error)
}
