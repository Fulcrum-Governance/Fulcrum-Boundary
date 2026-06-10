package demo

import (
	"io"
	"os"
)

// ANSI SGR escape sequences used by the demo colorizer. They are intentionally
// the small, widely supported subset (basic 8-color foreground plus dim/bold);
// the demos are status output, not a TUI, so richer styling is out of scope.
const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	ansiDim   = "\033[2m"
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
)

// Colorizer applies terminal styling to demo output, but only when the
// destination is an interactive terminal. It is the single gate that keeps
// demo output copy-paste-clean: when stdout is piped, redirected, or captured
// (anything that is not a character device), every method returns its argument
// unchanged so no ANSI bytes ever reach a file, a test buffer, or `| cat`.
//
// Disable rules (any one disables color):
//   - the NO_COLOR environment variable is set to any value (https://no-color.org);
//   - TERM is "dumb" or empty;
//   - the writer is not an *os.File backed by a character device (i.e. not a TTY).
//
// A nil *Colorizer is valid and behaves as fully disabled, so callers can hold
// an optional colorizer without nil checks at every call site.
type Colorizer struct {
	enabled bool
}

// NewColorizer builds a Colorizer whose styling is enabled only when w is an
// interactive terminal and the environment does not opt out. Pass the same
// io.Writer the demo will actually write to (typically os.Stdout); the TTY
// decision is made against that writer's underlying file descriptor so piped
// runs stay plain.
func NewColorizer(w io.Writer) *Colorizer {
	return &Colorizer{enabled: colorEnabled(w, os.LookupEnv)}
}

// colorEnabled centralizes the enable decision so it can be unit-tested without
// a real terminal. lookupEnv is injected (os.LookupEnv in production) so tests
// can drive NO_COLOR / TERM deterministically.
func colorEnabled(w io.Writer, lookupEnv func(string) (string, bool)) bool {
	if _, ok := lookupEnv("NO_COLOR"); ok {
		return false
	}
	if term, ok := lookupEnv("TERM"); !ok || term == "" || term == "dumb" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a character device (a TTY). It uses only the
// standard library: a writer is a terminal when it is an *os.File whose mode
// has os.ModeCharDevice set. Non-file writers (bytes.Buffer, pipes, regular
// files) are never terminals, which is exactly the "no ANSI when not a TTY"
// guarantee the demos need.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// Enabled reports whether this colorizer will emit ANSI styling. It is exposed
// so callers can branch on color availability for layout (not required for the
// styling methods, which already no-op when disabled).
func (c *Colorizer) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Colorizer) wrap(open, s string) string {
	if !c.Enabled() {
		return s
	}
	return open + s + ansiReset
}

// Deny styles a denial token (DENY and similar terminal verdicts) in red. This
// is the demos' headline moment, so it gets the strongest color.
func (c *Colorizer) Deny(s string) string { return c.wrap(ansiRed, s) }

// Pass styles a passing check or ALLOW verdict in green.
func (c *Colorizer) Pass(s string) string { return c.wrap(ansiGreen, s) }

// Fail styles a failing check in red. It shares red with Deny because a failed
// demo check and a denial are both "stop and look" states.
func (c *Colorizer) Fail(s string) string { return c.wrap(ansiRed, s) }

// Dim styles low-salience evidence (record ids, decision hashes) so the eye
// skips past it to the verdict.
func (c *Colorizer) Dim(s string) string { return c.wrap(ansiDim, s) }

// Bold styles a heading or label for emphasis without implying a verdict color.
func (c *Colorizer) Bold(s string) string { return c.wrap(ansiBold, s) }

// Verdict styles an action token by its meaning: DENY/FAIL in red, ALLOW/PASS
// in green, everything else (WARN, ESCALATE, REQUIRE_APPROVAL, unknown) left
// uncolored so the demo never implies a "safe" green for an escalation. The
// match is case-insensitive on the trimmed token but the returned string keeps
// the caller's original casing.
func (c *Colorizer) Verdict(s string) string {
	switch normalizeVerdictToken(s) {
	case "DENY", "FAIL":
		return c.Deny(s)
	case "ALLOW", "PASS":
		return c.Pass(s)
	default:
		return s
	}
}

func normalizeVerdictToken(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		if r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		out = append(out, r)
	}
	return string(out)
}
