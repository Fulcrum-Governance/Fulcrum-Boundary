package claims

import (
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
	}

	seen := map[string]bool{}
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		for _, match := range matches {
			rel, err := filepath.Rel(repoRoot, match)
			if err != nil {
				t.Fatalf("rel path for %s: %v", match, err)
			}
			rel = filepath.ToSlash(rel)
			if seen[rel] {
				continue
			}
			seen[rel] = true
			paths = append(paths, rel)
		}
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
		"docs/LAUNCH_TRUTH_FREEZE.md",
		"docs/RELEASE_TRUTH_RECONCILIATION.md":
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
