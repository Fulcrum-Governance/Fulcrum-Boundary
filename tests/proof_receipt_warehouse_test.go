package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/governance/proofreceipt"
)

// TestProofReceiptBindsToWarehouseVerbatimRecord reproduces the IO M8 adopt path
// (Fulcrum/internal/evidence/store.go): BuildDecisionRecord -> json.Marshal
// (verbatim record_json) -> VerifyDecisionRecord(record, nil, "", ""). It then
// proves a proof-receipt sidecar binds to the verbatim bytes by decision_hash
// without the marshaled record changing, and that tampering the record breaks
// the binding (negative case).
func TestProofReceiptBindsToWarehouseVerbatimRecord(t *testing.T) {
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Transport: governance.TransportMCP, ToolName: "create_or_update_file",
		Action: "deny", Reason: "lethal-trifecta deny", TrustScore: 1,
		TrustState: governance.TrustStateTrusted.String(),
	})
	// The warehouse stores these exact bytes (record_json) verbatim.
	recordJSON, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if verr := governance.VerifyDecisionRecord(rec, nil, "", ""); verr != nil {
		t.Fatalf("warehouse verify-on-ingest must pass: %v", verr)
	}

	receipt := proofreceipt.AttachAll(rec, "fulcrum-proof-checker/0.1.0", "sha256:build",
		[]proofreceipt.Invariant{
			proofreceipt.CheckBudget(proofreceipt.BudgetWitness{Limit: 50, SpentBefore: 10, Requested: 5, SpentAfter: 15, DecisionHash: rec.DecisionHash}),
			proofreceipt.CheckPrivilege(proofreceipt.PrivilegeWitness{RequestedCaps: []string{"repo:write"}, AuthorizedCaps: []string{"repo:write"}, DecisionHash: rec.DecisionHash}),
		}, time.Time{})

	// The verbatim bytes are unchanged by attaching the sidecar.
	after, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	if string(recordJSON) != string(after) {
		t.Fatalf("record_json changed after attach:\n before=%s\n after =%s", recordJSON, after)
	}
	if err := receipt.VerifyBinding(rec); err != nil {
		t.Fatalf("sidecar must bind to verbatim record: %v", err)
	}

	// Negative: a record whose action was tampered yields a different
	// decision_hash, so the sidecar no longer binds.
	tampered := rec
	tampered.Action = "allow"
	if err := receipt.VerifyBinding(tampered); err == nil {
		t.Fatal("sidecar must NOT bind to a tampered record")
	}
}
