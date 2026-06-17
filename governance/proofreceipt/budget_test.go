package proofreceipt

import (
	"math"
	"testing"
)

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

func TestCheckBudget_NoOverflow(t *testing.T) {
	// SpentBefore = MaxInt64, Requested = 1: the true sum (MaxInt64+1) is
	// astronomically larger than Limit=100, so the request is way over budget
	// and MUST produce ResultFail.
	//
	// Under a naive int64 implementation, SpentBefore+Requested wraps to
	// math.MinInt64 (a large negative), which is <= 100, causing the predicate
	// to wrongly evaluate as ResultPass — the overflow bug.
	//
	// SpentAfter: we set it to math.MinInt64 (the int64-wrapped sum) so that
	// the naive consistency check (SpentAfter == SpentBefore+Requested) also
	// passes under the buggy implementation, giving the attacker the most
	// favorable possible witness. The correct big.Int path sees the true sum
	// and must reject regardless.
	w := BudgetWitness{
		Limit:       100,
		SpentBefore: math.MaxInt64,
		Requested:   1,
		SpentAfter:  math.MinInt64, // int64 wrap of MaxInt64+1; crafted to fool naive impl
	}
	if got := CheckBudget(w).Result; got != ResultFail {
		t.Fatalf("overflow witness must fail (MaxInt64+1 >> 100), got %q", got)
	}
}
