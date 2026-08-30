// install_drift_test.go pins every CANONICAL install / version reference in the
// user-facing docs to the current release, so the install docs cannot silently
// drift behind a published tag again (the failure this guard is built for: the
// install docs kept pinning a superseded release tag long after a newer release
// shipped).
//
// The release-stamp target is the "Current release target: `vX.Y.Z`" line in
// docs/RELEASE_TRUTH_PUBLIC.md. A stamp candidate is internally consistent for
// its future tag while its release-truth section still states that publication
// has not happened.
//
// Only genuinely historical surfaces are exempt (per-version release notes,
// archived internal reconciliations, the append-only changelog, the LOCKED spec
// authoring baseline, and the version-titled v0.9.0 launch artifact). Every
// active install/usage doc — including the ones this lane fixes — is GOVERNED,
// which is the whole point of the guard.
package docs

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	currentReleaseAnchor     = regexp.MustCompile("(?m)^Current release target: `(v[0-9]+\\.[0-9]+\\.[0-9]+)`")
	releaseStampAnchor       = regexp.MustCompile("(?m)^Release-stamp target: `(v[0-9]+\\.[0-9]+\\.[0-9]+)` — \\*\\*not published\\*\\*$")
	currentReleaseDateAnchor = regexp.MustCompile("(?m)^Current release date: `([0-9]{4}-[0-9]{2}-[0-9]{2})`$")

	// Canonical, copy-pasteable install / version references that must track the
	// current release. Each capturing group yields the pinned version.
	canonicalInstallRefs = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"go install cmd/boundary", regexp.MustCompile(`github\.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"go install verify-witnessed", regexp.MustCompile(`github\.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"actions/mcp-audit ref", regexp.MustCompile(`Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"ghcr.io container tag", regexp.MustCompile(`ghcr\.io/fulcrum-governance/boundary:(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"surface-status diagram node", regexp.MustCompile(`\[Boundary (v[0-9]+\.[0-9]+\.[0-9]+)\]`)},
	}
	prePublicationConditionals = []string{
		"when the approved",
		"once both approved",
		"once the approved",
		"until then",
		"tags publish",
		"will publish",
		"not yet published",
		"once published",
	}
)

// historical surfaces that legitimately pin an older tag; exempt from the guard.
func isHistoricalSurface(rel string) bool {
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "docs/internal/") || strings.HasPrefix(rel, "docs/releases/") {
		return true
	}
	switch rel {
	case "CHANGELOG.md", // append-only history + compare links
		"docs/LAUNCH_README.md": // version-titled v0.9.0 launch artifact
		return true
	}
	return false
}

func TestCanonicalInstallRefsTrackCurrentRelease(t *testing.T) {
	root := repoRoot(t)
	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	anchorMatches := currentReleaseAnchor.FindAllStringSubmatch(releaseTruth, -1)
	if len(anchorMatches) != 1 {
		t.Fatalf("expected exactly one \"Current release target: `vX.Y.Z`\" anchor in docs/RELEASE_TRUTH_PUBLIC.md, found %d — the drift anchor moved or was reformatted", len(anchorMatches))
	}
	want := anchorMatches[0][1]

	refsFound := 0
	tracked, trackedErr := trackedMarkdownSurfaces(root)
	if trackedErr != nil {
		t.Skipf("skipping tracked markdown install-drift guard: git tracked-file list unavailable outside a git checkout: %v", trackedErr)
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".mmd" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if !tracked[filepath.ToSlash(rel)] {
			return nil
		}
		if isHistoricalSurface(rel) || filepath.ToSlash(rel) == "docs/RELEASE_TRUTH_PUBLIC.md" {
			return nil
		}
		body := read(t, root, rel)
		for _, ref := range canonicalInstallRefs {
			for _, m := range ref.re.FindAllStringSubmatch(body, -1) {
				refsFound++
				if got := m[1]; got != want {
					t.Errorf("install drift in %s: %s pins %s but the current release is %s — bump it (or, if genuinely historical, add the file to isHistoricalSurface)", rel, ref.name, got, want)
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking the repo for install references: %v", walkErr)
	}
	if refsFound == 0 {
		t.Fatal("found zero canonical install references across the non-historical docs — the scan scope or patterns broke, so this guard is vacuous")
	}

	stampStart := strings.Index(releaseTruth, "## Release-stamp status — not published")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before this release stamp:")
	if stampStart == -1 || baselineStart == -1 || stampStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: release-stamp section must precede its published baseline")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md release-stamp section", releaseTruth[stampStart:baselineStart], want)
}

func TestNoPrePublicationConditionals(t *testing.T) {
	root := repoRoot(t)
	if releaseStampAnchor.MatchString(read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")) {
		// A release-stamp window is open: maintained docs legitimately carry
		// pre-publication conditionals for the stamped target. The post-tag
		// reconciliation removes the stamp anchor, which re-arms this guard.
		t.Skip("release-stamp window open (Release-stamp target anchor present); pre-publication conditionals are expected until the reconciliation lands")
	}
	tracked, trackedErr := trackedMarkdownSurfaces(root)
	if trackedErr != nil {
		t.Skipf("skipping tracked markdown pre-publication conditional guard: git tracked-file list unavailable outside a git checkout: %v", trackedErr)
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".mmd" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if !tracked[rel] {
			return nil
		}
		if isHistoricalSurface(rel) || rel == "docs/RELEASE_TRUTH_PUBLIC.md" {
			return nil
		}
		body := read(t, root, rel)
		bodyLower := strings.ToLower(body)
		for _, phrase := range prePublicationConditionals {
			searchFrom := 0
			for {
				idx := strings.Index(bodyLower[searchFrom:], phrase)
				if idx == -1 {
					break
				}
				offset := searchFrom + idx
				line := 1 + strings.Count(body[:offset], "\n")
				t.Errorf("%s:%d: stale pre-publication conditional %q was written for a release-publication window that has closed; reword the maintained doc instead of exempting it", rel, line, phrase)
				searchFrom = offset + len(phrase)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking the repo for pre-publication conditionals: %v", walkErr)
	}
}

func trackedMarkdownSurfaces(root string) (map[string]bool, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "*.md", "*.mmd").Output()
	if err != nil {
		return nil, err
	}
	tracked := make(map[string]bool)
	for _, rel := range strings.Fields(string(out)) {
		tracked[filepath.ToSlash(rel)] = true
	}
	return tracked, nil
}

func TestReleaseStampCandidateIsInternallyConsistent(t *testing.T) {
	root := repoRoot(t)
	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	current := oneAnchor(t, "current release target", currentReleaseAnchor, releaseTruth)
	stamp := oneAnchor(t, "not-published release-stamp target", releaseStampAnchor, releaseTruth)
	releaseDate := oneAnchor(t, "current release date", currentReleaseDateAnchor, releaseTruth)
	if stamp != current {
		t.Fatalf("release-stamp target %s must match current release target %s", stamp, current)
	}

	stampStart := strings.Index(releaseTruth, "## Release-stamp status — not published")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before this release stamp:")
	if stampStart == -1 || baselineStart == -1 || stampStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: release-stamp section must precede its published baseline")
	}
	stampSection := releaseTruth[stampStart:baselineStart]
	if !strings.Contains(stampSection, "The public release remains `v0.12.0` until both approved tags publish.") {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: release-stamp status must preserve the v0.12.0 published baseline until both tags publish")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md release-stamp section", stampSection, stamp)

	citation := read(t, root, "CITATION.cff")
	if !strings.Contains(citation, `version: "`+strings.TrimPrefix(stamp, "v")+`"`) {
		t.Fatalf("CITATION.cff must match release-stamp version %s", stamp)
	}
	if !strings.Contains(citation, `date-released: "`+releaseDate+`"`) {
		t.Fatalf("CITATION.cff must match release-stamp date %s", releaseDate)
	}

	changelog := read(t, root, "CHANGELOG.md")
	stampHeading := "## [" + strings.TrimPrefix(stamp, "v") + "] - " + releaseDate
	stampLink := "[" + strings.TrimPrefix(stamp, "v") + "]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.12.0..." + stamp
	if !strings.Contains(changelog, stampHeading) || !strings.Contains(changelog, stampLink) {
		t.Fatalf("CHANGELOG.md must carry the %s release-stamp heading and compare link", stamp)
	}
	if !strings.Contains(changelog, "[Unreleased]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/"+stamp+"...HEAD") {
		t.Fatalf("CHANGELOG.md: [Unreleased] must start from release-stamp target %s", stamp)
	}

	stampNotes := read(t, root, "docs/releases/"+stamp+".md")
	if !strings.Contains(stampNotes, "**Release-stamp candidate — not published.**") ||
		!strings.Contains(stampNotes, "until both tags are") {
		t.Fatalf("docs/releases/%s.md must label its stamp candidate unavailable until both tags publish", stamp)
	}
	assertInstallRefsEqual(t, "release-stamp notes", stampNotes, stamp)
}

func assertInstallRefsEqual(t *testing.T, name, body, want string) {
	t.Helper()
	refsFound := 0
	for _, ref := range canonicalInstallRefs {
		for _, match := range ref.re.FindAllStringSubmatch(body, -1) {
			refsFound++
			if got := match[1]; got != want {
				t.Errorf("%s: %s pins %s, want %s", name, ref.name, got, want)
			}
		}
	}
	if refsFound == 0 {
		t.Fatalf("%s: found zero canonical install references; guard would be vacuous", name)
	}
}

func oneAnchor(t *testing.T, name string, re *regexp.Regexp, body string) string {
	t.Helper()
	matches := re.FindAllStringSubmatch(body, -1)
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %s anchor, found %d", name, len(matches))
	}
	return matches[0][1]
}
