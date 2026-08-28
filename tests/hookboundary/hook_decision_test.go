// Package hookboundary_test is the black-box suite for the Claude Code
// PreToolUse hook: the `boundary hook pretooluse` decision path driven as a real
// process, and the shipped POSIX wrapper that invokes it.
package hookboundary_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// boundaryBin is the binary every test in this suite drives. It is built once in
// TestMain rather than per test, because several tests need the same on-disk
// binary and building it is the slowest thing here.
var boundaryBin string

// repoRoot is the checkout this suite was compiled from.
var repoRoot string

func TestMain(m *testing.M) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller failed")
	}
	// tests/hookboundary/ -> repo root (two levels up).
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	dir, err := os.MkdirTemp("", "boundary-hook-suite")
	if err != nil {
		panic("temp dir: " + err.Error())
	}
	boundaryBin = filepath.Join(dir, "boundary")
	if runtime.GOOS == "windows" {
		boundaryBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", boundaryBin, "./cmd/boundary")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build boundary: " + err.Error() + "\n" + string(output))
	}
	unquarantineDarwinTestBinary(boundaryBin)

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func unquarantineDarwinTestBinary(path string) {
	if runtime.GOOS != "darwin" {
		return
	}
	_ = exec.Command("/usr/bin/xattr", "-d", "com.apple.quarantine", path).Run()
}

// hookDecision is the PreToolUse decision shape Claude Code reads.
type hookDecision struct {
	Decision           string `json:"decision"`
	Reason             string `json:"reason"`
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// reason returns whichever reason the decision carries, preferring the current
// hookSpecificOutput field.
func (d hookDecision) reason() string {
	if d.HookSpecificOutput.PermissionDecisionReason != "" {
		return d.HookSpecificOutput.PermissionDecisionReason
	}
	return d.Reason
}

// runHook drives `boundary hook pretooluse` as a real process with records under
// dir, and returns the exit code plus stdout and stderr.
func runHook(t *testing.T, dir, event string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(boundaryBin, "hook", "pretooluse", "--dir", dir)
	cmd.Stdin = strings.NewReader(event)
	// Claude Code launches a PreToolUse hook with its working directory set to
	// the project directory, and the edit route resolves absolute targets against
	// that. Running from dir makes the temp directory the project root here.
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BOUNDARY_HOOK_DIR="+dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	code := 0
	if err := cmd.Run(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("run hook: %v\nstderr=%s", err, stderr.String())
		}
		code = exitErr.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

// decodeDecision parses stdout as a PreToolUse decision. Empty stdout is a
// silent allow and is reported as the zero value.
func decodeDecision(t *testing.T, stdout string) hookDecision {
	t.Helper()
	var decoded hookDecision
	if strings.TrimSpace(stdout) == "" {
		return decoded
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("stdout is not hook JSON: %v\n%s", err, stdout)
	}
	if decoded.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Fatalf("hookEventName = %q, want PreToolUse\n%s",
			decoded.HookSpecificOutput.HookEventName, stdout)
	}
	return decoded
}

// readRecords returns every decision record written to dir's JSONL log.
func readRecords(t *testing.T, dir string) []governance.DecisionRecordV1 {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "decision-records.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("read decision log: %v", err)
	}
	var records []governance.DecisionRecordV1
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record governance.DecisionRecordV1
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decision log line is not a record: %v\n%s", err, line)
		}
		records = append(records, record)
	}
	return records
}

// assertOneVerifiableRecord checks that dir holds exactly one record, that it
// carries the expected action, and that it verifies the way
// `boundary verify-record` verifies it.
func assertOneVerifiableRecord(t *testing.T, dir, wantAction string) governance.DecisionRecordV1 {
	t.Helper()
	records := readRecords(t, dir)
	if len(records) != 1 {
		t.Fatalf("record count = %d, want exactly 1: %#v", len(records), records)
	}
	record := records[0]
	if record.Action != wantAction {
		t.Fatalf("record action = %q, want %q", record.Action, wantAction)
	}
	if !governance.SupportedDecisionRecordSchemaVersion(record.SchemaVersion) {
		t.Fatalf("schema_version = %q is not verifiable by this build", record.SchemaVersion)
	}
	if err := governance.VerifyDecisionRecord(record, nil, "", ""); err != nil {
		t.Fatalf("record does not verify: %v", err)
	}
	return record
}

// TestHookDeniesACompoundLineOnItsChainedTail is the BOU-5 wiring contract seen
// from outside the package: the hook must decompose the line, deny on the
// dangerous tail rather than allowing on the benign head, name the offending
// segment, and leave a verifiable record.
func TestHookDeniesACompoundLineOnItsChainedTail(t *testing.T) {
	dir := t.TempDir()
	code, stdout, _ := runHook(t, dir,
		`{"hook_event_name":"PreToolUse","session_id":"sess-compound",`+
			`"tool_name":"Bash","tool_input":{"command":"git status && rm -rf ~"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (the JSON drives the block, not the exit code)", code)
	}

	decision := decodeDecision(t, stdout)
	if decision.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision = %q, want deny\n%s",
			decision.HookSpecificOutput.PermissionDecision, stdout)
	}
	if decision.Decision != "block" {
		t.Fatalf("legacy decision key = %q, want block (older clients read it)", decision.Decision)
	}
	for _, want := range []string{"rm -rf ~", "compound segments", "C4"} {
		if !strings.Contains(decision.reason(), want) {
			t.Fatalf("reason %q does not carry %q", decision.reason(), want)
		}
	}

	record := assertOneVerifiableRecord(t, dir, "deny")
	if !strings.Contains(record.Reason, "rm -rf ~") {
		t.Fatalf("record reason %q does not name the offending segment", record.Reason)
	}
	if !strings.HasPrefix(record.TraceID, "sess-compound#") {
		t.Fatalf("trace_id = %q, want the session prefix", record.TraceID)
	}
}

// TestHookDecomposesEverySmugglingShape covers the shapes the leading-command
// classifier let through: a benign head followed by a destructive tail reached
// through each operator the decomposer models.
func TestHookDecomposesEverySmugglingShape(t *testing.T) {
	for name, command := range map[string]string{
		"and chain":            "git status && rm -rf ~",
		"or chain":             "git status || rm -rf ~",
		"semicolon chain":      "echo hi; rm -rf ~",
		"command substitution": "echo $(rm -rf ~)",
		"shell -c payload":     "sh -c 'rm -rf /'",
		"env prefix":           "FOO=bar rm -rf /",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, _ := runHook(t, dir,
				`{"tool_name":"Bash","tool_input":{"command":"`+command+`"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			decision := decodeDecision(t, stdout)
			if decision.HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("permissionDecision = %q, want deny for %q\n%s",
					decision.HookSpecificOutput.PermissionDecision, command, stdout)
			}
			assertOneVerifiableRecord(t, dir, "deny")
		})
	}
}

// TestHookAsksOnAnUndecomposableLine pins the other half of the wiring: a line
// Boundary could not decompose is escalated to the user as a DECISION, recorded
// like any other, and never allowed through.
func TestHookAsksOnAnUndecomposableLine(t *testing.T) {
	for name, command := range map[string]string{
		"heredoc":              "cat <<EOF",
		"process substitution": "diff <(ls) <(ls -a)",
		"eval":                 "eval \\\"$PAYLOAD\\\"",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, _ := runHook(t, dir,
				`{"tool_name":"Bash","tool_input":{"command":"`+command+`"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			decision := decodeDecision(t, stdout)
			if decision.HookSpecificOutput.PermissionDecision != "ask" {
				t.Fatalf("permissionDecision = %q, want ask\n%s",
					decision.HookSpecificOutput.PermissionDecision, stdout)
			}
			if !strings.Contains(decision.reason(), "could not be safely decomposed") {
				t.Fatalf("reason %q does not explain the escalation", decision.reason())
			}
			assertOneVerifiableRecord(t, dir, "require_approval")
		})
	}
}

// TestHookDeniesEditsToGovernanceControlSurfaces is BOU-5b end to end: an agent
// must not be able to edit its own permission settings, its own hook, or the
// record of what the hook decided.
func TestHookDeniesEditsToGovernanceControlSurfaces(t *testing.T) {
	for _, target := range []string{
		".claude/settings.json",
		".claude/settings.local.json",
		".claude/hooks/pretooluse.sh",
		"integrations/claude-code/pretooluse-boundary.sh",
		".boundary/hook/decision-records.jsonl",
		"/abs/project/.claude/settings.json",
	} {
		t.Run(target, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, _ := runHook(t, dir,
				`{"tool_name":"Write","tool_input":{"file_path":"`+target+`","content":"x"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			decision := decodeDecision(t, stdout)
			if decision.HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("permissionDecision = %q, want deny for a control surface\n%s",
					decision.HookSpecificOutput.PermissionDecision, stdout)
			}
			if !strings.Contains(decision.reason(), "E7") {
				t.Fatalf("reason %q does not name the control-path class", decision.reason())
			}
			assertOneVerifiableRecord(t, dir, "deny")
		})
	}
}

// TestHookDeniesCommandsThatWriteGovernanceControlSurfaces is the sibling of the
// edit-route test above, on the route that can reach the same files. A
// self-protection that denies `.claude/settings.json` as an `Edit` while
// permitting `cp evil .claude/settings.json` as a `Bash` call is not closed:
// the agent rewrites its own PreToolUse wiring and the next tool call is
// ungoverned.
func TestHookDeniesCommandsThatWriteGovernanceControlSurfaces(t *testing.T) {
	for name, command := range map[string]string{
		"cp over settings":            "cp /tmp/evil .claude/settings.json",
		"mv over settings":            "mv /tmp/evil.json .claude/settings.json",
		"cp over a hook":              "cp /tmp/evil.sh .claude/hooks/pretooluse.sh",
		"cp over the wrapper":         "cp /tmp/evil.sh integrations/claude-code/pretooluse-boundary.sh",
		"touch a record":              "touch .boundary/hook/decision-records.jsonl",
		"redirect over settings":      "cat /tmp/evil.json > .claude/settings.json",
		"redirect over abs settings":  "ls -R / > /abs/project/.claude/settings.json",
		"chained after a benign head": "git status && cp /tmp/evil .claude/settings.json",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, _ := runHook(t, dir,
				`{"tool_name":"Bash","tool_input":{"command":"`+command+`"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			decision := decodeDecision(t, stdout)
			if decision.HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("permissionDecision = %q, want deny for %q\n%s",
					decision.HookSpecificOutput.PermissionDecision, command, stdout)
			}
			if !strings.Contains(decision.reason(), "governance control surface") {
				t.Fatalf("reason %q does not name the control surface", decision.reason())
			}
			assertOneVerifiableRecord(t, dir, "deny")
		})
	}
}

// TestHookDeniesAnAbsoluteEditOutsideTheProject is the edit route's
// outside-project-scope contract seen from outside the process. Claude Code's
// edit tools pass ABSOLUTE paths, so this is the ordinary shape of a write that
// escapes the project — the case that used to be reclassified as a repo-relative
// tail and allowed silently.
func TestHookDeniesAnAbsoluteEditOutsideTheProject(t *testing.T) {
	for name, target := range map[string]string{
		"system file":      "/etc/passwd",
		"governing binary": "/usr/local/bin/boundary",
		"shell profile":    "/root/.zshrc",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, _ := runHook(t, dir,
				`{"tool_name":"Write","tool_input":{"file_path":"`+target+`","content":"x"}}`)
			if code != 0 {
				t.Fatalf("exit = %d, want 0", code)
			}
			decision := decodeDecision(t, stdout)
			if decision.HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("permissionDecision = %q, want deny for %q\n%s",
					decision.HookSpecificOutput.PermissionDecision, target, stdout)
			}
			if !strings.Contains(decision.reason(), "E7") {
				t.Fatalf("reason %q does not name the outside-scope class", decision.reason())
			}
			assertOneVerifiableRecord(t, dir, "deny")
		})
	}
}

// An absolute target INSIDE the project must keep the class its position earns,
// so the outside-scope deny does not become "every absolute path denies" — which
// would be every edit Claude Code makes.
func TestHookClassifiesAnAbsoluteEditInsideTheProject(t *testing.T) {
	dir := t.TempDir()
	// runHook runs the binary with its working directory set to dir, which is
	// the project root the edit route resolves absolute targets against.
	code, stdout, _ := runHook(t, dir,
		`{"tool_name":"Write","tool_input":{"file_path":"`+filepath.Join(dir, "docs", "guide.md")+`"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want a silent allow for a doc inside the project", stdout)
	}
	assertOneVerifiableRecord(t, dir, "allow")
}

// TestHookAllowsOrdinaryAgentContentUnderClaude keeps the control-surface rules
// narrow: skills and commands are ordinary content, not a governance surface.
func TestHookAllowsOrdinaryAgentContentUnderClaude(t *testing.T) {
	dir := t.TempDir()
	code, stdout, _ := runHook(t, dir,
		`{"tool_name":"Write","tool_input":{"file_path":".claude/skills/thing/SKILL.md"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want a silent allow", stdout)
	}
	assertOneVerifiableRecord(t, dir, "allow")
}
