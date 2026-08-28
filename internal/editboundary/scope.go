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
	if reason, ok := controlSurfaceReason(parts); ok {
		return PathCheck{Path: RedactPath(cleaned), Safe: false, Reason: reason}
	}
	return PathCheck{Path: cleaned, Safe: true}
}

// ControlSurfacePaths returns the path shapes CheckProjectPath treats as
// governance control surfaces and rejects as ClassOutsideProjectScope.
//
// It is the human-readable statement of the rules controlSurfaceReason
// implements, for documentation and operator-facing output. It is a list of
// SHAPES, not an inventory of every way governance could be disabled: a hook
// wired from a path not listed here is not matched.
func ControlSurfacePaths() []string {
	return []string{
		".boundary/**",
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/**",
		"**/claude-code/pretooluse-boundary.sh",
	}
}

// ControlSurfacePath reports whether raw names a governance control surface —
// see ControlSurfacePaths for the shapes and controlSurfaceReason for why they
// are closed at the classifier — and the reason to record when it does.
//
// It answers the same question CheckProjectPath answers for a patch, for callers
// that hold a bare path rather than a diff: the command route needs it so a write
// to `.claude/settings.json` is refused whether it arrives as an Edit tool call
// or as `cp evil .claude/settings.json`. Only the control-surface rule is
// applied; absolute paths, `..` traversal, and `.git` are CheckProjectPath's
// business and are not reported here.
//
// The path is matched by shape, with no filesystem access: it is not resolved,
// symlinks are not followed, and a component list is compared case-insensitively
// at any position, so a repo-relative, absolute, or vendored copy matches alike.
// Both `/` and `\` separate components, because a backslash form naming the same
// file must not slip past the check.
func ControlSurfacePath(raw string) (reason string, ok bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	cleaned := path.Clean(strings.ReplaceAll(trimmed, `\`, "/"))
	return controlSurfaceReason(strings.Split(strings.TrimPrefix(cleaned, "/"), "/"))
}

// controlSurfaceReason reports whether the cleaned path components name a
// control surface — a file that decides HOW an agent is governed, or that holds
// the evidence of what was governed — and the reason to record when they do.
//
// These paths sit inside the working tree but outside the edit ENVELOPE, the
// same distinction `.git` already draws above: a patch may edit the code a
// project builds, but not rewrite the gate the patch itself passed through, and
// not rewrite the record of that passage. An agent that can edit its own
// PreToolUse hook configuration can turn the hook off before its next tool call,
// so this is closed at the classifier rather than left to policy.
//
// Matching is deliberately narrow and shape-based, and matches at ANY position
// in the path so a repo-relative, absolute-tail, or vendored copy is treated
// alike:
//
//   - any `.boundary` directory component — decision records, evidence bundles,
//     and the command shims all live there;
//   - a `.claude` component followed by `settings.json`, `settings.local.json`,
//     or anything under `hooks/` — the rest of `.claude/` (skills, commands,
//     docs) stays ordinary editable content;
//   - a `claude-code/pretooluse-boundary.sh` suffix — the shell hook itself.
//
// Comparison is case-insensitive: on a case-insensitive filesystem
// `.Claude/Settings.json` names the same file, so a case flip must not slip past
// the check.
//
// It is not an inventory of every way an agent could disable governance: a hook
// wired from a path not listed here, or a settings file outside these shapes, is
// not matched. See ControlSurfacePaths.
func controlSurfaceReason(parts []string) (string, bool) {
	lower := make([]string, len(parts))
	for i, part := range parts {
		lower[i] = strings.ToLower(part)
	}
	for i, part := range lower {
		switch part {
		case ".boundary":
			return "Boundary control and evidence path is outside the edit envelope scope", true
		case ".claude":
			rest := lower[i+1:]
			if len(rest) == 1 && (rest[0] == "settings.json" || rest[0] == "settings.local.json") {
				return "agent permission settings path is outside the edit envelope scope", true
			}
			if len(rest) > 1 && rest[0] == "hooks" {
				return "agent hook path is outside the edit envelope scope", true
			}
		case "claude-code":
			if i+1 == len(lower)-1 && lower[i+1] == "pretooluse-boundary.sh" {
				return "Boundary hook script path is outside the edit envelope scope", true
			}
		}
	}
	return "", false
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
