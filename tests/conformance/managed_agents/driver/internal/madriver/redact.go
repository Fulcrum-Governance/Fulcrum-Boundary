package madriver

import "regexp"

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

const redactedPlaceholder = "[REDACTED]"

// RedactString removes every secret-shaped substring from s.
func RedactString(s string) string {
	out := s
	for _, pattern := range secretPatterns {
		out = pattern.ReplaceAllString(out, redactedPlaceholder)
	}
	return out
}

// LooksSecret reports whether s contains any secret-shaped substring. It is
// the predicate behind the output-directory scan; it never returns the match,
// only that one exists, so callers cannot accidentally propagate it.
func LooksSecret(s string) bool {
	for _, pattern := range secretPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}
