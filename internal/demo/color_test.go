package demo

import (
	"bytes"
	"strings"
	"testing"
)

func TestColorizerDisabledForNonTerminalWriter(t *testing.T) {
	// A bytes.Buffer is never a terminal, so the colorizer must be a no-op and
	// never emit ANSI escape bytes — this is the guarantee that keeps piped and
	// captured demo output copy-paste clean.
	c := NewColorizer(&bytes.Buffer{})
	if c.Enabled() {
		t.Fatalf("colorizer must be disabled for a non-terminal writer")
	}
	for name, got := range map[string]string{
		"Deny":    c.Deny("DENY"),
		"Pass":    c.Pass("pass"),
		"Fail":    c.Fail("fail"),
		"Dim":     c.Dim("rec_123"),
		"Bold":    c.Bold("Header"),
		"Verdict": c.Verdict("DENY"),
	} {
		if strings.ContainsRune(got, '\033') {
			t.Fatalf("%s emitted an ANSI escape for a non-terminal writer: %q", name, got)
		}
	}
	if got := c.Deny("DENY"); got != "DENY" {
		t.Fatalf("disabled Deny = %q, want unchanged %q", got, "DENY")
	}
}

func TestNilColorizerIsPlain(t *testing.T) {
	var c *Colorizer
	if c.Enabled() {
		t.Fatalf("nil colorizer must report disabled")
	}
	if got := c.Verdict("DENY"); got != "DENY" {
		t.Fatalf("nil Verdict = %q, want %q", got, "DENY")
	}
	if got := c.Bold("x"); got != "x" {
		t.Fatalf("nil Bold = %q, want %q", got, "x")
	}
}

func TestColorEnabledDecision(t *testing.T) {
	// A *os.File-less writer can still exercise the env-var gates: colorEnabled
	// short-circuits to false before the TTY check whenever NO_COLOR is set or
	// TERM is dumb/empty, and otherwise defers to isTerminal (false for a
	// buffer). This pins the precedence without needing a real PTY.
	buf := &bytes.Buffer{}
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no_color_set", map[string]string{"NO_COLOR": "1", "TERM": "xterm"}, false},
		{"no_color_empty_value_still_disables", map[string]string{"NO_COLOR": "", "TERM": "xterm"}, false},
		{"term_dumb", map[string]string{"TERM": "dumb"}, false},
		{"term_missing", map[string]string{}, false},
		{"term_xterm_but_not_tty", map[string]string{"TERM": "xterm"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				v, ok := tc.env[key]
				return v, ok
			}
			if got := colorEnabled(buf, lookup); got != tc.want {
				t.Fatalf("colorEnabled = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestVerdictColorMappingWhenEnabled(t *testing.T) {
	// Force-enable to assert the semantic mapping: DENY/FAIL red, ALLOW/PASS
	// green, WARN/ESCALATE uncolored (never a misleading "safe" green).
	c := &Colorizer{enabled: true}
	if !strings.Contains(c.Verdict("DENY"), ansiRed) {
		t.Fatalf("DENY should be red: %q", c.Verdict("DENY"))
	}
	if !strings.Contains(c.Verdict("allow"), ansiGreen) {
		t.Fatalf("allow should be green: %q", c.Verdict("allow"))
	}
	if got := c.Verdict("require_approval"); strings.ContainsRune(got, '\033') {
		t.Fatalf("require_approval must stay uncolored: %q", got)
	}
	if got := c.Verdict("WARN"); strings.ContainsRune(got, '\033') {
		t.Fatalf("WARN must stay uncolored: %q", got)
	}
}
