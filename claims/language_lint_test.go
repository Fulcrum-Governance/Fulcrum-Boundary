package claims

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

type languageRule struct {
	name         string
	terms        []string
	headlineOnly bool
	allowLine    func(string) bool
}

func TestPublicLanguageLint(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	for _, rel := range languageLintPaths(t, repoRoot) {
		if languageControlFile(rel) {
			continue
		}
		text := readFile(t, filepath.Join(repoRoot, rel))
		lines := strings.Split(text, "\n")
		for lineNo, line := range lines {
			trimmed := strings.TrimSpace(line)
			lower := strings.ToLower(trimmed)
			for _, rule := range publicLanguageRules() {
				if rule.headlineOnly && !strings.HasPrefix(trimmed, "#") {
					continue
				}
				for _, term := range rule.terms {
					if !strings.Contains(lower, strings.ToLower(term)) {
						continue
					}
					if rule.allowLine != nil && rule.allowLine(lower) {
						continue
					}
					t.Fatalf("%s:%d contains controlled language %q (%s): %s", rel, lineNo+1, term, rule.name, trimmed)
				}
			}
		}
	}
}

func languageLintPaths(t *testing.T, repoRoot string) []string {
	t.Helper()
	patterns := []string{
		"README.md",
		"CHANGELOG.md",
		"docs/*.md",
		"docs/adapters/*.md",
		"docs/firewall/*.md",
		"docs/secure-mcp/*.md",
		"docs/policies/*.md",
		"docs/deployment/*.md",
		"docs/command-boundary/*.md",
		"docs/edit-boundary/*.md",
		"docs/releases/*.md",
		"verifiers/*/README.md",
	}

	seen := map[string]bool{}
	var paths []string
	add := func(match string) {
		rel, err := filepath.Rel(repoRoot, match)
		if err != nil {
			t.Fatalf("rel path for %s: %v", match, err)
		}
		rel = filepath.ToSlash(rel)
		if seen[rel] {
			return
		}
		seen[rel] = true
		paths = append(paths, rel)
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, match := range matches {
			add(match)
		}
	}
	// filepath.Glob has no recursive `**` (it matches like a single `*`), so the
	// docs-site tree is walked explicitly to lint nested pages at any depth.
	if err := filepath.WalkDir(filepath.Join(repoRoot, "docs-site"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			add(path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk docs-site: %v", err)
	}
	return paths
}

func publicLanguageRules() []languageRule {
	return []languageRule{
		{
			name:         "generic platform lead",
			terms:        []string{"AI governance platform"},
			headlineOnly: true,
		},
		{
			name:      "SQL firewall overclaim",
			terms:     []string{"SQL firewall", "prevents all SQL injection"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "universal prompt-injection overclaim",
			terms:     []string{"prevents all prompt injection", "universal prompt-injection prevention"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "universal agent safety overclaim",
			terms:     []string{"universal agent safety"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "runtime proof overclaim",
			terms:     []string{"proved decision", "proved decisions"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "secure sandbox overclaim",
			terms:     []string{"secure sandbox", "secure sandboxing"},
			allowLine: sandboxCaveat,
		},
		{
			name:      "adapter maturity overclaim",
			terms:     []string{"all adapters production", "six production adapters", "seven production adapters"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "unverified competitive claim",
			terms:     []string{"no other tool does this", "no one else detects this"},
			allowLine: negatedOrControlled,
		},
		{
			name:      "GitHub production overclaim",
			terms:     []string{"fully secures GitHub", "production GitHub security", "detects every malicious issue"},
			allowLine: negatedOrControlled,
		},
		{
			// The decision record's canonicalization is RFC 8785/JCS, so a
			// claim conformant to that standard is true when SCOPED to the
			// record. A blanket "Boundary is standards-conformant" is not.
			// This rule permits the scoped, earned claim and the negated
			// disclaimer, and rejects the blanket overclaim.
			name:      "standards conformance overclaim",
			terms:     []string{"standards-conformant", "standards conformant", "rfc 8785 conformant", "rfc8785 conformant", "jcs conformant", "jcs-conformant", "fully conformant"},
			allowLine: conformanceScoped,
		},
		{
			// Only a Go binary and a Python verifier ship today. The format is
			// reproducible by any RFC 8785 implementation, but Boundary does not
			// provide a verifier in every language.
			name:      "any-language verification overclaim",
			terms:     []string{"verify in any language", "verifier in any language", "verify a record in any language", "verify records in any language", "verify any record in any language"},
			allowLine: negatedOrControlled,
		},
	}
}

func languageControlFile(rel string) bool {
	switch rel {
	case "docs/CLAIMS_LEDGER.md",
		"docs/COPY_RULES.md",
		"docs/LANGUAGE_SYSTEM.md",
		"docs/LEXICON.md",
		"docs/BOUNDARY_PRODUCT_PRIMITIVES.md",
		"docs/BOUNDARY_SPEC.md",
		"docs/internal/LAUNCH_TRUTH_FREEZE.md",
		"docs/internal/RELEASE_TRUTH_RECONCILIATION.md":
		return true
	default:
		return false
	}
}

func negatedOrControlled(line string) bool {
	allowed := []string{
		"not ",
		"do not ",
		"does not ",
		"must not ",
		"avoid ",
		"false",
		"forbidden",
		"prohibited",
		"without ",
		"unless ",
		"until ",
	}
	for _, term := range allowed {
		if strings.Contains(line, term) {
			return true
		}
	}
	return false
}

func sandboxCaveat(line string) bool {
	if negatedOrControlled(line) {
		return true
	}
	allowed := []string{
		"when implemented",
		"when actually provided",
		"may be described",
		"real, named, tested",
	}
	for _, term := range allowed {
		if strings.Contains(line, term) {
			return true
		}
	}
	return false
}

// conformanceScoped permits a standards-conformance claim only when it is
// negated/limitation-framed or scoped to the decision record (the surface that
// is actually RFC 8785/JCS conformant). A blanket whole-product conformance
// claim has neither and fails.
func conformanceScoped(line string) bool {
	if negatedOrControlled(line) {
		return true
	}
	scoped := []string{
		"decision record",
		"decision-record",
		"decision hash",
		"decision_hash",
		"the record",
		"record's",
		"record is rfc",
	}
	for _, term := range scoped {
		if strings.Contains(line, term) {
			return true
		}
	}
	return false
}

// TestConformanceLintRuleSemantics pins the closing honesty gate: the blanket
// standards-conformance overclaim is rejected, while the earned record-scoped
// claim and the negated disclaimer pass.
func TestConformanceLintRuleSemantics(t *testing.T) {
	cases := []struct {
		line  string
		allow bool
	}{
		{"boundary is standards-conformant", false},
		{"boundary is fully conformant with every standard", false},
		{"the decision record is rfc 8785 / jcs conformant", true},
		{"a record's decision_hash is jcs conformant and reproducible", true},
		{"it is not a claim that boundary as a whole is standards-conformant", true},
	}
	for _, tc := range cases {
		if got := conformanceScoped(strings.ToLower(tc.line)); got != tc.allow {
			t.Fatalf("conformanceScoped(%q) = %v, want %v", tc.line, got, tc.allow)
		}
	}
}
