package hookboundary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

func TestRouteForMapsToolNamesToBoundarySurfaces(t *testing.T) {
	cases := []struct {
		tool string
		want Route
	}{
		{"Bash", RouteCommand},
		{"bash", RouteCommand},
		{"Shell", RouteCommand},
		{"shell", RouteCommand},
		{"Edit", RouteEdit},
		{"Write", RouteEdit},
		{"MultiEdit", RouteEdit},
		{"NotebookEdit", RouteEdit},
		{"Read", RouteNone},
		{"Grep", RouteNone},
		{"Glob", RouteNone},
		{"WebFetch", RouteNone},
		{"mcp__github__create_issue", RouteNone},
		{"", RouteNone},
		{"BASH", RouteNone},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			if got := RouteFor(tc.tool); got != tc.want {
				t.Fatalf("RouteFor(%q) = %q, want %q", tc.tool, got, tc.want)
			}
		})
	}
}

func TestClassifyBashLineGovernsTheWholeLine(t *testing.T) {
	cases := []struct {
		name         string
		line         string
		wantClass    commandboundary.Class
		wantAction   commandboundary.RecommendedAction
		wantSegments int
	}{
		{"observe", "git status", commandboundary.ClassObserveRead, commandboundary.ActionAllow, 1},
		{"destructive", "rm -rf dist", commandboundary.ClassDestructiveMutation, commandboundary.ActionDeny, 1},
		{"repo mutation", "git push origin main", commandboundary.ClassRepositoryMutation, commandboundary.ActionRequireApproval, 1},
		{"local write", "touch notes.txt", commandboundary.ClassLocalFileWrite, commandboundary.ActionWarn, 1},
		// The shell hook's leading-command gap, now closed: the destructive
		// tail is decomposed and sets the aggregate verdict.
		{"and-chained tail", "git status && rm -rf /",
			commandboundary.ClassDestructiveMutation, commandboundary.ActionDeny, 2},
		{"semicolon-chained tail", "echo hi; rm -rf ~",
			commandboundary.ClassDestructiveMutation, commandboundary.ActionDeny, 2},
		{"command substitution", "echo $(rm -rf ~)",
			commandboundary.ClassDestructiveMutation, commandboundary.ActionDeny, 2},
		{"shell -c payload", "sh -c 'rm -rf /'",
			commandboundary.ClassDestructiveMutation, commandboundary.ActionDeny, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified, err := ClassifyBashLine(tc.line)
			if err != nil {
				t.Fatalf("ClassifyBashLine(%q): %v", tc.line, err)
			}
			if !classified.Parseable {
				t.Fatalf("line %q was not decomposable", tc.line)
			}
			if classified.Aggregate.Class != tc.wantClass {
				t.Fatalf("class = %q, want %q", classified.Aggregate.Class, tc.wantClass)
			}
			if classified.Aggregate.RecommendedAction != tc.wantAction {
				t.Fatalf("action = %q, want %q", classified.Aggregate.RecommendedAction, tc.wantAction)
			}
			if len(classified.Segments) != tc.wantSegments {
				t.Fatalf("segments = %d, want %d: %#v", len(classified.Segments), tc.wantSegments, classified.Segments)
			}
		})
	}
}

// A line the decomposer cannot model must escalate, never allow: that is the
// only posture the hook may take about a line it did not classify.
func TestClassifyBashLineEscalatesAnUndecomposableLine(t *testing.T) {
	for _, line := range []string{
		"cat <<EOF",
		"eval \"$PAYLOAD\"",
		"diff <(ls) <(ls -a)",
		"echo 'unterminated",
	} {
		t.Run(line, func(t *testing.T) {
			classified, err := ClassifyBashLine(line)
			if err != nil {
				t.Fatalf("ClassifyBashLine(%q): %v", line, err)
			}
			if classified.Parseable {
				t.Fatalf("line %q reported as fully decomposed", line)
			}
			if classified.Aggregate.RecommendedAction == commandboundary.ActionAllow ||
				classified.Aggregate.RecommendedAction == commandboundary.ActionWarn {
				t.Fatalf("undecomposable line %q resolved to %q", line, classified.Aggregate.RecommendedAction)
			}
			if !strings.Contains(classified.Aggregate.Reason, commandboundary.ReasonUndecomposable) {
				t.Fatalf("reason %q does not say the line was undecomposable", classified.Aggregate.Reason)
			}
		})
	}
}

func TestClassifyBashLineRejectsAnEmptyLine(t *testing.T) {
	for _, line := range []string{"", "   ", "\t\n"} {
		if _, err := ClassifyBashLine(line); err == nil {
			t.Fatalf("ClassifyBashLine(%q) = nil error, want no-command error", line)
		}
	}
}

func TestSynthesizeEditPatchNamesTheProjectRelativeTarget(t *testing.T) {
	got := string(SynthesizeEditPatch("/repo/config/.env", "/repo"))
	want := "diff --git a/config/.env b/config/.env\n" +
		"--- a/config/.env\n" +
		"+++ b/config/.env\n"
	if got != want {
		t.Fatalf("patch =\n%q\nwant\n%q", got, want)
	}
}

// An absolute target that is not under the project root must stay absolute in
// the synthesized diff, because that is what makes Edit Boundary classify it as
// an escape from the project rather than as a repo-relative tail.
func TestSynthesizeEditPatchKeepsAnOutsideTargetAbsolute(t *testing.T) {
	got := string(SynthesizeEditPatch("/etc/passwd", "/repo"))
	if !strings.Contains(got, "a//etc/passwd") {
		t.Fatalf("patch does not keep the absolute target:\n%q", got)
	}
}

// One directory can be spelled two ways — macOS serves /var through a symlink
// into /private — and a textual comparison would read the two spellings as two
// places and deny every edit in the project.
func TestProjectRelativeEditPathAcceptsAnAliasedProjectRoot(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	target := filepath.Join(alias, "src", "main.go")
	if got := ProjectRelativeEditPath(target, root); got != "src/main.go" {
		t.Fatalf("ProjectRelativeEditPath(%q, %q) = %q, want src/main.go", target, root, got)
	}
}

// A symlink inside the project that points outside it lands the write outside
// the project, so it must be judged where the write actually goes.
func TestProjectRelativeEditPathFollowsASymlinkOutOfTheProject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := ProjectRelativeEditPath(filepath.Join(link, "secrets.txt"), root)
	if !filepath.IsAbs(got) {
		t.Fatalf("path through an escaping symlink = %q, want it left absolute so it denies", got)
	}
}

func TestProjectRelativeEditPathResolvesAgainstTheProjectRoot(t *testing.T) {
	cases := []struct {
		name string
		path string
		root string
		want string
	}{
		{"inside the root", "/repo/src/main.go", "/repo", "src/main.go"},
		{"the root itself", "/repo", "/repo", "."},
		{"dotted component survives", "/repo/.git/hooks/pre-commit", "/repo", ".git/hooks/pre-commit"},
		{"trailing slash on the root", "/repo/src/main.go", "/repo/", "src/main.go"},
		{"outside the root stays absolute", "/etc/passwd", "/repo", "/etc/passwd"},
		{"sibling outside the root stays absolute", "/other/x.go", "/repo", "/other/x.go"},
		{"prefix lookalike stays absolute", "/repository/x.go", "/repo", "/repository/x.go"},
		{"no root leaves it absolute", "/etc/passwd", "", "/etc/passwd"},
		{"relative root leaves it absolute", "/etc/passwd", "repo", "/etc/passwd"},
		{"relative path is unchanged", "src/main.go", "/repo", "src/main.go"},
		{"empty path is unchanged", "", "/repo", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProjectRelativeEditPath(tc.path, tc.root); got != tc.want {
				t.Fatalf("ProjectRelativeEditPath(%q, %q) = %q, want %q", tc.path, tc.root, got, tc.want)
			}
		})
	}
}

func TestInspectEditPathClassifiesByPathShape(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantClass  editboundary.Class
		wantAction editboundary.RecommendedAction
	}{
		{"secret bearing", "config/.env", editboundary.ClassSecretBearing, editboundary.ActionDeny},
		{"ssh key", ".ssh/id_ed25519", editboundary.ClassSecretBearing, editboundary.ActionDeny},
		{"source", "internal/hookboundary/decide.go", editboundary.ClassSourceConfig, editboundary.ActionRequireApproval},
		{"workflow", ".github/workflows/ci.yml", editboundary.ClassExecutionBehavior, editboundary.ActionRequireApproval},
		{"doc", "docs/README.md", editboundary.ClassSafeContent, editboundary.ActionAllow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inspection, patch, err := InspectEditPath(tc.path, "/repo")
			if err != nil {
				t.Fatalf("InspectEditPath(%q): %v", tc.path, err)
			}
			if inspection.HighestClass != tc.wantClass {
				t.Fatalf("class = %q, want %q", inspection.HighestClass, tc.wantClass)
			}
			if inspection.RecommendedAction != tc.wantAction {
				t.Fatalf("action = %q, want %q", inspection.RecommendedAction, tc.wantAction)
			}
			if !strings.Contains(string(patch), "diff --git ") {
				t.Fatalf("patch is not a git diff header: %q", patch)
			}
		})
	}
}

// Claude Code's edit tools pass ABSOLUTE paths, so a write outside the project
// root is the ordinary shape of "escape from the project", not an edge case. It
// must classify E7 and deny — the case that used to be reclassified as a
// repo-relative tail and silently allowed.
func TestInspectEditPathDeniesAnAbsoluteTargetOutsideTheProject(t *testing.T) {
	cases := []struct {
		name string
		path string
		root string
	}{
		{"system file", "/etc/passwd", "/repo"},
		{"the governing binary", "/usr/local/bin/boundary", "/repo"},
		{"a shell profile", "/home/me/.zshrc", "/repo"},
		{"unknown project root", "/etc/passwd", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inspection, _, err := InspectEditPath(tc.path, tc.root)
			if err != nil {
				t.Fatalf("InspectEditPath(%q, %q): %v", tc.path, tc.root, err)
			}
			if inspection.HighestClass != editboundary.ClassOutsideProjectScope {
				t.Fatalf("class = %q, want %q", inspection.HighestClass, editboundary.ClassOutsideProjectScope)
			}
			if inspection.RecommendedAction != editboundary.ActionDeny {
				t.Fatalf("action = %q, want deny", inspection.RecommendedAction)
			}
		})
	}
}

// An absolute path INSIDE the project must keep the class its position in the
// project earns — including the ones that depend on the leading component, which
// the old leading-slash strip destroyed.
func TestInspectEditPathClassifiesAnAbsoluteTargetInsideTheProject(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		wantClass editboundary.Class
	}{
		{"git hook", "/repo/.git/hooks/pre-commit", editboundary.ClassOutsideProjectScope},
		{"agent settings", "/repo/.claude/settings.json", editboundary.ClassOutsideProjectScope},
		{"secret", "/repo/config/.env", editboundary.ClassSecretBearing},
		{"source", "/repo/internal/hookboundary/decide.go", editboundary.ClassSourceConfig},
		{"doc", "/repo/docs/README.md", editboundary.ClassSafeContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inspection, _, err := InspectEditPath(tc.path, "/repo")
			if err != nil {
				t.Fatalf("InspectEditPath(%q): %v", tc.path, err)
			}
			if inspection.HighestClass != tc.wantClass {
				t.Fatalf("class = %q, want %q", inspection.HighestClass, tc.wantClass)
			}
		})
	}
}
