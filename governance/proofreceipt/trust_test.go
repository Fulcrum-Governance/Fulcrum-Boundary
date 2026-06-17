package proofreceipt

import (
	"math"
	"testing"
)

func TestCheckTrustCircuit_OpenWhenBelowThreshold(t *testing.T) {
	// alpha=0, beta=10, threshold 3/10: trust=(0+1)/(0+10+2)=1/12 < 3/10 -> open.
	w := TrustCircuitWitness{Alpha: 0, Beta: 10, ThresholdNum: 3, ThresholdDen: 10,
		CircuitOpen: true, TheoremID: TheoremTrustTermination}
	inv := CheckTrustCircuit(w)
	if inv.Result != ResultPass {
		t.Fatalf("result = %q, want pass (open matches below-threshold)", inv.Result)
	}
	if inv.Predicate != "circuit_open iff (alpha+1)*q < p*(alpha+beta+2)" {
		t.Fatalf("predicate = %q", inv.Predicate)
	}
	if inv.TheoremID != TheoremTrustTermination {
		t.Fatalf("theorem_id = %q", inv.TheoremID)
	}
	if inv.InputsHash == "" {
		t.Fatal("InputsHash must be non-empty")
	}
}

func TestCheckTrustCircuit_ClosedWhenAtOrAboveThreshold(t *testing.T) {
	// alpha=10, beta=0, threshold 3/10: trust=11/12 >= 3/10 -> closed.
	w := TrustCircuitWitness{Alpha: 10, Beta: 0, ThresholdNum: 3, ThresholdDen: 10, CircuitOpen: false}
	if got := CheckTrustCircuit(w).Result; got != ResultPass {
		t.Fatalf("result = %q, want pass (closed matches at/above threshold)", got)
	}
}

func TestCheckTrustCircuit_InconsistentStateFails(t *testing.T) {
	// Below threshold but claims circuit closed -> ill-formed -> fail.
	w := TrustCircuitWitness{Alpha: 0, Beta: 10, ThresholdNum: 3, ThresholdDen: 10, CircuitOpen: false}
	if got := CheckTrustCircuit(w).Result; got != ResultFail {
		t.Fatalf("result = %q, want fail (state contradicts threshold)", got)
	}
}

func TestCheckTrustCircuit_IllFormedThresholdFails(t *testing.T) {
	if got := CheckTrustCircuit(TrustCircuitWitness{Alpha: 1, Beta: 1, ThresholdNum: 0, ThresholdDen: 10}).Result; got != ResultFail {
		t.Fatalf("p=0 must fail (0<p<q), got %q", got)
	}
	if got := CheckTrustCircuit(TrustCircuitWitness{Alpha: 1, Beta: 1, ThresholdNum: 10, ThresholdDen: 10}).Result; got != ResultFail {
		t.Fatalf("p==q must fail (0<p<q), got %q", got)
	}
}

// TestTrustBelowThreshold_NoOverflow verifies that trustBelowThreshold uses exact
// Nat semantics (math/big) so additions like alpha+beta+2 cannot wrap in uint64.
//
// Witness: alpha=0, beta=math.MaxUint64-1, p=3, q=10.
// Nat answer: (0+1)*10 = 10 < 3*(0 + (2^64-2) + 2) = 3*2^64 → TRUE.
// Plain uint64: alpha+beta+2 wraps to 0, so 10 < 0 → FALSE (wrong).
func TestTrustBelowThreshold_NoOverflow(t *testing.T) {
	// Below-threshold with wrapped sum: uint64 would compute alpha+beta+2 = 0 and
	// wrongly return false; math/big must return true.
	if !trustBelowThreshold(0, math.MaxUint64-1, 3, 10) {
		t.Fatal("trustBelowThreshold(0, MaxUint64-1, 3, 10) = false; want true (uint64 wraps alpha+beta+2 to 0, giving wrong answer)")
	}
	// Maximally-high-alpha agent is NOT below threshold (alpha+1 is huge; lhs >> rhs).
	if trustBelowThreshold(math.MaxUint64, 0, 3, 10) {
		t.Fatal("trustBelowThreshold(MaxUint64, 0, 3, 10) = true; want false (high alpha means high trust, not below threshold)")
	}
}

// TestCheckTrustCircuit_NoOverflowConsistency verifies that CheckTrustCircuit
// correctly reflects the exact math/big comparison when extreme uint64 values are
// present: CircuitOpen=true must pass (open ↔ below-threshold) and
// CircuitOpen=false must fail (open claim contradicts at-or-above result).
func TestCheckTrustCircuit_NoOverflowConsistency(t *testing.T) {
	w := TrustCircuitWitness{
		Alpha: 0, Beta: math.MaxUint64 - 1,
		ThresholdNum: 3, ThresholdDen: 10,
		CircuitOpen: true,
		TheoremID:   TheoremTrustTermination,
	}
	if got := CheckTrustCircuit(w).Result; got != ResultPass {
		t.Fatalf("CircuitOpen=true with extreme beta: result = %q, want pass", got)
	}
	w.CircuitOpen = false
	if got := CheckTrustCircuit(w).Result; got != ResultFail {
		t.Fatalf("CircuitOpen=false with extreme beta: result = %q, want fail", got)
	}
}

func TestTrustBelowThreshold_MatchesLeanEncoding(t *testing.T) {
	// Lean: (alpha+1)*q < p*(alpha+beta+2). alpha=0,beta=10,p=3,q=10:
	// (1)*10=10 < 3*(12)=36 -> true.
	if !trustBelowThreshold(0, 10, 3, 10) {
		t.Fatal("expected below threshold")
	}
	// alpha=10,beta=0,p=3,q=10: 11*10=110 < 3*12=36 -> false.
	if trustBelowThreshold(10, 0, 3, 10) {
		t.Fatal("expected not below threshold")
	}
}
