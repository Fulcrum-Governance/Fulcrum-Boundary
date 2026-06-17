package governance

import "testing"

// TestComputeDecisionHashGoldenIsPinned freezes the decision_hash algorithm for
// one fixed record. WS-1a adds CanonicalJSONBytes calling mustCanonicalJSON; any
// accidental change to that canonicalization path would change every stored
// record's hash and break the IO M8 warehouse, which re-verifies stored
// historical bytes. This golden catches such a change at the source.
func TestComputeDecisionHashGoldenIsPinned(t *testing.T) {
	rec := DecisionRecordV1{
		SchemaVersion: DecisionRecordSchemaVersion,
		EventType:     "governance_decision",
		Tool:          "create_or_update_file",
		Action:        "deny",
		Reason:        "protected branch",
		DecisionMode:  DecisionModeDeterministic,
		TrustScore:    1,
	}
	const want = "sha256:4ab80ef5d412af517d4d02e95bc6e617a697175cbbae095a7527729d0c4f2d91"
	if got := ComputeDecisionHash(rec); got != want {
		t.Fatalf("decision_hash algorithm changed (or golden not yet pinned):\n got=%s\nwant=%s", got, want)
	}
}
