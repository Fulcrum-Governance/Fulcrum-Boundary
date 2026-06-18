package demo_test

import (
	"os"
	"strings"
	"testing"
)

// TestDemoDocEnumeratesEvidencePackAndStaysPreview gates the WS-3 doc: it must
// describe the evidence pack (all spec artifacts named), must never claim
// production Secure GitHub / hash-chained / default-signed receipts, and any
// proof-receipt/checker sentence must be ledger-backed (the sidecar phrase from
// the WS-1f claim must already exist in claims/boundary_claims.yaml).
func TestDemoDocEnumeratesEvidencePackAndStaysPreview(t *testing.T) {
	body, err := os.ReadFile("../../docs/DEMO_GITHUB_LETHAL_TRIFECTA.md")
	if err != nil {
		t.Fatalf("read demo doc: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"--evidence-pack", "pack.json", "proof-receipt.json", "decision-record.json",
		"route-conformance.json", "tamper-cases.json", "caveats.md", "route-topology", "L0", "preview",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("demo doc missing required evidence-pack reference %q", want)
		}
	}
	lower := strings.ToLower(text)
	for _, forbidden := range []string{"production secure github", "hash-chained record", "signed receipt by default"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("demo doc contains forbidden production language %q", forbidden)
		}
	}
	// Gap D: the doc's proof-receipt/checker framing must be ledger-backed.
	// The sidecar phrase the WS-1f claim (BND-CLAIM-PROOF-002) introduced must
	// already exist in the ledger before this doc can use checker language.
	ledger, err := os.ReadFile("../../claims/boundary_claims.yaml")
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if strings.Contains(lower, "checker-validated proof receipt") &&
		!strings.Contains(string(ledger), "checker-validated proof receipt") {
		t.Fatal("doc uses 'checker-validated proof receipt' but the ledger does not back it (WS-1f must land first)")
	}
}
