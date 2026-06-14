package claims

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestForbiddenListIntegrity gives the ledger's per-claim
// public_language.forbidden lists real machine teeth at the ledger level.
//
// The forbidden lists are ADVISORY (see docs/CLAIMS_LEDGER.md "Public-language
// lists" and docs/BOUNDARY_SPEC.md §7.1): each entry records a capability
// framing the claim must never assert. They are governed in public copy by the
// hardcoded Gate-2 rules (publicLanguageRules) plus human review — NOT by
// literal substring enforcement against documents, because concept words such
// as "signature" and "decision hashes" appear legitimately in honest, hedged
// copy (enforcing them literally would brick truthful text). This test holds the
// lists to the integrity invariants that ARE safe to enforce mechanically: every
// entry is non-empty, no entry is duplicated within a claim, and no entry
// contradicts that same claim's allowed list. It fails the build on any
// violation, so the lists can no longer rot silently.
func TestForbiddenListIntegrity(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	ledger := loadLedger(t, repoRoot)

	for _, c := range ledger.Claims {
		allowed := map[string]bool{}
		for _, a := range c.PublicLanguage.Allowed {
			allowed[strings.ToLower(strings.TrimSpace(a))] = true
		}
		seen := map[string]bool{}
		for _, f := range c.PublicLanguage.Forbidden {
			norm := strings.ToLower(strings.TrimSpace(f))
			if norm == "" {
				t.Fatalf("%s has an empty public_language.forbidden entry", c.ID)
			}
			if seen[norm] {
				t.Fatalf("%s lists forbidden phrase %q more than once", c.ID, f)
			}
			seen[norm] = true
			if allowed[norm] {
				t.Fatalf("%s lists %q as both allowed and forbidden", c.ID, f)
			}
		}
	}
}

// TestForbiddenLintSync couples the two truth surfaces that govern public
// language so they cannot silently drift apart: the ledger's advisory forbidden
// lists and the hardcoded Gate-2 lint rules (publicLanguageRules). A forbidden
// phrase is "lint-governed" when some lint term is a SUBSTRING of it — that is
// exactly how TestPublicLanguageLint matches (strings.Contains), so any doc line
// carrying the forbidden phrase also carries the lint term and is caught (e.g.
// the forbidden "Boundary is a SQL firewall" is governed by the shorter lint
// term "SQL firewall"). This test pins the set of currently lint-governed
// forbidden phrases using those same substring semantics. If a lint rule term is
// removed or narrowed so one of these phrases is no longer covered, it drops out
// of the computed set and this test fails naming it — catching the silent
// ledger↔lint decoupling that an exact-equality check would miss. When the
// governed set changes intentionally, update wantWired after confirming a live
// rule still covers each phrase.
func TestForbiddenLintSync(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	ledger := loadLedger(t, repoRoot)

	var lintTerms []string
	for _, r := range publicLanguageRules() {
		for _, term := range r.terms {
			lintTerms = append(lintTerms, strings.ToLower(term))
		}
	}
	// governedByLint mirrors the lint's own substring matching.
	governedByLint := func(phrase string) bool {
		for _, term := range lintTerms {
			if strings.Contains(phrase, term) {
				return true
			}
		}
		return false
	}

	got := map[string]bool{}
	for _, c := range ledger.Claims {
		for _, f := range c.PublicLanguage.Forbidden {
			norm := strings.ToLower(strings.TrimSpace(f))
			if governedByLint(norm) {
				got[norm] = true
			}
		}
	}

	// The forbidden framings currently machine-governed by the language lint
	// (via substring), pinned. Verified against the real ledger and
	// publicLanguageRules().
	wantWired := map[string]bool{
		"boundary emits proved decisions":            true,
		"boundary fully secures github":              true,
		"boundary guarantees universal agent safety": true,
		"boundary is a sql firewall":                 true,
		"boundary is standards-conformant":           true,
		"boundary prevents all sql injection":        true,
		"six production adapters":                    true,
		"seven production adapters":                  true,
	}
	for phrase := range wantWired {
		if !got[phrase] {
			t.Fatalf("forbidden phrase %q must stay governed by a publicLanguageRules() term (via substring), but no live lint term covers it now (ledger/lint drift)", phrase)
		}
	}
	for phrase := range got {
		if !wantWired[phrase] {
			t.Fatalf("forbidden phrase %q is now lint-governed but is not pinned; if intended, add it to wantWired", phrase)
		}
	}
}
