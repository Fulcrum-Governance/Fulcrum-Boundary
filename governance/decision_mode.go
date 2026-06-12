package governance

// DecisionMode labels the epistemic confidence level of a governance decision.
// Every decision should carry an explicit mode so operators, auditors, and
// downstream systems know what kind of confidence they are looking at.
//
// The four modes are mutually exclusive and exhaustive for decisions that
// originate inside the Fulcrum governance stack:
//
//   - deterministic → static rule / deterministic code path
//   - classified    → probabilistic evaluator (e.g., Semantic Judge)
//   - proved        → machine-checkable formal proof
//   - human_approved → human reviewer approved the action
//
// From its own logic the Boundary pipeline produces only deterministic and
// classified decisions. The proved and human_approved modes originate in the
// upstream Foundry layer (fulcrum-io) when Lean 4 verification or human review
// occurs; Boundary never mints them. The kernel escalation-await seam may RELAY
// a human_approved resolution onto a pipeline decision (an approved/denied
// human review), but it relays — it does not originate — and it is guarded
// against relaying proved. See governance/pipeline.go (resolveEscalation) and
// docs/PROOF_BOUNDARY.md.
type DecisionMode string

const (
	// DecisionModeDeterministic indicates the decision was made by static
	// policy rule matching — no probabilistic inference involved.
	DecisionModeDeterministic DecisionMode = "deterministic"

	// DecisionModeClassified indicates the decision was made by a semantic
	// evaluator (e.g., LLM-based Semantic Judge) — probabilistic.
	DecisionModeClassified DecisionMode = "classified"

	// DecisionModeProved indicates the decision is backed by a machine-checkable
	// formal proof (e.g., Lean 4 budget safety invariant).
	DecisionModeProved DecisionMode = "proved"

	// DecisionModeHumanApproved indicates a human review resolved the action;
	// the decision Action carries the reviewer's verdict (approved → allow,
	// denied → deny). It is set when a human-review resolution is relayed from
	// the upstream Foundry layer (fulcrum-io), never minted by the Boundary
	// pipeline's own logic.
	DecisionModeHumanApproved DecisionMode = "human_approved"
)

// Valid returns true if the decision mode is one of the four recognized
// values. The empty-string zero value returns false so unset fields can be
// detected programmatically.
func (m DecisionMode) Valid() bool {
	switch m {
	case DecisionModeDeterministic, DecisionModeClassified,
		DecisionModeProved, DecisionModeHumanApproved:
		return true
	}
	return false
}
