package madriver

import (
	"regexp"
	"strings"
	"sync"
)

// secretPatterns are the shapes the driver removes from every string it
// writes to any file, log line, transcript, or error path. The list mirrors
// (and is intentionally broader than) the patterns the conformance harness
// scans for: Anthropic-style API keys, generic sk- tokens, bearer tokens,
// key/value secret assignments, session-secret assignments, and email
// addresses.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)(x-api-key|api[_-]?key|api[_-]?secret|authorization|session[_-]?secret|secret[_-]?token|access[_-]?token|password)["'\s]*[:=]["'\s]*[^\s"',}]{6,}`),
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
}

// secretLiterals holds exact secret values that must be scrubbed even when
// they match no pattern. The driver registers the live upstream key once at
// run start; tests register fake values. Registered values live for the life
// of the process — the driver is a single-run CLI, so there is no unregister.
var secretLiterals = struct {
	sync.Mutex
	values []string
}{}

// registerSecretValue adds an exact value to the scrub set. RedactString then
// removes the value itself plus its first and last 8 characters wherever they
// appear, so a key that matches no regex — or a leaked fragment of one —
// still cannot reach an output file.
func registerSecretValue(value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	secretLiterals.Lock()
	defer secretLiterals.Unlock()
	secretLiterals.values = append(secretLiterals.values, value)
}

// literalFragments returns the substrings of value that must never appear in
// output: the value itself, plus its first and last 8 characters as fragments
// when the value is long enough for them to be distinct.
func literalFragments(value string) []string {
	frags := []string{value}
	if len(value) > 16 {
		frags = append(frags, value[:8], value[len(value)-8:])
	}
	return frags
}

// scrubLiterals replaces every registered literal and fragment in s.
func scrubLiterals(s string) string {
	secretLiterals.Lock()
	defer secretLiterals.Unlock()
	out := s
	for _, value := range secretLiterals.values {
		for _, frag := range literalFragments(value) {
			out = strings.ReplaceAll(out, frag, redactedPlaceholder)
		}
	}
	return out
}

// hasLiteral reports whether s contains a registered literal or fragment.
func hasLiteral(s string) bool {
	secretLiterals.Lock()
	defer secretLiterals.Unlock()
	for _, value := range secretLiterals.values {
		for _, frag := range literalFragments(value) {
			if strings.Contains(s, frag) {
				return true
			}
		}
	}
	return false
}

const redactedPlaceholder = "[REDACTED]"

// RedactString removes every secret-shaped substring from s, including every
// registered exact literal and its edge fragments.
func RedactString(s string) string {
	out := scrubLiterals(s)
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, redactedPlaceholder)
	}
	return out
}

// LooksSecret reports whether s contains any secret-shaped substring or a
// registered literal. It is the predicate behind the output-directory scan;
// it never returns the match, only that one exists, so callers cannot
// accidentally propagate it.
func LooksSecret(s string) bool {
	if hasLiteral(s) {
		return true
	}
	for _, pattern := range secretPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}
