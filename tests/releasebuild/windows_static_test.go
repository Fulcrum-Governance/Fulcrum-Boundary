// Package releasebuild holds black-box tests that pin Boundary's release build
// matrix to its documented stance. windows_static_test.go enforces the permanent
// Windows static-only stance (issue #139): Windows ships the static
// (CGO_ENABLED=0) variant and the native-cgo release matrix carries no Windows
// lane. See docs/INSTALL.md "Static Vs Cgo Builds".
package releasebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from the test directory")
		}
		dir = parent
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// TestWindowsStaticBuildProduced asserts goreleaser still builds a Windows
// binary in the static (CGO_ENABLED=0) family — the only variant Windows users
// get. If Windows is dropped from the static build, they lose every supported
// archive.
func TestWindowsStaticBuildProduced(t *testing.T) {
	var gr struct {
		Builds []struct {
			Env  []string `yaml:"env"`
			Goos []string `yaml:"goos"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal([]byte(read(t, repoRoot(t), ".goreleaser.yaml")), &gr); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}
	for _, b := range gr.Builds {
		staticEnv, windows := false, false
		for _, e := range b.Env {
			if strings.Contains(e, "CGO_ENABLED=0") {
				staticEnv = true
			}
		}
		for _, g := range b.Goos {
			if g == "windows" {
				windows = true
			}
		}
		if staticEnv && windows {
			return // found the windows static build
		}
	}
	t.Fatal(".goreleaser.yaml no longer builds a Windows static (CGO_ENABLED=0) binary — Windows users would lose their only supported variant (#139 permanent static-only stance)")
}

// TestNoWindowsCgoLane pins the permanent stance that Windows ships static-only:
// the native-cgo release matrix must not gain a Windows runner. The cgo SQL
// classifier needs a C/MSYS2 toolchain the Windows release path does not carry;
// if a windows-cgo lane is ever added this fails, forcing a deliberate decision
// and a docs/INSTALL.md update rather than a silent capability change.
func TestNoWindowsCgoLane(t *testing.T) {
	var wf struct {
		Jobs map[string]struct {
			Strategy struct {
				Matrix struct {
					Include []struct {
						Goos string `yaml:"goos"`
					} `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(read(t, repoRoot(t), ".github/workflows/release.yml")), &wf); err != nil {
		t.Fatalf("parse .github/workflows/release.yml: %v", err)
	}
	cgo, ok := wf.Jobs["cgo-binaries"]
	if !ok {
		t.Fatal("release.yml has no cgo-binaries job — the matrix this guard reasons about moved")
	}
	if len(cgo.Strategy.Matrix.Include) == 0 {
		t.Fatal("cgo-binaries matrix include is empty — parse error or the matrix structure changed")
	}
	for _, inc := range cgo.Strategy.Matrix.Include {
		if inc.Goos == "windows" {
			t.Fatal("cgo-binaries matrix added a windows-cgo lane — Boundary's documented stance (docs/INSTALL.md) is Windows static-only; if this is intentional, update the stance + docs and remove this guard")
		}
	}
}
