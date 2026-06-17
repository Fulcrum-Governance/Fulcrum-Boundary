package proofreceipt

const TheoremBudgetLocal = "THM-BUDGET-LOCAL"

// budgetPredicate is the exact human-readable predicate THM-BUDGET-LOCAL
// constrains (BasicInvariants.lean budget_safety_guarantee): applyAction admits
// iff currentSpent + delta <= aggregateLimit.
const budgetPredicate = "spent_before + requested <= limit"

// BudgetWitness is the canonical witness for the budget invariant. Field names
// and types are frozen — they are hashed into Invariant.InputsHash, so the
// digest is stable across languages. SpentAfter is the admitted new spend
// (SpentBefore+Requested) when the predicate holds; on a denial it equals
// SpentBefore (no spend was applied).
type BudgetWitness struct {
	BudgetKey      string `json:"budget_key"`
	TenantID       string `json:"tenant_id"`
	AgentID        string `json:"agent_id"`
	Limit          int64  `json:"limit"`
	SpentBefore    int64  `json:"spent_before"`
	Requested      int64  `json:"requested"`
	SpentAfter     int64  `json:"spent_after"`
	PolicyHash     string `json:"policy_bundle_hash"`
	DecisionHash   string `json:"decision_hash"`
	TheoremID      string `json:"theorem_id"`
	CheckerVersion string `json:"checker_version"`
}

// CheckBudget evaluates the THM-BUDGET-LOCAL predicate over w and returns the
// Invariant line for a proof receipt. The predicate holds (pass) iff
// w.SpentBefore >= 0 AND w.Requested >= 0 AND w.SpentBefore+w.Requested <=
// w.Limit AND w.SpentAfter == w.SpentBefore+w.Requested; otherwise fail. This
// mirrors the Lean applyAction admit-and-update step, not a re-derivation.
func CheckBudget(w BudgetWitness) Invariant {
	result := ResultFail
	switch {
	case w.SpentBefore < 0 || w.Requested < 0:
		result = ResultFail
	case w.SpentBefore+w.Requested <= w.Limit && w.SpentAfter == w.SpentBefore+w.Requested:
		result = ResultPass
	}
	return Invariant{
		TheoremID:  TheoremBudgetLocal,
		Predicate:  budgetPredicate,
		InputsHash: CanonicalInputsHash(w),
		Result:     result,
	}
}
