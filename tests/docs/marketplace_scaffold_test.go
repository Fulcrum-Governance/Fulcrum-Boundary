package docs

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

type pluginManifest struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage"`
	License     string   `json:"license"`
	Keywords    []string `json:"keywords"`
}

type marketplacePlugin struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage"`
	License     string   `json:"license"`
	Keywords    []string `json:"keywords"`
	Source      struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
		Ref    string `json:"ref"`
		SHA    string `json:"sha"`
	} `json:"source"`
}

type marketplaceManifest struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

func TestMarketplaceScaffoldTracksPluginManifest(t *testing.T) {
	root := repoRoot(t)
	plugin := decodeJSON[pluginManifest](t, root, ".claude-plugin/plugin.json")
	marketplace := decodeJSON[marketplaceManifest](t, root, "integrations/claude-code/marketplace/.claude-plugin/marketplace.json")

	if marketplace.Name != "boundary-plugins" {
		t.Fatalf("marketplace name = %q, want boundary-plugins", marketplace.Name)
	}
	if len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace must contain exactly one plugin, found %d", len(marketplace.Plugins))
	}

	entry := marketplace.Plugins[0]
	for field, values := range map[string][2]string{
		"name":        {entry.Name, plugin.Name},
		"description": {entry.Description, plugin.Description},
		"homepage":    {entry.Homepage, plugin.Homepage},
		"license":     {entry.License, plugin.License},
	} {
		if values[0] != values[1] {
			t.Errorf("marketplace plugins[0].%s = %q, canonical plugin manifest = %q", field, values[0], values[1])
		}
	}
	if !slices.Equal(entry.Keywords, plugin.Keywords) {
		t.Errorf("marketplace plugins[0].keywords = %q, canonical plugin manifest = %q", entry.Keywords, plugin.Keywords)
	}
	if entry.Source.Source != "github" || entry.Source.Repo != "fulcrum-governance/fulcrum-boundary" {
		t.Errorf("marketplace source = %q/%q, want github/fulcrum-governance/fulcrum-boundary", entry.Source.Source, entry.Source.Repo)
	}
	if entry.Version != "" {
		t.Errorf("marketplace entry duplicates plugin.json version %q; omit it so plugin.json remains the single version authority", entry.Version)
	}
	if entry.Source.Ref != "v"+plugin.Version {
		t.Errorf("marketplace source ref = %q, want v%s from the canonical plugin manifest", entry.Source.Ref, plugin.Version)
	}

	releaseTruth := read(t, root, "docs/RELEASE_TRUTH_PUBLIC.md")
	publishedStart := strings.Index(releaseTruth, "## Published v"+plugin.Version+" release")
	baselineStart := strings.Index(releaseTruth, "## Published baseline before v"+plugin.Version+":")
	if publishedStart == -1 || baselineStart == -1 || publishedStart >= baselineStart {
		t.Fatal("docs/RELEASE_TRUTH_PUBLIC.md does not contain the current published-release section before its historical baseline")
	}
	releaseCommit := oneAnchor(
		t,
		"current approved release commit",
		regexp.MustCompile("(?s)both peel to the approved release commit\\s+`([0-9a-f]{40})`\\."),
		releaseTruth[publishedStart:baselineStart],
	)
	if entry.Source.SHA != releaseCommit {
		t.Errorf("marketplace source SHA = %q, want current approved release commit %q", entry.Source.SHA, releaseCommit)
	}
}

func TestMarketplaceScaffoldIsReadyForStandalonePublication(t *testing.T) {
	root := repoRoot(t)
	plugin := decodeJSON[pluginManifest](t, root, ".claude-plugin/plugin.json")
	readme := read(t, root, "integrations/claude-code/marketplace/README.md")
	normalizedREADME := strings.Join(strings.Fields(readme), " ")

	wants := []string{
		"/plugin marketplace add fulcrum-governance/boundary-plugins",
		"/plugin install boundary@boundary-plugins",
		"The plugin source is pinned to the immutable v" + plugin.Version + " release commit.",
		"Boundary governs only routed calls",
		"not authenticity, correctness, execution, or total coverage",
		"https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/v" + plugin.Version + "/docs/integrations/CLAUDE_CODE_HOOK.md",
		"https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/v" + plugin.Version + "/scripts/install-claude-code.sh",
	}
	for _, want := range wants {
		if !strings.Contains(normalizedREADME, want) {
			t.Errorf("standalone marketplace README is missing %q", want)
		}
	}

	for _, stale := range []string{
		"../../",
		"future, standalone",
		"No remote repository has been created",
		"nothing here has been pushed",
		"Until that repository exists",
	} {
		if strings.Contains(readme, stale) {
			t.Errorf("standalone marketplace README retains staging-only text %q", stale)
		}
	}

	wantFiles := []string{".claude-plugin/marketplace.json", "README.md"}
	var gotFiles []string
	scaffoldRoot := filepath.Join(root, "integrations/claude-code/marketplace")
	err := filepath.WalkDir(scaffoldRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(scaffoldRoot, path)
		if relErr != nil {
			return relErr
		}
		gotFiles = append(gotFiles, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk marketplace scaffold: %v", err)
	}
	slices.Sort(gotFiles)
	if !slices.Equal(gotFiles, wantFiles) {
		t.Fatalf("marketplace scaffold files = %q, want exact two-file set %q", gotFiles, wantFiles)
	}
}

func decodeJSON[T any](t *testing.T, root, rel string) T {
	t.Helper()
	var value T
	if err := json.Unmarshal([]byte(read(t, root, rel)), &value); err != nil {
		t.Fatalf("decode %s: %v", rel, err)
	}
	return value
}
