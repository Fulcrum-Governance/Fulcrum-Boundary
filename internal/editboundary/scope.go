package editboundary

import (
	"path"
	"strings"
	"unicode"
)

type PathCheck struct {
	Path   string
	Safe   bool
	Reason string
}

func CheckProjectPath(raw string) PathCheck {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "/dev/null" {
		return PathCheck{Path: trimmed, Safe: true}
	}
	if hasControlChar(trimmed) {
		return PathCheck{Path: RedactPath(trimmed), Safe: false, Reason: "path contains control character"}
	}
	if strings.Contains(trimmed, `\`) {
		return PathCheck{Path: RedactPath(trimmed), Safe: false, Reason: "path uses backslash or Windows traversal form"}
	}
	if isWindowsDrivePath(trimmed) || strings.HasPrefix(trimmed, "//") {
		return PathCheck{Path: RedactPath(trimmed), Safe: false, Reason: "path is outside project scope"}
	}
	if strings.HasPrefix(trimmed, "/") {
		return PathCheck{Path: RedactPath(trimmed), Safe: false, Reason: "absolute path is outside project scope"}
	}

	cleaned := path.Clean(strings.TrimPrefix(trimmed, "./"))
	if cleaned == "." {
		return PathCheck{Path: cleaned, Safe: true}
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return PathCheck{Path: RedactPath(cleaned), Safe: false, Reason: "parent-directory traversal is outside project scope"}
	}
	parts := strings.Split(cleaned, "/")
	for _, part := range parts {
		if part == ".." {
			return PathCheck{Path: RedactPath(cleaned), Safe: false, Reason: "parent-directory traversal is outside project scope"}
		}
	}
	if len(parts) > 0 && parts[0] == ".git" {
		return PathCheck{Path: RedactPath(cleaned), Safe: false, Reason: ".git mutation is outside the edit envelope scope"}
	}
	return PathCheck{Path: cleaned, Safe: true}
}

func hasControlChar(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func isWindowsDrivePath(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	r := value[0]
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
