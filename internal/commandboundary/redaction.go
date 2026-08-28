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

		if cleaned, ok := redactURLCredentials(arg); ok {
			redacted = append(redacted, cleaned)
			continue
		}

		redacted = append(redacted, arg)
	}
	return redacted
}

// redactURLCredentials rewrites the userinfo of a `scheme://user:password@host`
// URL, the canonical shape of a secret pasted onto a command line: a database
// DSN, a `git remote` with a token, a `curl` URL with basic auth.
//
// The password is replaced and the user name kept
// (`postgres://admin:[redacted]@db/prod`), so an operator can still tell which
// account was named. Userinfo with no colon is a bare token rather than a user
// name, so the whole of it is replaced (`https://[redacted]@github.com/o/r`).
//
// It reports false when arg carries no such credentials, which includes scp-style
// `user@host:path` remotes (no `://`) and an `@` that appears only in the path.
// Matching is by URL shape and is not a guarantee that no secret-looking value
// survives in some other argument position.
func redactURLCredentials(arg string) (string, bool) {
	marker := strings.Index(arg, "://")
	if marker < 0 {
		return arg, false
	}
	start := marker + len("://")
	rest := arg[start:]
	authority := rest
	if end := strings.IndexAny(rest, "/?#"); end >= 0 {
		authority = rest[:end]
	}
	at := strings.LastIndex(authority, "@")
	if at <= 0 {
		return arg, false
	}
	userinfo := authority[:at]
	if name, _, ok := strings.Cut(userinfo, ":"); ok && name != "" {
		return arg[:start] + name + ":" + redactedValue + arg[start+at:], true
	}
	return arg[:start] + redactedValue + arg[start+at:], true
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
