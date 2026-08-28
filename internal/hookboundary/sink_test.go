package hookboundary

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func sampleRecord(t *testing.T, action string) governance.DecisionRecordV1 {
	t.Helper()
	return governance.BuildDecisionRecord(governance.AuditEvent{
		Transport: TransportHook,
		ToolName:  "Bash",
		Action:    action,
		Reason:    "sink test",
		TraceID:   TraceID("sess", NextSequence()),
		Timestamp: time.Now().UTC(),
		AdapterID: AdapterID,
		RouteID:   RoutePrefix + "Bash",
	})
}

func TestSinkWritesBothArtifacts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".boundary", "hook")
	sink := Sink{Dir: dir}

	first := sampleRecord(t, "deny")
	paths, err := sink.Write(first)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if paths.Log != filepath.Join(dir, DecisionLogName) {
		t.Fatalf("log path = %q", paths.Log)
	}
	if filepath.Dir(paths.Record) != filepath.Join(dir, RecordsDirName) {
		t.Fatalf("record path = %q", paths.Record)
	}

	// The single-record file must be exactly one JSON object, which is what
	// verify-record consumes; the log must be one line per record.
	body, err := os.ReadFile(paths.Record)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	var decoded governance.DecisionRecordV1
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("record is not a single JSON object: %v", err)
	}
	if decoded.RecordID != first.RecordID {
		t.Fatalf("record_id = %q, want %q", decoded.RecordID, first.RecordID)
	}

	second := sampleRecord(t, "allow")
	if _, err := sink.Write(second); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	logBody, err := os.ReadFile(paths.Log)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(logBody), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("log has %d lines, want 2:\n%s", len(lines), logBody)
	}
	for i, line := range lines {
		var record governance.DecisionRecordV1
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line %d is not a record: %v", i, err)
		}
	}
}

func TestSinkUsesOwnerOnlyPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hook")
	paths, err := Sink{Dir: dir}.Write(sampleRecord(t, "allow"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	for _, path := range []string{paths.Record, paths.Log} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode = %v, want no group or world bits", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{dir, filepath.Join(dir, RecordsDirName)} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode = %v, want no group or world bits", path, info.Mode().Perm())
		}
	}
}

func TestSinkRefusesASymlinkedRecordDirectory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(root, "hook")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := Sink{Dir: link}.Write(sampleRecord(t, "deny"))
	if err == nil {
		t.Fatal("Write into a symlinked directory succeeded, want refusal")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v, want a symlink refusal", err)
	}
	if entries, readErr := os.ReadDir(target); readErr == nil && len(entries) != 0 {
		t.Fatalf("wrote %d entries through the symlink", len(entries))
	}
}

func TestSinkRefusesASymlinkedDecisionLog(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hook")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(root, "outside.jsonl")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, DecisionLogName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := Sink{Dir: dir}.Write(sampleRecord(t, "deny"))
	if err == nil {
		t.Fatal("Write to a symlinked log succeeded, want refusal")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error = %v, want a symlink refusal", err)
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatalf("read outside file: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("symlink target was written through: %q", body)
	}
}

func TestSinkRefusesANonDirectoryRecordPath(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "hook")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if _, err := (Sink{Dir: blocked}).Write(sampleRecord(t, "allow")); err == nil {
		t.Fatal("Write into a file path succeeded, want refusal")
	}
}

// TestSinkLeavesNothingBehindWhenTheRecordFileFails is the "treat it as
// unrecorded" contract. An unwritable records directory used to leave a
// hash-valid line in the audit log carrying the PRE-degradation verdict, while
// the caller escalated the verdict it actually enforced — evidence strictly more
// permissive than what happened.
func TestSinkLeavesNothingBehindWhenTheRecordFileFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "hook")
	sink := Sink{Dir: dir}
	if _, err := sink.Write(sampleRecord(t, "allow")); err != nil {
		t.Fatalf("seed Write: %v", err)
	}
	logPath := filepath.Join(dir, DecisionLogName)
	before := logLineCount(t, logPath)

	recordsDir := filepath.Join(dir, RecordsDirName)
	if err := os.Chmod(recordsDir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(recordsDir, 0o700) })

	if _, err := sink.Write(sampleRecord(t, "allow")); err == nil {
		t.Fatal("Write into an unwritable records directory succeeded, want refusal")
	}
	if after := logLineCount(t, logPath); after != before {
		t.Fatalf("log grew from %d to %d lines for a decision the caller must treat as unrecorded", before, after)
	}
}

// The mirror case: when the LOG append fails, the per-record file written a
// moment earlier is rolled back, so neither artifact survives a failed write.
func TestSinkRollsBackTheRecordFileWhenTheLogFails(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hook")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A symlinked log is the reproducible append failure: the sink refuses it.
	outside := filepath.Join(root, "outside.jsonl")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatalf("seed outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, DecisionLogName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := (Sink{Dir: dir}).Write(sampleRecord(t, "deny")); err == nil {
		t.Fatal("Write with a symlinked log succeeded, want refusal")
	}
	entries, err := os.ReadDir(filepath.Join(dir, RecordsDirName))
	if err != nil {
		t.Fatalf("read records dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("records directory holds %d file(s) for a failed write: %v", len(entries), entries)
	}
}

// TestWriteErrorSummaryOmitsThePath keeps the record location out of the text
// that reaches the model transcript, while Error keeps it for the operator.
func TestWriteErrorSummaryOmitsThePath(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "hook")
	sink := Sink{Dir: dir}
	if _, err := sink.Write(sampleRecord(t, "allow")); err != nil {
		t.Fatalf("seed Write: %v", err)
	}
	recordsDir := filepath.Join(dir, RecordsDirName)
	if err := os.Chmod(recordsDir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(recordsDir, 0o700) })

	_, err := sink.Write(sampleRecord(t, "allow"))
	if err == nil {
		t.Fatal("Write succeeded, want refusal")
	}
	var writeErr *WriteError
	if !errors.As(err, &writeErr) {
		t.Fatalf("error %T is not a *WriteError", err)
	}
	if !strings.Contains(writeErr.Error(), dir) {
		t.Fatalf("Error() = %q, want the full path for diagnostics", writeErr.Error())
	}
	if strings.Contains(writeErr.Summary(), dir) || strings.Contains(writeErr.Summary(), RecordsDirName) {
		t.Fatalf("Summary() = %q, want no record path", writeErr.Summary())
	}
	if !strings.Contains(writeErr.Summary(), "permission denied") {
		t.Fatalf("Summary() = %q, want the failure cause", writeErr.Summary())
	}
}

func logLineCount(t *testing.T, path string) int {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log %s: %v", path, err)
	}
	trimmed := strings.TrimRight(string(body), "\n")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

// TestRecordFileNameDerivesOnlyFromBoundaryValues pins that nothing from the
// event can steer a path: the name is the record's own UTC timestamp plus its
// record_id, which is derived from decision_hash.
func TestRecordFileNameDerivesOnlyFromBoundaryValues(t *testing.T) {
	record := sampleRecord(t, "deny")
	name := recordFileName(record, time.Now())
	if !strings.HasSuffix(name, "-"+record.RecordID+".json") {
		t.Fatalf("name = %q, want it to end with the record id", name)
	}
	if strings.ContainsAny(name, "/\\") {
		t.Fatalf("name = %q, want no path separators", name)
	}
	if _, err := time.Parse(recordTimestampFormat, strings.SplitN(name, "-", 2)[0]); err != nil {
		t.Fatalf("name %q does not lead with a parseable UTC timestamp: %v", name, err)
	}
}

func TestRecordFileNameFallsBackWhenTheRecordIsBlank(t *testing.T) {
	name := recordFileName(governance.DecisionRecordV1{}, time.Unix(0, 0).UTC())
	if !strings.HasSuffix(name, "-rec_unknown.json") {
		t.Fatalf("name = %q, want the explicit unknown-id fallback", name)
	}
}
