// Package madriver implements the Managed Agents conformance session driver.
// It runs a real Managed Agents session through the Boundary managedagents
// adapter (SessionProxy + ToolResolver + governance.Pipeline) and emits a raw
// event log plus a sanitized transcript in the shape the conformance harness
// under tests/conformance/managed_agents reads.
//
// The driver IS the Boundary front for this adapter: adapters/managedagents is
// a library surface (EventSource/EventSink/ConfirmationForwarder), and
// cmd/boundary does not expose a managed-agents listener, so embedding is the
// only fronting mechanism the code provides. In live mode the driver holds the
// upstream credential, matching the documented credential bypass model
// (docs/deployment/managed-agents-bypass-proofing.md).
package madriver

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	// ModeStub runs a fully offline session against an in-process fake of the
	// upstream Managed Agents API that emits scripted events.
	ModeStub = "stub"
	// ModeLive talks to the real upstream through the Boundary adapter stack.
	ModeLive = "live"

	// evidenceDirName is the operator evidence directory under the user's
	// home (~/.fulcrum-evidence/ma-conformance). It is intentionally outside
	// the git worktree; the path is derived from the home directory rather
	// than hardcoded so no machine-specific absolute path enters the repo.
	evidenceDirName = ".fulcrum-evidence/ma-conformance"

	// liveGoFileName is the operator-created gate file that must exist inside
	// the evidence directory before a live run is allowed to start.
	liveGoFileName = "LIVE_GO"

	// UpstreamKeyEnv names the environment variable that carries the upstream
	// API key in live mode. Stub mode must never read it.
	UpstreamKeyEnv = "BOUNDARY_MA_UPSTREAM_KEY"

	// BetaHeader is the anthropic-beta header value the Managed Agents docs
	// pin for this surface.
	BetaHeader = "managed-agents-2026-04-01"

	// DefaultAPIBase is the documented Managed Agents API base.
	DefaultAPIBase = "https://api.anthropic.com"

	hardMaxSpendUSD = 15.00
	defaultDenyTool = "delete_production_issue"
	defaultErrTool  = "fulcrum_failclosed_probe"
)

// Config is the parsed driver configuration.
type Config struct {
	Mode            string
	OutDir          string
	MaxSpendUSD     float64
	MaxTurns        int
	MaxOutputTokens int
	Timeout         time.Duration
	// UsageBlindEvents and UsageBlindWindow bound how long the stream may run
	// without any usable usage signal before the driver fails closed.
	UsageBlindEvents int
	UsageBlindWindow time.Duration
	AckSpend         bool // --i-understand-this-spends-money
	// AllowInsecureAPIBase bypasses the https+api.anthropic.com --api-base
	// check. It exists for tests only; a live run must never need it.
	AllowInsecureAPIBase bool
	AgentID              string
	EnvironmentID        string
	APIBase              string
	Prompt               string
	DenyTool             string
	ErrorTool            string
	TenantID             string
}

// DefaultOutDir returns the default evidence output directory,
// ~/.fulcrum-evidence/ma-conformance, which sits outside the git worktree.
func DefaultOutDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return evidenceDirName
	}
	return filepath.Join(home, evidenceDirName)
}

// LiveGoPath returns the path of the operator gate file a live run requires,
// ~/.fulcrum-evidence/ma-conformance/LIVE_GO.
func LiveGoPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(evidenceDirName, liveGoFileName)
	}
	return filepath.Join(home, evidenceDirName, liveGoFileName)
}

// DefaultConfig returns the flag defaults shared by main and tests.
func DefaultConfig() Config {
	return Config{
		Mode:             ModeStub,
		OutDir:           DefaultOutDir(),
		MaxSpendUSD:      5.00,
		MaxTurns:         6,
		MaxOutputTokens:  1024,
		Timeout:          10 * time.Minute,
		UsageBlindEvents: 8,
		UsageBlindWindow: 60 * time.Second,
		APIBase:          DefaultAPIBase,
		Prompt:           "Read the repository README and summarize it in one paragraph.",
		DenyTool:         defaultDenyTool,
		ErrorTool:        defaultErrTool,
		TenantID:         "ma-conformance",
	}
}

// validateCommon checks mode, spend ceilings, and the out-dir location.
// out-dir must resolve outside the repository worktree that contains this
// source file so raw evidence can never be committed by accident.
func (c Config) validateCommon(repoRoot string) error {
	if c.Mode != ModeStub && c.Mode != ModeLive {
		return fmt.Errorf("invalid --mode %q: want %q or %q", c.Mode, ModeStub, ModeLive)
	}
	if strings.TrimSpace(c.OutDir) == "" {
		return errors.New("--out-dir is required")
	}
	if err := outDirOutsideRepo(c.OutDir, repoRoot); err != nil {
		return err
	}
	if c.MaxSpendUSD <= 0 {
		return fmt.Errorf("--max-spend-usd must be > 0, got %.2f", c.MaxSpendUSD)
	}
	if c.MaxSpendUSD > hardMaxSpendUSD {
		return fmt.Errorf("--max-spend-usd %.2f exceeds hard maximum %.2f", c.MaxSpendUSD, hardMaxSpendUSD)
	}
	if c.MaxTurns <= 0 {
		return fmt.Errorf("--max-turns must be > 0, got %d", c.MaxTurns)
	}
	if c.MaxOutputTokens <= 0 {
		return fmt.Errorf("--max-output-tokens must be > 0, got %d", c.MaxOutputTokens)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("--timeout must be > 0, got %s", c.Timeout)
	}
	if c.UsageBlindEvents <= 0 {
		return fmt.Errorf("--usage-blind-events must be > 0, got %d", c.UsageBlindEvents)
	}
	if c.UsageBlindWindow <= 0 {
		return fmt.Errorf("--usage-blind-window must be > 0, got %s", c.UsageBlindWindow)
	}
	return nil
}

// checkLiveGates enforces the live-mode gates in order: the acknowledgement
// flag, the operator gate file, the --api-base policy, and only then the
// upstream key environment variable — the key value must not be read when an
// earlier gate has already failed. The key is checked for presence only and
// never logged or persisted.
func checkLiveGates(c Config, getenv func(string) string, fileExists func(string) bool) error {
	if !c.AckSpend {
		return errors.New("live mode refused: missing flag --i-understand-this-spends-money")
	}
	if !fileExists(LiveGoPath()) {
		return errors.New("live mode refused: missing gate file " + LiveGoPath())
	}
	if err := checkAPIBase(c); err != nil {
		return err
	}
	if strings.TrimSpace(getenv(UpstreamKeyEnv)) == "" {
		return errors.New("live mode refused: missing env " + UpstreamKeyEnv)
	}
	if strings.TrimSpace(c.AgentID) == "" {
		return errors.New("live mode requires --agent (upstream managed agent id)")
	}
	if strings.TrimSpace(c.EnvironmentID) == "" {
		return errors.New("live mode requires --environment-id (upstream environment id)")
	}
	return nil
}

// checkAPIBase refuses any --api-base other than the documented upstream
// before the first request can be built, so a mistyped or hostile base can
// never receive the key. --allow-insecure-api-base exists only so tests may
// point a fake upstream at httptest listeners.
func checkAPIBase(c Config) error {
	if c.AllowInsecureAPIBase {
		return nil
	}
	u, err := url.Parse(c.APIBase)
	if err != nil || u.Scheme != "https" || u.Host != "api.anthropic.com" {
		return fmt.Errorf("live mode requires --api-base https://api.anthropic.com, got %q", c.APIBase)
	}
	return nil
}

// validate runs the common checks plus live gates when Mode == ModeLive. The
// getenv/fileExists seams exist so tests can exercise the gates without the
// real credential or operator file. Stub mode never consults getenv.
func (c Config) validate(repoRoot string, getenv func(string) string, fileExists func(string) bool) error {
	if err := c.validateCommon(repoRoot); err != nil {
		return err
	}
	if c.Mode == ModeLive {
		return checkLiveGates(c, getenv, fileExists)
	}
	return nil
}

// outDirOutsideRepo rejects any output directory that resolves to the repo
// root or a path inside it. Both paths are symlink-resolved first — including
// the deepest existing ancestor of a not-yet-created out-dir — so a symlink
// cannot smuggle the evidence directory back inside the worktree.
func outDirOutsideRepo(outDir, repoRoot string) error {
	absOut, err := resolveSymlinks(outDir)
	if err != nil {
		return fmt.Errorf("resolve --out-dir: %w", err)
	}
	absRoot, err := resolveSymlinks(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absOut)
	if err != nil {
		return fmt.Errorf("compare --out-dir to repo root: %w", err)
	}
	// A relative path that does not start with ".." resolves inside the
	// worktree; reject the root itself ("."), too.
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return fmt.Errorf("--out-dir %q must be outside the git worktree %q", absOut, absRoot)
	}
	return nil
}

// resolveSymlinks returns the absolute, symlink-resolved form of path. When
// path does not exist yet, it resolves the deepest existing ancestor and
// re-appends the missing tail so callers can validate directories that will
// only be created later.
func resolveSymlinks(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir := abs
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(dir)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing ancestor for %q", path)
		}
		tail = append(tail, filepath.Base(dir))
		dir = parent
	}
}

// driverRepoRoot walks up from this source file to the directory containing
// go.mod. The driver always builds inside the Fulcrum-Boundary worktree, so
// the discovered root is the worktree that --out-dir must stay outside of.
func driverRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate driver source path")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above driver source")
		}
		dir = parent
	}
}
