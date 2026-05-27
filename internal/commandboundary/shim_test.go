package commandboundary

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInstallProjectShimsCreatesExecutableScripts(t *testing.T) {
	root := t.TempDir()

	result, err := InstallProjectShims(root, []string{"git", "npm", "git"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("created = %d, want 2: %#v", len(result.Created), result.Created)
	}
	if !slices.Equal(result.Commands, []string{"git", "npm"}) {
		t.Fatalf("commands = %#v", result.Commands)
	}

	path := filepath.Join(root, ".boundary", "bin", "git")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), shimMarker) || !strings.Contains(string(body), `exec boundary command run -- git "$@"`) {
		t.Fatalf("unexpected shim body:\n%s", string(body))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("shim is not executable: %s", info.Mode())
	}
}

func TestUninstallProjectShimsRemovesOnlyBoundaryShims(t *testing.T) {
	root := t.TempDir()
	if _, err := InstallProjectShims(root, []string{"git"}); err != nil {
		t.Fatal(err)
	}
	customPath := filepath.Join(root, ".boundary", "bin", "custom")
	if err := os.WriteFile(customPath, []byte("#!/usr/bin/env sh\necho custom\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := UninstallProjectShims(root, []string{"git", "custom", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 1 {
		t.Fatalf("removed = %#v", result.Removed)
	}
	if _, err := os.Stat(filepath.Join(root, ".boundary", "bin", "git")); !os.IsNotExist(err) {
		t.Fatalf("git shim still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("custom non-Boundary shim was removed: %v", err)
	}
	if len(result.Skipped) != 2 {
		t.Fatalf("skipped = %#v", result.Skipped)
	}
}

func TestShellEnvironmentIsProjectLocal(t *testing.T) {
	root := t.TempDir()

	env, err := ShellEnvironment(root, []string{
		"PATH=/usr/bin",
		"BOUNDARY_COMMAND_MODE=old",
		"BOUNDARY_PROJECT_ROOT=/old",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := ProjectBinDir(root) + string(os.PathListSeparator)
	if got := lookupEnv(env, "PATH"); !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("PATH = %q, want prefix %q", got, wantPrefix)
	}
	if got := lookupEnv(env, EnvCommandMode); got != "project" {
		t.Fatalf("%s = %q", EnvCommandMode, got)
	}
	if got := lookupEnv(env, EnvProjectRoot); got != root {
		t.Fatalf("%s = %q", EnvProjectRoot, got)
	}
}
