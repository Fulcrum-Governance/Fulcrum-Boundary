package boundarycli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/hookboundary"
)

// runHookEvent drives `boundary hook pretooluse` with event on stdin and records
// written under dir.
func runHookEvent(t *testing.T, dir, event string, extraArgs ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	args := append([]string{"pretooluse", "--dir", dir}, extraArgs...)
	code = runHook(args, strings.NewReader(event), &out, &errOut)
	return out.String(), errOut.String(), code
}

func TestHookPreToolUseDeniesADestructiveCommand(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runHookEvent(t, dir,
		`{"tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (the JSON decision blocks, not the exit code)", code)
	}
	var decoded struct {
		Decision           string `json:"decision"`
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
		t.Fatalf("stdout %q is not hook JSON: %v", stdout, err)
	}
	if decoded.Decision != "block" || decoded.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("deny output missing a shape: %s", stdout)
	}
}

func TestHookPreToolUseIsSilentOnAllow(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runHookEvent(t, dir,
		`{"tool_name":"Bash","tool_input":{"command":"git status"}}`)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want a silent allow", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want the happy path quiet", stderr)
	}
}

func TestHookPreToolUseAllowsUngovernedToolsSilently(t *testing.T) {
	dir := t.TempDir()
	stdout, _, code := runHookEvent(t, dir, `{"tool_name":"Read","tool_input":{"file_path":"config/.env"}}`)
	if code != 0 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q, want a silent allow", code, stdout)
	}
}

// TestHookPreToolUseRecordsVerifyWithVerifyRecord closes the loop the product
// rests on: the file the hook writes is the file `boundary verify-record`
// accepts.
func TestHookPreToolUseRecordsVerifyWithVerifyRecord(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runHookEvent(t, dir,
		`{"session_id":"sess-cli","tool_name":"Write","tool_input":{"file_path":"config/.env","content":"x"}}`); code != 0 {
		t.Fatalf("exit = %d", code)
	}

	entries, err := os.ReadDir(filepath.Join(dir, hookboundary.RecordsDirName))
	if err != nil {
		t.Fatalf("read records dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("records = %d, want exactly 1", len(entries))
	}
	recordPath := filepath.Join(dir, hookboundary.RecordsDirName, entries[0].Name())

	var stdout, stderr bytes.Buffer
	if code := Run([]string{"verify-record", recordPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("verify-record exit = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}

	body, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		t.Fatalf("record is not a single JSON object: %v", err)
	}
	if record.Action != "deny" || record.Adapter != hookboundary.TransportHook {
		t.Fatalf("record = %q on %q, want a deny on the hook transport", record.Action, record.Adapter)
	}
	if _, err := os.Stat(filepath.Join(dir, hookboundary.DecisionLogName)); err != nil {
		t.Fatalf("decision log missing: %v", err)
	}
}

func TestHookPreToolUseFailModeFlagOverridesTheDefault(t *testing.T) {
	cases := []struct {
		failMode string
		want     string
	}{
		{"", "ask"},
		{"closed", "deny"},
	}
	for _, tc := range cases {
		t.Run("failmode="+tc.failMode, func(t *testing.T) {
			dir := t.TempDir()
			args := []string{}
			if tc.failMode != "" {
				args = append(args, "--failmode", tc.failMode)
			}
			stdout, _, code := runHookEvent(t, dir, `{"tool_name":`, args...)
			if code != 0 {
				t.Fatalf("exit = %d, want 0 even on a fault", code)
			}
			var decoded struct {
				HookSpecificOutput struct {
					PermissionDecision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Fatalf("stdout %q is not hook JSON: %v", stdout, err)
			}
			if decoded.HookSpecificOutput.PermissionDecision != tc.want {
				t.Fatalf("permissionDecision = %q, want %q", decoded.HookSpecificOutput.PermissionDecision, tc.want)
			}
		})
	}
}

func TestHookPreToolUseFailOpenLeavesAnAdvisoryOnStderr(t *testing.T) {
	dir := t.TempDir()
	stdout, stderr, code := runHookEvent(t, dir, `{"tool_name":`, "--failmode", "open")
	if code != 0 || stdout != "" {
		t.Fatalf("exit=%d stdout=%q, want a silent allow", code, stdout)
	}
	if !strings.Contains(stderr, "fault, allowing") {
		t.Fatalf("stderr = %q, want the fail-open advisory", stderr)
	}
}

func TestHookHelpSurfaces(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"hook"}, []string{"Purpose:", "pretooluse", "never runs the command"}},
		{[]string{"hook", "--help"}, []string{"Purpose:", "pretooluse"}},
		{[]string{"hook", "pretooluse", "--help"},
			[]string{"Usage:", "boundary hook pretooluse", "read from stdin", "delivered preview"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tc.args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit = %d", code)
			}
			combined := stdout.String() + stderr.String()
			if strings.Contains(combined, "Usage of boundary") {
				t.Fatalf("help falls back to the raw flag header:\n%s", combined)
			}
			for _, want := range tc.want {
				if !strings.Contains(combined, want) {
					t.Fatalf("help missing %q:\n%s", want, combined)
				}
			}
		})
	}
}

func TestHookRejectsAnUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"hook", "posttooluse"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "unknown hook subcommand") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestHookPreToolUseRejectsAPositionalArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHook([]string{"pretooluse", "event.json"}, strings.NewReader("{}"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "read from stdin") {
		t.Fatalf("stderr = %q, want it to point at stdin", stderr.String())
	}
}

func TestRootHelpListsTheHookCommand(t *testing.T) {
	var stdout bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("root help exit %d", code)
	}
	if !strings.Contains(stdout.String(), "hook pretooluse") {
		t.Fatalf("root help missing the hook command:\n%s", stdout.String())
	}
}
