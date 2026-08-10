// channels_test.go pins the install-CHANNEL wiring behind BND-CLAIM-DIST-001:
// the tag-gated release pipeline must declare every one-command channel —
// static archives, the SHA256SUMS checksum manifest, the Homebrew tap cask,
// and the ghcr.io container image (plus the native-cgo archive + SHA256SUMS-cgo
// lane). Like its sibling windows_static_test.go and tests/supplychain, these
// assert the pipeline CONFIGURATION is present and tag-gated; they do NOT claim a
// live publish (a real publish happens only on a v* tag, recorded out-of-band in
// docs/RELEASE_TRUTH_PUBLIC.md). If a channel is removed from the release config
// the build breaks with a claim-traceable error rather than silently shipping a
// narrower DIST-001.
package releasebuild

import (
	"regexp"
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
// `_static-nocgo` archives that SHA256SUMS and the Homebrew cask install).
func TestStaticArchivesConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml archives", goreleaser(t),
		"archives:", "id: static-archives", "_static-nocgo")
}

// TestChecksumManifestConfigured: the SHA256SUMS checksum manifest channel exists.
func TestChecksumManifestConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml checksum", goreleaser(t),
		"checksum:", "name_template: SHA256SUMS", "algorithm: sha256")
}

// TestHomebrewCaskConfigured: the Homebrew channel publishes the static
// archives to the fulcrum-governance/homebrew-tap cask.
func TestHomebrewCaskConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml homebrew_casks", goreleaser(t),
		"homebrew_casks:", "name: homebrew-tap", "static-archives")
	if strings.Contains(goreleaser(t), "\nbrews:") {
		t.Fatal(".goreleaser.yaml: deprecated brews configuration remains")
	}
}

// TestContainerImageConfigured: the multi-platform container-image channel
// exists through GoReleaser's current dockers_v2 configuration.
func TestContainerImageConfigured(t *testing.T) {
	mustContainAll(t, ".goreleaser.yaml dockers_v2", goreleaser(t),
		"dockers_v2:", "linux/amd64", "linux/arm64",
		"ghcr.io/fulcrum-governance/boundary")
	if strings.Contains(goreleaser(t), "\ndockers:") || strings.Contains(goreleaser(t), "\ndocker_manifests:") {
		t.Fatal(".goreleaser.yaml: deprecated docker configuration remains")
	}
}

// TestReleasePipelineTagGated: channels publish only from a root v* tag or a
// recovery dispatch whose existing annotated root tag and peeled commit passed
// release-target validation. A no-input workflow_dispatch remains a snapshot.
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

// TestReleaseRecoveryUsesOneValidatedTarget pins the recovery contract added
// after a nested component tag was selected instead of the root release tag.
// A recovery dispatch must validate its existing annotated root SemVer tag and
// full peeled commit before any publish condition can run; every release build
// then checks out that exact commit and receives the same tag explicitly.
func TestReleaseRecoveryUsesOneValidatedTarget(t *testing.T) {
	wf := releaseWF(t)
	mustContainAll(t, "release.yml recovery inputs", wf,
		"workflow_dispatch:", "release_tag:", "expected_commit:", "expected_tag_object:")
	mustContainAll(t, "release.yml fail-closed target validation", wf,
		"git fetch --force --tags origin",
		"release tag must be a root tag, not a nested component tag",
		"release tag must be a strict root SemVer 2.0.0 tag (vMAJOR.MINOR.PATCH)",
		"release tag must be annotated; lightweight tags cannot be recovered",
		"expected_commit must be a full 40-character lowercase commit SHA",
		"expected_tag_object must be a full 40-character lowercase annotated-tag object SHA",
		"release tag object does not match expected_tag_object",
		"release tag peeled commit does not match expected_commit")
	mustContainAll(t, "release.yml exact checkout", wf,
		"release-check:", "goreleaser:", "cgo-binaries:", "cgo-checksums:",
		"ref: ${{ needs.release-target.outputs.release_sha }}")
	if got := strings.Count(wf, "ref: ${{ needs.release-target.outputs.release_sha }}"); got != 4 {
		t.Fatalf("release.yml has %d validated-target checkouts; release-check, goreleaser, cgo-binaries, and cgo-checksums must each use one", got)
	}
	mustContainAll(t, "release.yml target propagation", wf,
		"GORELEASER_CURRENT_TAG: ${{ needs.release-target.outputs.release_tag }}",
		"RELEASE_TAG: ${{ needs.release-target.outputs.release_tag }}",
		"gh release upload \"${{ needs.release-target.outputs.release_tag }}\"")
	mustContainAll(t, "release.yml tag-push and recovery bindings", wf,
		"expected_commit=\"$WORKFLOW_SHA\"",
		"expected_tag_object=\"$EXPECTED_TAG_OBJECT\"",
		"group: release-${{ inputs.release_tag || github.ref_name }}")

	if strings.Contains(wf, "github.ref_type") || strings.Contains(wf, "GITHUB_REF_NAME") {
		t.Fatal("release.yml still derives recovery publication from GitHub ref-name/type state instead of the validated release-target outputs")
	}
	if got := strings.Count(wf, "if: needs.release-target.outputs.publish == 'true'"); got < 7 {
		t.Fatalf("release.yml has only %d validated publication gates; release, Homebrew/container, cgo upload, checksums, and attestations must all require the validated target", got)
	}
}

// TestReleaseTagSemVerIsStrict pins the exact SemVer 2.0 validation used by
// release-target. Root core numbers cannot contain leading zeroes; prerelease
// and build identifiers are nonempty; numeric prerelease identifiers likewise
// cannot contain leading zeroes.
func TestReleaseTagSemVerIsStrict(t *testing.T) {
	const strictRootSemVer = `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?$`
	if !strings.Contains(releaseWF(t), "semver_re='"+strictRootSemVer+"'") {
		t.Fatal("release.yml no longer uses the strict SemVer 2.0 root-tag expression")
	}

	re := regexp.MustCompile(strictRootSemVer)
	for _, tag := range []string{"v0.0.0", "v1.2.3", "v1.2.3-alpha.1", "v1.2.3+build.001", "v1.2.3-alpha-1+build.2"} {
		if !re.MatchString(tag) {
			t.Fatalf("strict SemVer expression rejected valid tag %q", tag)
		}
	}
	for _, tag := range []string{"v01.2.3", "v1.02.3", "v1.2.03", "v1.2.3-01", "v1.2.3-..", "v1.2.3-alpha..1", "v1.2.3+build..1", "v1.2.3+", "v1.2"} {
		if re.MatchString(tag) {
			t.Fatalf("strict SemVer expression accepted invalid tag %q", tag)
		}
	}
}

// TestNoInputDispatchIsSnapshotOnly ensures manual dry-runs cannot enter a
// release path merely because a workflow was manually triggered. All recovery
// inputs are required to change the target job's publish output from false to
// true.
func TestNoInputDispatchIsSnapshotOnly(t *testing.T) {
	wf := releaseWF(t)
	mustContainAll(t, "release.yml no-input dispatch", wf,
		"if [ -z \"$DISPATCH_TAG\" ] && [ -z \"$EXPECTED_COMMIT\" ] && [ -z \"$EXPECTED_TAG_OBJECT\" ]; then",
		"echo 'publish=false'",
		"recovery publication requires release_tag, expected_commit, and expected_tag_object",
		"if: needs.release-target.outputs.publish != 'true'",
		"goreleaser snapshot dry-run (manual dispatch)",
		"GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}")
}

// TestBuildxPrecedesGoReleaser pins the Buildx-capable driver needed for the
// linux/amd64 + linux/arm64 container manifest. A default docker driver cannot
// publish that matrix, so setup must remain SHA-pinned and before GoReleaser.
func TestBuildxPrecedesGoReleaser(t *testing.T) {
	wf := releaseWF(t)
	const buildx = "docker/setup-buildx-action@e468171a9de216ec08956ac3ada2f0791b6bd435"
	buildxAt := strings.Index(wf, buildx)
	goreleaserAt := strings.Index(wf, "goreleaser/goreleaser-action@")
	if buildxAt < 0 {
		t.Fatalf("release.yml does not pin %s — multi-platform container publication would use the unsupported default docker driver", buildx)
	}
	if goreleaserAt < 0 || buildxAt > goreleaserAt {
		t.Fatal("release.yml configures Buildx after GoReleaser — the multi-platform container build can start with the unsupported default docker driver")
	}
}
