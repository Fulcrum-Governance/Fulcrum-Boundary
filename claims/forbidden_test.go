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
// lists and the hardcoded Gate-2 lint rules (publicLanguageRules). A small
// number of forbidden phrases are ALSO literal lint terms; this test pins that
// overlap. If the "adapter maturity overclaim" rule term is removed or renamed
// while the ledger still forbids the phrase (or the ledger phrase is dropped
// while the rule keeps it), the two surfaces decouple and this test fails,
// naming the drift. When the overlap changes intentionally, update wantWired and
// confirm a live lint rule still enforces the phrase.
func TestForbiddenLintSync(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	ledger := loadLedger(t, repoRoot)

	lintTerms := map[string]bool{}
	for _, r := range publicLanguageRules() {
		for _, term := range r.terms {
			lintTerms[strings.ToLower(term)] = true
		}
	}

	got := map[string]bool{}
	for _, c := range ledger.Claims {
		for _, f := range c.PublicLanguage.Forbidden {
			norm := strings.ToLower(strings.TrimSpace(f))
			if lintTerms[norm] {
				got[norm] = true
			}
		}
	}

	// The current ledger ↔ lint overlap, pinned. Verified against the real
	// ledger: exactly the two adapter-maturity phrases double as lint terms.
	wantWired := map[string]bool{
		"six production adapters":   true,
		"seven production adapters": true,
	}
	for phrase := range wantWired {
		if !got[phrase] {
			t.Fatalf("forbidden phrase %q must stay wired to a publicLanguageRules() term, but the ledger no longer forbids it (ledger/lint drift)", phrase)
		}
	}
	for phrase := range got {
		if !wantWired[phrase] {
			t.Fatalf("forbidden phrase %q newly overlaps a publicLanguageRules() term but is not pinned; if intended, add it to wantWired", phrase)
		}
	}
}
