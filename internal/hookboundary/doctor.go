package hookboundary

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/versioninfo"
)

// DoctorSchemaVersion identifies the JSON shape of a hook doctor report.
//
// It is versioned separately from the decision record schema on purpose: a
// report is diagnostic output about one machine at one moment, never evidence.
// Nothing hashes it, nothing verifies it, and no other Boundary surface consumes
// it.
const DoctorSchemaVersion = "boundary.hook.doctor.v1"

// CheckState is what one doctor check could establish. It reports the state of
// the CHECK, not a guarantee about the world: StateOK means the condition this
// check inspects was found in order, never that the route is closed.
type CheckState string

const (
	// StateOK means the check inspected the condition and found it in order.
	StateOK CheckState = "ok"
	// StateWarn means the check found something that weakens the hook without
	// stopping it — partial tool coverage, a duplicate registration, records
	// nobody has written in a while.
	StateWarn CheckState = "warn"
	// StateBroken means the check found the hook unable to do its job here:
	// nothing wired, hooks disabled, or evidence that cannot be written.
	StateBroken CheckState = "broken"
	// StateUnknown means the check could not determine the answer from this
	// machine. It is never rendered as an "ok": an unread enterprise policy or
	// an unreadable settings file is an open question, not a pass.
	StateUnknown CheckState = "unknown"
)

// Doctor check names. They are stable strings so a script can select one.
const (
	CheckBinary       = "boundary binary"
	CheckRegistration = "hook registration"
	CheckPeers        = "peer PreToolUse hooks"
	CheckEvidenceDir  = "evidence directory"
	CheckRecords      = "decision records"
)

// Settings scopes the doctor reads, in the order Claude Code layers them.
const (
	// ScopeProject is <project>/.claude/settings.json, the committed wiring.
	ScopeProject = "project"
	// ScopeProjectLocal is <project>/.claude/settings.local.json, the
	// uncommitted per-developer wiring.
	ScopeProjectLocal = "project-local"
	// ScopeUser is ~/.claude/settings.json, wiring that applies to every
	// project this user opens.
	ScopeUser = "user"
	// ScopeManaged is the enterprise managed settings file, read only for the
	// hook-disabling flags it may carry.
	ScopeManaged = "managed"
	// ScopePlugin is a Claude Code plugin's own hooks/hooks.json, which
	// registers hooks without any settings file naming them.
	ScopePlugin = "plugin"
)

// DormancyThreshold is how old the newest decision record may be before the
// records check warns.
//
// It is a heuristic for "nothing has been decided here in a while", not a
// health signal: a session that used no governed tool records nothing, so a
// dormant log is equally consistent with a working hook and an uninstalled one.
const DormancyThreshold = 7 * 24 * time.Hour

// PeerMergeNote states how a host resolves several PreToolUse hooks on one tool
// call. It is quoted in the peer check and in the rendered report so an operator
// who sees a peer listed also sees what the peer can and cannot do.
//
// Boundary neither implements nor verifies this merge: it is the host's rule,
// restated here so the peer listing means something.
const PeerMergeNote = "Claude Code merges several PreToolUse results by taking the most restrictive " +
	"(deny > defer > ask > allow), so a peer cannot loosen a Boundary deny — and Boundary cannot loosen a peer's. " +
	"Boundary does not implement or verify that merge."

// probeMarkerName is the file the evidence check writes and removes to prove the
// record directory is writable. It is deliberately NOT a record: it carries no
// decision, no hash, and a name no record file can have, so a reader cannot
// mistake a probe for something Boundary decided.
const probeMarkerName = ".doctor-probe"

// probeMarkerBody is what the probe writes, so a marker left behind by an
// interrupted run explains itself.
const probeMarkerBody = "boundary hook doctor writability probe; safe to delete\n"

// DoctorConfig configures one hook doctor run. The zero value is usable: it
// inspects the process's working directory as the project, the current user's
// home for the user scope, and DefaultRecordDir for evidence.
type DoctorConfig struct {
	// ProjectRoot is the project whose .claude settings are read. Empty means
	// the process's working directory.
	ProjectRoot string
	// HomeDir is the home directory whose ~/.claude/settings.json is read.
	// Empty means the current user's home. It is injected rather than read
	// from the environment so a test can supply a fixture home.
	HomeDir string
	// Dir is the decision record directory. Empty means DefaultRecordDir. A
	// relative value is resolved against ProjectRoot.
	Dir string
	// ManagedSettingsPath overrides the enterprise managed settings path.
	// Empty means the platform default (see managedSettingsPath).
	ManagedSettingsPath string
	// Version is the binary identity reported by the version check. A zero
	// value is filled in from versioninfo.Current.
	Version versioninfo.Info
	// Now supplies the report timestamp and the dormancy baseline. Empty means
	// time.Now.
	Now func() time.Time
}

// withDefaults returns a copy of cfg with every unset field resolved. A working
// directory or home directory that cannot be read is left EMPTY rather than
// guessed, and the checks that need it report unknown.
func (c DoctorConfig) withDefaults() DoctorConfig {
	if c.ProjectRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			c.ProjectRoot = wd
		}
	}
	if c.ProjectRoot != "" {
		if abs, err := filepath.Abs(c.ProjectRoot); err == nil {
			c.ProjectRoot = abs
		}
	}
	if c.HomeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			c.HomeDir = home
		}
	}
	if c.Dir == "" {
		c.Dir = DefaultRecordDir
	}
	if !filepath.IsAbs(c.Dir) && c.ProjectRoot != "" {
		c.Dir = filepath.Join(c.ProjectRoot, c.Dir)
	}
	if c.ManagedSettingsPath == "" {
		c.ManagedSettingsPath = managedSettingsPath()
	}
	if c.Version.SchemaVersion == "" {
		c.Version = versioninfo.Current()
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// DoctorCheck is one inspected condition and what it established.
type DoctorCheck struct {
	// Name is the stable check name, one of the Check* constants.
	Name string `json:"name"`
	// State is what the check established.
	State CheckState `json:"state"`
	// Detail is the operator-facing explanation, always populated.
	Detail string `json:"detail"`
}

// HookEntry is one PreToolUse hook command found in a settings file.
//
// Boundary reports whether it recognizes the command as its own by SHAPE only —
// nothing is executed and no command is resolved on PATH — so an entry that
// reaches Boundary through a wrapper this check does not recognize is reported
// as a peer, not as Boundary. Under-recognizing is the safe direction: it lists
// an extra peer rather than claiming a registration that may not exist.
type HookEntry struct {
	// Scope is the settings scope the entry was found in.
	Scope string `json:"scope"`
	// SettingsPath is the file the entry was read from.
	SettingsPath string `json:"settings_path"`
	// Matcher is the tool-name pattern as written in the settings file.
	Matcher string `json:"matcher"`
	// Type is the hook's declared type, normally "command".
	Type string `json:"type,omitempty"`
	// Command is the command string as written in the settings file. It is not
	// expanded, resolved, or executed.
	Command string `json:"command"`
	// Boundary reports whether the command matched a Boundary hook shape.
	Boundary bool `json:"boundary"`
	// Tools are the tool names in GovernedToolNames this matcher covers.
	Tools []string `json:"tools"`
	// MatcherUnderstood is false when the matcher could not be read as a
	// pattern, in which case Tools is empty and says nothing.
	MatcherUnderstood bool `json:"matcher_understood"`
}

// SettingsFile records what the doctor found at one settings path. A file that
// is absent is not a problem — most installations wire one scope, not three.
type SettingsFile struct {
	// Scope is the settings scope.
	Scope string `json:"scope"`
	// Path is the file the doctor looked for.
	Path string `json:"path"`
	// Present reports whether the file exists.
	Present bool `json:"present"`
	// Problem explains why a present file could not be read or parsed, empty
	// when it was read.
	Problem string `json:"problem,omitempty"`
	// HooksDisabled is the file's disableAllHooks value when it carries one.
	HooksDisabled *bool `json:"disable_all_hooks,omitempty"`
	// ManagedHooksOnly is the file's allowManagedHooksOnly value when it
	// carries one.
	ManagedHooksOnly *bool `json:"allow_managed_hooks_only,omitempty"`
}

// BypassNote names a route this hook does not govern.
//
// The list is FIXED, not discovered: these routes are open by construction, and
// a doctor that only reported what it happened to find would read as a coverage
// claim. State reports what this command could establish about the route, and
// never that the route is closed.
type BypassNote struct {
	// Route is a stable identifier for the ungoverned route.
	Route string `json:"route"`
	// State is what the doctor could establish: warn for a route that is open
	// by construction, unknown for one it cannot see from here.
	State CheckState `json:"state"`
	// Detail is the operator-facing explanation.
	Detail string `json:"detail"`
}

// DoctorReport is the whole hook doctor result.
//
// Status summarizes Checks only. Bypasses are deliberately EXCLUDED from it:
// they are permanent properties of a routed boundary, not defects to be cleared,
// and folding them in would make every report warn forever and train an operator
// to ignore the field that carries real problems.
type DoctorReport struct {
	SchemaVersion string `json:"schema_version"`
	// GeneratedAt is when the report was produced, in UTC.
	GeneratedAt time.Time `json:"generated_at"`
	// Status is the worst state among Checks.
	Status CheckState `json:"status"`
	// ProjectRoot is the project the settings scopes were read against.
	ProjectRoot string `json:"project_root"`
	// RecordDir is the evidence directory that was inspected.
	RecordDir string `json:"record_dir"`
	// Checks are the inspected conditions, in a fixed order.
	Checks []DoctorCheck `json:"checks"`
	// SettingsFiles are the settings paths the doctor looked at.
	SettingsFiles []SettingsFile `json:"settings_files"`
	// PluginManifests are the plugin hooks/hooks.json files that were found and
	// read. It is what this check could LOCATE, never the set of plugins Claude
	// Code has enabled.
	PluginManifests []SettingsFile `json:"plugin_manifests"`
	// PluginHooks are the PreToolUse entries those manifests declare, each
	// flagged for whether it matches a Boundary shape.
	PluginHooks []HookEntry `json:"plugin_hooks"`
	// Registrations are the PreToolUse entries recognized as Boundary's.
	Registrations []HookEntry `json:"registrations"`
	// Peers are the PreToolUse entries that are not Boundary's.
	Peers []HookEntry `json:"peer_hooks"`
	// MergeNote is PeerMergeNote, carried in the report so a consumer of the
	// JSON gets the caveat with the peer list.
	MergeNote string `json:"merge_note"`
	// Bypasses are the routes this hook does not govern.
	Bypasses []BypassNote `json:"bypasses"`
	// Caveats are the standing limitations of the whole report.
	Caveats []string `json:"caveats"`
}

// GovernedToolNames returns the Claude Code tool names this hook routes to a
// Boundary classifier, and therefore the baseline a matcher's coverage is
// measured against.
//
// It is the canonical Claude Code spelling of each tool. RouteFor also accepts
// the "Shell" and lowercase aliases, which no Claude Code matcher emits and
// which are not part of the coverage baseline; a matcher that covers these five
// covers every tool Claude Code will route here.
func GovernedToolNames() []string {
	return []string{"Bash", "Edit", "Write", "MultiEdit", "NotebookEdit"}
}

// Doctor inspects the local Claude Code hook installation and reports what it
// can establish about it — and, in the same report, what it cannot.
//
// Everything it does is READ-ONLY except one probe: the evidence check writes a
// marker file into the record directory and removes it, because "is this
// directory writable" cannot be answered by stat alone and a hook that cannot
// record escalates every call. The marker is never a record.
//
// Nothing here executes a hook, resolves a command on PATH, or reaches the
// network. Registration is matched by path SHAPE, so this reports a wiring that
// looks like Boundary's, not a proof that Boundary decides. It never claims sole
// routing: see Bypasses, which is a fixed list rather than a discovered one.
func Doctor(cfg DoctorConfig) DoctorReport {
	cfg = cfg.withDefaults()

	files, entries := readHookSettings(cfg)
	pluginFiles, pluginEntries := readPluginHookManifests(cfg)
	managed := readManagedSettings(cfg.ManagedSettingsPath)
	registrations, peers := splitHookEntries(entries)
	pluginRegistrations, _ := splitHookEntries(pluginEntries)

	checks := []DoctorCheck{
		versionCheck(cfg.Version),
		registrationCheck(files, registrations, pluginRegistrations, managed),
		peerCheck(peers),
		evidenceDirCheck(cfg.Dir),
		recordsCheck(cfg.Dir, cfg.Now()),
	}

	return DoctorReport{
		SchemaVersion:   DoctorSchemaVersion,
		GeneratedAt:     cfg.Now().UTC(),
		Status:          worstState(checks),
		ProjectRoot:     cfg.ProjectRoot,
		RecordDir:       cfg.Dir,
		Checks:          checks,
		SettingsFiles:   append(files, managed.file),
		PluginManifests: pluginFiles,
		PluginHooks:     pluginEntries,
		Registrations:   registrations,
		Peers:           peers,
		MergeNote:       PeerMergeNote,
		Bypasses:        bypassNotes(managed),
		Caveats:         doctorCaveats(),
	}
}

// worstState returns the state a report leads with: broken outranks warn,
// warn outranks unknown, and unknown outranks ok. Unknown sits ABOVE ok on
// purpose — a question the doctor could not answer must not read as a pass.
func worstState(checks []DoctorCheck) CheckState {
	worst := StateOK
	for _, check := range checks {
		if stateRank(check.State) > stateRank(worst) {
			worst = check.State
		}
	}
	return worst
}

func stateRank(state CheckState) int {
	switch state {
	case StateOK:
		return 0
	case StateUnknown:
		return 1
	case StateWarn:
		return 2
	case StateBroken:
		return 3
	default:
		return 1
	}
}

// versionCheck reports the identity this binary claims for itself. It is a
// self-report: nothing here attests that the binary is the build it names.
func versionCheck(info versioninfo.Info) DoctorCheck {
	version := strings.TrimSpace(info.Version)
	if version == "" || version == versioninfo.Unknown {
		return DoctorCheck{
			Name:  CheckBinary,
			State: StateWarn,
			Detail: "this binary reports no version (" + versioninfo.Unknown + "), which a `go run` or unstamped build does; " +
				"decision records will carry the same empty version, so a record cannot be tied back to a release",
		}
	}
	detail := "boundary " + version
	if commit := strings.TrimSpace(info.Commit); commit != "" && commit != versioninfo.Unknown {
		detail += " (commit " + commit + ")"
	}
	detail += ", " + info.GoVersion + " on " + runtime.GOOS + "/" + runtime.GOARCH +
		"; this is the binary's self-report, not an attestation"
	return DoctorCheck{Name: CheckBinary, State: StateOK, Detail: detail}
}

// registrationCheck reports whether Boundary's PreToolUse hook is wired, at
// which scopes, and over which tools.
//
// The states are graded by what an agent could still do: nothing wired (or hooks
// disabled outright) is BROKEN, because no tool call reaches Boundary at all;
// partial tool coverage or a duplicate registration is WARN, because Boundary
// decides some calls but not the ones the operator probably believes.
//
// An enterprise policy that was actually READ scores here rather than only in
// the bypass list. A machine whose managed settings switch hooks off runs no
// hook whatever this project wires, and a headline status that called that "ok"
// because the project file looks right would be the one lie this command must
// not tell.
//
// A registration a PLUGIN manifest declares is graded separately, as UNKNOWN,
// and never as ok or broken. A hooks/hooks.json states what a plugin registers
// when Claude Code has that plugin enabled; whether it is enabled is not in the
// file, so neither answer can be given from here. Unknown is the honest grade
// and, unlike broken, exits 0 — a plugin install must not be told "Boundary is
// not in front of this agent", which is the one thing exit 1 is supposed to
// mean. Only settings-scope registrations can reach ok, so a manifest can never
// manufacture a clean bill.
func registrationCheck(files []SettingsFile, registrations, pluginRegistrations []HookEntry, managed managedSettings) DoctorCheck {
	if managed.read && managed.disable {
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateBroken,
			Detail: managed.file.Path + " sets disableAllHooks, so no PreToolUse hook runs on this machine and " +
				"nothing this project wires is routed to Boundary",
		}
	}
	if disabled := disabledBy(files); disabled != "" {
		return DoctorCheck{
			Name:   CheckRegistration,
			State:  StateBroken,
			Detail: disabled + " sets disableAllHooks, so no PreToolUse hook runs and no tool call is routed to Boundary",
		}
	}
	if problem := settingsProblem(files); problem != "" {
		return DoctorCheck{Name: CheckRegistration, State: StateBroken, Detail: problem}
	}
	if len(files) == 0 {
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateUnknown,
			Detail: "no settings scope could be searched: neither a project root nor a home directory resolved, " +
				"so whether Boundary is wired is unknown rather than answered",
		}
	}
	if len(registrations) == 0 {
		if len(pluginRegistrations) > 0 {
			return DoctorCheck{
				Name:  CheckRegistration,
				State: StateUnknown,
				Detail: "no PreToolUse hook matching a Boundary shape in " + scopeList(files) + ", but " +
					entryPathList(pluginRegistrations) + " declares one. A plugin manifest registers its hooks only " +
					"when Claude Code has that plugin enabled, and that is not readable from the file, so whether this " +
					"session is routed to Boundary is unknown rather than answered. Prove it with /boundary:drill, or " +
					"wire it in settings with the snippet in docs/integrations/CLAUDE_CODE_HOOK.md",
			}
		}
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateBroken,
			Detail: "no PreToolUse hook matching a Boundary shape in " + scopeList(files) +
				", and no plugin manifest found from here declares one. This check reads settings files and the plugin " +
				"hooks.json manifests it can locate by path shape; a hook registered any other way is not visible to it. " +
				"Wire it with the snippet in docs/integrations/CLAUDE_CODE_HOOK.md",
		}
	}

	// A plugin manifest that declares the same hook is reported alongside the
	// settings wiring rather than folded into it. It does not raise the grade —
	// the settings wiring already carries that — and it must not lower it into a
	// duplicate warning either, because "the plugin also runs" is exactly the
	// thing a manifest cannot establish.
	suffix := ""
	if len(pluginRegistrations) > 0 {
		suffix = ". " + entryPathList(pluginRegistrations) + " also declares this hook; if Claude Code has that " +
			"plugin enabled, one tool call is decided twice and writes one record per run"
	}

	scopes := entryScopes(registrations)
	if duplicated := duplicatedTools(registrations); len(duplicated) > 0 {
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateWarn,
			Detail: fmt.Sprintf("wired %d times across %s, and %s match more than one registration; "+
				"Claude Code runs every matching hook, so one tool call is decided repeatedly and writes one record per run",
				len(registrations), strings.Join(scopes, ", "), joinTools(duplicated)) + suffix,
		}
	}
	covered := coveredTools(registrations)
	if missing := missingTools(covered); len(missing) > 0 {
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateWarn,
			Detail: "wired at " + strings.Join(scopes, ", ") + ", but the matcher does not cover " + joinTools(missing) +
				"; those tool calls are allowed silently and leave no record" + suffix,
		}
	}
	if managed.read && managed.only {
		return DoctorCheck{
			Name:  CheckRegistration,
			State: StateWarn,
			Detail: "wired at " + strings.Join(scopes, ", ") + " over " + joinTools(GovernedToolNames()) + ", but " +
				managed.file.Path + " sets allowManagedHooksOnly, so a hook wired outside the managed file may not run" + suffix,
		}
	}
	return DoctorCheck{
		Name:  CheckRegistration,
		State: StateOK,
		Detail: "wired at " + strings.Join(scopes, ", ") + " over " + joinTools(GovernedToolNames()) +
			"; matched by path shape in the settings file, not by resolving the command" + suffix,
	}
}

// entryPathList names the distinct files a set of entries came from, in the
// order they appear, so a detail line says which manifest it is talking about.
func entryPathList(entries []HookEntry) string {
	seen := map[string]bool{}
	var paths []string
	for _, entry := range entries {
		if seen[entry.SettingsPath] {
			continue
		}
		seen[entry.SettingsPath] = true
		paths = append(paths, entry.SettingsPath)
	}
	if len(paths) == 0 {
		return "no file"
	}
	return strings.Join(paths, ", ")
}

// peerCheck reports other PreToolUse hooks that run on the same tool calls.
//
// A peer is not a defect and not a bypass: it is a second decider on the same
// event, and the operator should know Boundary's answer is merged with it rather
// than final. Every non-Boundary entry is listed even when no governed tool
// overlap is detected, because an overlap this check cannot see is exactly the
// case where a silent listing would mislead.
func peerCheck(peers []HookEntry) DoctorCheck {
	if len(peers) == 0 {
		return DoctorCheck{
			Name:   CheckPeers,
			State:  StateOK,
			Detail: "no other PreToolUse hook found in the settings scopes this check reads",
		}
	}
	overlapping := 0
	unreadable := 0
	for _, peer := range peers {
		if !peer.MatcherUnderstood {
			unreadable++
			continue
		}
		if len(peer.Tools) > 0 {
			overlapping++
		}
	}
	if overlapping == 0 && unreadable == 0 {
		return DoctorCheck{
			Name:  CheckPeers,
			State: StateOK,
			Detail: fmt.Sprintf("%d other PreToolUse hook(s) found, none matching a tool Boundary governs. %s",
				len(peers), PeerMergeNote),
		}
	}
	detail := fmt.Sprintf("%d other PreToolUse hook(s) found; %d also match a tool Boundary governs",
		len(peers), overlapping)
	if unreadable > 0 {
		detail += fmt.Sprintf(" and %d have a matcher this check could not read, so their overlap is unknown", unreadable)
	}
	return DoctorCheck{Name: CheckPeers, State: StateWarn, Detail: detail + ". " + PeerMergeNote}
}

// evidenceDirCheck reports whether a decision could be recorded here.
//
// It applies the sink's own refusals (see Sink) before anything else, because a
// condition the sink refuses is one where every decided call would be escalated:
// a symlinked directory or artifact is BROKEN, not a warning. Writability is
// then established the only way it can be — by writing a marker and removing it.
func evidenceDirCheck(dir string) DoctorCheck {
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return DoctorCheck{
			Name:  CheckEvidenceDir,
			State: StateWarn,
			Detail: dir + " does not exist yet; it is created on the first decided tool call, " +
				"so writability was not probed",
		}
	case err != nil:
		return DoctorCheck{Name: CheckEvidenceDir, State: StateUnknown, Detail: "could not inspect " + dir + ": " + err.Error()}
	case info.Mode()&fs.ModeSymlink != 0:
		return DoctorCheck{
			Name:  CheckEvidenceDir,
			State: StateBroken,
			Detail: dir + " is a symlink; the record sink refuses it, so every decided tool call would be escalated " +
				"rather than recorded",
		}
	case !info.IsDir():
		return DoctorCheck{
			Name:   CheckEvidenceDir,
			State:  StateBroken,
			Detail: dir + " exists and is not a directory; no decision record can be written there",
		}
	}

	for _, artifact := range []string{RecordsDirName, DecisionLogName, SessionSummaryLogName} {
		path := filepath.Join(dir, artifact)
		linked, lstatErr := os.Lstat(path)
		if lstatErr != nil {
			continue
		}
		if linked.Mode()&fs.ModeSymlink != 0 {
			return DoctorCheck{
				Name:   CheckEvidenceDir,
				State:  StateBroken,
				Detail: path + " is a symlink; the sink refuses it, so writes that would land there are refused",
			}
		}
	}

	if err := probeWritable(dir); err != nil {
		return DoctorCheck{
			Name:  CheckEvidenceDir,
			State: StateBroken,
			Detail: "could not write into " + dir + " (" + err.Error() + "); a decision that cannot be recorded is " +
				"escalated to you rather than allowed unrecorded",
		}
	}
	if info.Mode().Perm()&0o077 != 0 {
		return DoctorCheck{
			Name:  CheckEvidenceDir,
			State: StateWarn,
			Detail: fmt.Sprintf("%s is writable but mode is %v; records name governed commands and paths, "+
				"and the sink creates this directory 0700", dir, info.Mode().Perm()),
		}
	}
	return DoctorCheck{
		Name:   CheckEvidenceDir,
		State:  StateOK,
		Detail: dir + " exists, is owner-only, and accepted a probe write that was then removed",
	}
}

// probeWritable writes a marker file and removes it. O_EXCL means an existing
// path is never followed or clobbered, so a planted symlink is refused here too.
// A marker that cannot be removed is reported as the failure it is rather than
// left silently behind.
func probeWritable(dir string) error {
	path := filepath.Join(dir, probeMarkerName)
	// #nosec G304 -- the path is the operator-selected record directory joined
	// with a fixed marker name; no event-supplied string reaches it.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, recordFileMode)
	if err != nil {
		return unwrapPathError(err)
	}
	if _, err := file.WriteString(probeMarkerBody); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return unwrapPathError(err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return unwrapPathError(err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("probe marker %s could not be removed: %w", probeMarkerName, unwrapPathError(err))
	}
	return nil
}

// unwrapPathError reduces a *fs.PathError to its cause, so a detail line states
// the failure once instead of repeating the path it already names.
func unwrapPathError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err
	}
	return err
}

// recordsCheck reports how much evidence exists and how old the newest of it is.
//
// Dormancy is a WARN and never a broken: a session that used no governed tool
// decides nothing and records nothing, so an old log is equally consistent with a
// working hook and an uninstalled one. The check says which it observed, not
// which it concluded.
func recordsCheck(dir string, now time.Time) DoctorCheck {
	logPath := filepath.Join(dir, DecisionLogName)
	count, newest, unreadable, err := summarizeDecisionLog(logPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return DoctorCheck{
			Name:  CheckRecords,
			State: StateWarn,
			Detail: "no decision log at " + logPath + "; no routed tool call has been decided and recorded yet " +
				"(a session that used no governed tool records nothing)",
		}
	case err != nil:
		return DoctorCheck{Name: CheckRecords, State: StateUnknown, Detail: "could not read " + logPath + ": " + err.Error()}
	}

	suffix := ""
	if unreadable > 0 {
		suffix = fmt.Sprintf(", and %d line(s) could not be read as a record", unreadable)
	}
	if count == 0 {
		return DoctorCheck{
			Name:   CheckRecords,
			State:  StateWarn,
			Detail: logPath + " holds no readable record" + suffix,
		}
	}
	age := now.Sub(newest)
	summary := fmt.Sprintf("%d record(s) in %s; newest %s (%s ago)%s",
		count, logPath, newest.UTC().Format(time.RFC3339), age.Round(time.Second), suffix)
	if age > DormancyThreshold {
		return DoctorCheck{
			Name:  CheckRecords,
			State: StateWarn,
			Detail: summary + "; nothing has been decided here in over " + DormancyThreshold.String() +
				", which is what an uninstalled hook and an unused one both look like",
		}
	}
	return DoctorCheck{Name: CheckRecords, State: StateOK, Detail: summary}
}

// summarizeDecisionLog returns the record count, the newest record timestamp,
// and how many lines could not be read as a record. A record with no timestamp
// counts but does not move the newest mark.
func summarizeDecisionLog(path string) (count int, newest time.Time, unreadable int, err error) {
	err = scanDecisionLog(path, func(entry logEntry, ok bool) {
		if !ok {
			unreadable++
			return
		}
		count++
		if entry.Timestamp.After(newest) {
			newest = entry.Timestamp
		}
	})
	return count, newest, unreadable, err
}

// claudeSettings is the subset of a Claude Code settings file the doctor reads.
// Unknown fields are ignored: this is a diagnostic reader, not a schema.
type claudeSettings struct {
	Hooks struct {
		PreToolUse []claudeMatcherGroup `json:"PreToolUse"`
	} `json:"hooks"`
	DisableAllHooks       *bool `json:"disableAllHooks"`
	AllowManagedHooksOnly *bool `json:"allowManagedHooksOnly"`
}

type claudeMatcherGroup struct {
	Matcher string       `json:"matcher"`
	Hooks   []claudeHook `json:"hooks"`
}

type claudeHook struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// settingsScope names one settings file the doctor looks for.
type settingsScope struct {
	scope string
	path  string
}

// hookSettingsScopes returns the settings files the doctor reads, in layering
// order. A scope whose directory could not be resolved is omitted rather than
// guessed, and the registration check reports the gap.
func hookSettingsScopes(cfg DoctorConfig) []settingsScope {
	var scopes []settingsScope
	if cfg.ProjectRoot != "" {
		scopes = append(scopes,
			settingsScope{ScopeProject, filepath.Join(cfg.ProjectRoot, ".claude", "settings.json")},
			settingsScope{ScopeProjectLocal, filepath.Join(cfg.ProjectRoot, ".claude", "settings.local.json")},
		)
	}
	if cfg.HomeDir != "" {
		scopes = append(scopes, settingsScope{ScopeUser, filepath.Join(cfg.HomeDir, ".claude", "settings.json")})
	}
	return scopes
}

// Plugin hook manifest discovery.
const (
	// pluginManifestRel is a plugin's hook manifest, relative to its root. It
	// registers hooks for anyone who has that plugin enabled, with no settings
	// file naming them.
	pluginManifestRel = "hooks/hooks.json"
	// pluginMarkerRel is the file that makes a directory a plugin root. A
	// manifest is only looked for beside one, so an unrelated hooks/hooks.json
	// is not read as a plugin registration.
	pluginMarkerRel = ".claude-plugin/plugin.json"
	// pluginManifestLimit bounds how many manifests one report will carry, so a
	// home directory with a very large plugin collection cannot make this
	// command unbounded. Discovery stops at the limit rather than truncating
	// silently mid-parse.
	pluginManifestLimit = 64
)

// pluginHookManifests returns the plugin hook manifests this machine exposes to
// the doctor, in a fixed search order.
//
// It is a SHAPE search, not an inventory of enabled plugins, and it deliberately
// reads nothing about which plugins Claude Code has switched on — that lives in
// the host's own state, not in the manifests, and guessing at it is how a doctor
// starts reporting a coverage it cannot see. A manifest found here means "this
// plugin declares these hooks", never "these hooks run".
//
// Three roots are searched, each requiring a pluginMarkerRel beside the
// manifest:
//
//   - the project root itself, so a checkout that ships a plugin (this repo
//     does) is seen rather than reported as nothing;
//   - <home>/.claude/skills/*, which is where this repo's own
//     `install-claude-code.sh --plugin-drop` puts the bundle;
//   - <home>/.claude/plugins/*/*, the two-level layout a marketplace or repo
//     install produces.
//
// Globs are one directory level each and nothing is walked recursively. A plugin
// installed somewhere else is simply not found, and the registration check says
// so rather than concluding it is absent.
func pluginHookManifests(cfg DoctorConfig) []settingsScope {
	var roots []string
	if cfg.ProjectRoot != "" {
		roots = append(roots, cfg.ProjectRoot)
	}
	if cfg.HomeDir != "" {
		claude := filepath.Join(cfg.HomeDir, ".claude")
		roots = append(roots, globDirs(filepath.Join(claude, "skills", "*"))...)
		roots = append(roots, globDirs(filepath.Join(claude, "plugins", "*", "*"))...)
	}

	var scopes []settingsScope
	seen := map[string]bool{}
	for _, root := range roots {
		if len(scopes) >= pluginManifestLimit {
			break
		}
		manifest := filepath.Join(root, filepath.FromSlash(pluginManifestRel))
		if seen[manifest] {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(pluginMarkerRel))); err != nil {
			continue
		}
		// Listed on EXISTENCE, not readability: a manifest that is present but
		// refused (a FIFO, an oversized file) must reach readSettingsFile so the
		// report says it was found and could not be read, rather than vanishing.
		if _, err := os.Lstat(manifest); err != nil {
			continue
		}
		seen[manifest] = true
		scopes = append(scopes, settingsScope{ScopePlugin, manifest})
	}
	return scopes
}

// globDirs expands one glob pattern to the directories it matches. A pattern
// that will not compile, or matches nothing, yields nothing: discovery that
// finds less is reported as "not found here", never as "not installed".
func globDirs(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, match)
	}
	return dirs
}

// readPluginHookManifests reads every plugin hook manifest discovery found and
// returns what was found at each path alongside every PreToolUse entry it
// declares.
//
// A manifest's PreToolUse block has the same shape as a settings file's, so it
// is parsed by the same reader and inherits the same refusals. Only the entries
// are used: a plugin's disableAllHooks value, if one ever appeared there, is not
// a policy this doctor would honor, and the check reads none.
func readPluginHookManifests(cfg DoctorConfig) (files []SettingsFile, entries []HookEntry) {
	files = []SettingsFile{}
	entries = []HookEntry{}
	for _, scope := range pluginHookManifests(cfg) {
		file, settings := readSettingsFile(scope.scope, scope.path)
		files = append(files, file)
		if settings == nil {
			continue
		}
		entries = append(entries, hookEntries(scope.scope, scope.path, settings)...)
	}
	return files, entries
}

// readHookSettings reads every settings scope and returns what was found at each
// path alongside every PreToolUse entry, in scope order.
func readHookSettings(cfg DoctorConfig) (files []SettingsFile, entries []HookEntry) {
	files = []SettingsFile{}
	entries = []HookEntry{}
	for _, scope := range hookSettingsScopes(cfg) {
		file, settings := readSettingsFile(scope.scope, scope.path)
		files = append(files, file)
		if settings == nil {
			continue
		}
		entries = append(entries, hookEntries(scope.scope, scope.path, settings)...)
	}
	return files, entries
}

// maxSettingsFileBytes bounds one settings file this reader will accept.
//
// A settings file is hand-written JSON that a person maintains; the ones this
// repo ships are a few hundred bytes and a large real one is a few kilobytes.
// The cap exists for the same reason the decision log has one (see
// maxLogLineBytes): a corrupted or hostile file must not make a reader allocate
// without limit. A settings file is strictly LESS trusted than the log — the
// doctor reads whatever sits at those paths, including a file some other process
// put there — so the bound is not optional here.
const maxSettingsFileBytes = 1 << 20 // 1 MiB

// readSettingsFile reads one settings file. An absent file is reported as absent
// and is not a problem; a present file that cannot be read or parsed is reported
// with the reason, because what it wires cannot be established from here.
//
// Two refusals bound what "read" can cost, and both report a Problem rather than
// blocking or allocating:
//
//   - Only a REGULAR file is opened. A FIFO at .claude/settings.json would
//     otherwise block this process forever on open, and a command whose whole job
//     is to answer "is Boundary in front of this agent" must answer rather than
//     hang. A character device (/dev/zero) would read without end.
//   - At most maxSettingsFileBytes are read, and a file that exceeds it is
//     reported as too large instead of parsed.
//
// The regular-file test follows symlinks on purpose, unlike the sink's
// refuseSymlink: the sink WRITES and must not follow a planted link, while this
// only reads, and a symlinked ~/.claude/settings.json is an ordinary dotfiles
// setup that must not be reported as broken. The test is a point-in-time answer —
// a path swapped between the stat and the open is not caught — which is why the
// size bound is enforced on the read itself rather than taken from the stat.
func readSettingsFile(scope, path string) (SettingsFile, *claudeSettings) {
	file := SettingsFile{Scope: scope, Path: path}
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return file, nil
	case err != nil:
		file.Present = true
		file.Problem = "could not be inspected: " + unwrapPathError(err).Error()
		return file, nil
	}
	file.Present = true
	if !info.Mode().IsRegular() {
		file.Problem = "is not a regular file (mode " + info.Mode().String() + "), so it was not opened; " +
			"what it wires cannot be read here"
		return file, nil
	}

	body, err := readCapped(path, maxSettingsFileBytes)
	if err != nil {
		file.Problem = "could not be read: " + unwrapPathError(err).Error()
		return file, nil
	}

	var settings claudeSettings
	if err := json.Unmarshal(body, &settings); err != nil {
		file.Problem = "is not valid JSON, so what it wires cannot be read here: " + err.Error()
		return file, nil
	}
	file.HooksDisabled = settings.DisableAllHooks
	file.ManagedHooksOnly = settings.AllowManagedHooksOnly
	return file, &settings
}

// errSettingsTooLarge is the cause reported for a settings file over the cap.
var errSettingsTooLarge = errors.New("file is larger than the " +
	strconv.Itoa(maxSettingsFileBytes) + "-byte settings read limit")

// readCapped reads at most limit bytes from path and refuses a file that has
// more. It reads limit+1 bytes so "exactly at the limit" is accepted and
// "one byte over" is refused, rather than silently parsing a truncated prefix —
// a truncated settings file would parse as invalid JSON and be reported as
// malformed, which is a different and misleading answer.
func readCapped(path string, limit int64) ([]byte, error) {
	// #nosec G304 -- the path is composed from the doctor's own project root,
	// home directory, or the platform's managed settings location, plus a fixed
	// file name; no event-supplied string reaches it.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errSettingsTooLarge
	}
	return body, nil
}

// hookEntries flattens one settings file's PreToolUse wiring into entries.
func hookEntries(scope, path string, settings *claudeSettings) []HookEntry {
	var entries []HookEntry
	for _, group := range settings.Hooks.PreToolUse {
		tools, understood := matcherTools(group.Matcher)
		for _, hook := range group.Hooks {
			entries = append(entries, HookEntry{
				Scope:             scope,
				SettingsPath:      path,
				Matcher:           group.Matcher,
				Type:              hook.Type,
				Command:           hook.Command,
				Boundary:          IsBoundaryHookCommand(hook.Command),
				Tools:             tools,
				MatcherUnderstood: understood,
			})
		}
	}
	return entries
}

// splitHookEntries separates the entries Boundary recognizes as its own from the
// rest. Both slices are non-nil so the JSON report renders lists, not nulls.
func splitHookEntries(entries []HookEntry) (registrations, peers []HookEntry) {
	registrations = []HookEntry{}
	peers = []HookEntry{}
	for _, entry := range entries {
		if entry.Boundary {
			registrations = append(registrations, entry)
			continue
		}
		peers = append(peers, entry)
	}
	return registrations, peers
}

// IsBoundaryHookCommand reports whether a settings hook command looks like
// Boundary's PreToolUse hook.
//
// It is a SHAPE match on the string as written — the wrapper script's file name,
// or the binary's `hook pretooluse` lane. Nothing is expanded, resolved, or
// executed, so a command that reaches Boundary some other way (a shell function,
// a wrapper of the wrapper, a renamed script) is not recognized. That direction
// is deliberate: an unrecognized entry is reported as a peer, which over-reports
// peers rather than claiming a registration that may not exist.
func IsBoundaryHookCommand(command string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(command), `\`, "/"))
	if normalized == "" {
		return false
	}
	return strings.Contains(normalized, "pretooluse-boundary.sh") ||
		strings.Contains(normalized, "hook pretooluse")
}

// matcherTools returns the governed tool names a settings matcher covers.
//
// An empty matcher and "*" mean every tool, which is how Claude Code reads them.
// Anything else is compiled as an ANCHORED regular expression, so "Bash|Edit"
// covers exactly those two. A matcher that will not compile returns false: the
// coverage is then unknown and the caller must not report the entry as covering
// anything. A host that matches a pattern more loosely than an anchored regexp
// is not modelled, so this errs toward reporting a tool as uncovered.
func matcherTools(matcher string) (tools []string, understood bool) {
	pattern := strings.TrimSpace(matcher)
	if pattern == "" || pattern == "*" {
		return GovernedToolNames(), true
	}
	compiled, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return nil, false
	}
	tools = []string{}
	for _, tool := range GovernedToolNames() {
		if compiled.MatchString(tool) {
			tools = append(tools, tool)
		}
	}
	return tools, true
}

// coveredTools returns the union of the tools the given entries match.
func coveredTools(entries []HookEntry) map[string]bool {
	covered := map[string]bool{}
	for _, entry := range entries {
		for _, tool := range entry.Tools {
			covered[tool] = true
		}
	}
	return covered
}

// missingTools returns the governed tools no entry covers, in canonical order.
func missingTools(covered map[string]bool) []string {
	var missing []string
	for _, tool := range GovernedToolNames() {
		if !covered[tool] {
			missing = append(missing, tool)
		}
	}
	return missing
}

// duplicatedTools returns the governed tools matched by more than one Boundary
// registration. Two registrations with disjoint matchers are a split install,
// not a duplicate, so overlap — not entry count — is what is reported.
func duplicatedTools(registrations []HookEntry) []string {
	counts := map[string]int{}
	for _, entry := range registrations {
		for _, tool := range entry.Tools {
			counts[tool]++
		}
	}
	var duplicated []string
	for _, tool := range GovernedToolNames() {
		if counts[tool] > 1 {
			duplicated = append(duplicated, tool)
		}
	}
	return duplicated
}

// entryScopes returns the distinct scopes the entries were found in, in the
// order they appear.
func entryScopes(entries []HookEntry) []string {
	seen := map[string]bool{}
	var scopes []string
	for _, entry := range entries {
		if seen[entry.Scope] {
			continue
		}
		seen[entry.Scope] = true
		scopes = append(scopes, entry.Scope)
	}
	return scopes
}

// scopeList names the settings paths that were searched, so "not wired" says
// where the doctor looked.
func scopeList(files []SettingsFile) string {
	if len(files) == 0 {
		return "no readable settings scope"
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Path)
	}
	return strings.Join(names, ", ")
}

// disabledBy returns the path of the first settings file that switches hooks off
// entirely, or "" when none does.
func disabledBy(files []SettingsFile) string {
	for _, file := range files {
		if file.HooksDisabled != nil && *file.HooksDisabled {
			return file.Path
		}
	}
	return ""
}

// settingsProblem returns the first present-but-unreadable settings file as a
// sentence, or "" when every present file was read.
func settingsProblem(files []SettingsFile) string {
	for _, file := range files {
		if file.Present && file.Problem != "" {
			return file.Path + " " + file.Problem
		}
	}
	return ""
}

// joinTools renders a tool list for an operator-facing sentence.
func joinTools(tools []string) string {
	if len(tools) == 0 {
		return "no tool"
	}
	return strings.Join(tools, ", ")
}

// managedSettings is what the doctor could establish about enterprise policy.
type managedSettings struct {
	file    SettingsFile
	read    bool
	disable bool
	only    bool
}

// managedSettingsPath returns the platform's enterprise managed settings path.
// It is a path SHAPE, not a discovery: an administrator who places policy
// somewhere else is not seen from here, which is exactly why the bypass note
// reports unknown rather than a clean bill.
func managedSettingsPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/ClaudeCode/managed-settings.json"
	case "windows":
		return filepath.Join(os.Getenv("PROGRAMDATA"), "ClaudeCode", "managed-settings.json")
	default:
		return "/etc/claude-code/managed-settings.json"
	}
}

// readManagedSettings reads the two enterprise flags that can switch hooks off
// above this project. Anything other than a clean read leaves read false, and
// the bypass note says the state could not be verified from here.
func readManagedSettings(path string) managedSettings {
	state := managedSettings{file: SettingsFile{Scope: ScopeManaged, Path: path}}
	if strings.TrimSpace(path) == "" {
		state.file.Problem = "no managed settings path is known for this platform"
		return state
	}
	file, settings := readSettingsFile(ScopeManaged, path)
	state.file = file
	if settings == nil {
		return state
	}
	state.read = true
	state.disable = settings.DisableAllHooks != nil && *settings.DisableAllHooks
	state.only = settings.AllowManagedHooksOnly != nil && *settings.AllowManagedHooksOnly
	return state
}

// bypassNote renders the enterprise-policy line of the bypass report.
func (m managedSettings) bypassNote() BypassNote {
	if !m.read {
		return BypassNote{
			Route: "enterprise-managed-settings",
			State: StateUnknown,
			Detail: "cannot verify from here: " + m.file.Path + " was not readable, so disableAllHooks and " +
				"allowManagedHooksOnly are unknown. An administrator can switch hooks off, or restrict them to " +
				"managed ones, without this project seeing it",
		}
	}
	switch {
	case m.disable:
		return BypassNote{
			Route:  "enterprise-managed-settings",
			State:  StateBroken,
			Detail: m.file.Path + " sets disableAllHooks: no PreToolUse hook runs, whatever this project wires",
		}
	case m.only:
		return BypassNote{
			Route: "enterprise-managed-settings",
			State: StateWarn,
			Detail: m.file.Path + " sets allowManagedHooksOnly: a hook wired in a project or user scope may not run, " +
				"so this project's registration is not proof Boundary is invoked",
		}
	default:
		return BypassNote{
			Route: "enterprise-managed-settings",
			State: StateOK,
			Detail: m.file.Path + " was read and sets neither disableAllHooks nor allowManagedHooksOnly; " +
				"that is a statement about this file only",
		}
	}
}

// bypassNotes returns the routes this hook does not govern.
//
// The list is fixed rather than discovered. These routes are open by
// construction — routed interception IS the boundary — and a report that only
// listed what it happened to find would read as a coverage claim by omission.
func bypassNotes(managed managedSettings) []BypassNote {
	return []BypassNote{
		{
			Route: "mcp-tools",
			State: StateWarn,
			Detail: "MCP tool calls do not reach this hook and are not governed here. Govern them at the MCP route " +
				"(docs/GOVERN_MCP_SERVER.md); this report makes no claim about whether that route is installed",
		},
		{
			Route: "subagent-spawned-processes",
			State: StateWarn,
			Detail: "A process a governed tool starts runs its own commands with no PreToolUse event. The hook decides " +
				"the tool call, never the process tree underneath it",
		},
		{
			Route: "shell-outside-claude-code",
			State: StateWarn,
			Detail: "A terminal, an editor, cron, CI, or an SSH session runs commands with no hook in front of them. " +
				"Closing that path is a deployment responsibility, not a hook flag",
		},
		{
			Route: "unmatched-tools",
			State: StateWarn,
			Detail: "A tool outside the wired matcher is allowed silently and leaves no record, because nothing was " +
				"decided about it",
		},
		{
			Route: "other-boundary-routes",
			State: StateUnknown,
			Detail: "This report covers the Claude Code PreToolUse hook route only. It asserts nothing about the " +
				"GitHub, MCP, gateway, or CLI routes, installed or not",
		},
		managed.bypassNote(),
	}
}

// doctorCaveats are the standing limitations of the whole report.
func doctorCaveats() []string {
	return []string{
		"This report describes the wiring and evidence it can read on this machine. It is not a claim that Boundary is " +
			"the only route an agent has to a shell or to the filesystem.",
		"Registration is matched by path shape in settings files. Nothing is executed and no command is resolved, so a " +
			"registration reported here is a wiring that looks like Boundary's, not proof that Boundary decided anything.",
		"Plugin hook manifests are found by path shape in a fixed set of locations, and a manifest states what a plugin " +
			"registers WHEN Claude Code has that plugin enabled. Whether it is enabled is not in the file, so a manifest " +
			"registration is reported as unknown, never as wired; and a plugin installed outside those locations is not " +
			"seen here at all.",
		"Command Boundary and Edit Boundary are delivered previews. A wired hook is not a production GA guarantee, and " +
			"their classification posture may change.",
		"Decision records are hash-verifiable for integrity, not authenticity, and are not proof that an action was " +
			"executed or prevented.",
	}
}

// WriteDoctorText renders a report as human-readable text.
//
// It prints every section the JSON carries, including the bypass list and the
// caveats: an operator reading the terminal must not get a shorter, cleaner story
// than a script reading the JSON.
func WriteDoctorText(w io.Writer, report DoctorReport) error {
	var b strings.Builder
	b.WriteString("Boundary Claude Code hook doctor\n")
	fmt.Fprintf(&b, "status:  %s\n", report.Status)
	fmt.Fprintf(&b, "project: %s\n", emptyAsUnknown(report.ProjectRoot))
	fmt.Fprintf(&b, "records: %s\n", emptyAsUnknown(report.RecordDir))

	b.WriteString("\nChecks:\n")
	for _, check := range report.Checks {
		fmt.Fprintf(&b, "  %-9s %s\n", "["+string(check.State)+"]", check.Name)
		fmt.Fprintf(&b, "            %s\n", check.Detail)
	}

	b.WriteString("\nSettings scopes:\n")
	for _, file := range report.SettingsFiles {
		fmt.Fprintf(&b, "  %-14s %s%s\n", file.Scope, file.Path, settingsSuffix(file))
	}

	b.WriteString("\nPlugin hook manifests:\n")
	if len(report.PluginManifests) == 0 {
		b.WriteString("  none found in the locations this check searches\n")
	} else {
		for _, file := range report.PluginManifests {
			fmt.Fprintf(&b, "  %-14s %s%s\n", file.Scope, file.Path, settingsSuffix(file))
		}
		writeEntries(&b, report.PluginHooks, "no PreToolUse entry declared by those manifests")
	}

	b.WriteString("\nBoundary registrations:\n")
	writeEntries(&b, report.Registrations, "none found")
	b.WriteString("\nPeer PreToolUse hooks:\n")
	writeEntries(&b, report.Peers, "none found")
	if len(report.Peers) > 0 {
		fmt.Fprintf(&b, "  note: %s\n", report.MergeNote)
	}

	b.WriteString("\nNot governed by this hook:\n")
	for _, note := range report.Bypasses {
		fmt.Fprintf(&b, "  %-9s %s\n", "["+string(note.State)+"]", note.Route)
		fmt.Fprintf(&b, "            %s\n", note.Detail)
	}

	b.WriteString("\nCaveats:\n")
	for _, caveat := range report.Caveats {
		fmt.Fprintf(&b, "  - %s\n", caveat)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// writeEntries renders one hook entry list, or a placeholder when it is empty.
func writeEntries(b *strings.Builder, entries []HookEntry, empty string) {
	if len(entries) == 0 {
		fmt.Fprintf(b, "  %s\n", empty)
		return
	}
	for _, entry := range entries {
		fmt.Fprintf(b, "  %-14s %s\n", entry.Scope, entry.SettingsPath)
		fmt.Fprintf(b, "                 matcher %q -> tools %s\n", entry.Matcher, entryTools(entry))
		fmt.Fprintf(b, "                 command %s\n", entry.Command)
	}
}

// entryTools renders an entry's covered tools, distinguishing "covers nothing"
// from "the matcher could not be read".
func entryTools(entry HookEntry) string {
	if !entry.MatcherUnderstood {
		return "unknown (matcher is not a readable pattern)"
	}
	if len(entry.Tools) == 0 {
		return "none of the tools Boundary governs"
	}
	return strings.Join(entry.Tools, ", ")
}

// settingsSuffix annotates a settings path with what was found there.
func settingsSuffix(file SettingsFile) string {
	switch {
	case !file.Present:
		return " (not present)"
	case file.Problem != "":
		return " (" + file.Problem + ")"
	case file.HooksDisabled != nil && *file.HooksDisabled:
		return " (present; disableAllHooks is set)"
	default:
		return " (present)"
	}
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
