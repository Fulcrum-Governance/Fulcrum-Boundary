package claims

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCLIBypassEvidenceScope pins the claim-scope wording the
// cli-reference-v1 bypass evidence relies on, in the two authored surfaces
// that state it:
//
//  1. The topology Dockerfile's protection-model comment must scope its
//     execve claim to the probed routes of cli-reference-v1. The earlier
//     phrasing ("a process can execve the tool only as a child of a governed
//     `boundary command run`") was a universal claim contradicted by the
//     governed find -exec → shell forwarding tracked as FUL-653.
//  2. The deployment doc must disclose that the governed find -exec shape
//     can forward to a shell holding boundary_exec, and must not describe
//     the governed route as shell-free.
func TestCLIBypassEvidenceScope(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}

	dockerfile := readFile(t, filepath.Join(repoRoot, "tests/bypass/cli-reference/Dockerfile"))
	if strings.Contains(dockerfile, "execve the tool only as a child") {
		t.Fatal("cli-reference Dockerfile states an unscoped universal execve claim; scope it to the probed routes of cli-reference-v1 (see FUL-653)")
	}
	if !strings.Contains(dockerfile, "only probed route") {
		t.Fatal("cli-reference Dockerfile lost its probed-route scoping for the execve claim")
	}

	doc := readFile(t, filepath.Join(repoRoot, "docs/deployment/cli-bypass-proofing.md"))
	if !strings.Contains(doc, "FUL-653") {
		t.Fatal("cli-bypass-proofing.md must disclose that a governed `find -exec` can forward to a shell holding boundary_exec (tracked as FUL-653)")
	}
	for i, line := range strings.Split(doc, "\n") {
		if lower := strings.ToLower(line); strings.Contains(lower, "shell-free") && !negatedOrControlled(lower) {
			t.Fatalf("cli-bypass-proofing.md:%d describes the governed route as shell-free: %s", i+1, strings.TrimSpace(line))
		}
	}
}
