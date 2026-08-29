package boundarycli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDrillFixture materializes exactly what /boundary:drill's step 2 writes.
func writeDrillFixture(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(drillDirName, "vault"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := []byte("drill fixture -- safe to delete, never targeted for real\n")
	if err := os.WriteFile(filepath.Join(drillDirName, "vault", "fixture.txt"), body, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// TestDrillCleanupRemovesExactlyTheFixture is G3-A blocker 5's contract: after
// the drill, cleanup leaves no .boundary-drill residue — and touches nothing
// else, including the real .boundary evidence tree beside it.
func TestDrillCleanupRemovesExactlyTheFixture(t *testing.T) {
	t.Chdir(t.TempDir())
	writeDrillFixture(t)
	evidence := filepath.Join(".boundary", "hook")
	if err := os.MkdirAll(evidence, 0o700); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrill([]string{"cleanup"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Lstat(drillDirName); !os.IsNotExist(err) {
		t.Fatalf("%s still present after cleanup (err=%v)", drillDirName, err)
	}
	if _, err := os.Lstat(evidence); err != nil {
		t.Fatalf("cleanup touched the sibling evidence tree: %v", err)
	}
	if !strings.Contains(stdout.String(), "removed "+drillDirName) {
		t.Fatalf("stdout does not report the removal:\n%s", stdout.String())
	}
}

// TestDrillCleanupIsIdempotent: no fixture, no error — a re-run after success
// must not fail the drill's closing step.
func TestDrillCleanupIsIdempotent(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runDrill([]string{"cleanup"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to remove") {
		t.Fatalf("stdout does not state the no-op:\n%s", stdout.String())
	}
}

// TestDrillCleanupLeavesUnexpectedContent: content the drill did not write is
// listed and left, and the exit code says the cleanup declined.
func TestDrillCleanupLeavesUnexpectedContent(t *testing.T) {
	t.Chdir(t.TempDir())
	writeDrillFixture(t)
	stray := filepath.Join(drillDirName, "user-notes.md")
	if err := os.WriteFile(stray, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("write stray: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrill([]string{"cleanup"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 when content is left\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Lstat(stray); err != nil {
		t.Fatalf("cleanup removed content it does not own: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(drillDirName, "vault", "fixture.txt")); !os.IsNotExist(err) {
		t.Fatalf("the fixture itself was not removed (err=%v)", err)
	}
	if !strings.Contains(stderr.String(), "user-notes.md") {
		t.Fatalf("stderr does not name the leftover:\n%s", stderr.String())
	}
}

// TestDrillCleanupRefusesSymlinks: a symlinked drill directory or vault is an
// attempt to aim the cleanup somewhere else; both are refused unfollowed.
func TestDrillCleanupRefusesSymlinks(t *testing.T) {
	t.Chdir(t.TempDir())
	target := t.TempDir()
	sentinel := filepath.Join(target, "vault", "fixture.txt")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(sentinel, []byte("must survive\n"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	if err := os.Symlink(target, drillDirName); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runDrill([]string{"cleanup"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1 for a symlinked %s\nstdout=%s", code, drillDirName, stdout.String())
	}
	if _, err := os.Lstat(sentinel); err != nil {
		t.Fatalf("cleanup followed the symlink and removed %s: %v", sentinel, err)
	}
	if !strings.Contains(stderr.String(), "symlink") {
		t.Fatalf("stderr does not explain the refusal:\n%s", stderr.String())
	}
}

// TestDrillCleanupRejectsArguments: the verb takes none, so an argument is a
// wiring mistake, not a broader delete.
func TestDrillCleanupRejectsArguments(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := runDrill([]string{"cleanup", "extra"}, &stdout, &stderr); code != 1 {
		t.Fatalf("exit = %d, want 1 for an unexpected argument", code)
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("stderr does not explain the rejection:\n%s", stderr.String())
	}
}

// TestHookPreToolUsePrintRecordReturnsTheRecordIdentity is G3-A blocker 3's
// selection contract: the staged submission itself returns the identity of the
// record it wrote, so the drill verifies THAT record — never the newest file,
// which the drill's own next inspection would immediately outdate.
func TestHookPreToolUsePrintRecordReturnsTheRecordIdentity(t *testing.T) {
	dir := t.TempDir()
	event := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf .boundary-drill/vault"}}`

	var stdout, stderr bytes.Buffer
	code := runHook([]string{"pretooluse", "--dir", dir, "--print-record"}, strings.NewReader(event), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("stdout = %d line(s), want the decision then the pointer:\n%s", len(lines), stdout.String())
	}
	var pointer struct {
		SchemaVersion string `json:"schema_version"`
		RecordID      string `json:"record_id"`
		RecordPath    string `json:"record_path"`
		Verdict       string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &pointer); err != nil {
		t.Fatalf("pointer line is not JSON: %v\n%s", err, lines[1])
	}
	if pointer.SchemaVersion != recordPointerSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", pointer.SchemaVersion, recordPointerSchemaVersion)
	}
	if pointer.Verdict != "deny" {
		t.Fatalf("verdict = %q, want deny for the staged compound", pointer.Verdict)
	}
	if pointer.RecordID == "" || pointer.RecordPath == "" {
		t.Fatalf("pointer is missing the record identity: %+v", pointer)
	}
	body, err := os.ReadFile(pointer.RecordPath)
	if err != nil {
		t.Fatalf("record_path does not read back: %v", err)
	}
	if !strings.Contains(string(body), pointer.RecordID) {
		t.Fatalf("record at %s does not carry record_id %s", pointer.RecordPath, pointer.RecordID)
	}
}

// Without the flag, hook stdout is exactly the decision — the live wrapper's
// contract is byte-identical to before the flag existed.
func TestHookPreToolUseWithoutPrintRecordEmitsOnlyTheDecision(t *testing.T) {
	dir := t.TempDir()
	event := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf dist"}}`

	var stdout, stderr bytes.Buffer
	if code := runHook([]string{"pretooluse", "--dir", dir}, strings.NewReader(event), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %d line(s), want exactly the decision:\n%s", len(lines), stdout.String())
	}
	if strings.Contains(stdout.String(), recordPointerSchemaVersion) {
		t.Fatalf("pointer emitted without the flag:\n%s", stdout.String())
	}
}
