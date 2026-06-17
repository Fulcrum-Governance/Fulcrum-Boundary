package proofreceipt

const TheoremPrivilegeStatic = "THM-PRIVILEGE-STATIC"

// privilegePredicate is the exact predicate THM-PRIVILEGE-STATIC constrains
// (BasicInvariants.lean subset_iff_diff_empty): requested capabilities are a
// subset of authorized capabilities, i.e. requested \ authorized = ∅.
const privilegePredicate = "requested_caps subset authorized_caps"

// PrivilegeWitness is the canonical witness for the static-privilege invariant.
// RequestedCaps and AuthorizedCaps are capability identifiers (the Lean Cap is
// String). The checker treats them as sets (order- and duplicate-insensitive),
// matching the Lean Set Cap model; it does NOT evaluate policy semantics.
type PrivilegeWitness struct {
	AgentID        string   `json:"agent_id"`
	TenantID       string   `json:"tenant_id"`
	RequestedCaps  []string `json:"requested_caps"`
	AuthorizedCaps []string `json:"authorized_caps"`
	PolicyHash     string   `json:"policy_bundle_hash"`
	DecisionHash   string   `json:"decision_hash"`
	TheoremID      string   `json:"theorem_id"`
	CheckerVersion string   `json:"checker_version"`
}

// CheckPrivilege evaluates THM-PRIVILEGE-STATIC over w. The predicate holds
// (pass) iff every element of RequestedCaps is in AuthorizedCaps (set difference
// empty); otherwise fail. An empty RequestedCaps trivially passes (∅ ⊆
// anything), matching Lean set inclusion. Static capability-subset only, not
// semantic policy correctness.
func CheckPrivilege(w PrivilegeWitness) Invariant {
	authorized := make(map[string]struct{}, len(w.AuthorizedCaps))
	for _, c := range w.AuthorizedCaps {
		authorized[c] = struct{}{}
	}
	result := ResultPass
	for _, c := range w.RequestedCaps {
		if _, ok := authorized[c]; !ok {
			result = ResultFail
			break
		}
	}
	return Invariant{
		TheoremID:  TheoremPrivilegeStatic,
		Predicate:  privilegePredicate,
		InputsHash: CanonicalInputsHash(w),
		Result:     result,
	}
}
