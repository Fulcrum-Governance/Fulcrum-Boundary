// Package docs holds black-box tests that keep developer documentation honest
// to the code it describes. host_setup_test.go pins docs/firewall/HOST_SETUP.md
// (the per-host install tutorials, issue #138) to the real per-host MCP config
// paths in internal/firewall/discover.go, and keeps the issue-required elements
// (doctor confirmation, routed-only caveat per host, conformance-checklist link,
// client selectors) present.
package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestHostSetupComponentsTrackDiscovery couples the path COMPONENTS the tutorial
// documents to internal/firewall/discover.go: each must appear in BOTH files. If
// discover.go drops a component the marker vanishes there (fail → the code moved
// and the tutorial must follow); if the tutorial stops naming a discovered
// component the marker vanishes here. This is doc↔code coupling on the building
// blocks; the exact per-host rows are pinned by TestHostSetupDocumentsEachPath.
func TestHostSetupComponentsTrackDiscovery(t *testing.T) {
	root := repoRoot(t)
	discover := read(t, root, "internal/firewall/discover.go")
	doc := read(t, root, "docs/firewall/HOST_SETUP.md")

	components := []string{
		"claude_desktop_config.json", ".mcp.json", ".cursor", ".vscode",
		"Application Support", "Cursor", "Code", "User", "AppData", "Claude",
	}
	for _, m := range components {
		if !strings.Contains(discover, m) {
			t.Fatalf("path component %q is gone from internal/firewall/discover.go — update the markers and the tutorial to match the real discovered paths", m)
		}
		if !strings.Contains(doc, m) {
			t.Fatalf("docs/firewall/HOST_SETUP.md no longer names path component %q that discover.go discovers — the tutorial has drifted from the code", m)
		}
	}
}

// TestHostSetupDocumentsEachPath pins every per-host config path row as a
// distinct full literal, so deleting a host's row or corrupting its directory
// (the user-scope / Windows rows the generic component markers cannot detect)
// fails the build. These are doc-side literals: discover.go assembles the same
// paths via filepath.Join, so the slash/backslash-rendered forms below appear
// only in the tutorial.
func TestHostSetupDocumentsEachPath(t *testing.T) {
	doc := read(t, repoRoot(t), "docs/firewall/HOST_SETUP.md")
	paths := []string{
		// Claude Desktop
		"Library/Application Support/Claude/claude_desktop_config.json",
		".config/Claude/claude_desktop_config.json",
		`%AppData%\Claude\claude_desktop_config.json`,
		// Cursor
		"Library/Application Support/Cursor/User/mcp.json",
		".config/Cursor/User/mcp.json",
		".cursor/mcp.json",
		// VS Code
		"Library/Application Support/Code/User/mcp.json",
		".config/Code/User/mcp.json",
		`%AppData%\Code\User\mcp.json`,
		".vscode/mcp.json",
		// repo-local (Claude Code)
		".mcp.json",
	}
	for _, p := range paths {
		if !strings.Contains(doc, p) {
			t.Fatalf("docs/firewall/HOST_SETUP.md no longer documents the config path %q — a per-host path row was dropped or corrupted", p)
		}
	}
}

// TestHostSetupPerHostCaveat asserts each of the four host sections carries the
// routed-only / bypass caveat that issue #138 requires per host (not only the
// global banner), so a future edit cannot quietly drop it from one host.
func TestHostSetupPerHostCaveat(t *testing.T) {
	doc := read(t, repoRoot(t), "docs/firewall/HOST_SETUP.md")
	hosts := []string{"## Claude Desktop", "## Claude Code", "## Cursor", "## VS Code"}
	for i, h := range hosts {
		start := strings.Index(doc, h)
		if start < 0 {
			t.Fatalf("HOST_SETUP.md missing host section %q", h)
		}
		end := len(doc)
		// Section runs until the next "## " heading.
		if next := strings.Index(doc[start+len(h):], "\n## "); next >= 0 {
			end = start + len(h) + next
		}
		if !strings.Contains(doc[start:end], "bypass") {
			t.Fatalf("host section %q (#%d) is missing a routed-only/bypass caveat (#138 requires it per host)", h, i)
		}
	}
}

// TestHostSetupRequiredElements pins the elements issue #138 requires each
// tutorial to carry, so a future edit cannot quietly drop the honest framing or
// a runnable command form.
func TestHostSetupRequiredElements(t *testing.T) {
	doc := read(t, repoRoot(t), "docs/firewall/HOST_SETUP.md")
	required := []string{
		"boundary doctor",                // how to confirm the route
		"Routed-only",                    // the global caveat banner
		"ROUTE_CONFORMANCE_CHECKLIST.md", // canonical checklist link
		"--client claude",                // Claude Desktop selector
		"--client repo",                  // Claude Code (repo-local) selector
		"--client cursor",                // Cursor selector
		"--client vscode",                // VS Code selector
		"--dry-run",                      // preview-first guidance
		"boundary uninstall --receipt",   // reversibility, the runnable form
	}
	for _, r := range required {
		if !strings.Contains(doc, r) {
			t.Fatalf("docs/firewall/HOST_SETUP.md is missing required element %q (issue #138)", r)
		}
	}
}
