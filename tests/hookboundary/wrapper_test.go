package hookboundary_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// wrapperPath is the shipped POSIX hook entrypoint under test.
func wrapperPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("pretooluse-boundary.sh is a POSIX sh entrypoint")
	}
	path := filepath.Join(repoRoot, "integrations", "claude-code", "pretooluse-boundary.sh")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hook wrapper is missing: %v", err)
	}
	return path
}

// runWrapper executes the wrapper with an explicit environment and the event on
// stdin, exactly as Claude Code invokes it.
func runWrapper(t *testing.T, env []string, event string) (int, string, string) {
	t.Helper()
	cmd := exec.Command("sh", wrapperPath(t))
	cmd.Stdin = strings.NewReader(event)
	cmd.Dir = t.TempDir()
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run wrapper: %v\nstderr=%s", err, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

// TestWrapperSyntaxIsPOSIX is the `sh -n` gate: the wrapper must parse under a
// plain POSIX shell, not just under the developer's login shell.
func TestWrapperSyntaxIsPOSIX(t *testing.T) {
	output, err := exec.Command("sh", "-n", wrapperPath(t)).CombinedOutput()
	if err != nil {
		t.Fatalf("sh -n failed: %v\n%s", err, string(output))
	}
	if len(bytes.TrimSpace(output)) != 0 {
		t.Fatalf("sh -n produced output: %s", string(output))
	}
}

// TestWrapperUsesNoJSONParser pins the thin-wrapper property: the script must
// not reimplement any part of the decision, so a missing or differently
// versioned jq can never change a verdict.
func TestWrapperUsesNoJSONParser(t *testing.T) {
	source := wrapperCode(t)
	for _, banned := range []string{"jq", "python", "perl", "sed", "awk", "grep"} {
		if strings.Contains(source, banned) {
			t.Fatalf("wrapper code reintroduced a parser (%q); all parsing belongs to the binary:\n%s",
				banned, source)
		}
	}
	if !strings.Contains(source, `exec "$BOUNDARY_BIN" hook pretooluse`) {
		t.Fatalf("wrapper does not exec `boundary hook pretooluse`:\n%s", source)
	}
}

// wrapperCode returns the wrapper with comment-only lines removed, so a test
// about what the script DOES is not confused by prose about what it does not do.
func wrapperCode(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(wrapperPath(t))
	if err != nil {
		t.Fatalf("read wrapper: %v", err)
	}
	var code []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		code = append(code, line)
	}
	return strings.Join(code, "\n")
}

// TestWrapperAsksWhenBoundaryIsNotInstalled is BOU-5's headline behavior. Claude
// Code treats a hook that cannot run as a hook error and lets the tool call
// proceed ungoverned; the wrapper deliberately overrides that by answering with
// a valid "ask" decision and exit 0, so the operator sees the ungoverned state.
func TestWrapperAsksWhenBoundaryIsNotInstalled(t *testing.T) {
	// A PATH with no `boundary` on it, and no BOUNDARY_BIN override.
	env := []string{"PATH=" + emptyToolPath(t), "HOME=" + t.TempDir()}
	code, stdout, stderr := runWrapper(t, env,
		`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; a non-zero exit is read as a hook error and fails open\nstderr=%s",
			code, stderr)
	}
	decision := decodeDecision(t, stdout)
	if decision.HookSpecificOutput.PermissionDecision != "ask" {
		t.Fatalf("permissionDecision = %q, want ask\n%s",
			decision.HookSpecificOutput.PermissionDecision, stdout)
	}
	reason := decision.reason()
	for _, want := range []string{
		"not installed",
		"not governed",
		"brew install fulcrum-governance/tap/boundary",
		"docs/install.md",
		"boundary_bin",
	} {
		if !strings.Contains(strings.ToLower(reason), want) {
			t.Fatalf("uninstalled reason %q does not carry %q", reason, want)
		}
	}
	// The event must not be echoed back: the wrapper answers with fixed text.
	if strings.Contains(stdout, "rm -rf /") {
		t.Fatalf("wrapper echoed the event into its decision:\n%s", stdout)
	}
}

// TestWrapperAsksWhenBoundaryBinPointsNowhere covers the other missing-binary
// shape: BOUNDARY_BIN set to a path that does not exist.
func TestWrapperAsksWhenBoundaryBinPointsNowhere(t *testing.T) {
	env := []string{
		"PATH=" + emptyToolPath(t),
		"BOUNDARY_BIN=" + filepath.Join(t.TempDir(), "no-such-boundary"),
	}
	code, stdout, _ := runWrapper(t, env, `{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got := decodeDecision(t, stdout).HookSpecificOutput.PermissionDecision; got != "ask" {
		t.Fatalf("permissionDecision = %q, want ask", got)
	}
}

// TestWrapperAsksWhenTheBinaryPredatesTheHookLane covers the install shape the
// existence probe alone cannot see: a `boundary` that IS on PATH but was built
// before `hook pretooluse` existed. It exits non-zero with empty stdout, which
// Claude Code reads as a hook error and lets the tool call through ungoverned and
// silently — the exact condition the ask-on-missing-binary branch exists to
// prevent. Realistic trigger: the hook script is installed or updated from this
// repo while an older brew- or tarball-installed binary is still on PATH.
func TestTheWrapperAsksWhenTheBinaryPredatesTheHookLane(t *testing.T) {
	stubDir := t.TempDir()
	stub := filepath.Join(stubDir, "boundary")
	// Mirrors internal/boundarycli/cli.go's unknown-command path.
	body := "#!/bin/sh\nprintf 'unknown command \"%s\"\\n' \"$1\" >&2\nexit 1\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	env := []string{
		"PATH=" + stubDir + string(os.PathListSeparator) + emptyToolPath(t),
		"HOME=" + t.TempDir(),
	}
	code, stdout, stderr := runWrapper(t, env,
		`{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`)

	if code != 0 {
		t.Fatalf("exit = %d, want 0; a non-zero exit is read as a hook error and fails open\nstderr=%s",
			code, stderr)
	}
	decision := decodeDecision(t, stdout)
	if decision.HookSpecificOutput.PermissionDecision != "ask" {
		t.Fatalf("permissionDecision = %q, want ask\n%s",
			decision.HookSpecificOutput.PermissionDecision, stdout)
	}
	reason := strings.ToLower(decision.reason())
	for _, want := range []string{"not govern", "too old", "hook pretooluse", "boundary_bin"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("too-old reason %q does not carry %q", reason, want)
		}
	}
	// The wrapper answers with fixed text; the event must not be echoed back.
	if strings.Contains(stdout, "rm -rf /") {
		t.Fatalf("wrapper echoed the event into its decision:\n%s", stdout)
	}
}

// TestWrapperDelegatesToTheInstalledBinary is the round trip: with the real
// binary on PATH the wrapper must hand over cleanly, so a deny reaches Claude
// Code as a deny and an allow stays silent.
func TestWrapperDelegatesToTheInstalledBinary(t *testing.T) {
	cases := []struct {
		name           string
		command        string
		wantPermission string
		wantAction     string
	}{
		{"destructive command denies", "rm -rf /", "deny", "deny"},
		{"observe command allows silently", "git status", "", "allow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordDir := t.TempDir()
			env := []string{
				// The binary is reached through PATH, exercising `command -v`.
				"PATH=" + filepath.Dir(boundaryBin) + string(os.PathListSeparator) + emptyToolPath(t),
				"BOUNDARY_HOOK_DIR=" + recordDir,
				"HOME=" + t.TempDir(),
			}
			code, stdout, stderr := runWrapper(t, env,
				`{"tool_name":"Bash","tool_input":{"command":"`+tc.command+`"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr)
			}
			if tc.wantPermission == "" {
				if strings.TrimSpace(stdout) != "" {
					t.Fatalf("stdout = %q, want a silent allow", stdout)
				}
			} else {
				decision := decodeDecision(t, stdout)
				if decision.HookSpecificOutput.PermissionDecision != tc.wantPermission {
					t.Fatalf("permissionDecision = %q, want %q\n%s",
						decision.HookSpecificOutput.PermissionDecision, tc.wantPermission, stdout)
				}
				if decision.Decision != "block" {
					t.Fatalf("legacy decision key = %q, want block", decision.Decision)
				}
			}
			// Delegation is real only if the binary actually decided and recorded.
			assertOneVerifiableRecord(t, recordDir, tc.wantAction)
		})
	}
}

// TestWrapperHonorsBoundaryBin covers the documented override for a binary that
// is not on PATH.
func TestWrapperHonorsBoundaryBin(t *testing.T) {
	recordDir := t.TempDir()
	env := []string{
		"PATH=" + emptyToolPath(t),
		"BOUNDARY_BIN=" + boundaryBin,
		"BOUNDARY_HOOK_DIR=" + recordDir,
	}
	code, stdout, stderr := runWrapper(t, env,
		`{"tool_name":"Write","tool_input":{"file_path":".claude/settings.json"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr)
	}
	if got := decodeDecision(t, stdout).HookSpecificOutput.PermissionDecision; got != "deny" {
		t.Fatalf("permissionDecision = %q, want deny", got)
	}
	assertOneVerifiableRecord(t, recordDir, "deny")
}

// TestWrapperDebugStaysOnStderr keeps stdout parseable: Claude Code reads stdout
// as JSON, so diagnostics must never land there.
func TestWrapperDebugStaysOnStderr(t *testing.T) {
	env := []string{"PATH=" + emptyToolPath(t), "BOUNDARY_HOOK_DEBUG=1"}
	code, stdout, stderr := runWrapper(t, env, `{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if got := decodeDecision(t, stdout).HookSpecificOutput.PermissionDecision; got != "ask" {
		t.Fatalf("permissionDecision = %q, want ask", got)
	}
	if !strings.Contains(stderr, "boundary-hook:") {
		t.Fatalf("BOUNDARY_HOOK_DEBUG produced no stderr diagnostics: %q", stderr)
	}
}

// TestWrapperDebugReachesTheBinaryAcrossExec pins the passthrough: the wrapper
// hands its whole environment over, so debug output comes from both sides and a
// silent allow still leaves stdout empty.
func TestWrapperDebugReachesTheBinaryAcrossExec(t *testing.T) {
	recordDir := t.TempDir()
	env := []string{
		"PATH=" + emptyToolPath(t),
		"BOUNDARY_BIN=" + boundaryBin,
		"BOUNDARY_HOOK_DIR=" + recordDir,
		"BOUNDARY_HOOK_DEBUG=1",
	}
	code, stdout, stderr := runWrapper(t, env, `{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want a silent allow even under debug", stdout)
	}
	// The wrapper's own line, then the binary's — proving the knob survived exec.
	if !strings.Contains(stderr, "boundary-hook: delegating") {
		t.Fatalf("wrapper emitted no delegation diagnostic: %q", stderr)
	}
	if !strings.Contains(stderr, "boundary hook: route=") {
		t.Fatalf("BOUNDARY_HOOK_DEBUG did not reach the binary: %q", stderr)
	}
	assertOneVerifiableRecord(t, recordDir, "allow")
}

// emptyToolPath returns a PATH entry pointing at an empty directory, so the
// wrapper's `command -v` lookup finds nothing.
func emptyToolPath(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
