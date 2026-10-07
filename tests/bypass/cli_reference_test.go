//go:build bypass

package bypass

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// cli-reference-v1 topology constants. The governed tool lives at an absolute
// path that is not on PATH, executable only by group boundary_exec; the sole
// `boundary` on the agent's PATH is a setgid boundary_exec shim that execs the
// real CLI, raising the group only when argv is `command run` — every other
// subcommand, including `boundary shell`, is exec'd with the caller's real
// gid. The agent container drops all capabilities so docker exec entry-point
// checks cannot be bypassed by a bounding-set DAC_OVERRIDE. See
// docs/deployment/cli-bypass-proofing.md.
const (
	cliTopologyID = "cli-reference-v1"
	cliToolPath   = "/opt/fulcrum/tools/fulcrum-demo-tool"
	cliToolName   = "fulcrum-demo-tool"
	// cliMarker must match the marker printed by cli-reference/demotool/main.go.
	cliMarker = "FULCRUM-DEMO-TOOL-MARKER"
	// cliRecordPath is where `boundary command run` appends command decision
	// records by default, resolved against the agent working directory
	// (/home/agent).
	cliRecordPath = "/home/agent/.boundary/command/decision-records.jsonl"
)

// TestCLIReferenceTopology attacks the cli-reference-v1 reference topology:
// eight adversarial probes that must all fail to execute the protected tool —
// including `boundary shell` launched through the setgid shim, which must not
// carry boundary_exec — two governed-route probes that assert the Boundary
// wrapper works and records its decision, and two extra probes. Every
// expectation is asserted on real exit codes, real output, and the decision
// record file.
func TestCLIReferenceTopology(t *testing.T) {
	RequireDocker(t)

	h := Harness{Dir: "cli-reference", Service: "agent", User: "agent"}
	h.Up(t, "BOUNDARY_COMMIT="+gitCommit(t))
	defer h.Down(t)

	evidence := Evidence{
		TopologyID:      cliTopologyID,
		BoundaryVersion: h.BoundaryVersion(t),
		ImageDigests:    h.ImageDigests(t),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	blocked := BlockedJudge(cliMarker)
	probes := []Probe{
		{
			Name:        "direct-absolute-path",
			Description: "Invoke the protected binary directly by absolute path.",
			Expected:    "non-zero exit, permission denied, no marker",
			Steps:       [][]string{{cliToolPath}},
			Judge:       blocked,
		},
		{
			Name:        "path-lookup",
			Description: "Invoke the tool by name relying on PATH resolution.",
			Expected:    "non-zero exit, not found, no marker",
			Steps:       [][]string{{cliToolName}},
			Judge:       blocked,
		},
		{
			Name:        "sh-c-absolute-path",
			Description: "Invoke the protected binary through /bin/sh -c.",
			Expected:    "non-zero exit, permission denied, no marker",
			Steps:       [][]string{{"/bin/sh", "-c", cliToolPath}},
			Judge:       blocked,
		},
		{
			Name:        "interpreter-one-liner",
			Description: "Invoke the protected binary through a busybox awk system() one-liner.",
			Expected:    "non-zero exit, permission denied, no marker",
			Steps:       [][]string{{"awk", fmt.Sprintf("BEGIN{rc=system(%q); exit (rc==0?0:1)}", cliToolPath)}},
			Judge:       blocked,
		},
		{
			Name:        "env-absolute-path",
			Description: "Invoke the protected binary through env.",
			Expected:    "non-zero exit, permission denied, no marker",
			Steps:       [][]string{{"env", cliToolPath}},
			Judge:       blocked,
		},
		{
			Name:        "copy-or-symlink",
			Description: "Copy the binary into a writable directory, then symlink and run it.",
			Expected:    "copy fails (unreadable) and symlinked run is denied; no marker",
			Steps: [][]string{
				{"cp", cliToolPath, "/tmp/fulcrum-demo-tool-copy"},
				{"/bin/sh", "-c", "ln -sf " + cliToolPath + " /tmp/fulcrum-demo-tool-link && /tmp/fulcrum-demo-tool-link"},
			},
			Judge: blocked,
		},
		{
			Name:        "su-or-sudo-escalation",
			Description: "Switch user to root via sudo or su to reach the protected binary.",
			Expected:    "sudo unavailable and su denied; no marker",
			Steps: [][]string{
				{"sudo", "-n", cliToolPath},
				{"/bin/sh", "-c", "su root -c " + cliToolPath + " </dev/null"},
			},
			Judge: blocked,
		},
		{
			Name: "shell-via-shim",
			Description: "Hand the protected tool's absolute path to a `boundary shell` subshell " +
				"launched through the setgid shim, with and without --no-install. The shim " +
				"must raise boundary_exec only for `command run`; unelevated, the subshell's " +
				"execve is denied like every other ungoverned route and no command decision " +
				"is recorded.",
			Expected: "non-zero exit, permission denied, no marker",
			Steps: [][]string{
				{"sh", "-c", "echo " + cliToolPath + " | boundary shell --no-install"},
				{"sh", "-c", "echo " + cliToolPath + " | boundary shell"},
			},
			Judge: blocked,
		},
		{
			Name: "governed-allow",
			Description: "An allowed routed command reaches the protected binary through the setgid " +
				"wrapper shim: the preview command policy only executes allowlisted argv[0] names, and " +
				"`find` is the C0 (observe) command that hands a path to execve, so the governed " +
				"allow route is `boundary command run -- find <tools dir> -name <tool> -exec <tool> ;`. " +
				"The marker can only appear if the tool executed under the wrapper's boundary_exec group.",
			Expected: "exit 0, marker on stdout, decision record action=allow executed=true",
			Steps: [][]string{
				{"boundary", "command", "run", "--",
					"find", "/opt/fulcrum/tools", "-name", cliToolName, "-exec", cliToolPath, ";"},
				{"tail", "-n", "1", cliRecordPath},
			},
			Judge: cliGovernedAllowJudge,
		},
		{
			Name: "governed-deny",
			Description: "A denied routed command naming the protected binary is blocked before the " +
				"tool runs: the --token= argument classifies the request C6 (credential access) " +
				"and the preview policy denies it.",
			Expected: "non-zero exit, no marker, decision record action=deny executed=false",
			Steps: [][]string{
				{"boundary", "command", "run", "--",
					cliToolPath, "--token=bypass-probe-sentinel"},
				{"tail", "-n", "1", cliRecordPath},
			},
			Judge: cliGovernedDenyJudge,
		},
		{
			Name: "find-exec-as-agent",
			Description: "The governed-allow shape attempted directly by the agent: find -exec must " +
				"fail without the boundary_exec group, closing the same path the wrapper opens.",
			Expected: "non-zero exit, permission denied, no marker",
			Steps:    [][]string{{"find", "/opt/fulcrum/tools", "-name", cliToolName, "-exec", cliToolPath, ";"}},
			Judge:    blocked,
		},
		{
			Name:        "exec-wrapper-applet",
			Description: "Invoke the protected binary through a second exec-handing applet (nohup).",
			Expected:    "non-zero exit, permission denied, no marker",
			Steps:       [][]string{{"nohup", cliToolPath}},
			Judge:       blocked,
		},
	}

	failed := 0
	for _, p := range probes {
		report := RunProbe(t, h, p)
		evidence.Probes = append(evidence.Probes, report)
		if !report.Passed {
			failed++
			t.Errorf("probe %s FAILED: expected %s; observed %s", report.Name, report.Expected, report.Observed)
			for i, step := range report.Steps {
				t.Errorf("  step %d argv=%q exit=%d\n  stdout: %s\n  stderr: %s",
					i+1, strings.Join(step.Argv, " "), step.ExitCode, step.Stdout, step.Stderr)
			}
		}
	}

	jsonPath, mdPath := WriteEvidence(t, "evidence", evidence)
	t.Logf("wrote bypass evidence: %s and %s", jsonPath, mdPath)
	if failed > 0 {
		t.Fatalf("%d of %d probes failed", failed, len(probes))
	}
}

// cliGovernedAllowJudge asserts the governed allow probe: exit 0, the
// protected tool's marker on stdout, an allow verdict on stderr, and a
// decision record with action=allow executed=true.
func cliGovernedAllowJudge(steps []ExecResult) (bool, string) {
	run, tail := steps[0], steps[1]
	if run.ExitCode != 0 {
		return false, fmt.Sprintf("governed run exited %d, want 0 (stderr: %s)", run.ExitCode, run.Stderr)
	}
	if !markerIn(cliMarker, run.Stdout) {
		return false, "protected tool marker missing from governed run stdout"
	}
	if !strings.Contains(run.Stderr, "action=allow") {
		return false, fmt.Sprintf("allow verdict missing from boundary stderr: %s", run.Stderr)
	}
	record, ok := parseRecordTail(tail)
	if !ok {
		return false, fmt.Sprintf("decision record tail unreadable (exit %d): %s", tail.ExitCode, tail.Stdout+tail.Stderr)
	}
	if record.Action != "allow" || !record.Executed {
		return false, fmt.Sprintf("decision record action=%q executed=%t, want allow/true", record.Action, record.Executed)
	}
	return true, fmt.Sprintf("exit 0, marker observed, decision record action=%q executed=%t", record.Action, record.Executed)
}

// cliGovernedDenyJudge asserts the governed deny probe: non-zero exit, no
// marker, a deny verdict on stderr, and a decision record with action=deny
// executed=false.
func cliGovernedDenyJudge(steps []ExecResult) (bool, string) {
	run, tail := steps[0], steps[1]
	if run.ExitCode == 0 {
		return false, "governed denied run exited 0"
	}
	if markerIn(cliMarker, run.Stdout) {
		return false, "denied command executed the protected tool (marker appeared)"
	}
	if !strings.Contains(run.Stderr, "action=deny") {
		return false, fmt.Sprintf("deny verdict missing from boundary stderr: %s", run.Stderr)
	}
	record, ok := parseRecordTail(tail)
	if !ok {
		return false, fmt.Sprintf("decision record tail unreadable (exit %d): %s", tail.ExitCode, tail.Stdout+tail.Stderr)
	}
	if record.Action != "deny" || record.Executed {
		return false, fmt.Sprintf("decision record action=%q executed=%t, want deny/false", record.Action, record.Executed)
	}
	return true, fmt.Sprintf("exit %d, no marker, decision record action=%q executed=%t", run.ExitCode, record.Action, record.Executed)
}

// commandRecordTail is the subset of boundary.command_decision.v1 fields the
// governed probes assert on.
type commandRecordTail struct {
	Action   string `json:"action"`
	Executed bool   `json:"executed"`
	Command  string `json:"command"`
}

func parseRecordTail(tail ExecResult) (commandRecordTail, bool) {
	var record commandRecordTail
	if tail.ExitCode != 0 {
		return record, false
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(tail.Stdout)), &record); err != nil {
		return record, false
	}
	return record, true
}

// gitCommit returns the worktree HEAD for the image build stamp, or "unknown"
// when git cannot report it.
func gitCommit(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Logf("git rev-parse HEAD failed (%v); BOUNDARY_COMMIT will be unknown", err)
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
