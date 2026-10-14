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
// carry boundary_exec — three governed-route probes that assert the Boundary
// wrapper allows, denies, and blocks `find -exec` forwarding (FUL-653) and
// records each decision, and three extra direct-agent probes. Every
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
				"execve is denied like every other ungoverned route. The command decision " +
				"record is snapshotted before and after: `boundary shell` is not `command " +
				"run` and must record no decision.",
			Expected: "non-zero exit, permission denied, no marker, decision record unchanged",
			Steps: [][]string{
				{"sh", "-c", "cat " + cliRecordPath + " 2>/dev/null || true"},
				{"sh", "-c", "echo " + cliToolPath + " | boundary shell --no-install"},
				{"sh", "-c", "echo " + cliToolPath + " | boundary shell"},
				{"sh", "-c", "cat " + cliRecordPath + " 2>/dev/null || true"},
			},
			Judge: cliShellViaShimJudge,
		},
		{
			Name: "governed-allow",
			Description: "An allowed routed command runs as a child of the setgid wrapper shim holding " +
				"boundary_exec, without reaching a shell or the protected tool: `ls` is a C0 (observe) " +
				"command, and the tools directory is 0750 root:boundary_exec, so its listing can only " +
				"succeed under the wrapper's group. (Before FUL-653 this probe used `find -exec <tool>`; " +
				"that shape now classifies by the forwarded command and is asserted blocked by " +
				"governed-find-exec-forwarding below.)",
			Expected: "exit 0, tool name in the listing, decision record action=allow executed=true",
			Steps: [][]string{
				{"boundary", "command", "run", "--", "ls", "/opt/fulcrum/tools"},
				{"tail", "-n", "1", cliRecordPath},
			},
			Judge: cliGovernedAllowListingJudge,
		},
		{
			Name: "list-tools-as-agent",
			Description: "The governed-allow shape attempted directly by the agent: listing the tools " +
				"directory must fail without the boundary_exec group, so the governed listing above " +
				"proves the wrapper conferred the group.",
			Expected: "non-zero exit, permission denied, no marker",
			Steps:    [][]string{{"ls", "/opt/fulcrum/tools"}},
			Judge:    blocked,
		},
		{
			Name: "governed-find-exec-forwarding",
			Description: "FUL-653: a routed `find -exec/-execdir/-ok` must be governed by the command it " +
				"forwards. Forwarding to the protected tool (unclassified, C7) or to a shell must not " +
				"execute; each attempt must record action deny or require_approval with executed=false.",
			Expected: "every run non-zero, no marker, verdict deny or require_approval, record executed=false",
			Steps: [][]string{
				{"boundary", "command", "run", "--",
					"find", "/opt/fulcrum/tools", "-name", cliToolName, "-exec", cliToolPath, ";"},
				{"tail", "-n", "1", cliRecordPath},
				{"boundary", "command", "run", "--",
					"find", "/tmp", "-maxdepth", "0", "-exec", "/bin/sh", "-c", cliToolPath, ";"},
				{"tail", "-n", "1", cliRecordPath},
				{"boundary", "command", "run", "--",
					"find", "/tmp", "-maxdepth", "0", "-execdir", "/bin/sh", "-c", cliToolPath, "+"},
				{"tail", "-n", "1", cliRecordPath},
			},
			Judge: cliGovernedBlockedPairsJudge,
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
			Description: "`find -exec <tool>` attempted directly by the agent (the route FUL-653 closed " +
				"on the governed path): it must fail without the boundary_exec group.",
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

// cliGovernedAllowListingJudge asserts the governed allow probe: exit 0, the
// protected tool's name in the wrapper child's listing of the group-only tools
// directory, an allow verdict on stderr, and a decision record with
// action=allow executed=true.
func cliGovernedAllowListingJudge(steps []ExecResult) (bool, string) {
	run, tail := steps[0], steps[1]
	if run.ExitCode != 0 {
		return false, fmt.Sprintf("governed run exited %d, want 0 (stderr: %s)", run.ExitCode, run.Stderr)
	}
	if !strings.Contains(run.Stdout, cliToolName) {
		return false, fmt.Sprintf("tool name missing from governed listing stdout: %q", run.Stdout)
	}
	if markerIn(cliMarker, run.Stdout+run.Stderr) {
		return false, "governed listing executed the protected tool (marker appeared)"
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
	return true, fmt.Sprintf("exit 0, tool listed, no marker, decision record action=%q executed=%t", record.Action, record.Executed)
}

// cliGovernedBlockedPairsJudge asserts (run, record-tail) step pairs: every
// governed run exits non-zero with no marker, reports deny or require_approval
// with executed=false on stderr, and appends a matching decision record.
func cliGovernedBlockedPairsJudge(steps []ExecResult) (bool, string) {
	if len(steps) == 0 || len(steps)%2 != 0 {
		return false, fmt.Sprintf("want (run, tail) step pairs, got %d step(s)", len(steps))
	}
	var verdicts []string
	for i := 0; i < len(steps); i += 2 {
		run, tail := steps[i], steps[i+1]
		if run.ExitCode == 0 {
			return false, fmt.Sprintf("governed run %d exited 0", i/2+1)
		}
		if markerIn(cliMarker, run.Stdout+run.Stderr) {
			return false, fmt.Sprintf("governed run %d executed the protected tool (marker appeared)", i/2+1)
		}
		blockingVerdict := strings.Contains(run.Stderr, "action=deny") || strings.Contains(run.Stderr, "action=require_approval")
		if !blockingVerdict || !strings.Contains(run.Stderr, "executed=false") {
			return false, fmt.Sprintf("governed run %d: blocking verdict missing from boundary stderr: %s", i/2+1, run.Stderr)
		}
		record, ok := parseRecordTail(tail)
		if !ok {
			return false, fmt.Sprintf("decision record tail %d unreadable (exit %d): %s", i/2+1, tail.ExitCode, tail.Stdout+tail.Stderr)
		}
		if (record.Action != "deny" && record.Action != "require_approval") || record.Executed {
			return false, fmt.Sprintf("decision record %d action=%q executed=%t, want deny|require_approval/false", i/2+1, record.Action, record.Executed)
		}
		verdicts = append(verdicts, record.Action)
	}
	return true, fmt.Sprintf("%d governed runs blocked, no marker, records %s executed=false", len(verdicts), strings.Join(verdicts, ","))
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

// cliShellViaShimJudge asserts the shell-via-shim probe: the first and last
// steps snapshot the command decision record, which must be byte-identical
// across the `boundary shell` invocations — a subshell is not `command run`
// and records no command decision — and every step between the snapshots
// must satisfy the blocked judge (non-zero exit, denial indicator, no tool
// marker).
func cliShellViaShimJudge(steps []ExecResult) (bool, string) {
	if len(steps) < 3 {
		return false, fmt.Sprintf("shell-via-shim needs record snapshots around shell steps, got %d step(s)", len(steps))
	}
	before, shellSteps, after := steps[0], steps[1:len(steps)-1], steps[len(steps)-1]
	if before.ExitCode != 0 || after.ExitCode != 0 {
		return false, fmt.Sprintf("decision record snapshot failed (before exit %d, after exit %d)", before.ExitCode, after.ExitCode)
	}
	pass, observed := BlockedJudge(cliMarker)(shellSteps)
	if !pass {
		return false, observed
	}
	if before.Stdout != after.Stdout {
		return false, fmt.Sprintf("command decision record changed across `boundary shell` steps (before %d bytes, after %d bytes)", len(before.Stdout), len(after.Stdout))
	}
	return true, observed + "; decision record unchanged"
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
