package commandboundary_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/commandboundary"
)

func TestCommandInstallProjectCreatesShims(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "install", "--project", "--project-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Installed 18 project shims in .boundary/bin") {
		t.Fatalf("unexpected install output:\n%s", stdout.String())
	}
	gitShim := filepath.Join(root, ".boundary", "bin", "git")
	body, err := os.ReadFile(gitShim)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `exec boundary command run -- git "$@"`) {
		t.Fatalf("unexpected git shim:\n%s", string(body))
	}
}

func TestCommandUninstallProjectRemovesShims(t *testing.T) {
	root := t.TempDir()
	if code := boundarycli.Run([]string{"command", "install", "--project", "--project-root", root}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("install exit = %d", code)
	}

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "uninstall", "--project", "--project-root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Removed 18 project shims from .boundary/bin") {
		t.Fatalf("unexpected uninstall output:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".boundary", "bin", "git")); !os.IsNotExist(err) {
		t.Fatalf("git shim still exists or stat failed: %v", err)
	}
}

func TestCommandInstallRequiresProjectScope(t *testing.T) {
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"command", "install"}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("expected install without --project to fail")
	}
	if !strings.Contains(stderr.String(), "--project is required") {
		t.Fatalf("unexpected stderr:\n%s", stderr.String())
	}
}

func TestBoundaryShellPrintEnvIsProjectLocal(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"shell", "--project-root", root, "--print-env"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"Boundary Command Shell",
		"Shims: .boundary/bin",
		"Direct commands without shims are outside Boundary.",
		`export BOUNDARY_COMMAND_MODE="project"`,
		`export BOUNDARY_PROJECT_ROOT="` + root + `"`,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("shell output missing %q:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".boundary", "bin", "git")); err != nil {
		t.Fatalf("shell did not install project shims: %v", err)
	}
	for _, profile := range []string{".zshrc", ".bashrc", ".profile", filepath.Join(".config", "fish", "config.fish")} {
		if _, err := os.Stat(filepath.Join(root, profile)); !os.IsNotExist(err) {
			t.Fatalf("shell wrote global profile-like file %s: %v", profile, err)
		}
	}
}

func TestBoundaryShellNoInstallDoesNotCreateShims(t *testing.T) {
	root := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := boundarycli.Run([]string{"shell", "--project-root", root, "--no-install", "--print-env"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(commandboundary.ProjectBinDir(root)); !os.IsNotExist(err) {
		t.Fatalf("shell --no-install created shim directory: %v", err)
	}
}
