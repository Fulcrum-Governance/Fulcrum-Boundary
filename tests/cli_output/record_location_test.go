package cli_output_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

// The record-location UX contract (Phase 0A Step 1): every record-emitting
// command prints a uniform pair of lines — "decision record id: <record_id>"
// for an identifier and "decision record path: <path>" for a written file.
// These tests pin both the stable line shape and the predictable on-disk path
// so the find -> verify step stays copy-paste across the proof-lane demos,
// redteam, and the evidence bundle.

const (
	recordIDPrefix   = "decision record id: rec_"
	recordPathPrefix = "decision record path: "
)

// run executes a boundary command in-process and returns stdout, stderr, exit.
func run(args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := boundarycli.Run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func lineWithPrefix(t *testing.T, out, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	t.Fatalf("output missing line with prefix %q:\n%s", prefix, out)
	return ""
}

// assertVerifiableRecordFile confirms a written JSONL record path exists, every
// line decodes to a schema "1" DecisionRecordV1 with a record_id and a
// sha256-prefixed decision_hash, and the printed record id names one of the
// records in the file — i.e. the printed path and id refer to the same artifact
// that verify-record consumes.
func assertVerifiableRecordFile(t *testing.T, path, wantID string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record path %q: %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("record file %q is empty", path)
	}
	foundID := false
	for _, line := range lines {
		var rec governance.DecisionRecordV1
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode record in %q: %v\n%s", path, err, line)
		}
		if rec.RecordID == "" || !strings.HasPrefix(rec.DecisionHash, "sha256:") {
			t.Fatalf("record lacks id/hash: %#v", rec)
		}
		if rec.SchemaVersion != "1" {
			t.Fatalf("record schema_version = %q, want \"1\"", rec.SchemaVersion)
		}
		if rec.RecordID == wantID {
			foundID = true
		}
	}
	if wantID != "" && !foundID {
		t.Fatalf("printed record id %q not found among records in %q", wantID, path)
	}
}

func TestGitHubDemoOutPrintsUniformRecordLocation(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "demo.json")
	stdout, stderr, code := run("demo", "github-lethal-trifecta", "--json", "--out", report)
	if code != 0 {
		t.Fatalf("demo exit = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	id := lineWithPrefix(t, stdout, recordIDPrefix)
	path := lineWithPrefix(t, stdout, recordPathPrefix)
	// The predictable, documented landing path for this proof lane.
	wantPath := filepath.Join(dir, "github-lethal-trifecta-artifacts", "decision-records.jsonl")
	if path != wantPath {
		t.Fatalf("record path = %q, want %q", path, wantPath)
	}
	assertVerifiableRecordFile(t, path, "rec_"+id)
}

// TestGitHubDemoDefaultPrintsIDButNotPath confirms that without --out the
// github proof lane prints the record id but no path line: its workspace is a
// temp directory deleted on return, so advertising a path would point at a file
// that no longer exists. The id/path lines stay honest about persistence.
func TestGitHubDemoDefaultPrintsIDButNotPath(t *testing.T) {
	stdout, stderr, code := run("demo", "github-lethal-trifecta")
	if code != 0 {
		t.Fatalf("demo exit = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, recordIDPrefix) {
		t.Fatalf("default github demo missing uniform record id line:\n%s", stdout)
	}
	if strings.Contains(stdout, recordPathPrefix) {
		t.Fatalf("default github demo (no --out) must not advertise a discarded record path:\n%s", stdout)
	}
}

// TestCommandSecretExfilDemoDefaultPrintsIDButNotPath confirms the Command
// Boundary proof lane, without --out, prints the record id and no path line —
// it persists no file, so there is nothing to point verify-record at.
func TestCommandSecretExfilDemoDefaultPrintsIDButNotPath(t *testing.T) {
	stdout, stderr, code := run("demo", "command-secret-exfil")
	if code != 0 {
		t.Fatalf("demo exit = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, recordIDPrefix) {
		t.Fatalf("default command-secret-exfil demo missing uniform record id line:\n%s", stdout)
	}
	if strings.Contains(stdout, recordPathPrefix) {
		t.Fatalf("default command-secret-exfil demo (no --out) must not print a record path:\n%s", stdout)
	}
}

func TestCommandSecretExfilDemoOutPrintsUniformRecordLocation(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "demo.txt")
	stdout, stderr, code := run("demo", "command-secret-exfil", "--out", report)
	if code != 0 {
		t.Fatalf("demo exit = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	id := lineWithPrefix(t, stdout, recordIDPrefix)
	path := lineWithPrefix(t, stdout, recordPathPrefix)
	wantPath := filepath.Join(dir, "command-secret-exfil-artifacts", "decision-records.jsonl")
	if path != wantPath {
		t.Fatalf("record path = %q, want %q", path, wantPath)
	}
	assertVerifiableRecordFile(t, path, "rec_"+id)
}

// TestCommandSecretExfilDemoVerifyRecordRoundTrip proves the find -> verify step
// is genuinely copy-paste: the path the proof lane prints feeds verify-record.
func TestCommandSecretExfilDemoVerifyRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	report := filepath.Join(dir, "demo.txt")
	stdout, _, code := run("demo", "command-secret-exfil", "--out", report)
	if code != 0 {
		t.Fatalf("demo exit = %d", code)
	}
	path := lineWithPrefix(t, stdout, recordPathPrefix)
	// Split the single JSONL line into a standalone record file for verify-record.
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record path: %v", err)
	}
	recordFile := filepath.Join(dir, "record.json")
	first := strings.SplitN(strings.TrimSpace(string(body)), "\n", 2)[0]
	if err := os.WriteFile(recordFile, []byte(first), 0o600); err != nil {
		t.Fatalf("write record file: %v", err)
	}
	vOut, vErr, vCode := run("verify-record", recordFile)
	if vCode != 0 {
		t.Fatalf("verify-record exit = %d stdout=%s stderr=%s", vCode, vOut, vErr)
	}
	if !strings.Contains(vOut, "record verification: ok") {
		t.Fatalf("verify-record output missing ok line:\n%s", vOut)
	}
}

// TestRedteamPrintsRecordIDNotPath confirms the in-memory-only surface emits the
// uniform id line and, because it writes no file, no path line — the id/path
// collision that previously existed is gone.
func TestRedteamPrintsRecordIDNotPath(t *testing.T) {
	stdout, stderr, code := run("redteam")
	if code != 0 {
		t.Fatalf("redteam exit = %d stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, recordIDPrefix) {
		t.Fatalf("redteam output missing uniform record id line:\n%s", stdout)
	}
	if strings.Contains(stdout, recordPathPrefix) {
		t.Fatalf("redteam writes no record file; it must not print a record path line:\n%s", stdout)
	}
	// The legacy ambiguous token must be gone.
	if strings.Contains(stdout, "decision record: rec_") {
		t.Fatalf("redteam still uses the ambiguous \"decision record:\" token:\n%s", stdout)
	}
}

// TestEvidenceBundlePrintsRecordPathsForCopiedRecords confirms the evidence
// bundle reports each copied decision-record artifact under the same uniform
// path line, resolved inside the bundle.
func TestEvidenceBundlePrintsRecordPathsForCopiedRecords(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".boundary", "command")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	// A minimal, parseable decision-record JSONL the bundle will copy and tag.
	rec := governance.BuildDecisionRecord(governance.AuditEvent{
		Action:   "deny",
		ToolName: "demo",
	})
	line, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal record: %v", err)
	}
	recordSrc := filepath.Join(source, "decision-records.jsonl")
	if err := os.WriteFile(recordSrc, append(line, '\n'), 0o600); err != nil {
		t.Fatalf("write source record: %v", err)
	}
	out := filepath.Join(root, "bundle")
	stdout, stderr, code := run("evidence", "bundle", "--from", filepath.Join(root, ".boundary"), "--out", out)
	if code != 0 {
		t.Fatalf("evidence bundle exit = %d stdout=%s stderr=%s", code, stdout, stderr)
	}
	path := lineWithPrefix(t, stdout, recordPathPrefix)
	if filepath.Base(path) != "decision-records.jsonl" {
		t.Fatalf("evidence record path basename = %q, want decision-records.jsonl (line=%q)", filepath.Base(path), path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("evidence-reported record path does not exist: %v", err)
	}
}

// TestCommandRunPrintsUniformRecordPath confirms the command boundary surface
// emits the uniform path line (to stderr) instead of the legacy record= token.
func TestCommandRunPrintsUniformRecordPath(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "records.jsonl")
	_, stderr, _ := run("command", "run", "--record-out", recordPath, "--", "pwd")
	if !strings.Contains(stderr, recordPathPrefix+recordPath) {
		t.Fatalf("command run stderr missing uniform record path line:\n%s", stderr)
	}
	if strings.Contains(stderr, "record="+recordPath) {
		t.Fatalf("command run still uses the legacy record= token:\n%s", stderr)
	}
}
