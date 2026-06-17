package claims

import (
	"strings"
	"testing"
)

// TestProofReceiptLintRuleSemantics pins the scoped checker claim: the blanket
// "decision record is validated by a formally verified checker" overclaim is
// rejected, while the sidecar-scoped claim and the negated disclaimer pass.
// Generic scope words (invariant, budget, trust-circuit, receipt) no longer
// satisfy the predicate; only an explicit sidecar/proof-receipt reference or
// negation does.
func TestProofReceiptLintRuleSemantics(t *testing.T) {
	cases := []struct {
		line  string
		allow bool
	}{
		{"boundary's decision record is validated by a formally verified checker", false},
		{"the proof-receipt sidecar is validated by a formally verified checker", true},
		{"a checker-validated proof receipt attests the budget and static-privilege invariants", true},
		{"the decision record itself is not validated by a formally verified checker", true},
		// Generic scope words alone must NOT allow the claim through.
		{"boundary's decision record is validated by a formally verified checker for every invariant", false},
		{"the entire decision pipeline is validated by a formally verified checker, ensuring receipt integrity", false},
		{"a formally verified checker validates the budget invariant", false},
		// Explicit sidecar reference within a broader sentence is still allowed.
		{"the proof-receipt sidecar's trust-circuit invariant is checker-validated", true},
	}
	for _, tc := range cases {
		if got := proofReceiptScoped(strings.ToLower(tc.line)); got != tc.allow {
			t.Fatalf("proofReceiptScoped(%q) = %v, want %v", tc.line, got, tc.allow)
		}
	}
}
