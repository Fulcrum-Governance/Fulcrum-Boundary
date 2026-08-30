package hookboundary_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the regression wall around the second BOU-13 defect: split
// effective-binary resolution. In founder Run A against dbb9070, the installer
// preflight and the installed hook wrapper resolved `${BOUNDARY_BIN:-boundary}`
// — the candidate — while /boundary:drill's own commands invoked bare
// `boundary` and resolved an incompatible v0.12.0 on PATH. The drill then
// reported the decision path unavailable while the candidate hook was actively
// recording the drill's own commands as C0 allows.
//
// The contract under test: every shipped skill invokes Boundary through the
// exact spelling `"${BOUNDARY_BIN:-boundary}"` — BOUNDARY_BIN when set, else
// `boundary` on PATH — the same resolution pretooluse-boundary.sh execs and
// scripts/install-claude-code.sh --plugin-drop validates. These tests EXECUTE
// the shipped command forms under a split PATH/BOUNDARY_BIN environment and
// assert which binary actually ran; they do not count source strings.

// effectiveSpelling is the one effective-binary invocation form the shipped
// integration uses. It must match integrations/claude-code/pretooluse-boundary.sh
// and the recognition constant in internal/commandboundary/classifier.go.
const effectiveSpelling = `"${BOUNDARY_BIN:-boundary}"`

// writeLoggingStub writes a POSIX-sh fake boundary into its own directory and
// returns that directory plus the log file the stub appends every invocation's
// arguments to. compatible=false mirrors the founder's v0.12.0: `version`
// self-reports, every other verb fails with `unknown command`. compatible=true
// accepts everything, drains piped stdin with shell builtins only, and exits 0.
func writeLoggingStub(t *testing.T, compatible bool) (dir, logPath string) {
	t.Helper()
	dir = t.TempDir()
	logPath = filepath.Join(dir, "invocations.log")
	var body string
	if compatible {
		body = "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >>'" + logPath + "'\n" +
			"if [ ! -t 0 ]; then while IFS= read -r _line; do :; done; fi\n" +
			"exit 0\n"
	} else {
		body = "#!/bin/sh\n" +
			"printf '%s\\n' \"$*\" >>'" + logPath + "'\n" +
			"case \"$1\" in\n" +
			"version) printf 'Fulcrum Boundary v0.12.0 (stub)\\n'; exit 0 ;;\n" +
			"*) printf 'unknown command \"%s\"\\n' \"$1\" >&2; exit 1 ;;\n" +
			"esac\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "boundary"), []byte(body), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	return dir, logPath
}

// loggedInvocations returns the stub's log lines, or nil when the stub was
// never executed.
func loggedInvocations(t *testing.T, logPath string) []string {
	t.Helper()
	body, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read stub log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(body)), "\n")
}

// runShellForm runs one shipped command form the way the Bash tool runs it: a
// POSIX shell -c line, in dir, under exactly env.
func runShellForm(t *testing.T, dir string, env []string, form string) (int, string, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", form)
	cmd.Dir = dir
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run %q: %v\nstderr=%s", form, err, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

// skillSource reads one shipped skill's instructions.
func skillSource(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot, "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatalf("read skill %s: %v", name, err)
	}
	return string(body)
}

// skillForm is one Boundary invocation a shipped skill instructs, in the form
// the skill ships it. anchors pins the exact text to the SKILL.md files that
// carry it, so the executed form and the shipped instruction cannot drift
// apart; run is the executable line (anchor placeholders like `<file>`
// replaced with a concrete argument); wantArgv is the argument line the
// effective binary must log when the form executes.
type skillForm struct {
	name     string
	anchors  map[string]string
	run      string
	wantArgv string
}

func shippedSkillForms() []skillForm {
	const drillProbe = `printf '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | "${BOUNDARY_BIN:-boundary}" hook pretooluse`
	const drillStage = `printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf .boundary-drill/vault"}}' | "${BOUNDARY_BIN:-boundary}" hook pretooluse --print-record`
	return []skillForm{
		{
			name: "version",
			anchors: map[string]string{
				"drill":   effectiveSpelling + ` version`,
				"protect": effectiveSpelling + ` version`,
			},
			run:      effectiveSpelling + ` version`,
			wantArgv: "version",
		},
		{
			name:     "hook doctor --json",
			anchors:  map[string]string{"drill": effectiveSpelling + ` hook doctor --json`},
			run:      effectiveSpelling + ` hook doctor --json`,
			wantArgv: "hook doctor --json",
		},
		{
			name:     "hook doctor",
			anchors:  map[string]string{"protect": effectiveSpelling + ` hook doctor`},
			run:      effectiveSpelling + ` hook doctor`,
			wantArgv: "hook doctor",
		},
		{
			name:     "hook --help",
			anchors:  map[string]string{"drill": effectiveSpelling + ` hook --help`},
			run:      effectiveSpelling + ` hook --help`,
			wantArgv: "hook --help",
		},
		{
			name:     "piped hook pretooluse probe",
			anchors:  map[string]string{"drill": drillProbe},
			run:      drillProbe,
			wantArgv: "hook pretooluse",
		},
		{
			name:     "piped hook pretooluse --print-record staging",
			anchors:  map[string]string{"drill": drillStage},
			run:      drillStage,
			wantArgv: "hook pretooluse --print-record",
		},
		{
			name: "verify-record",
			anchors: map[string]string{
				"drill":  effectiveSpelling + ` verify-record`,
				"report": effectiveSpelling + ` verify-record`,
				"verify": effectiveSpelling + ` verify-record`,
			},
			run:      effectiveSpelling + ` verify-record r.json`,
			wantArgv: "verify-record r.json",
		},
		{
			name:     "verify-record --json",
			anchors:  map[string]string{"verify": effectiveSpelling + ` verify-record --json`},
			run:      effectiveSpelling + ` verify-record --json r.json`,
			wantArgv: "verify-record --json r.json",
		},
		{
			name: "explain",
			anchors: map[string]string{
				"drill":  effectiveSpelling + ` explain`,
				"verify": effectiveSpelling + ` explain`,
			},
			run:      effectiveSpelling + ` explain r.json`,
			wantArgv: "explain r.json",
		},
		{
			name:     "drill cleanup",
			anchors:  map[string]string{"drill": effectiveSpelling + ` drill cleanup`},
			run:      effectiveSpelling + ` drill cleanup`,
			wantArgv: "drill cleanup",
		},
	}
}

// TestSkillFormsExecuteTheEffectiveBinaryNotPATH is the functional
// split-binary regression: PATH resolves an incompatible fake v0.12.0 while
// BOUNDARY_BIN names a compatible candidate — the founder's Run A environment
// — and every shipped skill command form must execute the candidate and leave
// the PATH binary untouched. Each form is first pinned to the SKILL.md text
// that ships it, so this cannot silently pass against forms the skills no
// longer instruct.
func TestSkillFormsExecuteTheEffectiveBinaryNotPATH(t *testing.T) {
	skipUnlessPOSIXShell(t)
	for _, form := range shippedSkillForms() {
		t.Run(form.name, func(t *testing.T) {
			for skill, anchor := range form.anchors {
				if !strings.Contains(skillSource(t, skill), anchor) {
					t.Fatalf("skills/%s/SKILL.md no longer carries the shipped form %q; update this test's form table in the same change", skill, anchor)
				}
			}
			pathDir, pathLog := writeLoggingStub(t, false)
			candidateDir, candidateLog := writeLoggingStub(t, true)
			env := []string{
				"PATH=" + pathDir,
				"HOME=" + t.TempDir(),
				"BOUNDARY_BIN=" + filepath.Join(candidateDir, "boundary"),
			}
			code, stdout, stderr := runShellForm(t, t.TempDir(), env, form.run)
			if code != 0 {
				t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
			}
			got := loggedInvocations(t, candidateLog)
			if len(got) != 1 || got[0] != form.wantArgv {
				t.Fatalf("candidate invocations = %q, want exactly [%q]", got, form.wantArgv)
			}
			if leaked := loggedInvocations(t, pathLog); leaked != nil {
				t.Fatalf("the PATH binary was executed (%q); the shipped form must resolve BOUNDARY_BIN first", leaked)
			}
		})
	}
}

// TestEffectiveFormFallsBackToPATHOnlyWhenBoundaryBinIsUnset pins the other
// half of the resolution order: with no BOUNDARY_BIN in the environment, the
// same shipped spelling reaches `boundary` on PATH.
func TestEffectiveFormFallsBackToPATHOnlyWhenBoundaryBinIsUnset(t *testing.T) {
	skipUnlessPOSIXShell(t)
	pathDir, pathLog := writeLoggingStub(t, true)
	env := []string{
		"PATH=" + pathDir,
		"HOME=" + t.TempDir(),
	}
	code, stdout, stderr := runShellForm(t, t.TempDir(), env, effectiveSpelling+` version`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if got := loggedInvocations(t, pathLog); len(got) != 1 || got[0] != "version" {
		t.Fatalf("PATH invocations = %q, want exactly [\"version\"]", got)
	}
}

// TestInvalidBoundaryBinFailsWithoutFallingBackToPATH pins the cautious
// failure mode: an explicit override that names nothing executable must fail
// the command, and must NOT silently execute a different binary from PATH —
// even a compatible one sitting right there.
func TestInvalidBoundaryBinFailsWithoutFallingBackToPATH(t *testing.T) {
	skipUnlessPOSIXShell(t)
	pathDir, pathLog := writeLoggingStub(t, true)
	env := []string{
		"PATH=" + pathDir,
		"HOME=" + t.TempDir(),
		"BOUNDARY_BIN=" + filepath.Join(t.TempDir(), "no-such-boundary"),
	}
	code, stdout, _ := runShellForm(t, t.TempDir(), env, effectiveSpelling+` version`)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero: a broken override must fail, not succeed\nstdout=%s", stdout)
	}
	if leaked := loggedInvocations(t, pathLog); leaked != nil {
		t.Fatalf("a broken BOUNDARY_BIN silently fell back to the PATH binary (%q)", leaked)
	}
}

// TestSplitBinaryDrillStagingReachesTheRealCandidate closes the loop with the
// real binary in the founder's exact split: PATH still resolves the
// incompatible fake v0.12.0, BOUNDARY_BIN names the real candidate TestMain
// built, and the drill's own step-3 staging line — run exactly as the skill
// ships it — must produce the candidate's C4 deny plus a verifiable record,
// with the PATH binary never executed.
func TestSplitBinaryDrillStagingReachesTheRealCandidate(t *testing.T) {
	skipUnlessPOSIXShell(t)
	const drillStage = `printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf .boundary-drill/vault"}}' | "${BOUNDARY_BIN:-boundary}" hook pretooluse --print-record`
	if !strings.Contains(skillSource(t, "drill"), drillStage) {
		t.Fatalf("skills/drill/SKILL.md no longer carries the staged line this test runs; update both together")
	}
	pathDir, pathLog := writeLoggingStub(t, false)
	project := t.TempDir()
	env := []string{
		"PATH=" + pathDir,
		"HOME=" + t.TempDir(),
		"BOUNDARY_BIN=" + boundaryBin,
		"BOUNDARY_HOOK_DIR=" + project,
	}
	code, stdout, stderr := runShellForm(t, project, env, drillStage)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout lines = %d, want the decision plus the record pointer\n%s", len(lines), stdout)
	}
	decision := decodeDecision(t, lines[0])
	if decision.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision = %q, want deny\n%s", decision.HookSpecificOutput.PermissionDecision, stdout)
	}
	if !strings.Contains(lines[1], "boundary.hook.record-pointer.v1") {
		t.Fatalf("second line is not the record pointer: %s", lines[1])
	}
	assertOneVerifiableRecord(t, project, "deny")
	if leaked := loggedInvocations(t, pathLog); leaked != nil {
		t.Fatalf("the incompatible PATH binary was executed (%q) instead of the candidate", leaked)
	}
}

// TestSkillsShipNoBareBoundaryInvocations is the structural guard — in
// addition to, never instead of, the functional executions above. Every
// fenced command block in every shipped skill, and every "Run `boundary …`"
// step instruction, must invoke Boundary through the effective spelling. Prose
// that NAMES a verb (`boundary verify-record` recomputes …) stays bare on
// purpose, as does the receipt text /boundary:report writes for a third party
// on another machine — those are not commands this plugin's skills execute.
func TestSkillsShipNoBareBoundaryInvocations(t *testing.T) {
	skillDirs, err := os.ReadDir(filepath.Join(repoRoot, "skills"))
	if err != nil {
		t.Fatalf("read skills/: %v", err)
	}
	for _, entry := range skillDirs {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			source := skillSource(t, name)
			inFence := false
			for i, line := range strings.Split(source, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "```") {
					inFence = !inFence
					continue
				}
				if inFence {
					for _, stage := range strings.Split(trimmed, "|") {
						fields := strings.Fields(stage)
						if len(fields) > 0 && fields[0] == "boundary" {
							t.Errorf("skills/%s/SKILL.md line %d invokes bare `boundary` in a command block; use %s so BOUNDARY_BIN resolves: %s",
								name, i+1, effectiveSpelling, trimmed)
						}
					}
					continue
				}
				if strings.Contains(line, "Run `boundary ") {
					t.Errorf("skills/%s/SKILL.md line %d instructs a bare `boundary` invocation; use %s: %s",
						name, i+1, effectiveSpelling, trimmed)
				}
			}
		})
	}
}

// skipUnlessPOSIXShell skips the sh-driven functional tests on hosts with no
// POSIX shell, mirroring wrapperPath's posture.
func skipUnlessPOSIXShell(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no POSIX sh on this host")
	}
}
