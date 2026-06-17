package proofreceipt

import "testing"

func TestCheckPrivilege_SubsetPasses(t *testing.T) {
	w := PrivilegeWitness{RequestedCaps: []string{"repo:write"},
		AuthorizedCaps: []string{"repo:read", "repo:write"}, TheoremID: TheoremPrivilegeStatic}
	inv := CheckPrivilege(w)
	if inv.Result != ResultPass {
		t.Fatalf("result = %q, want pass (subset)", inv.Result)
	}
	if inv.Predicate != "requested_caps subset authorized_caps" {
		t.Fatalf("predicate = %q", inv.Predicate)
	}
	if inv.TheoremID != TheoremPrivilegeStatic {
		t.Fatalf("theorem_id = %q", inv.TheoremID)
	}
}

func TestCheckPrivilege_NonSubsetFails(t *testing.T) {
	w := PrivilegeWitness{RequestedCaps: []string{"repo:write", "repo:admin"},
		AuthorizedCaps: []string{"repo:read", "repo:write"}}
	if got := CheckPrivilege(w).Result; got != ResultFail {
		t.Fatalf("result = %q, want fail (repo:admin not authorized)", got)
	}
}

func TestCheckPrivilege_EmptyRequestPasses(t *testing.T) {
	if got := CheckPrivilege(PrivilegeWitness{RequestedCaps: nil, AuthorizedCaps: []string{"x"}}).Result; got != ResultPass {
		t.Fatalf("empty requested set must pass, got %q", got)
	}
}

func TestCheckPrivilege_OrderAndDuplicateInsensitive(t *testing.T) {
	a := CheckPrivilege(PrivilegeWitness{RequestedCaps: []string{"a", "b", "a"}, AuthorizedCaps: []string{"b", "a"}})
	if a.Result != ResultPass {
		t.Fatalf("duplicates within an authorized set must still pass, got %q", a.Result)
	}
}
