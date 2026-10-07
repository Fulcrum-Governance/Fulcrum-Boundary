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

// denyIndicators are output fragments that prove a step was refused by the
// platform — execve EACCES/ENOENT, an unprivileged su, an unreadable file —
// rather than failing for some unrelated reason. Each entry was observed in
// cli-reference-v1 output: busybox "Permission denied" / "can't stat", the OCI
// runtime's "permission denied" / "executable file not found", and busybox
// su's "must be suid to work properly". Matching is case-insensitive over
// stdout+stderr.
var denyIndicators = []string{
	"denied",
	"not found",
	"no such file",
	"not permitted",
	"must be suid",
	"refused",
	"authentication failure",
	"incorrect password",
}

// BlockedJudge returns a Judge for an adversarial probe: every step must exit
// non-zero, no step may print the marker (the only output that can come from
// executing the protected tool, checked across stdout and stderr), and every
// step must show a platform denial indicator — a step that fails for an
// incidental reason is not evidence the attack was blocked.
func BlockedJudge(marker string) func([]ExecResult) (bool, string) {
	return func(steps []ExecResult) (bool, string) {
		var codes []string
		for i, step := range steps {
			codes = append(codes, fmt.Sprint(step.ExitCode))
			output := step.Stdout + step.Stderr
			if strings.Contains(output, marker) {
				return false, fmt.Sprintf("bypass succeeded: marker appeared in step %d output (exit codes %s)", i+1, strings.Join(codes, ","))
			}
			if step.ExitCode == 0 {
				return false, fmt.Sprintf("step %d exited 0 without blocking (exit codes %s)", i+1, strings.Join(codes, ","))
			}
			lowered := strings.ToLower(output)
			denied := false
			for _, indicator := range denyIndicators {
				if strings.Contains(lowered, indicator) {
					denied = true
					break
				}
			}
			if !denied {
				return false, fmt.Sprintf("step %d exited %d without a denial marker (exit codes %s; stdout %q; stderr %q)",
					i+1, step.ExitCode, strings.Join(codes, ","), step.Stdout, step.Stderr)
			}
		}
		return true, fmt.Sprintf("all %d step(s) exited non-zero with a denial marker, no tool marker (exit codes %s)", len(steps), strings.Join(codes, ","))
	}
}

// markerIn reports whether the protected tool's marker appeared in output.
func markerIn(marker, output string) bool {
	return strings.Contains(output, marker)
}
