package proofreceipt

import "testing"

func TestCheckBudget_AdmitWhenWithinLimit(t *testing.T) {
	w := BudgetWitness{Limit: 100, SpentBefore: 40, Requested: 60, SpentAfter: 100,
		DecisionHash: "sha256:rec", TheoremID: TheoremBudgetLocal, CheckerVersion: "0.1.0"}
	inv := CheckBudget(w)
	if inv.Result != ResultPass {
		t.Fatalf("result = %q, want pass (40+60<=100)", inv.Result)
	}
	if inv.TheoremID != TheoremBudgetLocal {
		t.Fatalf("theorem_id = %q", inv.TheoremID)
	}
	if inv.Predicate != "spent_before + requested <= limit" {
		t.Fatalf("predicate = %q", inv.Predicate)
	}
	if inv.InputsHash == "" || inv.InputsHash[:7] != "sha256:" {
		t.Fatalf("inputs_hash = %q, want sha256:-prefixed", inv.InputsHash)
	}
}

func TestCheckBudget_FailWhenOverLimit(t *testing.T) {
	w := BudgetWitness{Limit: 100, SpentBefore: 80, Requested: 40, SpentAfter: 80}
	if got := CheckBudget(w).Result; got != ResultFail {
		t.Fatalf("result = %q, want fail (80+40>100)", got)
	}
}

func TestCheckBudget_FailWhenSpentAfterInconsistent(t *testing.T) {
	// Predicate holds (10+10<=100) but SpentAfter lies about the applied spend.
	w := BudgetWitness{Limit: 100, SpentBefore: 10, Requested: 10, SpentAfter: 999}
	if got := CheckBudget(w).Result; got != ResultFail {
		t.Fatalf("result = %q, want fail (spent_after must equal 20)", got)
	}
}

func TestCheckBudget_FailOnNegativeInputs(t *testing.T) {
	if got := CheckBudget(BudgetWitness{Limit: 100, SpentBefore: -1, Requested: 10}).Result; got != ResultFail {
		t.Fatalf("negative spent_before must fail, got %q", got)
	}
	if got := CheckBudget(BudgetWitness{Limit: 100, SpentBefore: 0, Requested: -5}).Result; got != ResultFail {
		t.Fatalf("negative requested must fail, got %q", got)
	}
}

func TestCheckBudget_BoundaryEquality(t *testing.T) {
	// Exactly at limit must admit (Lean uses <=).
	w := BudgetWitness{Limit: 100, SpentBefore: 100, Requested: 0, SpentAfter: 100}
	if got := CheckBudget(w).Result; got != ResultPass {
		t.Fatalf("spent==limit, requested 0 must pass, got %q", got)
	}
}
