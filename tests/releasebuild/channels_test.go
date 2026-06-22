// channels_test.go pins the install-CHANNEL wiring behind BND-CLAIM-DIST-001:
// the tag-gated release pipeline must declare every one-command channel —
// static archives, the SHA256SUMS checksum manifest, the Homebrew tap formula,
// and the ghcr.io container image (plus the native-cgo archive + SHA256SUMS-cgo
// lane). Like its sibling windows_static_test.go and tests/supplychain, these
// assert the pipeline CONFIGURATION is present and tag-gated; they do NOT claim a
// live publish (a real publish happens only on a v* tag, recorded out-of-band in
// docs/RELEASE_TRUTH_PUBLIC.md). If a channel is removed from the release config
// the build breaks with a claim-traceable error rather than silently shipping a
// narrower DIST-001.
package releasebuild

import (
	"strings"
	"testing"
)

// goreleaserConfig and releaseWorkflow are read once per test from the repo root.
func goreleaser(t *testing.T) string { return read(t, repoRoot(t), ".goreleaser.yaml") }
func releaseWF(t *testing.T) string  { return read(t, repoRoot(t), ".github/workflows/release.yml") }

func mustContainAll(t *testing.T, name, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Fatalf("%s: missing %q — BND-CLAIM-DIST-001 install-channel wiring would silently narrow", name, w)
		}
	}
}

// TestStaticArchivesConfigured: the static archive channel exists (the
// `_static-nocgo` archives that SHA256SUMS and the Homebrew formula install).
func TestStaticArchivesConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml archives", goreleaser(t),
		"archives:", "id: static-archives", "_static-nocgo")
}

// TestChecksumManifestConfigured: the SHA256SUMS checksum manifest channel exists.
func TestChecksumManifestConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml checksum", goreleaser(t),
		"checksum:", "name_template: SHA256SUMS", "algorithm: sha256")
}

// TestHomebrewFormulaConfigured: the Homebrew channel publishes the static
// archives to the fulcrum-governance/homebrew-tap formula.
func TestHomebrewFormulaConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml brews", goreleaser(t),
		"brews:", "name: homebrew-tap", "static-archives")
}

// TestContainerImageConfigured: the container-image channel exists — per-arch
// images plus the multi-arch ghcr.io manifest.
func TestContainerImageConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml dockers", goreleaser(t),
		"dockers:", "docker_manifests:", "ghcr.io/fulcrum-governance/boundary")
}

// TestReleasePipelineTagGated: channels publish from a TAG-gated pipeline — the
// load-bearing "tag-gated" word in the claim. workflow_dispatch runs are
// snapshot dry-runs; only a v* tag publishes.
func TestReleasePipelineTagGated(t *testing.T) {
	mustContainAll(t, "release.yml trigger", releaseWF(t), "tags: ['v*']")
}

// TestCgoArchiveChannelWired: the native-cgo archive channel and its separate
// SHA256SUMS-cgo manifest are wired (complements tests/supplychain/wiring_test.go,
// which pins the SBOM/provenance on these same jobs).
func TestCgoArchiveChannelWired(t *testing.T) {
	mustContainAll(t, "release.yml cgo channel", releaseWF(t),
		"cgo-binaries:", "cgo-checksums:", "SHA256SUMS-cgo")
}
