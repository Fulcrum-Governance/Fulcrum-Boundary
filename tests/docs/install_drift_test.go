// install_drift_test.go pins every CANONICAL install / version reference in the
// user-facing docs to the current release, so the install docs cannot silently
// drift behind a published tag again (the failure this guard is built for: the
// install docs kept pinning a superseded release tag long after a newer release
// shipped).
//
// The published release is the "Current release: `vX.Y.Z`" line in
// docs/RELEASE_TRUTH_PUBLIC.md. The report must keep its copy-pasteable install
// commands and publication facts aligned with that released version.
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
	currentReleaseAnchor     = regexp.MustCompile("(?m)^Current release: `(v[0-9]+\\.[0-9]+\\.[0-9]+)`")
	publishedReleaseAnchor   = regexp.MustCompile("(?m)^Published release: `(v[0-9]+\\.[0-9]+\\.[0-9]+)`$")
	currentReleaseDateAnchor = regexp.MustCompile("(?m)^Current release date: `([0-9]{4}-[0-9]{2}-[0-9]{2})`$")
	publicInstallSmokeAnchor = regexp.MustCompile(`The public install smoke passed on ([0-9]{4}-[0-9]{2}-[0-9]{2})[ .]`)
	stalePublicInstallSmoke  = regexp.MustCompile("(?i)public-install smoke(?: against the published `v[0-9]+\\.[0-9]+\\.[0-9]+` channels)? has not run")

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
		"next tagged release",
	}
	staleMarketplaceAvailability = []string{
		"planned marketplace repository",
		"marketplace repository does not exist",
		"they do not work yet",
		"marketplace scaffold in this repository remains unpublished",
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
		t.Fatalf("expected exactly one \"Current release: `vX.Y.Z`\" anchor in docs/RELEASE_TRUTH_PUBLIC.md, found %d — the drift anchor moved or was reformatted", len(anchorMatches))
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

	publishedStart := strings.Index(releaseTruth, "## Published "+want+" release")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before "+want+":")
	if publishedStart == -1 || baselineStart == -1 || publishedStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: published-release section must precede its published baseline")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md published-release section", releaseTruth[publishedStart:baselineStart], want)
}

func TestNoPrePublicationConditionals(t *testing.T) {
	root := repoRoot(t)
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

func TestPublishedMarketplaceTruthIsCurrent(t *testing.T) {
	root := repoRoot(t)
	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	current := oneAnchor(t, "current release", currentReleaseAnchor, releaseTruth)
	publishedStart := strings.Index(releaseTruth, "## Published "+current+" release")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before "+current+":")
	if publishedStart == -1 || baselineStart == -1 || publishedStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: published-release section must precede its historical baseline")
	}
	currentSection := releaseTruth[publishedStart:baselineStart]

	// The public boundary-plugins repository was validated at this earlier
	// release and is not updated for the current one. Its validation facts live
	// in that release's own section and must stay intact; the current section
	// must say plainly that the public package was not updated.
	const (
		validatedRelease  = "v0.13.1"
		validatedCommit   = "8a5762888be8404f8a4a0e64a2ad6206667b71b6"
		publicRepo        = "https://github.com/Fulcrum-Governance/boundary-plugins"
		marketplaceAdd    = "/plugin marketplace add fulcrum-governance/boundary-plugins"
		pluginInstall     = "/plugin install boundary@boundary-plugins"
		marketplaceAddCLI = "claude plugin marketplace add Fulcrum-Governance/boundary-plugins --scope user"
		pluginInstallCLI  = "claude plugin install boundary@boundary-plugins --scope user --yes"
		claudeVersion     = "Claude Code `2.1.258`"
	)
	validatedStart := strings.Index(releaseTruth, "## Published "+validatedRelease+" release")
	validatedEnd := strings.Index(releaseTruth, "## Published baseline before "+validatedRelease+":")
	if validatedStart == -1 || validatedEnd == -1 || validatedStart >= validatedEnd || validatedStart < baselineStart {
		t.Fatalf("docs/RELEASE_TRUTH_PUBLIC.md: the %s published record must remain intact beneath the current release", validatedRelease)
	}
	validatedSection := releaseTruth[validatedStart:validatedEnd]

	if !strings.Contains(currentSection, publicRepo) ||
		!strings.Contains(currentSection, "marketplace repository is not updated for `"+current+"`") ||
		!strings.Contains(currentSection, validatedCommit) {
		t.Errorf("docs/RELEASE_TRUTH_PUBLIC.md current release section must link %s and state that it is not updated for %s and remains pinned to %s", publicRepo, current, validatedCommit)
	}
	activeSurfaces := map[string]string{
		"README.md": read(t, root, "README.md"),
		"docs/RELEASE_TRUTH_PUBLIC.md current release":   currentSection,
		"docs/RELEASE_TRUTH_PUBLIC.md validated release": validatedSection,
	}
	for name, body := range activeSurfaces {
		if !strings.Contains(body, publicRepo) {
			t.Errorf("%s is missing the public marketplace repository %q", name, publicRepo)
		}
		bodyLower := strings.ToLower(body)
		for _, stale := range staleMarketplaceAvailability {
			if strings.Contains(bodyLower, stale) {
				t.Errorf("%s retains stale marketplace availability claim %q", name, stale)
			}
		}
	}
	readme := activeSurfaces["README.md"]
	for _, want := range []string{marketplaceAdd, pluginInstall, claudeVersion, validatedCommit, "has not been updated for `" + current + "`"} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md is missing marketplace fact %q", want)
		}
	}
	for _, want := range []string{claudeVersion, validatedCommit, marketplaceAddCLI, pluginInstallCLI} {
		if !strings.Contains(validatedSection, want) {
			t.Errorf("docs/RELEASE_TRUTH_PUBLIC.md %s section is missing executed marketplace validation fact %q", validatedRelease, want)
		}
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

func TestPublishedReleaseTruthIsInternallyConsistent(t *testing.T) {
	root := repoRoot(t)
	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	current := oneAnchor(t, "current release", currentReleaseAnchor, releaseTruth)
	published := oneAnchor(t, "published release", publishedReleaseAnchor, releaseTruth)
	releaseDate := oneAnchor(t, "current release date", currentReleaseDateAnchor, releaseTruth)
	if published != current {
		t.Fatalf("published release %s must match current release %s", published, current)
	}

	publishedStart := strings.Index(releaseTruth, "## Published "+published+" release")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before "+published+":")
	if publishedStart == -1 || baselineStart == -1 || publishedStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: published-release section must precede its published baseline")
	}
	publishedSection := releaseTruth[publishedStart:baselineStart]
	rootRef := "refs/tags/" + published
	nestedRef := "refs/tags/verify-witnessed/" + published
	if !strings.Contains(publishedSection, "The annotated root tag object") ||
		!strings.Contains(publishedSection, "(`"+rootRef+"`)") ||
		!strings.Contains(publishedSection, "(`"+nestedRef+"`)") ||
		!strings.Contains(publishedSection, "both peel to the approved release commit") {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md: published-release section must name both actual tag refs and state that both annotated tags peel to the approved commit")
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md published-release section", publishedSection, published)

	// The prior published record stays intact beneath the current one.
	const prior = "v0.13.1"
	priorStart := strings.Index(releaseTruth, "## Published "+prior+" release")
	priorEnd := strings.Index(releaseTruth, "## Published baseline before "+prior+":")
	if priorStart == -1 || priorEnd == -1 || priorStart >= priorEnd || priorStart < baselineStart {
		t.Fatalf("docs/RELEASE_TRUTH_PUBLIC.md: the published %s record must remain intact beneath the current release", prior)
	}
	priorSection := releaseTruth[priorStart:priorEnd]
	if !strings.Contains(priorSection, "The annotated root tag object") ||
		!strings.Contains(priorSection, "(`refs/tags/"+prior+"`)") ||
		!strings.Contains(priorSection, "(`refs/tags/verify-witnessed/"+prior+"`)") ||
		!strings.Contains(priorSection, "both peel to the approved release commit") {
		t.Fatalf("docs/RELEASE_TRUTH_PUBLIC.md: the published %s section must keep both actual tag refs and the peel statement", prior)
	}
	assertInstallRefsEqual(t, "docs/RELEASE_TRUTH_PUBLIC.md published "+prior+" section", priorSection, prior)

	citation := read(t, root, "CITATION.cff")
	if !strings.Contains(citation, `version: "`+strings.TrimPrefix(published, "v")+`"`) {
		t.Fatalf("CITATION.cff must match published release version %s", published)
	}
	if !strings.Contains(citation, `date-released: "`+releaseDate+`"`) {
		t.Fatalf("CITATION.cff must match published release date %s", releaseDate)
	}

	changelog := read(t, root, "CHANGELOG.md")
	releaseHeading := "## [" + strings.TrimPrefix(published, "v") + "] - " + releaseDate
	releaseLink := "[" + strings.TrimPrefix(published, "v") + "]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/" + prior + "..." + published
	if !strings.Contains(changelog, releaseHeading) || !strings.Contains(changelog, releaseLink) {
		t.Fatalf("CHANGELOG.md must carry the %s published-release heading and compare link", published)
	}
	if !strings.Contains(changelog, "[Unreleased]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/"+published+"...HEAD") {
		t.Fatalf("CHANGELOG.md: [Unreleased] must start from published release %s", published)
	}

	releaseNotes := read(t, root, "docs/releases/"+published+".md")
	normalizedReleaseNotes := strings.Join(strings.Fields(strings.ReplaceAll(releaseNotes, "\n>", "\n")), " ")
	normalizedPublishedSection := strings.Join(strings.Fields(publishedSection), " ")
	if !strings.Contains(normalizedReleaseNotes, "**Published release — "+releaseDate+".**") ||
		!strings.Contains(normalizedReleaseNotes, "not a draft or prerelease") {
		t.Fatalf("docs/releases/%s.md must identify the public release state", published)
	}
	truthSmokeDate := oneAnchor(t, "public release-truth smoke date", publicInstallSmokeAnchor, normalizedPublishedSection)
	notesSmokeDate := oneAnchor(t, "current release-note smoke date", publicInstallSmokeAnchor, normalizedReleaseNotes)
	if truthSmokeDate != notesSmokeDate {
		t.Fatalf("public-install smoke date differs between release truth (%s) and current release notes (%s)", truthSmokeDate, notesSmokeDate)
	}
	for surface, body := range map[string]string{
		"docs/RELEASE_TRUTH_PUBLIC.md": strings.Join(strings.Fields(releaseTruth), " "),
		"current release notes":        normalizedReleaseNotes,
	} {
		if stalePublicInstallSmoke.MatchString(body) {
			t.Fatalf("%s retains a stale claim that the current public-install smoke has not run", surface)
		}
	}
	assertInstallRefsEqual(t, "release notes", releaseNotes, published)
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
