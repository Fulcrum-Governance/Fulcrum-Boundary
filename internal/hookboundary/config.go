package hookboundary

import (
	"os"
	"strings"
	"time"
)

// Environment knobs the hook reads. They mirror the shell hook's names so an
// existing installation keeps its configuration when it switches to the binary.
const (
	// EnvFailMode selects the fault posture: open, closed, or ask (default).
	EnvFailMode = "BOUNDARY_HOOK_FAILMODE"
	// EnvAgentID labels the acting agent in decision records (advisory).
	EnvAgentID = "BOUNDARY_HOOK_AGENT_ID"
	// EnvRecordDir overrides the directory decision records are written to.
	EnvRecordDir = "BOUNDARY_HOOK_DIR"
	// EnvDebug, when non-empty, asks the CLI for diagnostic lines on stderr.
	EnvDebug = "BOUNDARY_HOOK_DEBUG"
)

// Defaults for a hook decision when the caller and the environment say nothing.
const (
	// DefaultAgentID labels records that carry no explicit agent id.
	DefaultAgentID = "claude-code"
	// DefaultTenantID matches the local tenant the other preview surfaces use.
	DefaultTenantID = "local"
)

// FailMode is what the hook does on an internal FAULT — an unreadable or
// malformed event, a tool_input with nothing to classify, a classifier error.
// It never governs a Boundary decision: a policy deny always blocks.
type FailMode string

const (
	// FailModeAsk escalates a fault to the user (the default). Boundary could
	// not decide, so it declines to answer for the operator in either
	// direction.
	FailModeAsk FailMode = "ask"
	// FailModeOpen allows a fault silently, the shell hook's original posture,
	// so a broken hook never bricks an interactive session.
	FailModeOpen FailMode = "open"
	// FailModeClosed denies a fault.
	FailModeClosed FailMode = "closed"
)

// ParseFailMode reads a fail mode from a configuration value. An empty or
// unrecognized value resolves to FailModeAsk: an operator typo must not silently
// widen or narrow the fault posture.
func ParseFailMode(raw string) FailMode {
	switch FailMode(strings.ToLower(strings.TrimSpace(raw))) {
	case FailModeOpen:
		return FailModeOpen
	case FailModeClosed:
		return FailModeClosed
	default:
		return FailModeAsk
	}
}

// Verdict returns the verdict a fault resolves to under this fail mode.
func (m FailMode) Verdict() Verdict {
	switch m {
	case FailModeOpen:
		return VerdictAllow
	case FailModeClosed:
		return VerdictDeny
	default:
		return VerdictRequireApproval
	}
}

// Config configures one hook decision. The zero value is usable: it writes
// records under DefaultRecordDir relative to the hook process's own working
// directory, asks on faults, and labels records with DefaultAgentID.
type Config struct {
	// Dir is the directory decision records are written to. Empty means
	// DefaultRecordDir. It is always a path the operator or this process
	// chose — never one taken from the event.
	Dir string
	// ProjectRoot is the directory an absolute edit target is resolved against
	// on the edit route (see ProjectRelativeEditPath). Empty resolves to the
	// hook process's own working directory, which is the project directory
	// Claude Code launches a PreToolUse hook in.
	//
	// It is deliberately NOT taken from the event's cwd: the event is untrusted
	// input, and a producer that could choose the project root could relativize
	// any path into scope. When it cannot be resolved, every absolute edit
	// target stays absolute and denies as outside project scope.
	ProjectRoot string
	// FailMode is the fault posture. Empty means FailModeAsk.
	FailMode FailMode
	// AgentID labels the acting agent in records. Empty means DefaultAgentID.
	AgentID string
	// TenantID labels the owning tenant. Empty means DefaultTenantID.
	TenantID string
	// BoundaryVersion is recorded as boundary_version. Empty is recorded as
	// empty rather than guessed.
	BoundaryVersion string
	// Now supplies the decision timestamp. Empty means time.Now.
	Now func() time.Time
}

// ConfigFromEnv builds a Config from the hook's environment knobs. getenv is
// injected so callers (and tests) control the environment explicitly; pass
// os.Getenv in production.
func ConfigFromEnv(getenv func(string) string, boundaryVersion string) Config {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return Config{
		Dir:             strings.TrimSpace(getenv(EnvRecordDir)),
		FailMode:        ParseFailMode(getenv(EnvFailMode)),
		AgentID:         strings.TrimSpace(getenv(EnvAgentID)),
		BoundaryVersion: boundaryVersion,
	}
}

// withDefaults returns a copy of cfg with every unset field resolved, so the
// decision path never has to re-check for empties.
//
// ProjectRoot is the one field resolved from outside the struct: an unset root
// falls back to the process's working directory, matching where the record sink
// already writes. A working directory that cannot be read leaves it empty, which
// denies absolute edit targets rather than admitting them.
func (c Config) withDefaults() Config {
	if c.Dir == "" {
		c.Dir = DefaultRecordDir
	}
	if c.ProjectRoot == "" {
		if wd, err := os.Getwd(); err == nil {
			c.ProjectRoot = wd
		}
	}
	if c.FailMode == "" {
		c.FailMode = FailModeAsk
	}
	if c.AgentID == "" {
		c.AgentID = DefaultAgentID
	}
	if c.TenantID == "" {
		c.TenantID = DefaultTenantID
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}
