package hookboundary

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/versioninfo"
)

// boundarySnippet is the wiring docs/integrations/CLAUDE_CODE_HOOK.md ships.
const boundarySnippet = `{"hooks":{"PreToolUse":[{"matcher":"Bash|Edit|Write|MultiEdit|NotebookEdit",` +
	`"hooks":[{"type":"command","command":"$CLAUDE_PROJECT_DIR/integrations/claude-code/pretooluse-boundary.sh"}]}]}}`

// doctorNow is a fixed clock so dormancy assertions do not depend on wall time.
var doctorNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// fixture builds an isolated project + home pair. Every doctor test runs against
// these, never against the developer's real ~/.claude, so the matrix is hermetic.
type fixture struct {
	root string
	home string
	dir  string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	base := t.TempDir()
	f := fixture{
		root: filepath.Join(base, "project"),
		home: filepath.Join(base, "home"),
	}
	f.dir = filepath.Join(f.root, ".boundary", "hook")
	for _, path := range []string{f.root, f.home} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	return f
}

// config points the doctor at the fixture and at a managed settings path that
// does not exist, so no test reads the host's enterprise policy.
func (f fixture) config() DoctorConfig {
	return DoctorConfig{
		ProjectRoot:         f.root,
		HomeDir:             f.home,
		Dir:                 f.dir,
		ManagedSettingsPath: filepath.Join(f.root, "absent-managed-settings.json"),
		Version: versioninfo.Info{
			SchemaVersion: versioninfo.SchemaVersion,
			Version:       "9.9.9",
			Commit:        "abc123",
			GoVersion:     "go1.25.0",
			Module:        versioninfo.ModulePath,
		},
		Now: func() time.Time { return doctorNow },
	}
}

func (f fixture) writeSettings(t *testing.T, scope, body string) {
	t.Helper()
	var path string
	switch scope {
	case ScopeProject:
		path = filepath.Join(f.root, ".claude", "settings.json")
	case ScopeProjectLocal:
		path = filepath.Join(f.root, ".claude", "settings.local.json")
	case ScopeUser:
		path = filepath.Join(f.home, ".claude", "settings.json")
	default:
		t.Fatalf("unknown scope %q", scope)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func checkByName(t *testing.T, report DoctorReport, name string) DoctorCheck {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("report has no %q check: %+v", name, report.Checks)
	return DoctorCheck{}
}

func TestDoctorReportsAWiredProjectHook(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateOK {
		t.Fatalf("registration state = %q (%s), want ok", check.State, check.Detail)
	}
	if len(report.Registrations) != 1 {
		t.Fatalf("registrations = %d, want 1: %+v", len(report.Registrations), report.Registrations)
	}
	got := report.Registrations[0]
	if got.Scope != ScopeProject || !got.Boundary {
		t.Fatalf("registration = %+v, want a project-scope Boundary entry", got)
	}
	if strings.Join(got.Tools, ",") != strings.Join(GovernedToolNames(), ",") {
		t.Fatalf("tools = %v, want every governed tool", got.Tools)
	}
	if len(report.Peers) != 0 {
		t.Fatalf("peers = %+v, want none", report.Peers)
	}
}

func TestDoctorReportsNoRegistrationAsBroken(t *testing.T) {
	f := newFixture(t)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q, want broken when nothing is wired", check.State)
	}
	if report.Status != StateBroken {
		t.Fatalf("status = %q, want broken", report.Status)
	}
	// The detail must say where it looked, so "not wired" is actionable.
	for _, want := range []string{".claude/settings.json", ".claude/settings.local.json"} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("detail %q does not name the %s scope it searched", check.Detail, want)
		}
	}
}

// pluginSnippet is the wiring hooks/hooks.json ships. It registers the same
// PreToolUse hook through ${CLAUDE_PLUGIN_ROOT} rather than a settings file.
const pluginSnippet = `{"hooks":{"PreToolUse":[{"matcher":"Bash|Edit|Write|MultiEdit|NotebookEdit",` +
	`"hooks":[{"type":"command","command":"\"${CLAUDE_PLUGIN_ROOT}/integrations/claude-code/pretooluse-boundary.sh\""}]}]}}`

// writePlugin materializes a plugin at root: the .claude-plugin/plugin.json
// marker that makes it a plugin, and the hooks/hooks.json manifest that
// registers its hooks.
func writePlugin(t *testing.T, root, manifest string) string {
	t.Helper()
	marker := filepath.Join(root, ".claude-plugin", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(marker), err)
	}
	if err := os.WriteFile(marker, []byte(`{"name":"boundary","version":"0.0.0"}`), 0o600); err != nil {
		t.Fatalf("write plugin marker: %v", err)
	}
	path := filepath.Join(root, "hooks", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o600); err != nil {
		t.Fatalf("write hooks manifest: %v", err)
	}
	return path
}

// A plugin install registers the hook from hooks/hooks.json with no settings
// file naming it. Reporting that as broken asserted "nothing in this project is
// routed to Boundary" about a project that may well be routed, and exited 1 —
// the code a setup script reads as "Boundary is not in front of this agent".
// It must be UNKNOWN: the manifest is real, its being enabled is not readable.
func TestDoctorReportsAPluginOnlyRegistrationAsUnknown(t *testing.T) {
	f := newFixture(t)
	manifest := writePlugin(t, f.root, pluginSnippet)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateUnknown {
		t.Fatalf("registration state = %q (%s), want unknown for a plugin-only install", check.State, check.Detail)
	}
	if report.Status == StateBroken {
		t.Fatalf("status = broken for a plugin-only install: %+v", report.Checks)
	}
	if !strings.Contains(check.Detail, manifest) {
		t.Fatalf("detail %q does not name the manifest it found", check.Detail)
	}
	if !strings.Contains(check.Detail, "enabled") {
		t.Fatalf("detail %q does not say why the manifest is not proof the hook runs", check.Detail)
	}
	// A manifest must never become a settings registration: only settings
	// wiring can reach ok, so Registrations stays the settings-only list.
	if len(report.Registrations) != 0 {
		t.Fatalf("registrations = %+v, want none (the manifest is not settings wiring)", report.Registrations)
	}
	if len(report.PluginHooks) != 1 || !report.PluginHooks[0].Boundary {
		t.Fatalf("plugin hooks = %+v, want one Boundary-shaped entry", report.PluginHooks)
	}
	if len(report.PluginManifests) != 1 || report.PluginManifests[0].Path != manifest {
		t.Fatalf("plugin manifests = %+v, want the one at %s", report.PluginManifests, manifest)
	}
}

// The plugin-drop install lands under ~/.claude/skills/<name>, not in the
// project, so discovery must reach the home locations too.
func TestDoctorFindsAPluginDroppedIntoTheHomeDirectory(t *testing.T) {
	f := newFixture(t)
	dropped := writePlugin(t, filepath.Join(f.home, ".claude", "skills", "boundary"), pluginSnippet)
	marketplace := writePlugin(t, filepath.Join(f.home, ".claude", "plugins", "marketplaces", "boundary"), pluginSnippet)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateUnknown {
		t.Fatalf("registration state = %q (%s), want unknown", check.State, check.Detail)
	}
	found := map[string]bool{}
	for _, file := range report.PluginManifests {
		found[file.Path] = true
	}
	for _, want := range []string{dropped, marketplace} {
		if !found[want] {
			t.Fatalf("manifest %s not discovered; found %+v", want, report.PluginManifests)
		}
	}
}

// A hooks/hooks.json with no plugin.json beside it is not a plugin. Reading one
// as a registration would let any directory that happens to hold that path name
// itself a Boundary install.
func TestDoctorIgnoresAHooksManifestWithoutAPluginMarker(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.root, "hooks", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(pluginSnippet), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	report := Doctor(f.config())
	if len(report.PluginManifests) != 0 {
		t.Fatalf("plugin manifests = %+v, want none without a plugin.json marker", report.PluginManifests)
	}
	if check := checkByName(t, report, CheckRegistration); check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
}

// With nothing wired anywhere the check may say where it looked, and may not
// conclude that nothing is routed: it did not read every place a hook can come
// from, and saying otherwise is a claim it cannot support.
func TestDoctorNotWiredDetailDoesNotOverclaim(t *testing.T) {
	f := newFixture(t)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q, want broken", check.State)
	}
	if strings.Contains(check.Detail, "nothing in this project is routed to Boundary") {
		t.Fatalf("detail asserts more than it read: %q", check.Detail)
	}
	if !strings.Contains(check.Detail, "not visible to it") {
		t.Fatalf("detail %q does not bound the claim to what it could read", check.Detail)
	}
}

// Settings wiring plus a plugin manifest must keep the settings grade. The
// manifest is reported alongside it and neither raises the state to a false ok
// nor lowers it to a duplicate warning the manifest cannot establish.
func TestDoctorKeepsTheSettingsGradeWhenAPluginAlsoDeclaresTheHook(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	manifest := writePlugin(t, f.root, pluginSnippet)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateOK {
		t.Fatalf("registration state = %q (%s), want ok from the settings wiring", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, manifest) {
		t.Fatalf("detail %q does not mention the manifest that also declares the hook", check.Detail)
	}
}

// A hook wired in two scopes runs twice on one tool call. That is not fatal, but
// it doubles the record stream, so it must not read as a clean install.
func TestDoctorReportsADuplicateRegistrationAcrossScopes(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	f.writeSettings(t, ScopeUser, boundarySnippet)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateWarn {
		t.Fatalf("registration state = %q (%s), want warn", check.State, check.Detail)
	}
	if len(report.Registrations) != 2 {
		t.Fatalf("registrations = %d, want 2", len(report.Registrations))
	}
	for _, want := range []string{ScopeProject, ScopeUser, "one record per run"} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("detail %q missing %q", check.Detail, want)
		}
	}
}

// Two Boundary registrations with disjoint matchers are a split install, not a
// duplicate: nothing is decided twice, so nothing should warn about duplication.
func TestDoctorAcceptsASplitRegistrationAcrossDisjointMatchers(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"boundary hook pretooluse"}]}]}}`)
	f.writeSettings(t, ScopeProjectLocal,
		`{"hooks":{"PreToolUse":[{"matcher":"Edit|Write|MultiEdit|NotebookEdit",`+
			`"hooks":[{"type":"command","command":"boundary hook pretooluse"}]}]}}`)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateOK {
		t.Fatalf("registration state = %q (%s), want ok for a split install", check.State, check.Detail)
	}
}

func TestDoctorReportsPartialMatcherCoverage(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"boundary hook pretooluse"}]}]}}`)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateWarn {
		t.Fatalf("registration state = %q (%s), want warn for partial coverage", check.State, check.Detail)
	}
	for _, tool := range []string{"Edit", "Write", "MultiEdit", "NotebookEdit"} {
		if !strings.Contains(check.Detail, tool) {
			t.Fatalf("detail %q does not name uncovered tool %q", check.Detail, tool)
		}
	}
	if strings.Contains(check.Detail, "does not cover Bash") {
		t.Fatalf("detail %q reports a covered tool as uncovered", check.Detail)
	}
}

// A peer hook is a second decider on the same event. It must be listed, and the
// merge order must travel with the listing.
func TestDoctorReportsPeerHooksWithTheMergeNote(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	f.writeSettings(t, ScopeUser,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/other-guard.sh"}]}]}}`)

	report := Doctor(f.config())
	if len(report.Peers) != 1 {
		t.Fatalf("peers = %+v, want the one non-Boundary hook", report.Peers)
	}
	if report.Peers[0].Boundary {
		t.Fatal("a foreign command was recognized as a Boundary registration")
	}
	check := checkByName(t, report, CheckPeers)
	if check.State != StateWarn {
		t.Fatalf("peer state = %q (%s), want warn", check.State, check.Detail)
	}
	for _, want := range []string{"deny > defer > ask > allow", "does not implement or verify"} {
		if !strings.Contains(check.Detail, want) {
			t.Fatalf("peer detail %q missing %q", check.Detail, want)
		}
	}
	if report.MergeNote != PeerMergeNote {
		t.Fatalf("merge note = %q, want it carried in the report", report.MergeNote)
	}
}

// A peer whose matcher touches no governed tool is still listed — silence about
// it is what would mislead — but it does not warn.
func TestDoctorListsANonOverlappingPeerWithoutWarning(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	f.writeSettings(t, ScopeUser,
		`{"hooks":{"PreToolUse":[{"matcher":"WebFetch","hooks":[{"type":"command","command":"/usr/local/bin/web-guard.sh"}]}]}}`)

	report := Doctor(f.config())
	if len(report.Peers) != 1 {
		t.Fatalf("peers = %+v, want the non-overlapping hook listed", report.Peers)
	}
	check := checkByName(t, report, CheckPeers)
	if check.State != StateOK {
		t.Fatalf("peer state = %q (%s), want ok when nothing overlaps", check.State, check.Detail)
	}
}

// A matcher that will not compile leaves coverage UNKNOWN. Reporting it as
// "covers nothing" would be a guess in the permissive direction.
func TestDoctorReportsAnUnreadableMatcherAsUnknownCoverage(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	f.writeSettings(t, ScopeUser,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash(","hooks":[{"type":"command","command":"/usr/local/bin/other.sh"}]}]}}`)

	report := Doctor(f.config())
	if len(report.Peers) != 1 {
		t.Fatalf("peers = %+v", report.Peers)
	}
	if report.Peers[0].MatcherUnderstood {
		t.Fatalf("matcher %q was reported as understood", report.Peers[0].Matcher)
	}
	if len(report.Peers[0].Tools) != 0 {
		t.Fatalf("tools = %v, want none claimed for an unreadable matcher", report.Peers[0].Tools)
	}
	check := checkByName(t, report, CheckPeers)
	if check.State != StateWarn || !strings.Contains(check.Detail, "could not read") {
		t.Fatalf("peer check = %q / %q, want a warn naming the unreadable matcher", check.State, check.Detail)
	}
}

func TestDoctorReportsMalformedSettingsAsBroken(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, `{"hooks": {`)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "not valid JSON") {
		t.Fatalf("detail %q does not say the file could not be parsed", check.Detail)
	}
}

// A FIFO at .claude/settings.json used to block this process forever on open, so
// the one command whose whole job is to answer "is Boundary in front of this
// agent" answered nothing at all — the worst outcome available to it, and
// reachable by a single `mkfifo`. The doctor must refuse a non-regular file and
// report why.
//
// The deadline is what makes this a regression guard rather than a hang: before
// the fix the Doctor call never returns, so the test fails on the deadline
// instead of wedging the suite. A goroutine left blocked on that open is
// deliberate — it is the failure being reported, and the test binary is on its
// way out.
func TestDoctorRefusesANonRegularSettingsFileWithoutBlocking(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mkfifo is a POSIX facility")
	}
	f := newFixture(t)
	path := filepath.Join(f.root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if output, err := exec.Command("mkfifo", path).CombinedOutput(); err != nil {
		t.Skipf("mkfifo unavailable here: %v (%s)", err, output)
	}

	done := make(chan DoctorReport, 1)
	go func() { done <- Doctor(f.config()) }()

	var report DoctorReport
	select {
	case report = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("Doctor blocked on a FIFO at .claude/settings.json instead of refusing it")
	}

	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "not a regular file") {
		t.Fatalf("detail %q does not say the file was refused for its type", check.Detail)
	}
	for _, file := range report.SettingsFiles {
		if file.Scope != ScopeProject {
			continue
		}
		if !file.Present || !strings.Contains(file.Problem, "not a regular file") {
			t.Fatalf("project settings file = %+v, want present with a not-a-regular-file problem", file)
		}
	}
}

// A settings file is untrusted input the doctor reads from a fixed path, so a
// file far larger than any hand-written settings must be refused rather than
// pulled into memory whole.
func TestDoctorRefusesAnOversizedSettingsFile(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, strings.Repeat("x", maxSettingsFileBytes+1))

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "read limit") {
		t.Fatalf("detail %q does not say the file exceeded the read limit", check.Detail)
	}
}

// The regular-file test follows symlinks on purpose: a symlinked
// ~/.claude/settings.json is what every dotfiles manager produces, and refusing
// it the way the WRITE path refuses a symlink would report a correctly wired
// project as broken. This pins that direction so a later tightening cannot
// quietly break it.
func TestDoctorReadsASymlinkedSettingsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on Windows")
	}
	f := newFixture(t)
	real := filepath.Join(f.root, "dotfiles-settings.json")
	if err := os.WriteFile(real, []byte(boundarySnippet), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	link := filepath.Join(f.root, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unavailable here: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateOK {
		t.Fatalf("registration state = %q (%s), want ok for a symlinked settings file", check.State, check.Detail)
	}
}

// disableAllHooks switches every PreToolUse hook off, so a wired Boundary entry
// in the same file decides nothing. Wiring must not outrank the off switch.
func TestDoctorReportsDisableAllHooksAsBroken(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, `{"disableAllHooks":true,`+strings.TrimPrefix(boundarySnippet, "{"))

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "disableAllHooks") {
		t.Fatalf("detail %q does not name the flag", check.Detail)
	}
}

// The evidence probe must prove writability without leaving anything behind, and
// above all without leaving something a reader could mistake for a record.
func TestDoctorProbesWritabilityAndRemovesTheMarker(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateOK {
		t.Fatalf("evidence state = %q (%s), want ok", check.State, check.Detail)
	}
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("doctor left %d file(s) behind: %v", len(entries), entries)
	}
}

func TestDoctorReportsAnAbsentEvidenceDirectoryWithoutCreatingIt(t *testing.T) {
	f := newFixture(t)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateWarn {
		t.Fatalf("evidence state = %q (%s), want warn", check.State, check.Detail)
	}
	if _, err := os.Stat(f.dir); !os.IsNotExist(err) {
		t.Fatalf("doctor created the record directory: %v", err)
	}
}

// The sink refuses a symlinked record directory, so every decided call would be
// escalated. That is broken, not a warning.
func TestDoctorReportsASymlinkedEvidenceDirectoryAsBroken(t *testing.T) {
	f := newFixture(t)
	target := filepath.Join(f.root, "elsewhere")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.dir), 0o700); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.Symlink(target, f.dir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateBroken {
		t.Fatalf("evidence state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "symlink") {
		t.Fatalf("detail %q does not name the symlink refusal", check.Detail)
	}
}

func TestDoctorReportsASymlinkedDecisionLogAsBroken(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	outside := filepath.Join(f.root, "outside.jsonl")
	if err := os.WriteFile(outside, nil, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(f.dir, DecisionLogName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateBroken {
		t.Fatalf("evidence state = %q (%s), want broken", check.State, check.Detail)
	}
}

func TestDoctorReportsAnUnwritableEvidenceDirectoryAsBroken(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f := newFixture(t)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(f.dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.dir, 0o700) })

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateBroken {
		t.Fatalf("evidence state = %q (%s), want broken", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "escalated") {
		t.Fatalf("detail %q does not say what an unrecordable decision does", check.Detail)
	}
}

func TestDoctorWarnsOnAGroupReadableEvidenceDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	f := newFixture(t)
	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(f.dir, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckEvidenceDir)
	if check.State != StateWarn {
		t.Fatalf("evidence state = %q (%s), want warn on a world-readable directory", check.State, check.Detail)
	}
}

// seedLog writes decision log lines. It writes the fields a summary reader
// consumes, which is exactly what a real record carries for them.
func seedLog(t *testing.T, dir string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, DecisionLogName), []byte(body), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}
}

func recordLine(t *testing.T, trace, action string, ts time.Time) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"trace_id":  trace,
		"action":    action,
		"timestamp": ts.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal line: %v", err)
	}
	return string(body)
}

func TestDoctorReportsRecordCountAndFreshness(t *testing.T) {
	f := newFixture(t)
	seedLog(t, f.dir,
		recordLine(t, "sess#1", "allow", doctorNow.Add(-2*time.Hour)),
		recordLine(t, "sess#2", "deny", doctorNow.Add(-time.Hour)),
	)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRecords)
	if check.State != StateOK {
		t.Fatalf("records state = %q (%s), want ok", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "2 record(s)") {
		t.Fatalf("detail %q does not report the count", check.Detail)
	}
}

// Dormancy is a warn and says so honestly: an unused hook and an uninstalled one
// leave the same evidence.
func TestDoctorWarnsOnADormantRecordLog(t *testing.T) {
	f := newFixture(t)
	seedLog(t, f.dir, recordLine(t, "sess#1", "allow", doctorNow.Add(-30*24*time.Hour)))

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRecords)
	if check.State != StateWarn {
		t.Fatalf("records state = %q (%s), want warn", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "uninstalled hook and an unused one both look like") {
		t.Fatalf("detail %q does not state what dormancy cannot distinguish", check.Detail)
	}
}

func TestDoctorReportsUnreadableLogLines(t *testing.T) {
	f := newFixture(t)
	seedLog(t, f.dir,
		recordLine(t, "sess#1", "deny", doctorNow.Add(-time.Minute)),
		"{not json",
	)

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRecords)
	if !strings.Contains(check.Detail, "1 line(s) could not be read") {
		t.Fatalf("detail %q does not report the unreadable line", check.Detail)
	}
}

func TestDoctorWarnsWhenNoRecordsExistYet(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	report := Doctor(f.config())
	check := checkByName(t, report, CheckRecords)
	if check.State != StateWarn {
		t.Fatalf("records state = %q (%s), want warn", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "used no governed tool records nothing") {
		t.Fatalf("detail %q does not explain the benign case", check.Detail)
	}
}

func TestDoctorVersionCheckWarnsOnAnUnstampedBuild(t *testing.T) {
	f := newFixture(t)
	cfg := f.config()
	cfg.Version = versioninfo.Info{SchemaVersion: versioninfo.SchemaVersion, Version: versioninfo.Unknown, GoVersion: "go1.25.0"}

	check := checkByName(t, Doctor(cfg), CheckBinary)
	if check.State != StateWarn {
		t.Fatalf("version state = %q (%s), want warn", check.State, check.Detail)
	}
}

func TestDoctorVersionCheckReportsAStampedBuild(t *testing.T) {
	f := newFixture(t)

	check := checkByName(t, Doctor(f.config()), CheckBinary)
	if check.State != StateOK {
		t.Fatalf("version state = %q (%s), want ok", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "9.9.9") || !strings.Contains(check.Detail, "not an attestation") {
		t.Fatalf("detail %q must name the version and refuse to attest it", check.Detail)
	}
}

// The bypass report is the honest half of this command. It is a FIXED list, it
// never scores into the overall status, and no part of it may read as coverage.
func TestDoctorBypassReportIsFixedAndNeverClaimsSoleRouting(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	seedLog(t, f.dir, recordLine(t, "sess#1", "allow", doctorNow.Add(-time.Minute)))

	report := Doctor(f.config())
	if report.Status != StateOK {
		t.Fatalf("status = %q, want ok; bypasses must not score into it", report.Status)
	}
	want := map[string]bool{
		"mcp-tools":                   false,
		"subagent-spawned-processes":  false,
		"shell-outside-claude-code":   false,
		"unmatched-tools":             false,
		"other-boundary-routes":       false,
		"enterprise-managed-settings": false,
	}
	for _, note := range report.Bypasses {
		if _, ok := want[note.Route]; !ok {
			t.Fatalf("unexpected bypass route %q", note.Route)
		}
		want[note.Route] = true
	}
	for route, seen := range want {
		if !seen {
			t.Fatalf("bypass route %q missing from a fully healthy report", route)
		}
	}
	for _, caveat := range report.Caveats {
		if strings.Contains(caveat, "not a claim that Boundary is the only route") {
			return
		}
	}
	t.Fatalf("caveats do not disclaim sole routing: %v", report.Caveats)
}

// The enterprise flags cannot be read from an ordinary project, so the report
// must say unknown rather than pass.
func TestDoctorReportsUnreadableManagedSettingsAsUnknown(t *testing.T) {
	f := newFixture(t)

	report := Doctor(f.config())
	for _, note := range report.Bypasses {
		if note.Route != "enterprise-managed-settings" {
			continue
		}
		if note.State != StateUnknown {
			t.Fatalf("managed settings state = %q, want unknown", note.State)
		}
		for _, want := range []string{"cannot verify from here", "disableAllHooks", "allowManagedHooksOnly"} {
			if !strings.Contains(note.Detail, want) {
				t.Fatalf("detail %q missing %q", note.Detail, want)
			}
		}
		return
	}
	t.Fatal("report carries no enterprise-managed-settings note")
}

func TestDoctorReadsManagedSettingsWhenTheyAreReadable(t *testing.T) {
	f := newFixture(t)
	managed := filepath.Join(f.root, "managed-settings.json")
	if err := os.WriteFile(managed, []byte(`{"disableAllHooks":true}`), 0o600); err != nil {
		t.Fatalf("write managed: %v", err)
	}
	cfg := f.config()
	cfg.ManagedSettingsPath = managed

	report := Doctor(cfg)
	for _, note := range report.Bypasses {
		if note.Route != "enterprise-managed-settings" {
			continue
		}
		if note.State != StateBroken {
			t.Fatalf("managed settings state = %q (%s), want broken", note.State, note.Detail)
		}
		return
	}
	t.Fatal("report carries no enterprise-managed-settings note")
}

// A managed policy that was actually read must score into the headline status.
// A project file that looks correctly wired on a machine where no hook runs is
// the one case a green status would be a lie.
func TestDoctorManagedDisableAllHooksOutranksAWiredProject(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	managed := filepath.Join(f.root, "managed-settings.json")
	if err := os.WriteFile(managed, []byte(`{"disableAllHooks":true}`), 0o600); err != nil {
		t.Fatalf("write managed: %v", err)
	}
	cfg := f.config()
	cfg.ManagedSettingsPath = managed

	report := Doctor(cfg)
	check := checkByName(t, report, CheckRegistration)
	if check.State != StateBroken {
		t.Fatalf("registration state = %q (%s), want broken", check.State, check.Detail)
	}
	if report.Status != StateBroken {
		t.Fatalf("status = %q, want broken when no hook runs on this machine", report.Status)
	}
}

func TestDoctorManagedHooksOnlyDowngradesAWiredProject(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)
	managed := filepath.Join(f.root, "managed-settings.json")
	if err := os.WriteFile(managed, []byte(`{"allowManagedHooksOnly":true}`), 0o600); err != nil {
		t.Fatalf("write managed: %v", err)
	}
	cfg := f.config()
	cfg.ManagedSettingsPath = managed

	check := checkByName(t, Doctor(cfg), CheckRegistration)
	if check.State != StateWarn {
		t.Fatalf("registration state = %q (%s), want warn", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "allowManagedHooksOnly") {
		t.Fatalf("detail %q does not name the flag", check.Detail)
	}
}

// With no project root and no home there is nothing to search, which is a
// question the doctor could not answer rather than a hook that is missing.
func TestDoctorReportsAnUnsearchableEnvironmentAsUnknown(t *testing.T) {
	// withDefaults fills a missing project root and home from the process, so
	// the check is exercised directly with the empty scope list it would then
	// receive.
	check := registrationCheck(nil, nil, nil, managedSettings{})
	if check.State != StateUnknown {
		t.Fatalf("registration state = %q (%s), want unknown", check.State, check.Detail)
	}
	if !strings.Contains(check.Detail, "no settings scope could be searched") {
		t.Fatalf("detail %q does not distinguish unknown from missing", check.Detail)
	}
}

// StateUnknown must outrank StateOK: a question the doctor could not answer is
// not a pass.
func TestWorstStateRanksUnknownAboveOK(t *testing.T) {
	cases := []struct {
		states []CheckState
		want   CheckState
	}{
		{[]CheckState{StateOK, StateOK}, StateOK},
		{[]CheckState{StateOK, StateUnknown}, StateUnknown},
		{[]CheckState{StateUnknown, StateWarn}, StateWarn},
		{[]CheckState{StateWarn, StateBroken}, StateBroken},
	}
	for _, tc := range cases {
		checks := make([]DoctorCheck, 0, len(tc.states))
		for _, state := range tc.states {
			checks = append(checks, DoctorCheck{State: state})
		}
		if got := worstState(checks); got != tc.want {
			t.Fatalf("worstState(%v) = %q, want %q", tc.states, got, tc.want)
		}
	}
}

func TestIsBoundaryHookCommandMatchesShapesNotResolvedCommands(t *testing.T) {
	recognized := []string{
		"$CLAUDE_PROJECT_DIR/integrations/claude-code/pretooluse-boundary.sh",
		"/opt/tools/claude-code/pretooluse-boundary.sh",
		"boundary hook pretooluse",
		"/usr/local/bin/boundary hook pretooluse --failmode closed",
	}
	for _, command := range recognized {
		if !IsBoundaryHookCommand(command) {
			t.Fatalf("command %q was not recognized as Boundary's hook", command)
		}
	}
	// Under-recognizing is the safe direction: these are reported as peers.
	foreign := []string{"", "/usr/local/bin/other-guard.sh", "boundary command classify", "my-boundary-wrapper"}
	for _, command := range foreign {
		if IsBoundaryHookCommand(command) {
			t.Fatalf("command %q was claimed as a Boundary registration", command)
		}
	}
}

// The doctor's coverage baseline must stay in step with what RouteFor actually
// routes: a tool listed here that no longer routes would report coverage the
// hook does not give.
func TestGovernedToolNamesAllRoute(t *testing.T) {
	for _, tool := range GovernedToolNames() {
		if RouteFor(tool) == RouteNone {
			t.Fatalf("GovernedToolNames lists %q, which RouteFor does not route", tool)
		}
	}
}

func TestDoctorTextRendersEverySection(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)

	var out bytes.Buffer
	if err := WriteDoctorText(&out, Doctor(f.config())); err != nil {
		t.Fatalf("WriteDoctorText: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Boundary Claude Code hook doctor",
		"status:",
		"Checks:",
		"Settings scopes:",
		"Boundary registrations:",
		"Peer PreToolUse hooks:",
		"Not governed by this hook:",
		"Caveats:",
		CheckBinary,
		CheckRegistration,
		CheckEvidenceDir,
		CheckRecords,
		"mcp-tools",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, text)
		}
	}
}

func TestDoctorReportRoundTripsAsJSON(t *testing.T) {
	f := newFixture(t)
	f.writeSettings(t, ScopeProject, boundarySnippet)

	body, err := json.Marshal(Doctor(f.config()))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded DoctorReport
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SchemaVersion != DoctorSchemaVersion {
		t.Fatalf("schema_version = %q", decoded.SchemaVersion)
	}
	if len(decoded.Checks) != 5 || len(decoded.Bypasses) != 6 {
		t.Fatalf("checks = %d, bypasses = %d", len(decoded.Checks), len(decoded.Bypasses))
	}
	// Lists must render as lists, never as null, so a consumer can range over
	// them without a nil check.
	for _, key := range []string{`"registrations":[`, `"peer_hooks":[`, `"settings_files":[`} {
		if !strings.Contains(string(body), key) {
			t.Fatalf("JSON does not render %s as a list:\n%s", key, body)
		}
	}
}
