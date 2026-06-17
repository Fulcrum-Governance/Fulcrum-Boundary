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
		// blanket-subject claims must be rejected even when a scope word is stuffed in
		{"the whole pipeline is validated by a formally verified checker sidecar", false},
		{"boundary's decision record is validated by a formally verified checker, see the proof-receipt sidecar", false},
		{"the entire decision pipeline is a checker-validated proof receipt", false},
		// Quantifier / broad-subject overclaims must reject even when a scope word is stuffed in
		// (Codex + Opus review: keyword-stuffing "sidecar"/"proof receipt" must not narrow the subject).
		{"a formally verified checker sidecar validates every decision", false},
		{"the sidecar's formally verified checker validates all records", false},
		{"the proof receipt sidecar checker-validates the complete pipeline", false},
		{"every boundary decision is validated by a formally verified checker — see the sidecar", false},
		{"boundary's verdict is validated by a formally verified checker, per the proof-receipt sidecar", false},
		{"the system is checker-validated; details in the proof receipt", false},
		{"all boundary outputs are validated by a formally verified checker (proof receipt)", false},
		// Deliberate strictness (Opus FP, decided as want=false): a single line that asserts
		// checker-validation while naming the decision record it binds to is rejected as
		// overclaim-prone; state the binding in a separate sentence.
		{"the proof-receipt sidecar binds to the decision record and is checker-validated", false},
		// Re-review: symmetric quantifier coverage — a broad subject under ANY quantifier
		// must reject even with a stuffed scope word.
		{"a formally verified checker sidecar validates each output", false},
		{"the sidecar's formally verified checker validates each record", false},
		{"the sidecar's formally verified checker validates any decision", false},
		{"the sidecar's formally verified checker validates any record", false},
		{"the proof-receipt sidecar checker-validates the decisions", false},
		{"the proof-receipt sidecar checker-validates the outputs", false},
		{"a formally verified checker sidecar validates some decisions", false},
		{"a formally verified checker sidecar validates most records", false},
		// Re-review: novel broad subjects co-mentioned with a scope word still reject.
		{"the audit log is validated by a formally verified checker, see the sidecar", false},
		{"the policy decision is checker-validated; details in the proof receipt", false},
		{"boundary is validated by a formally verified checker — see the sidecar", false},
		// Re-review: quantifiers scoped to the sidecar's OWN invariants are correct and MUST
		// pass (guard against the matrix over-rejecting honest copy).
		{"each invariant in the proof-receipt sidecar is checker-validated", true},
		{"all three invariants in the proof-receipt sidecar are checker-validated", true},
	}
	for _, tc := range cases {
		if got := proofReceiptScoped(strings.ToLower(tc.line)); got != tc.allow {
			t.Fatalf("proofReceiptScoped(%q) = %v, want %v", tc.line, got, tc.allow)
		}
	}
}
