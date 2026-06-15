// Package supplychain holds black-box tests that pin Boundary's release
// supply-chain wiring (SBOM generation + build-provenance attestation) so it
// cannot silently regress. These assert the pipeline CONFIGURATION is present;
// the artifacts themselves are produced only by a real release run (provenance)
// or a `goreleaser release --snapshot` (SBOM). See docs/SUPPLY_CHAIN.md and
// BND-CLAIM-DIST-002.
package supplychain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot walks up from the test directory to the module root (the directory
// holding go.mod) so the test is independent of where `go test` is invoked.
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

// TestSBOMConfigured pins the SPDX SBOM wiring in .goreleaser.yaml. It fails the
// build if SBOM generation for the static archives is removed or its format
// changed away from SPDX/syft — verified to actually run via
// `goreleaser release --snapshot` (six archives produce six *.spdx.json).
func TestSBOMConfigured(t *testing.T) {
	gr := read(t, repoRoot(t), ".goreleaser.yaml")
	for _, want := range []string{"sboms:", "artifacts: archive", "cmd: syft", "spdx-json=$document"} {
		if !strings.Contains(gr, want) {
			t.Fatalf(".goreleaser.yaml is missing SBOM wiring %q — SPDX SBOM generation (BND-CLAIM-DIST-002) would silently stop", want)
		}
	}
}

// TestSyftInstalledInReleaseWorkflow guards the release-breaking regression the
// adversarial review reproduced: goreleaser execs `syft` from PATH to build the
// SBOMs, but neither ubuntu-latest nor goreleaser-action provides it. Without an
// explicit install step a `v*` tag release (and the dispatch dry-run) aborts at
// the SBOM stage with `exec: "syft": executable file not found`. `goreleaser
// check` cannot catch this (it validates config, never runs the cmd), so this
// test is the gate that does.
func TestSyftInstalledInReleaseWorkflow(t *testing.T) {
	wf := read(t, repoRoot(t), ".github/workflows/release.yml")
	if !strings.Contains(wf, "download-syft@") {
		t.Fatal("release.yml does not install syft (anchore/sbom-action/download-syft) before goreleaser runs — goreleaser execs syft for SBOMs and the runner has none, so a v* release aborts at the SBOM stage")
	}
}

// releaseWorkflow is the slice of the workflow schema this test reasons about.
type releaseWorkflow struct {
	Jobs map[string]struct {
		Permissions map[string]string `yaml:"permissions"`
		Steps       []struct {
			Name string `yaml:"name"`
			Uses string `yaml:"uses"`
			If   string `yaml:"if"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// TestProvenanceAttestationWired attributes the attestation wiring to the actual
// jobs (parsed YAML, not whole-file substring counts): the SHA-pinned action,
// each job's id-token/attestations permissions, a tag-gated attest step per
// artifact family, and that no attest step runs un-gated on dispatch dry-runs.
func TestProvenanceAttestationWired(t *testing.T) {
	wf := read(t, repoRoot(t), ".github/workflows/release.yml")

	// Repo convention: actions are pinned to a commit SHA, not a moving tag.
	const pinnedAction = "actions/attest-build-provenance@96278af6caaf10aea03fd8d33a09a777ca52d62f"
	if !strings.Contains(wf, pinnedAction) {
		t.Fatalf("release.yml no longer pins %s — build-provenance attestation (BND-CLAIM-DIST-002) is removed or unpinned", pinnedAction)
	}

	var doc releaseWorkflow
	if err := yaml.Unmarshal([]byte(wf), &doc); err != nil {
		t.Fatalf("parse release.yml: %v", err)
	}
	// Both artifact families must be attested: static (goreleaser job) and
	// native-cgo (cgo-binaries job).
	for _, jobName := range []string{"goreleaser", "cgo-binaries"} {
		job, ok := doc.Jobs[jobName]
		if !ok {
			t.Fatalf("release.yml missing job %q", jobName)
		}
		if job.Permissions["id-token"] != "write" || job.Permissions["attestations"] != "write" {
			t.Fatalf("job %q lacks id-token:write + attestations:write; attestation fails at release time (permissions=%v)", jobName, job.Permissions)
		}
		attestSteps := 0
		for _, s := range job.Steps {
			if !strings.Contains(s.Uses, "attest-build-provenance@") {
				continue
			}
			attestSteps++
			if !strings.Contains(s.If, "tag") {
				t.Fatalf("job %q attestation step %q is not tag-gated (if=%q) — it would run on workflow_dispatch dry-runs", jobName, s.Name, s.If)
			}
		}
		if attestSteps < 1 {
			t.Fatalf("job %q has no attest-build-provenance step", jobName)
		}
	}
}
