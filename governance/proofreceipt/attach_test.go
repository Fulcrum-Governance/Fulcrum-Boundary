package proofreceipt

import (
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// TestAttachAll_DoesNotPerturbDecisionHash is the load-bearing WS-1e invariant:
// attaching a sidecar must not change the record's decision_hash. The record's
// hash before and after building+binding a receipt must be byte-identical, and
// the verbatim record must still pass governance.VerifyDecisionRecord with the
// warehouse's exact arguments.
func TestAttachAll_DoesNotPerturbDecisionHash(t *testing.T) {
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport: governance.TransportMCP, ToolName: "create_or_update_file",
		Action: "deny", Reason: "protected branch", TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
	})
	before := rec.DecisionHash
	invs := []Invariant{
		CheckBudget(BudgetWitness{Limit: 100, SpentBefore: 0, Requested: 1, SpentAfter: 1, DecisionHash: rec.DecisionHash}),
		CheckPrivilege(PrivilegeWitness{RequestedCaps: []string{"repo:read"}, AuthorizedCaps: []string{"repo:read"}, DecisionHash: rec.DecisionHash}),
		CheckTrustCircuit(TrustCircuitWitness{Alpha: 10, Beta: 0, ThresholdNum: 3, ThresholdDen: 10, CircuitOpen: false, DecisionHash: rec.DecisionHash}),
	}
	r := AttachAll(rec, "fulcrum-proof-checker/0.1.0", "sha256:build", invs, time.Time{})
	if rec.DecisionHash != before {
		t.Fatalf("record.DecisionHash mutated by attach: %s -> %s", before, rec.DecisionHash)
	}
	if governance.ComputeDecisionHash(rec) != before {
		t.Fatalf("recomputed decision_hash drifted: %s", governance.ComputeDecisionHash(rec))
	}
	// Warehouse-exact verification call must still pass on the verbatim record.
	if err := governance.VerifyDecisionRecord(rec, nil, "", ""); err != nil {
		t.Fatalf("verbatim record must still verify: %v", err)
	}
	if err := r.VerifyBinding(rec); err != nil {
		t.Fatalf("receipt must bind to verbatim record: %v", err)
	}
	if len(r.Invariants) != 3 {
		t.Fatalf("want 3 invariants, got %d", len(r.Invariants))
	}
}

func TestAttachAll_DropsEmptyTheoremLines(t *testing.T) {
	rec := governance.BuildDecisionRecord(governance.AuditEvent{Transport: governance.TransportMCP, ToolName: "x", Action: "allow", TrustScore: 1})
	r := AttachAll(rec, "c", "h", []Invariant{{}, CheckBudget(BudgetWitness{Limit: 1, SpentBefore: 0, Requested: 0, SpentAfter: 0})}, time.Now())
	if len(r.Invariants) != 1 {
		t.Fatalf("empty-theorem line must be dropped, got %d invariants", len(r.Invariants))
	}
}
