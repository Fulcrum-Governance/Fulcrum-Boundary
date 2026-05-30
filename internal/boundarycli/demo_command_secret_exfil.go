package boundarycli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

// commandSecretExfilDemoScenario is the headline scenario surfaced by
// `boundary demo command-secret-exfil`: an untrusted task proposes posting a
// secret-looking environment file to an external sink. Command Boundary
// classifies it as C6 (credential/secret access) and denies it before any
// execution. The scenario is fixture-only: no real .env is read, no network
// call is made, and nothing is executed.
const commandSecretExfilDemoScenario = "command-curl-env-exfil"

func runCommandSecretExfilDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo command-secret-exfil", stderr, commandHelp{
		Purpose: "Run a fixture-only Command Boundary denial demo for secret exfiltration.",
		Usage:   "boundary demo command-secret-exfil [--json]",
		Common: []string{
			"boundary demo command-secret-exfil",
			"boundary demo command-secret-exfil --json",
		},
		Notes: []string{
			"Fixture mode reads no real .env, makes no network call, and executes nothing.",
			"The demo proves pre-execution denial for the routed command path, not control of unrouted shells (see the routed-only doctrine).",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	result, err := redteam.Run(context.Background(), redteam.RunOptions{
		PackID: "command-secret-exfil",
		Mode:   redteam.ModeFixture,
	})
	if err != nil {
		fmt.Fprintf(stderr, "command-secret-exfil demo: %v\n", err)
		return 1
	}

	scenario, ok := findScenarioResult(result, commandSecretExfilDemoScenario)
	if !ok {
		fmt.Fprintf(stderr, "command-secret-exfil demo: scenario %q not found in pack\n", commandSecretExfilDemoScenario)
		return 1
	}

	if *jsonOutput {
		if err := writeCommandSecretExfilDemoJSON(stdout, scenario); err != nil {
			fmt.Fprintf(stderr, "command-secret-exfil demo: %v\n", err)
			return 1
		}
	} else {
		writeCommandSecretExfilDemoText(stdout, scenario)
	}

	if !scenario.Passed {
		return 1
	}
	return 0
}

func findScenarioResult(result *redteam.RunResult, scenarioID string) (redteam.ScenarioResult, bool) {
	if result == nil {
		return redteam.ScenarioResult{}, false
	}
	for _, sr := range result.Results {
		if sr.ScenarioID == scenarioID {
			return sr, true
		}
	}
	return redteam.ScenarioResult{}, false
}

func writeCommandSecretExfilDemoText(w io.Writer, sr redteam.ScenarioResult) {
	fmt.Fprintln(w, "Command Boundary demo: secret exfiltration (fixture-only)")
	fmt.Fprintln(w, "fixture-only: true   credentials: none   network: none   live mutation: none")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "source: untrusted_task")
	fmt.Fprintf(w, "proposed command: %s\n", sr.Command)
	fmt.Fprintln(w, "risk signals: secret-access (env-file path) + network-egress (curl) -> secret-access dominates")
	fmt.Fprintf(w, "class: %s   risk: %s\n", sr.CommandClass, sr.CommandRisk)
	fmt.Fprintf(w, "expected: %s   actual: %s   result: %s\n", upperVerdict(sr.ExpectedAction), upperVerdict(sr.ActualAction), passLabel(sr.Passed))
	fmt.Fprintf(w, "reason: %s   matched rule: %s\n", sr.Reason, sr.MatchedRule)
	fmt.Fprintf(w, "executed: %t\n", sr.Executed)
	fmt.Fprintf(w, "decision record: %s\n", sr.DecisionRecord.RecordID)
	fmt.Fprintf(w, "decision hash: %s\n", sr.DecisionRecord.DecisionHash)
	fmt.Fprintf(w, "decision mode: %s\n", sr.DecisionRecord.DecisionMode)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "The secret-looking argument is redacted in all output. The command is")
	fmt.Fprintln(w, "denied before execution; it never runs. Command Boundary governs only")
	fmt.Fprintln(w, "commands routed through Boundary; direct shell, CI, and SSH are bypasses.")
}

func writeCommandSecretExfilDemoJSON(w io.Writer, sr redteam.ScenarioResult) error {
	payload := map[string]any{
		"schema_version":   "boundary.demo.command_secret_exfil.v1",
		"demo":             "command-secret-exfil",
		"source":           "untrusted_task",
		"fixture_only":     sr.FixtureOnly,
		"no_live_mutation": sr.NoLiveMutation,
		"scenario_id":      sr.ScenarioID,
		"proposed_command": sr.Command,
		"class":            sr.CommandClass,
		"risk":             sr.CommandRisk,
		"expected_action":  sr.ExpectedAction,
		"actual_action":    sr.ActualAction,
		"executed":         sr.Executed,
		"passed":           sr.Passed,
		"reason":           sr.Reason,
		"matched_rule":     sr.MatchedRule,
		"decision_record":  sr.DecisionRecord,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func upperVerdict(action string) string {
	switch action {
	case "deny":
		return "DENY"
	case "allow":
		return "ALLOW"
	case "warn":
		return "WARN"
	case "escalate":
		return "ESCALATE"
	case "require_approval":
		return "REQUIRE_APPROVAL"
	default:
		return action
	}
}

func passLabel(passed bool) string {
	if passed {
		return "pass"
	}
	return "fail"
}
