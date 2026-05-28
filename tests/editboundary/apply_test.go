package editboundary_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/editboundary"
)

func TestEditApplyAllowedPatchAppliesAndRecords(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, filepath.Join(dir, "docs", "example.md"), "# Example\n")
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "docs.diff"), "--record-out", recordPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "docs", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# Example\nMore detail.\n" {
		t.Fatalf("patched body = %q", string(body))
	}
	record := readEditRecord(t, recordPath)
	if !record.Applied || !record.ApplierInvoked || record.Action != "allow" {
		t.Fatalf("record = %#v", record)
	}
}

func TestEditApplyDeniedPatchDoesNotInvokeApplier(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "secret.diff"), "--record-out", recordPath}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !errorsIsNotExist(err) {
		t.Fatalf("secret-bearing file was created or unexpected stat error: %v", err)
	}
	record := readEditRecord(t, recordPath)
	if record.Applied || record.ApplierInvoked || record.Action != "deny" {
		t.Fatalf("record = %#v", record)
	}
}

func TestEditApplyApprovalDoesNotOverrideHardDeny(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "secret.diff"), "--record-out", recordPath, "--require-approval"}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); !errorsIsNotExist(err) {
		t.Fatalf("secret-bearing file was created or unexpected stat error: %v", err)
	}
	record := readEditRecord(t, recordPath)
	if record.Applied || record.ApplierInvoked || record.Action != "deny" || !record.ApprovalPresent {
		t.Fatalf("record = %#v", record)
	}
	if record.ApprovalMode != "local_flag" {
		t.Fatalf("approval mode = %q", record.ApprovalMode)
	}
}

func TestEditApplyRequireApprovalGate(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, filepath.Join(dir, "package.json"), "{\n  \"name\": \"example\"\n}\n")
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "package-scripts.diff"), "--record-out", recordPath}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit without approval = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "postinstall") {
		t.Fatalf("package script patch applied without approval:\n%s", string(body))
	}

	stdout.Reset()
	stderr.Reset()
	code = boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "package-scripts.diff"), "--record-out", recordPath, "--require-approval"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit with approval = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	body, err = os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "postinstall") {
		t.Fatalf("approved package script patch was not applied:\n%s", string(body))
	}
	records := readEditRecords(t, recordPath)
	if len(records) != 2 || records[0].Applied || !records[1].Applied || !records[1].ApprovalPresent {
		t.Fatalf("records = %#v", records)
	}
}

func TestEditApplyDryRunNeverInvokesApplier(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeFile(t, filepath.Join(dir, "docs", "example.md"), "# Example\n")
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", fixturePath(t, "docs.diff"), "--record-out", recordPath, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	body, err := os.ReadFile(filepath.Join(dir, "docs", "example.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "# Example\n" {
		t.Fatalf("dry-run applied patch: %q", string(body))
	}
	record := readEditRecord(t, recordPath)
	if record.Applied || record.ApplierInvoked || !record.DryRun {
		t.Fatalf("record = %#v", record)
	}
}

func TestEditApplyTraversalPatchDoesNotTouchOutsideFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	outside := filepath.Join(filepath.Dir(dir), "outside-edit-boundary-sentinel.txt")
	writeFile(t, outside, "keep\n")
	t.Cleanup(func() { _ = os.Remove(outside) })
	patchPath := filepath.Join(dir, "traversal.diff")
	writeFile(t, patchPath, `diff --git a/../outside-edit-boundary-sentinel.txt b/../outside-edit-boundary-sentinel.txt
--- a/../outside-edit-boundary-sentinel.txt
+++ b/../outside-edit-boundary-sentinel.txt
@@ -1 +1 @@
-keep
+changed
`)
	recordPath := filepath.Join(dir, "records.jsonl")

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"edit", "apply", "--patch", patchPath, "--record-out", recordPath}, &stdout, &stderr)
	if code != 126 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "keep\n" {
		t.Fatalf("outside sentinel changed: %q", string(body))
	}
	record := readEditRecord(t, recordPath)
	if record.Applied || record.ApplierInvoked || record.Class != editboundary.ClassOutsideProjectScope {
		t.Fatalf("record = %#v", record)
	}
}

func fixturePath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "fixtures", "editboundary", name)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readEditRecord(t *testing.T, path string) editboundary.EditDecisionRecord {
	t.Helper()
	records := readEditRecords(t, path)
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	return records[0]
}

func readEditRecords(t *testing.T, path string) []editboundary.EditDecisionRecord {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	records := make([]editboundary.EditDecisionRecord, 0, len(lines))
	for _, line := range lines {
		var record editboundary.EditDecisionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode record: %v\n%s", err, line)
		}
		records = append(records, record)
	}
	return records
}

func errorsIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
