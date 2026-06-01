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
// command prints uniform lines — "decision record id: <record_id>" for an
// identifier, "decision record path: <path>" for a single-record JSON object,
// and "decision record log: <path>" for a multi-record .jsonl log. These tests
// pin both the stable line shape and the predictable on-disk path, and prove
// the contract that distinguishes the two file lines: the path line always
// names a single-record JSON object that `boundary verify-record` consumes
// (exit 0), while the log line names a multi-record .jsonl that verify-record
// rejects. That separation is what keeps the find -> verify step copy-paste
// across the proof-lane demos, redteam, and the evidence bundle.

const (
	recordIDPrefix   = "decision record id: rec_"
	recordPathPrefix = "decision record path: "
	recordLogPrefix  = "decision record log: "
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

// assertVerifiableRecordFile confirms the path printed under "decision record
// path:" is a single-record JSON object: the file exists, decodes to exactly
// one DecisionRecordV1 (a single top-level object, not JSONL) with a record_id
// and a sha256-prefixed decision_hash, and carries a supported schema_version.
// A single top-level object is exactly what `boundary verify-record` consumes,
// so this enforces the path-line contract at the file level; the round-trip
// tests below additionally prove verify-record accepts it. wantID, when set,
// is the record_id the path line's sibling id line printed (the headline
// record); it must equal the object's record_id so the two lines name the same
// artifact.
func assertVerifiableRecordFile(t *testing.T, path, wantID string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record path %q: %v", path, err)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		t.Fatalf("record file %q is empty", path)
	}
	// The path line must point at a single JSON object, never a multi-record
	// JSONL log: decoding must consume the whole file with nothing trailing,
	// which is the exact condition verify-record enforces.
	dec := json.NewDecoder(strings.NewReader(trimmed))
	var rec governance.DecisionRecordV1
	if err := dec.Decode(&rec); err != nil {
		t.Fatalf("decode single record object in %q: %v\n%s", path, err, trimmed)
	}
	if dec.More() {
		t.Fatalf("record path %q must hold a single JSON object (verify-record input), not a multi-record log:\n%s", path, trimmed)
	}
	if rec.RecordID == "" || !strings.HasPrefix(rec.DecisionHash, "sha256:") {
		t.Fatalf("record lacks id/hash: %#v", rec)
	}
	// Accept any supported decision-record schema version. The proof-lane demos
	// route through the pipeline/adapters, which populate additive route-context,
	// so their records are schema_version "2"; a record with no route-context
	// stays "1". Both remain verifiable.
	if !governance.SupportedDecisionRecordSchemaVersion(rec.SchemaVersion) {
		t.Fatalf("record schema_version = %q, want a supported version (\"1\" or \"2\")", rec.SchemaVersion)
	}
	if wantID != "" && rec.RecordID != wantID {
		t.Fatalf("printed record id %q does not match the single record in %q (record_id %q)", wantID, path, rec.RecordID)
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
	// The path line points at the single-record JSON object: the predictable,
	// documented verify-record landing path for this proof lane.
	wantPath := filepath.Join(dir, "github-lethal-trifecta-artifacts", "decision-record.json")
	if path != wantPath {
		t.Fatalf("record path = %q, want %q", path, wantPath)
	}
	assertVerifiableRecordFile(t, path, "rec_"+id)
	// The multi-record .jsonl log is surfaced under the separate log line, never
	// the path line, so verify-record never receives a multi-record file.
	logPath := lineWithPrefix(t, stdout, recordLogPrefix)
	wantLog := filepath.Join(dir, "github-lethal-trifecta-artifacts", "decision-records.jsonl")
	if logPath != wantLog {
		t.Fatalf("record log = %q, want %q", logPath, wantLog)
	}
	if logPath == path {
		t.Fatalf("log line and path line must name different files; both were %q", path)
	}
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
	// The path line points at the single-record JSON object.
	wantPath := filepath.Join(dir, "command-secret-exfil-artifacts", "decision-record.json")
	if path != wantPath {
		t.Fatalf("record path = %q, want %q", path, wantPath)
	}
	assertVerifiableRecordFile(t, path, "rec_"+id)
	// The .jsonl log is surfaced under the separate log line.
	logPath := lineWithPrefix(t, stdout, recordLogPrefix)
	wantLog := filepath.Join(dir, "command-secret-exfil-artifacts", "decision-records.jsonl")
	if logPath != wantLog {
		t.Fatalf("record log = %q, want %q", logPath, wantLog)
	}
	if logPath == path {
		t.Fatalf("log line and path line must name different files; both were %q", path)
	}
}

// TestDemoVerifyRecordRoundTrip proves the find -> verify step is genuinely
// copy-paste for BOTH proof lanes: the exact path each demo prints under
// "decision record path:" is fed straight into `boundary verify-record` with no
// massaging, and it must exit 0. This is the reviewer-requested regression guard
// for PR #110: github-lethal-trifecta previously printed a 2-record .jsonl that
// verify-record rejected ("invalid character '{' after top-level value"). The
// path line must now name a single-record JSON object for every lane.
func TestDemoVerifyRecordRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// report is the --out report basename for the lane.
		report string
		// logIsMultiRecord is true when the lane's .jsonl log holds >1 record
		// (github writes both the redteam and write-denial records), so
		// verify-record must reject the log. The command lane logs a single
		// record, which verify-record tolerates, so the rejection check is
		// skipped there — the path-vs-log distinction is still asserted by the
		// basenames and the round-trip in TestGitHubDemoOutPrintsUniformRecordLocation.
		logIsMultiRecord bool
	}{
		{
			name:             "github-lethal-trifecta",
			args:             []string{"demo", "github-lethal-trifecta", "--json", "--out"},
			report:           "demo.json",
			logIsMultiRecord: true,
		},
		{
			name:             "command-secret-exfil",
			args:             []string{"demo", "command-secret-exfil", "--out"},
			report:           "demo.txt",
			logIsMultiRecord: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			report := filepath.Join(dir, tc.report)
			stdout, stderr, code := run(append(tc.args, report)...)
			if code != 0 {
				t.Fatalf("demo exit = %d stdout=%s stderr=%s", code, stdout, stderr)
			}
			// Feed the printed path directly to verify-record — no splitting,
			// no rewriting. The contract is that the printed path is already a
			// verify-record-consumable single-record JSON object.
			path := lineWithPrefix(t, stdout, recordPathPrefix)
			vOut, vErr, vCode := run("verify-record", path)
			if vCode != 0 {
				t.Fatalf("verify-record %q exit = %d stdout=%s stderr=%s", path, vCode, vOut, vErr)
			}
			if !strings.Contains(vOut, "record verification: ok") {
				t.Fatalf("verify-record output missing ok line:\n%s", vOut)
			}
			// When the companion .jsonl log is genuinely multi-record, it is a
			// file verify-record must reject — confirming the two lines are not
			// interchangeable and the path line earns its contract. (This is the
			// exact case that regressed in PR #110.)
			if tc.logIsMultiRecord {
				logPath := lineWithPrefix(t, stdout, recordLogPrefix)
				_, _, logCode := run("verify-record", logPath)
				if logCode == 0 {
					t.Fatalf("verify-record unexpectedly accepted the multi-record log %q; the log line must not be a verify-record input", logPath)
				}
			}
		})
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
// bundle reports each copied decision-record artifact under the uniform log
// line, resolved inside the bundle. Copied artifacts are the command/edit
// boundary append-mode logs (multi-record .jsonl), which verify-record rejects,
// so they belong under "decision record log:", not "decision record path:".
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
	logPath := lineWithPrefix(t, stdout, recordLogPrefix)
	if filepath.Base(logPath) != "decision-records.jsonl" {
		t.Fatalf("evidence record log basename = %q, want decision-records.jsonl (line=%q)", filepath.Base(logPath), logPath)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("evidence-reported record log does not exist: %v", err)
	}
	// The copied log is surfaced under the log line, never the path line: the
	// path line's "single-record, verify-record-consumable" contract must not be
	// claimed for a copied multi-record audit log.
	if strings.Contains(stdout, recordPathPrefix) {
		t.Fatalf("evidence bundle must not print a record path line for a copied multi-record log:\n%s", stdout)
	}
}

// TestCommandRunPrintsUniformRecordLog confirms the command boundary surface
// emits the uniform log line (to stderr) instead of the legacy record= token.
// Its --record-out is an append-mode multi-record JSONL log of
// boundary.command_decision.v1 records, not a single verify-record
// DecisionRecordV1 object, so it is surfaced under the log label — keeping the
// path label reserved for verify-record-consumable single objects.
func TestCommandRunPrintsUniformRecordLog(t *testing.T) {
	dir := t.TempDir()
	recordPath := filepath.Join(dir, "records.jsonl")
	_, stderr, _ := run("command", "run", "--record-out", recordPath, "--", "pwd")
	if !strings.Contains(stderr, recordLogPrefix+recordPath) {
		t.Fatalf("command run stderr missing uniform record log line:\n%s", stderr)
	}
	if strings.Contains(stderr, "record="+recordPath) {
		t.Fatalf("command run still uses the legacy record= token:\n%s", stderr)
	}
	// The command log is not a verify-record input; it must not claim the path
	// line's single-object contract.
	if strings.Contains(stderr, recordPathPrefix) {
		t.Fatalf("command run must not print a record path line for its multi-record log:\n%s", stderr)
	}
}
