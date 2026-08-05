// install_drift_test.go pins every CANONICAL install / version reference in the
// user-facing docs to the current release, so the install docs cannot silently
// drift behind a published tag again (the failure this guard is built for: the
// install docs kept pinning a superseded release tag long after a newer release
// shipped).
//
// The published source of truth is the "Current release target: `vX.Y.Z`" line
// in docs/RELEASE_TRUTH_PUBLIC.md. A separate, explicitly unavailable candidate
// target may exist without advancing active docs, CHANGELOG, or CITATION.
//
// Only genuinely historical surfaces are exempt (per-version release notes,
// archived internal reconciliations, the append-only changelog, the LOCKED spec
// authoring baseline, and the version-titled v0.9.0 launch artifact). Every
// active install/usage doc — including the ones this lane fixes — is GOVERNED,
// which is the whole point of the guard.
package docs

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	currentReleaseAnchor     = regexp.MustCompile("(?m)^Current release target: `(v[0-9]+\\.[0-9]+\\.[0-9]+)`")
	candidateReleaseAnchor   = regexp.MustCompile("(?m)^Candidate release target: `(v[0-9]+\\.[0-9]+\\.[0-9]+)` — \\*\\*not published\\*\\*$")
	currentReleaseDateAnchor = regexp.MustCompile("(?m)^Current release date: `([0-9]{4}-[0-9]{2}-[0-9]{2})`$")

	// Canonical, copy-pasteable install / version references that must track the
	// current release. Each capturing group yields the pinned version.
	canonicalInstallRefs = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"go install cmd/boundary", regexp.MustCompile(`github\.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"go install verify-witnessed", regexp.MustCompile(`github\.com/fulcrum-governance/fulcrum-boundary/verify-witnessed@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"actions/mcp-audit ref", regexp.MustCompile(`Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"ghcr.io container tag", regexp.MustCompile(`ghcr\.io/fulcrum-governance/boundary:(v[0-9]+\.[0-9]+\.[0-9]+)`)},
		{"surface-status diagram node", regexp.MustCompile(`\[Boundary (v[0-9]+\.[0-9]+\.[0-9]+)\]`)},
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

	publishedStart := strings.Index(releaseTruth, "## Published baseline:")
	if publishedStart == -1 {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: missing published-baseline section")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md published baseline", releaseTruth[publishedStart:], want)
}

func TestUnpublishedCandidateCannotStampPublishedTruth(t *testing.T) {
	root := repoRoot(t)
	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	published := oneAnchor(t, "current release target", currentReleaseAnchor, releaseTruth)
	candidate := oneAnchor(t, "not-published candidate release target", candidateReleaseAnchor, releaseTruth)
	releaseDate := oneAnchor(t, "current release date", currentReleaseDateAnchor, releaseTruth)
	if candidate == published {
		t.Fatalf("candidate %s must be distinct from published release %s", candidate, published)
	}

	candidateStart := strings.Index(releaseTruth, "## Candidate status — not published")
	publishedStart := strings.Index(releaseTruth, "## Published baseline:")
	if candidateStart == -1 || publishedStart == -1 || candidateStart >= publishedStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: candidate section must precede the published baseline")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md candidate section", releaseTruth[candidateStart:publishedStart], candidate)
	if strings.Contains(releaseTruth[publishedStart:], candidate) {
		t.Fatalf("docs/RELEASE_TRUTH_PUBLIC.md: unpublished candidate %s leaked into the published baseline", candidate)
	}

	citation := read(t, root, "CITATION.cff")
	if !strings.Contains(citation, `version: "`+strings.TrimPrefix(published, "v")+`"`) {
		t.Fatalf("CITATION.cff must retain published version %s while candidate %s is unavailable", published, candidate)
	}
	if !strings.Contains(citation, `date-released: "`+releaseDate+`"`) {
		t.Fatalf("CITATION.cff must retain published release date %s while candidate %s is unavailable", releaseDate, candidate)
	}

	changelog := read(t, root, "CHANGELOG.md")
	if strings.Contains(changelog, "## ["+strings.TrimPrefix(candidate, "v")+"]") ||
		strings.Contains(changelog, "["+strings.TrimPrefix(candidate, "v")+"]:") {
		t.Fatalf("CHANGELOG.md: unavailable candidate %s must remain under [Unreleased], without a dated heading or compare link", candidate)
	}
	unreleasedStart := strings.Index(changelog, "## [Unreleased]")
	publishedHeading := strings.Index(changelog, "## ["+strings.TrimPrefix(published, "v")+"]")
	if unreleasedStart == -1 || publishedHeading == -1 || unreleasedStart >= publishedHeading ||
		strings.Contains(changelog[publishedHeading:], candidate) {
		t.Fatalf("CHANGELOG.md: every %s entry must remain inside [Unreleased] before %s", candidate, published)
	}

	candidateNotes := read(t, root, "docs/releases/"+candidate+".md")
	if !strings.Contains(candidateNotes, "**Status: unavailable.**") ||
		!strings.Contains(candidateNotes, "until both tags are") {
		t.Fatalf("docs/releases/%s.md must label all candidate commands unavailable until both tags publish", candidate)
	}
	assertInstallRefsEqual(t, "candidate release notes", candidateNotes, candidate)

	allowed := map[string]bool{
		"CHANGELOG.md":                       true,
		"docs/RELEASE_TRUTH_PUBLIC.md":       true,
		"docs/releases/" + candidate + ".md": true,
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".context", "node_modules", "vendor", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".mmd" && ext != ".cff" {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(read(t, root, rel), candidate) && !allowed[rel] {
			t.Errorf("%s: unpublished candidate %s appears outside a classified candidate surface", rel, candidate)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking public docs for candidate references: %v", walkErr)
	}
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
