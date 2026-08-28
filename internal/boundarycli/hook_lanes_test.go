package boundarycli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/hookboundary"
)

// TestHookDoctorExitsNonZeroWhenNothingIsWired is the reason the doctor has an
// exit code at all: a setup script must be able to tell "Boundary is not in front
// of this agent" from "Boundary is wired with a caveat".
func TestHookDoctorExitsNonZeroWhenNothingIsWired(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHook([]string{"doctor", "--dir", t.TempDir()}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 in a directory with no hook wiring\n%s", code, stdout.String())
	}
	text := stdout.String()
	for _, want := range []string{"Boundary Claude Code hook doctor", "hook registration", "Not governed by this hook:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("report missing %q:\n%s", want, text)
		}
	}
}

func TestHookDoctorJSONCarriesTheWholeReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	runHook([]string{"doctor", "--json", "--dir", t.TempDir()}, strings.NewReader(""), &stdout, &stderr)

	var report hookboundary.DoctorReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not a doctor report: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != hookboundary.DoctorSchemaVersion {
		t.Fatalf("schema_version = %q", report.SchemaVersion)
	}
	if len(report.Checks) == 0 || len(report.Bypasses) == 0 || len(report.Caveats) == 0 {
		t.Fatalf("report is missing a section: %+v", report)
	}
	if report.MergeNote != hookboundary.PeerMergeNote {
		t.Fatalf("merge note = %q", report.MergeNote)
	}
}

func TestHookDoctorRejectsAPositionalArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHook([]string{"doctor", "somewhere"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--dir") {
		t.Fatalf("stderr = %q, want it to point at the flag", stderr.String())
	}
}

// SessionEnd is bookkeeping for a session that has already ended: it writes
// nothing to stdout and exits 0 whatever happened.
func TestHookSessionEndIsSilentAndExitsZero(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runHook([]string{"sessionend", "--dir", dir},
		strings.NewReader(`{"hook_event_name":"SessionEnd","session_id":"cli-session"}`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want nothing; SessionEnd output is not a decision", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want the ordinary path quiet", stderr.String())
	}

	body, err := os.ReadFile(filepath.Join(dir, hookboundary.SessionSummaryLogName))
	if err != nil {
		t.Fatalf("read summary log: %v", err)
	}
	var summary hookboundary.SessionSummary
	if err := json.Unmarshal(bytes.TrimSpace(body), &summary); err != nil {
		t.Fatalf("summary is not JSON: %v", err)
	}
	if summary.SessionID != "cli-session" || summary.ReceiptHint != hookboundary.ReceiptHint {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestHookSessionEndExitsZeroOnAMalformedEvent(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runHook([]string{"sessionend", "--dir", dir}, strings.NewReader(`{"session_id":`), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 even on a malformed event", code)
	}
	if stdout.String() != "" || stderr.String() != "" {
		t.Fatalf("stdout=%q stderr=%q, want silence", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, hookboundary.SessionSummaryLogName)); !os.IsNotExist(err) {
		t.Fatalf("a summary was written for an unidentifiable session: %v", err)
	}
}

// The pretooluse and sessionend lanes must agree on the trace prefix, or the
// summary counts nothing that the hook actually recorded.
func TestHookSessionEndCountsWhatPreToolUseRecorded(t *testing.T) {
	dir := t.TempDir()
	for _, event := range []string{
		`{"session_id":"joined","tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`,
		`{"session_id":"joined","tool_name":"Bash","tool_input":{"command":"git status"}}`,
	} {
		if _, _, code := runHookEvent(t, dir, event); code != 0 {
			t.Fatalf("pretooluse exit = %d", code)
		}
	}

	var stdout, stderr bytes.Buffer
	if code := runHook([]string{"sessionend", "--dir", dir},
		strings.NewReader(`{"session_id":"joined"}`), &stdout, &stderr); code != 0 {
		t.Fatalf("sessionend exit = %d", code)
	}
	body, err := os.ReadFile(filepath.Join(dir, hookboundary.SessionSummaryLogName))
	if err != nil {
		t.Fatalf("read summary log: %v", err)
	}
	var summary hookboundary.SessionSummary
	if err := json.Unmarshal(bytes.TrimSpace(body), &summary); err != nil {
		t.Fatalf("summary is not JSON: %v", err)
	}
	if summary.Decisions != 2 || summary.Denied != 1 || summary.Allowed != 1 {
		t.Fatalf("summary = %+v, want 2 decisions (1 deny, 1 allow)", summary)
	}
}

func TestHookLaneHelpSurfaces(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"hook"}, []string{"doctor", "sessionend"}},
		{[]string{"hook", "doctor", "--help"},
			[]string{"Usage:", "boundary hook doctor", "fixed, not discovered", "Exits 1 when a check is broken"}},
		{[]string{"hook", "sessionend", "--help"},
			[]string{"Usage:", "boundary hook sessionend", "not a decision", "is not evidence"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tc.args, &stdout, &stderr); code != 0 {
				t.Fatalf("exit = %d", code)
			}
			combined := stdout.String() + stderr.String()
			for _, want := range tc.want {
				if !strings.Contains(combined, want) {
					t.Fatalf("help missing %q:\n%s", want, combined)
				}
			}
		})
	}
}

func TestRootHelpListsTheHookLanes(t *testing.T) {
	var stdout bytes.Buffer
	if code := Run([]string{"--help"}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("root help exit %d", code)
	}
	for _, want := range []string{"hook pretooluse", "hook doctor", "hook sessionend"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("root help missing %q:\n%s", want, stdout.String())
		}
	}
}

// Completion must offer every hook subverb, or a shipped script silently lags
// the dispatch it is meant to mirror.
func TestCompletionOffersEveryHookSubcommand(t *testing.T) {
	got := compoundSubcommands["hook"]
	want := map[string]bool{"pretooluse": false, "doctor": false, "sessionend": false}
	for _, name := range got {
		if _, ok := want[name]; !ok {
			t.Fatalf("completion offers unknown hook subcommand %q", name)
		}
		want[name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("completion does not offer hook subcommand %q", name)
		}
	}
	for _, name := range got {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"hook", name, "--help"}, &stdout, &stderr); code != 0 {
			t.Fatalf("hook %s --help exit = %d; completion offers a verb that does not dispatch", name, code)
		}
	}
}
