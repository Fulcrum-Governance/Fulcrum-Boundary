package redteam

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func WriteJSON(w io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(body, '\n'))
	return err
}

func WriteText(w io.Writer, result *RunResult) error {
	if result == nil {
		return fmt.Errorf("redteam result is nil")
	}
	if _, err := fmt.Fprintf(w, "redteam mode: %s\n", result.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "pack: %s\n", result.PackID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "live mutation: %s\n", boolNone(result.MutatesLiveSystems)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "real secrets: %s\n", boolNone(result.RealSecretsUsed)); err != nil {
		return err
	}
	for _, scenario := range result.Results {
		if _, err := fmt.Fprintf(w, "scenario: %s\n", scenario.ScenarioID); err != nil {
			return err
		}
		if scenario.Command != "" {
			if _, err := fmt.Fprintf(w, "attack: %s\n", scenario.PackID); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "command: %s\n", scenario.Command); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "class: %s\n", scenario.CommandClass); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "risk: %s\n", scenario.CommandRisk); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "executed: %t\n", scenario.Executed); err != nil {
				return err
			}
		}
		if scenario.Patch != "" {
			if _, err := fmt.Fprintf(w, "attack: %s\n", scenario.PackID); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "patch: %s\n", scenario.Patch); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "class: %s\n", scenario.EditClass); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "risk: %s\n", scenario.EditRisk); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "applied: %t\n", scenario.Applied); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "expected: %s\n", strings.ToUpper(scenario.ExpectedAction)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "actual: %s\n", strings.ToUpper(scenario.ActualAction)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "result: %s\n", passFail(scenario.Passed)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "reason: %s\n", scenario.Reason); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "matched rule: %s\n", scenario.MatchedRule); err != nil {
			return err
		}
		record := scenario.DecisionRecord
		if _, err := fmt.Fprintf(w, "decision record: %s\n", record.RecordID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "decision hash: %s\n", record.DecisionHash); err != nil {
			return err
		}
	}
	return nil
}

func WritePackList(w io.Writer, summaries []PackSummary) error {
	for _, summary := range summaries {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", summary.ID, summary.Status, summary.Description); err != nil {
			return err
		}
	}
	return nil
}

func boolNone(value bool) string {
	if value {
		return "yes"
	}
	return "none"
}

func passFail(value bool) string {
	if value {
		return "pass"
	}
	return "fail"
}
