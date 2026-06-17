package proofreceipt

import "math/big"

const TheoremTrustTermination = "THM-TRUST-TERMINATION"

// trustPredicate is the exact predicate THM-TRUST-TERMINATION constrains
// (TrustTermination.lean trust_termination_invariant + wellFormed): the circuit
// is open iff trust is below threshold, with trust below threshold encoded as
// (alpha+1)*q < p*(alpha+beta+2).
const trustPredicate = "circuit_open iff (alpha+1)*q < p*(alpha+beta+2)"

// TrustCircuitWitness is the canonical witness for the trust-circuit invariant.
// Alpha/Beta are the Beta-distribution interaction counts; ThresholdNum/
// ThresholdDen encode the isolation threshold p/q (e.g. 3/10 for 0.3, the
// DefaultTrustIsolationThreshold). CircuitOpen is the runtime claim that the
// agent's circuit is open (ISOLATED or TERMINATED in governance.TrustState
// terms). The checker validates wellFormedness: CircuitOpen must equal
// (alpha+1)*q < p*(alpha+beta+2), exactly the Lean Nat cross-multiplication.
type TrustCircuitWitness struct {
	AgentID        string `json:"agent_id"`
	Alpha          uint64 `json:"alpha"`
	Beta           uint64 `json:"beta"`
	ThresholdNum   uint64 `json:"threshold_num"`
	ThresholdDen   uint64 `json:"threshold_den"`
	CircuitOpen    bool   `json:"circuit_open"`
	DecisionHash   string `json:"decision_hash"`
	TheoremID      string `json:"theorem_id"`
	CheckerVersion string `json:"checker_version"`
}

// trustBelowThreshold mirrors the Lean trustBelowThreshold (TrustTermination.lean
// trustNum α := α+1, trustDen α β := α+β+2): (alpha+1)*q < p*(alpha+beta+2). Uses
// math/big so the Nat cross-multiplication is exact for all uint64 inputs — no
// wraparound occurs even when alpha+1 or alpha+beta+2 would overflow uint64.
func trustBelowThreshold(alpha, beta, p, q uint64) bool {
	aPlus1 := new(big.Int).Add(new(big.Int).SetUint64(alpha), big.NewInt(1))
	lhs := new(big.Int).Mul(aPlus1, new(big.Int).SetUint64(q)) // (alpha+1)*q
	aBeta2 := new(big.Int).Add(new(big.Int).SetUint64(alpha), new(big.Int).SetUint64(beta))
	aBeta2.Add(aBeta2, big.NewInt(2))                          // alpha+beta+2
	rhs := new(big.Int).Mul(new(big.Int).SetUint64(p), aBeta2) // p*(alpha+beta+2)
	return lhs.Cmp(rhs) < 0
}

// CheckTrustCircuit evaluates the THM-TRUST-TERMINATION well-formedness predicate
// over w. It passes iff w.CircuitOpen equals trustBelowThreshold(alpha,beta,p,q)
// AND the threshold is well-formed (0 < p < q). It is a circuit-transition
// consistency check, not a per-decision termination proof, and triggers no
// process action.
func CheckTrustCircuit(w TrustCircuitWitness) Invariant {
	result := ResultFail
	wellFormedThreshold := w.ThresholdNum > 0 && w.ThresholdNum < w.ThresholdDen
	if wellFormedThreshold {
		below := trustBelowThreshold(w.Alpha, w.Beta, w.ThresholdNum, w.ThresholdDen)
		if w.CircuitOpen == below {
			result = ResultPass
		}
	}
	return Invariant{
		TheoremID:  TheoremTrustTermination,
		Predicate:  trustPredicate,
		InputsHash: CanonicalInputsHash(w),
		Result:     result,
	}
}
