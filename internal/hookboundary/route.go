package hookboundary

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

// Route names the Boundary surface a PreToolUse tool call is routed to.
type Route string

const (
	// RouteNone means the tool is not governed by this hook. It is allowed
	// silently and leaves no decision record: nothing was decided.
	RouteNone Route = ""
	// RouteCommand routes the call to Command Boundary (preview).
	RouteCommand Route = "command"
	// RouteEdit routes the call to Edit Boundary (preview).
	RouteEdit Route = "edit"
)

// RouteFor maps a Claude Code tool name to the Boundary surface that classifies
// it. The mapping is exactly the shell hook's: shell tools go to Command
// Boundary, the four file-mutation tools go to Edit Boundary, and every other
// tool — Read, Grep, Glob, web tools, MCP tools — is RouteNone. MCP tool calls
// are governed at the MCP route, not here.
func RouteFor(toolName string) Route {
	switch strings.TrimSpace(toolName) {
	case "Bash", "bash", "Shell", "shell":
		return RouteCommand
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return RouteEdit
	default:
		return RouteNone
	}
}

// ClassifyBashLine classifies a Bash tool_input command line through Command
// Boundary's compound-line decomposer.
//
// The WHOLE line is classified, not just its leading command. The line is split
// into simple commands on the shell operators (&&, ||, ;, |, &, newline), and
// command substitution, subshells, and `sh -c` payloads are decomposed
// recursively; every resulting segment is classified and the caller enforces
// LineClassification.Aggregate, the most restrictive segment. So a destructive
// command chained, substituted, or smuggled after a benign one is classified
// rather than skipped — the shell hook's leading-command gap is closed on this
// route.
//
// A construct the decomposer does not model — a heredoc, process substitution,
// `eval`, unbalanced quotes, or nesting past commandboundary.MaxLineDepth —
// comes back with Parseable false and an aggregate floored at require_approval,
// so an undecomposable line is escalated to the user and never allowed. Nothing
// on this path invokes a shell or executes any part of the line.
//
// It returns an error when the line is empty.
func ClassifyBashLine(line string) (commandboundary.LineClassification, error) {
	return commandboundary.ClassifyLine(line)
}

// maxCanonicalWalk bounds how many ancestors canonicalPath climbs looking for a
// directory the filesystem can resolve. It is a guard against a pathological
// path, not a limit any real project path reaches.
const maxCanonicalWalk = 64

// ProjectRelativeEditPath maps a Claude Code edit target onto the path Edit
// Boundary classifies, given the project root the hook process is governing.
//
// Claude Code's Edit, Write, MultiEdit, and NotebookEdit tools pass ABSOLUTE
// paths, so this — not the relative case — is the normal path through the edit
// route, and how it is resolved decides whether "outside project scope" means
// anything here:
//
//   - A path inside root is returned relative to root, so an ordinary edit is
//     classified by its position in the project (`/home/me/proj/src/main.go` ->
//     `src/main.go`), and a project-relative marker like a leading `.git` or
//     `.claude` component is still the first component the classifier sees.
//   - A path outside root, and any absolute path when root is empty or
//     unusable, is returned UNCHANGED and stays absolute, which Edit Boundary
//     classifies E7 "absolute path is outside project scope". Failing to
//     relativize therefore denies rather than allows.
//   - A relative path is returned unchanged: it is already what the classifier
//     expects.
//
// Both paths are canonicalized first (see canonicalPath), so a symlink inside
// the project that points outside it is caught rather than admitted, and the two
// spellings of one directory do not read as two places. That is real filesystem
// access, and it is a point-in-time answer: a symlink swapped between this check
// and Claude Code's write is not caught, the same TOCTOU every pre-execution
// boundary carries. On a case-insensitive filesystem a case-flipped root is
// still not matched, because case is not folded here.
func ProjectRelativeEditPath(filePath, root string) string {
	target := strings.TrimSpace(filePath)
	if target == "" || !filepath.IsAbs(target) {
		return target
	}
	root = strings.TrimSpace(root)
	if root == "" || !filepath.IsAbs(root) {
		return target
	}
	rel, err := filepath.Rel(canonicalPath(root), canonicalPath(target))
	if err != nil {
		return target
	}
	slashed := filepath.ToSlash(rel)
	if slashed == ".." || strings.HasPrefix(slashed, "../") {
		return target
	}
	return slashed
}

// canonicalPath resolves path's symlinks as far as the filesystem can: the
// deepest ancestor that exists is resolved, and the components below it are
// appended unchanged. A path whose ancestors resolve nowhere comes back cleaned
// but otherwise untouched, so this is safe on a path that does not exist yet —
// which an edit target usually is.
//
// It exists for two reasons. One directory can be spelled two ways: macOS serves
// /tmp and /var through symlinks into /private, so os.Getwd reports
// /private/var/… for a directory a tool calls /var/… , and comparing the
// spellings textually would report every edit as outside the project and deny
// it. And a symlink INSIDE the project that points outside it — `proj/link` ->
// `/etc` — resolves to where the write actually lands, so `proj/link/passwd` is
// judged as /etc/passwd rather than as a path under the project.
func canonicalPath(path string) string {
	cleaned := filepath.Clean(path)
	rest := ""
	for i := 0; i < maxCanonicalWalk; i++ {
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cleaned)
		if parent == cleaned {
			break
		}
		rest = filepath.Join(filepath.Base(cleaned), rest)
		cleaned = parent
	}
	return filepath.Clean(path)
}

// SynthesizeEditPatch builds the one-file diff Edit Boundary inspects for a
// Claude Code edit tool call: a git-style header naming the target path with no
// content hunk. The path is first resolved against root by
// ProjectRelativeEditPath, so an edit outside the project stays absolute and
// classifies as an escape from the project root.
//
// One consequence is deliberate, not an oversight: only the path SHAPE is
// classified, so content-based edit classes are not asserted by this route.
//
// What this route reliably denies is a write to a secret-bearing path (E4), a
// write outside the project root (E7), and a write to a governance control
// surface — see editboundary.ControlSurfacePaths — which also classifies E7.
func SynthesizeEditPatch(filePath, root string) []byte {
	target := ProjectRelativeEditPath(filePath, root)
	return fmt.Appendf(nil, "diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n", target, target, target, target)
}

// InspectEditPath classifies a Claude Code edit tool call by its target path,
// through Edit Boundary's patch inspector. It synthesizes the one-file diff with
// SynthesizeEditPatch, resolving filePath against the project root, and returns
// the inspection alongside the patch bytes that produced it, so the caller can
// bind the request to the exact patch inspected.
//
// It never reads, writes, or applies anything to the target file.
func InspectEditPath(filePath, root string) (editboundary.Inspection, []byte, error) {
	patch := SynthesizeEditPatch(filePath, root)
	inspection, err := editboundary.InspectPatch(patch)
	if err != nil {
		return editboundary.Inspection{}, patch, err
	}
	return inspection, patch, nil
}
