package boundarycli

import (
	"bytes"
	"strings"
	"testing"
)

// TestExplainAcceptsFlagsAfterThePositional pins the CLI half of the round-3
// exact-form truth fix: `boundary explain record.json --json` must parse the
// same way `boundary explain --json record.json` does, mirroring
// verify-record's interspersed parsing, because the first-party classifier
// recognizes both orders.
func TestExplainAcceptsFlagsAfterThePositional(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExplain([]string{"../../docs/examples/decision-record.example.json", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\nstderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
		t.Fatalf("--json after the positional did not emit JSON:\n%s", stdout.String())
	}
}

// TestExplainStillRequiresExactlyOneRecord keeps the positional contract:
// zero or two record paths remain usage errors under interspersed parsing.
func TestExplainStillRequiresExactlyOneRecord(t *testing.T) {
	for _, args := range [][]string{{"--json"}, {"a.json", "b.json"}} {
		var stdout, stderr bytes.Buffer
		if code := runExplain(args, &stdout, &stderr); code != 1 {
			t.Fatalf("runExplain(%v) exit = %d, want 1", args, code)
		}
		if !strings.Contains(stderr.String(), "usage:") {
			t.Fatalf("runExplain(%v) stderr is not a usage error: %s", args, stderr.String())
		}
	}
}
