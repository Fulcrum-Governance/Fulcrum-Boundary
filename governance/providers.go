package governance

import (
	"context"
	"time"
)

// PolicyUpdate is emitted by providers that can stream policy-bundle changes.
type PolicyUpdate struct {
	// Version identifies the policy bundle version this update carries.
	Version string
	// Rules is the static policy rule set for this version.
	Rules []StaticPolicyRule
	// PolicyBundleHash is the stable hash of the updated bundle (see
	// PolicyBundleHashFromDir).
	PolicyBundleHash string
	// ReceivedAt is when this update was observed.
	ReceivedAt time.Time
}

// PolicyProvider supplies the static policy rules used by Boundary. It is one of
// the seam interfaces in governance/providers.go: the standalone implementation
// (loading YAML from disk) lives in governance/standalone, and the kernel
// implementation (Fulcrum-backed) lives in governance/kernel. See
// docs/INTEGRATION.md.
type PolicyProvider interface {
	// LoadPolicies returns the current static policy rule set, or an error if
	// the rules cannot be loaded. It is called to obtain the active rules; an
	// error is a fault, not a verdict, and the caller decides how to treat it.
	LoadPolicies(ctx context.Context) ([]StaticPolicyRule, error)
	// WatchPolicyUpdates returns a channel that delivers a PolicyUpdate whenever
	// the policy bundle changes, or an error if a watch cannot be established.
	// A provider with no update mechanism may return a nil/never-firing channel.
	WatchPolicyUpdates(ctx context.Context) (<-chan PolicyUpdate, error)
}

// CostEstimate is the normalized cost signal used by kernel budget bridges.
type CostEstimate struct {
	// Amount is the estimated cost magnitude, in units of Unit.
	Amount int64
	// Unit names the unit Amount is expressed in (e.g. tokens).
	Unit string
	// Confidence is the predictor's confidence in the estimate, 0.0 to 1.0.
	Confidence float64
	// Source names the predictor or model that produced the estimate.
	Source string
}

// CostPredictor estimates the cost of a governed request so a BudgetEnforcer can
// decide whether it fits the remaining budget. It is a kernel-mode seam; the
// standalone path does not enforce budgets.
type CostPredictor interface {
	// PredictCost returns a CostEstimate for req, or an error if no estimate can
	// be produced. The estimate is advisory input to budget enforcement, not a
	// verdict.
	PredictCost(ctx context.Context, req GovernanceRequest) (CostEstimate, error)
}

// BudgetEnforcer admits or rejects spend against a tenant/agent budget. It is a
// kernel-mode seam; the standalone path does not enforce budgets. The
// proof-correspondence design intent is that a budget check denies when the
// remaining budget is below the requested cost (see docs/PROOF_BOUNDARY.md);
// that correspondence is a design constraint, not a runtime certificate.
type BudgetEnforcer interface {
	// CheckBudget reports whether cost is permitted for the given tenant and
	// agent. It returns true to admit and false to reject; the error is a fault
	// (the budget backend could not be consulted), distinct from a false
	// admit-or-reject verdict.
	CheckBudget(ctx context.Context, tenantID, agentID string, cost CostEstimate) (bool, error)
	// RecordSpend durably records that amount was spent by the given tenant and
	// agent, or returns an error if the spend could not be recorded.
	RecordSpend(ctx context.Context, tenantID, agentID string, amount int64) error
}

// EscalationHandler routes a request that policy marked for escalation to an
// out-of-band approver and returns the resulting decision. An implementation
// may return immediately after routing (fire-and-forget, decision still
// "escalate") or block for a bounded window and return the resolved verdict of
// an upstream human review (e.g. allow with DecisionModeHumanApproved). It is
// a kernel-mode seam; the standalone path has no escalation backend.
type EscalationHandler interface {
	// Escalate handles req for the given reason and returns the decision the
	// escalation resolved to, or an error if escalation could not be performed.
	// An error is a fault, not an approval.
	Escalate(ctx context.Context, req GovernanceRequest, reason string) (*GovernanceDecision, error)
}

// EnvelopeID identifies a single action envelope tracked across its lifecycle.
type EnvelopeID string

// EnvelopeState is the lifecycle state of an action envelope. The defined states
// are the EnvelopeState* constants below.
type EnvelopeState string

const (
	// EnvelopeStateCreated marks an envelope that has been opened for a request
	// but whose action has not yet started.
	EnvelopeStateCreated EnvelopeState = "CREATED"
	// EnvelopeStateRunning marks an envelope whose action is in progress.
	EnvelopeStateRunning EnvelopeState = "RUNNING"
	// EnvelopeStateAllowed marks an envelope whose action was permitted.
	EnvelopeStateAllowed EnvelopeState = "ALLOWED"
	// EnvelopeStateDenied marks an envelope whose action was denied.
	EnvelopeStateDenied EnvelopeState = "DENIED"
	// EnvelopeStateClosed marks an envelope that has reached a terminal state and
	// is no longer active.
	EnvelopeStateClosed EnvelopeState = "CLOSED"
)

// EnvelopeManager tracks the lifecycle of an action envelope. It is a
// kernel-mode seam: the standalone implementation
// (governance/standalone.LocalEnvelopeManager) returns or mints an EnvelopeID
// and treats transitions as no-ops, while the kernel implementation publishes
// state to NATS. See docs/INTEGRATION.md.
type EnvelopeManager interface {
	// CreateEnvelope opens an envelope for req and returns its EnvelopeID, or an
	// error if one cannot be opened. An implementation may reuse a caller-
	// supplied req.EnvelopeID or mint a new identifier.
	CreateEnvelope(ctx context.Context, req GovernanceRequest) (EnvelopeID, error)
	// TransitionEnvelope moves the identified envelope to state, or returns an
	// error if the transition cannot be applied.
	TransitionEnvelope(ctx context.Context, id EnvelopeID, state EnvelopeState) error
}

// ProofRef is documentation metadata linking a Boundary behavior to a formal
// proof in the Fulcrum-Proofs repo. It is descriptive provenance, not a runtime
// certificate: it records which theorem a behavior was designed to satisfy, not
// proof that the running code discharges it. See docs/PROOF_BOUNDARY.md.
type ProofRef struct {
	// Invariant names the Boundary behavior the reference is about.
	Invariant string
	// Theorem is the fully qualified Lean 4 theorem name (e.g.
	// Fulcrum.budget_safety_guarantee).
	Theorem string
	// Repo is the repository holding the proof (Fulcrum-Proofs).
	Repo string
	// Path is the proof's source file path within Repo.
	Path string
	// Correspondence is the correspondence type, e.g. "design", meaning the
	// runtime behavior was designed to satisfy the proved invariant — not that
	// the Go implementation was mechanically extracted from Lean.
	Correspondence string
	// Notes is optional free-form context about the correspondence.
	Notes string
}

// ProofCorrespondence is documentation metadata. It does not certify runtime
// decisions and must not be used to emit DecisionModeProved from Boundary.
type ProofCorrespondence interface {
	// GetCorrespondence returns the ProofRef documenting the named invariant, or
	// an error if no correspondence is recorded for it. The returned reference is
	// provenance metadata only and does not authorize a proved decision.
	GetCorrespondence(invariant string) (ProofRef, error)
}
