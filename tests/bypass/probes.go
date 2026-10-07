//go:build bypass

package bypass

import (
	"fmt"
	"strings"
	"testing"
)

// Probe is one bypass attempt or governed-route check. Steps are argv vectors
// executed in order inside the topology as the unprivileged user; Judge
// inspects the captured results and reports pass/fail plus a short observed
// summary for the evidence record.
//
// Expectations live in Judge, not in fixed strings: a probe asserts real exit
// codes, real stdout/stderr, and (for governed probes) the decision record
// Boundary wrote — never a string the test itself echoed.
type Probe struct {
	Name        string
	Description string
	Expected    string
	Steps       [][]string
	Judge       func(steps []ExecResult) (pass bool, observed string)
}

// RunProbe executes every step of p as the unprivileged user and returns the
// populated report entry.
func RunProbe(t *testing.T, h Harness, p Probe) ProbeReport {
	t.Helper()
	results := make([]ExecResult, 0, len(p.Steps))
	for _, argv := range p.Steps {
		results = append(results, h.Exec(t, argv...))
	}
	pass, observed := p.Judge(results)
	return ProbeReport{
		Name:        p.Name,
		Description: p.Description,
		Expected:    p.Expected,
		Observed:    observed,
		Passed:      pass,
		Steps:       results,
	}
}

// BlockedJudge returns a Judge for an adversarial probe: every step must exit
// non-zero and no step may print the marker, which is the only output that can
// come from executing the protected tool.
func BlockedJudge(marker string) func([]ExecResult) (bool, string) {
	return func(steps []ExecResult) (bool, string) {
		var codes []string
		for _, step := range steps {
			codes = append(codes, fmt.Sprint(step.ExitCode))
			if strings.Contains(step.Stdout, marker) {
				return false, fmt.Sprintf("bypass succeeded: marker appeared on stdout (exit codes %s)", strings.Join(codes, ","))
			}
			if step.ExitCode == 0 {
				return false, fmt.Sprintf("step exited 0 without blocking (exit codes %s)", strings.Join(codes, ","))
			}
		}
		return true, fmt.Sprintf("all %d step(s) blocked, no marker (exit codes %s)", len(steps), strings.Join(codes, ","))
	}
}

// markerIn reports whether the protected tool's marker appeared in output.
func markerIn(marker, output string) bool {
	return strings.Contains(output, marker)
}
