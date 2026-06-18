package claims

import (
	"path/filepath"
	"strings"
	"testing"
)

// mustAbs returns the repo root relative to the claims package directory.
// loadLedger/readFile already exist in claims_test.go; this is the only net-new
// helper (there is no existing exported repo-root helper in the claims package).
func mustAbs(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("abs repo root: %v", err)
	}
	return abs
}

// TestC2WiredWitnessLanguageIsLedgerBacked pins the C2 relabel sentences to
// ledger allowed entries. The relabel re-points "coupled to enforcement" onto
// the wired witness (budget/static-privilege) + circuit-transition (termination)
// + machine-checked equilibrium analysis (Nash/PoA). Each must be ledger-backed.
func TestC2WiredWitnessLanguageIsLedgerBacked(t *testing.T) {
	ledger := loadLedger(t, mustAbs(t))
	var allowed []string
	for _, c := range ledger.Claims {
		allowed = append(allowed, c.PublicLanguage.Allowed...)
	}
	joined := strings.ToLower(strings.Join(allowed, "\n"))
	for _, frag := range []string{
		"checker-validated proof receipt",
		"budget and static-privilege",
		"circuit transition",
		"machine-checked equilibrium analysis",
	} {
		if !strings.Contains(joined, frag) {
			t.Errorf("C2 relabel fragment %q is not backed by any ledger allowed line", frag)
		}
	}
}

// TestPublicSurfaceUsesC2Relabel asserts README uses the relabel and that the
// old "couples a trust equilibrium to enforcement as a runtime certificate"
// framing stays out of public copy (it remains a forbidden ledger entry, never
// an allowed claim). The README edit lands in WS-4.3; this body is skipped here
// and the skip is removed in WS-4.3 step 1 (the documented red->green handoff).
func TestPublicSurfaceUsesC2Relabel(t *testing.T) {
	root := mustAbs(t)
	readme := strings.ToLower(readFile(t, filepath.Join(root, "README.md")))
	if !strings.Contains(readme, "machine-checked equilibrium analysis") {
		t.Error("README must use the 'machine-checked equilibrium analysis' relabel")
	}
	if strings.Contains(readme, "couples a trust equilibrium to enforcement as a runtime certificate") {
		t.Error("README must not assert the trust-equilibrium-coupled-to-enforcement runtime certificate")
	}
}

// TestReleaseTruthCarriesC2Relabel asserts that RELEASE_TRUTH_PUBLIC.md carries
// the wired-witness and machine-checked equilibrium analysis relabel paragraphs
// added in WS-4.4.
func TestReleaseTruthCarriesC2Relabel(t *testing.T) {
	text := strings.ToLower(readFile(t, filepath.Join(mustAbs(t), "docs/RELEASE_TRUTH_PUBLIC.md")))
	if !strings.Contains(text, "machine-checked equilibrium analysis") {
		t.Error("RELEASE_TRUTH_PUBLIC must carry the equilibrium-analysis relabel")
	}
	if !strings.Contains(text, "wired witness") {
		t.Error("RELEASE_TRUTH_PUBLIC must carry the wired-witness relabel")
	}
}
