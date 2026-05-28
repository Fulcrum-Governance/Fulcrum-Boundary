package securegithub

import (
	"regexp"
	"strings"
)

var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)authorization:\s*[^\s]+`),
	regexp.MustCompile(`ghp_[A-Za-z0-9_]+`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`ghs_[A-Za-z0-9_]+`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(token|api[_-]?key|password|secret)=["']?[^"'\s]+`),
}

func redactCredentialText(text string) string {
	redacted := text
	for _, pattern := range credentialPatterns {
		redacted = pattern.ReplaceAllString(redacted, "[REDACTED]")
	}
	return redacted
}

func containsSecretLikeData(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "private key") || strings.Contains(lower, "authorization") {
		return true
	}
	for _, pattern := range credentialPatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
