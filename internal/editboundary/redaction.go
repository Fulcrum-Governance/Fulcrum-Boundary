package editboundary

import (
	"regexp"
	"strings"
)

const redactedValue = "[redacted]"
const redactedSecretPath = "[redacted-secret-path]"

var secretContentPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)\bAuthorization:\s*Bearer\s+\S+`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{12,}`),
	regexp.MustCompile(`(?i)\b(api[_-]?key|password|token|secret)\s*=\s*[^ \t]+`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9_]{12,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{12,}`),
	regexp.MustCompile(`(?i)https?://[^/\s:]+:[^@\s]+@`),
}

func RedactPath(raw string) string {
	cleaned := strings.TrimPrefix(strings.TrimSpace(raw), "@")
	if IsSecretPath(cleaned) {
		return redactedSecretPath
	}
	return cleaned
}

func IsSecretPath(raw string) bool {
	lower := strings.ToLower(strings.Trim(raw, `"'`))
	lower = strings.TrimPrefix(lower, "./")
	if lower == ".env" || strings.HasPrefix(lower, ".env.") || strings.Contains(lower, "/.env") {
		return true
	}
	if strings.Contains(lower, ".ssh/") || strings.Contains(lower, "id_rsa") || strings.Contains(lower, "id_ed25519") {
		return true
	}
	for _, marker := range []string{".npmrc", ".pypirc", "credentials", "secret", "secrets", "token"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func ContainsSecretContent(lines []string) bool {
	for _, line := range lines {
		for _, pattern := range secretContentPatterns {
			if pattern.MatchString(line) {
				return true
			}
		}
	}
	return false
}
