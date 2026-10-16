package adapters_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLIDirectShellBypassLimitation(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "direct-shell.txt")
	cmd := exec.Command("sh", "-c", "printf bypass > \"$1\"", "sh", marker)
	if err := cmd.Run(); err != nil {
		t.Fatalf("direct shell command failed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("direct shell command did not create marker: %v", err)
	}
}
