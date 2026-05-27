package commandboundary

import (
	"path/filepath"
	"strings"
)

const redactedValue = "[redacted]"

var sensitiveValueFlags = map[string]bool{
	"--token":    true,
	"--api-key":  true,
	"--password": true,
}

func RedactArgs(args []string) []string {
	redacted := make([]string, 0, len(args))
	redactNext := false
	for _, arg := range args {
		if redactNext {
			redacted = append(redacted, redactedValue)
			redactNext = false
			continue
		}

		lower := strings.ToLower(arg)
		if sensitiveValueFlags[lower] || lower == "authorization" || lower == "bearer" {
			redacted = append(redacted, arg)
			redactNext = true
			continue
		}

		if key, value, ok := strings.Cut(arg, "="); ok && isSensitiveFlag(strings.ToLower(key)) && value != "" {
			redacted = append(redacted, key+"="+redactedValue)
			continue
		}

		if isSensitiveArg(arg) {
			redacted = append(redacted, redactedValue)
			continue
		}

		redacted = append(redacted, arg)
	}
	return redacted
}

func isSensitiveFlag(flag string) bool {
	return sensitiveValueFlags[flag]
}

func isSensitiveArg(arg string) bool {
	lower := strings.ToLower(arg)
	if strings.Contains(lower, "authorization:") || strings.HasPrefix(lower, "bearer ") {
		return true
	}
	if strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "api_key") {
		return true
	}
	return isSecretPath(arg)
}

func isSecretPath(arg string) bool {
	cleaned := strings.TrimPrefix(arg, "@")
	cleaned = strings.Trim(cleaned, `"'`)
	lower := strings.ToLower(filepath.ToSlash(cleaned))
	if lower == ".env" || strings.HasSuffix(lower, "/.env") || strings.Contains(lower, "/.env.") || strings.HasPrefix(lower, ".env.") {
		return true
	}
	if strings.Contains(lower, ".ssh/id_") {
		return true
	}
	return false
}
