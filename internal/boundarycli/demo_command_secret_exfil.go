package boundarycli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	boundarydemo "github.com/fulcrum-governance/fulcrum-boundary/internal/demo"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

// commandSecretExfilDemoScenario is the headline scenario surfaced by
// `boundary demo command-secret-exfil`: an untrusted task proposes posting a
// secret-looking environment file to an external sink. Command Boundary
// classifies it as C6 (credential/secret access) and denies it before any
// execution. The scenario is fixture-only: no real .env is read, no network
// call is made, and nothing is executed.
const commandSecretExfilDemoScenario = "command-curl-env-exfil" // #nosec G101 -- scenario identifier slug for the fixture demo, not a credential; no real secret value is embedded (gosec matches the word "secret" in the constant name).

func runCommandSecretExfilDemo(args []string, stdout, stderr io.Writer) int {
	fs := newHelpFlagSet("boundary demo command-secret-exfil", stderr, commandHelp{
		Purpose: "Run a fixture-only Command Boundary denial demo for secret exfiltration.",
		Usage:   "boundary demo command-secret-exfil [--json] [--out PATH]",
		Common: []string{
			"boundary demo command-secret-exfil",
			"boundary demo command-secret-exfil --json",
			"boundary demo command-secret-exfil --out demo.txt",
		},
		Notes: []string{
			"Fixture mode reads no real .env, makes no network call, and executes nothing.",
			"The demo proves pre-execution denial for the routed command path, not control of unrouted shells (see the routed-only doctrine).",
			"--out retains the decision record at <dir>/command-secret-exfil-artifacts/decision-records.jsonl for boundary verify-record.",
		},
	})
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	outPath := fs.String("out", "", "write the demo report to a file and retain its decision record")
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

	var report bytes.Buffer
	if *jsonOutput {
		if err := writeCommandSecretExfilDemoJSON(&report, scenario); err != nil {
			fmt.Fprintf(stderr, "command-secret-exfil demo: %v\n", err)
			return 1
		}
	} else {
		writeCommandSecretExfilDemoText(&report, scenario)
	}

	if *outPath == "" {
		if _, err := io.Copy(stdout, &report); err != nil {
			fmt.Fprintf(stderr, "command-secret-exfil demo: %v\n", err)
			return 1
		}
	} else {
		recordPath, err := writeCommandSecretExfilArtifacts(*outPath, report.Bytes(), scenario)
		if err != nil {
			fmt.Fprintf(stderr, "command-secret-exfil demo: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "demo report: %s\n", *outPath)
		printRecordID(stdout, scenario.DecisionRecord.RecordID)
		printRecordPath(stdout, recordPath)
	}

	if !scenario.Passed {
		return 1
	}
	return 0
}

// writeCommandSecretExfilArtifacts writes the demo report to outPath and lands
// the scenario's decision record as JSONL in the demo's predictable artifact
// directory, returning the record path. It mirrors the github-lethal-trifecta
// demo's --out layout so both proof lanes expose the same find -> verify step.
func writeCommandSecretExfilArtifacts(outPath string, report []byte, scenario redteam.ScenarioResult) (string, error) {
	if err := writeDemoReportFile(outPath, report); err != nil {
		return "", err
	}
	dir, err := boundarydemo.ArtifactDir(outPath, "command-secret-exfil")
	if err != nil {
		return "", err
	}
	recordPath := filepath.Join(dir, boundarydemo.DefaultDecisionRecordFilename)
	if err := boundarydemo.WriteDecisionRecordsJSONL(recordPath, []governance.DecisionRecordV1{scenario.DecisionRecord}); err != nil {
		return "", err
	}
	return recordPath, nil
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
	fmt.Fprintf(w, "decision record id: %s\n", sr.DecisionRecord.RecordID)
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
