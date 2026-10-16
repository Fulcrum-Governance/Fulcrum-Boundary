package boundarycli

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits and ownership")
	}
}

func permOf(t *testing.T, path string) fs.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

// The serve record dir holds decision identities and reasons, so serve keeps it
// owner-only: it creates a missing dir 0700, tightens an owned dir that grants
// group or other access, and refuses a dir it does not own, a symlink, or a
// non-directory.
func TestPrepareServeRecordDir(t *testing.T) {
	skipOnWindows(t)

	t.Run("missing dir is created 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "serve")
		note, err := prepareServeRecordDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if note != "" {
			t.Fatalf("new dir reported a tightening: %q", note)
		}
		if got := permOf(t, dir); got != 0o700 {
			t.Fatalf("perm = %o, want 700", got)
		}
	})

	t.Run("owned dir with group or other bits is tightened to 0700", func(t *testing.T) {
		for _, mode := range []fs.FileMode{0o755, 0o750, 0o705, 0o777} {
			dir := filepath.Join(t.TempDir(), "serve")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, mode); err != nil {
				t.Fatal(err)
			}
			note, err := prepareServeRecordDir(dir)
			if err != nil {
				t.Fatalf("%o: %v", mode, err)
			}
			if got := permOf(t, dir); got != 0o700 {
				t.Fatalf("%o: perm = %o, want 700", mode, got)
			}
			if !strings.Contains(note, dir) || !strings.Contains(note, "0700") {
				t.Fatalf("%o: note %q does not name the dir and the new mode", mode, note)
			}
		}
	})

	t.Run("owned 0700 dir is left alone", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "serve")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		note, err := prepareServeRecordDir(dir)
		if err != nil || note != "" {
			t.Fatalf("note=%q err=%v, want neither", note, err)
		}
	})

	t.Run("dir owned by another user is refused", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "serve")
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		orig := serveCurrentUID
		serveCurrentUID = func() int { return os.Geteuid() + 1 }
		defer func() { serveCurrentUID = orig }()
		_, err := prepareServeRecordDir(dir)
		if err == nil || !strings.Contains(err.Error(), "not owned by the current user") {
			t.Fatalf("err = %v, want a not-owned refusal", err)
		}
		if got := permOf(t, dir); got != 0o755 {
			t.Fatalf("refused dir was changed to %o", got)
		}
	})

	t.Run("symlink is refused", func(t *testing.T) {
		target := t.TempDir()
		link := filepath.Join(t.TempDir(), "serve")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		_, err := prepareServeRecordDir(link)
		if err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("err = %v, want a symlink refusal", err)
		}
	})

	t.Run("non-directory is refused", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "serve")
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := prepareServeRecordDir(file)
		if err == nil || !strings.Contains(err.Error(), "not a directory") {
			t.Fatalf("err = %v, want a not-a-directory refusal", err)
		}
	})
}

// Record files and the decision log are written 0600 and records/ 0700.
func TestServeRecordPublisher_WritesOwnerOnlyFiles(t *testing.T) {
	skipOnWindows(t)
	dir := filepath.Join(t.TempDir(), "serve")
	pub, err := newServeRecordPublisher(governance.NewSlogAuditPublisher(slog.New(slog.NewTextHandler(io.Discard, nil))), dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.PublishChecked(context.Background(), governance.AuditEvent{
		EventType: "governance_decision", Transport: governance.TransportMCP, Action: "deny", Reason: "r",
		DecisionMode: governance.DecisionModeDeterministic,
	}); err != nil {
		t.Fatal(err)
	}
	if got := permOf(t, filepath.Join(dir, "records")); got != 0o700 {
		t.Fatalf("records/ perm = %o, want 700", got)
	}
	if got := permOf(t, pub.logPath()); got != 0o600 {
		t.Fatalf("log perm = %o, want 600", got)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "records", "*.json"))
	if len(files) != 1 {
		t.Fatalf("records = %v, want 1", files)
	}
	if got := permOf(t, files[0]); got != 0o600 {
		t.Fatalf("record perm = %o, want 600", got)
	}
}
