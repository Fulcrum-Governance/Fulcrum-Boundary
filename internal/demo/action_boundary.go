package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/redteam"
)

const ActionBoundarySchemaVersion = "boundary.demo.action_boundary.v1"

type ActionBoundaryOptions struct {
	Now time.Time
}

type ActionBoundaryResult struct {
	SchemaVersion       string                  `json:"schema_version"`
	Status              string                  `json:"status"`
	Passed              bool                    `json:"passed"`
	FixtureOnly         bool                    `json:"fixture_only"`
	RequiresCredentials bool                    `json:"requires_credentials"`
	RequiresNetwork     bool                    `json:"requires_network"`
	MutatesLiveSystems  bool                    `json:"mutates_live_systems"`
	Surfaces            []ActionBoundarySurface `json:"surfaces"`
	Proof               []string                `json:"proof"`
	Limitations         []string                `json:"limitations"`
}

type ActionBoundarySurface struct {
	Surface            string `json:"surface"`
	Label              string `json:"label"`
	Scenario           string `json:"scenario"`
	ExpectedAction     string `json:"expected_action"`
	ActualAction       string `json:"actual_action"`
	Reason             string `json:"reason"`
	Command            string `json:"command,omitempty"`
	Class              string `json:"class,omitempty"`
	Risk               string `json:"risk,omitempty"`
	RecommendedAction  string `json:"recommended_action,omitempty"`
	UpstreamCalled     bool   `json:"upstream_called"`
	ReadUpstreamCalled bool   `json:"read_upstream_called"`
	Executed           bool   `json:"executed"`
	Applied            bool   `json:"applied"`
	Passed             bool   `json:"passed"`
}

func RunActionBoundary(ctx context.Context, opts ActionBoundaryOptions) (*ActionBoundaryResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	githubResult, err := RunGitHubLethalTrifecta(ctx, GitHubLethalTrifectaOptions{Now: now})
	if err != nil {
		return nil, err
	}
	commandResult, err := redteam.Run(ctx, redteam.RunOptions{PackID: "command-repo-mutation", Mode: redteam.ModeFixture})
	if err != nil {
		return nil, err
	}
	commandScenario, ok := findScenario(commandResult, "command-git-push")
	if !ok {
		return nil, fmt.Errorf("command redteam fixture missing command-git-push scenario")
	}
	editResult, err := redteam.Run(ctx, redteam.RunOptions{PackID: "edit-secret-exfil", Mode: redteam.ModeFixture})
	if err != nil {
		return nil, err
	}
	editScenario, ok := findScenario(editResult, "edit-env-secret")
	if !ok {
		return nil, fmt.Errorf("edit redteam fixture missing edit-env-secret scenario")
	}

	surfaces := []ActionBoundarySurface{
		{
			Surface:            "mcp_secure_github",
			Label:              "MCP / Secure GitHub",
			Scenario:           githubResult.Scenario.ID,
			ExpectedAction:     githubResult.Scenario.ExpectedAction,
			ActualAction:       githubResult.Scenario.ActualAction,
			Reason:             githubResult.Scenario.Reason,
			UpstreamCalled:     githubResult.Scenario.UpstreamCalled,
			ReadUpstreamCalled: githubResult.Scenario.ReadUpstreamCalled,
			Passed:             githubResult.Passed,
		},
		{
			Surface:           "command_boundary",
			Label:             "Command Boundary",
			Scenario:          commandScenario.ScenarioID,
			ExpectedAction:    commandScenario.ExpectedAction,
			ActualAction:      commandScenario.ActualAction,
			Reason:            commandScenario.Reason,
			Command:           commandScenario.Command,
			Class:             commandScenario.CommandClass,
			Risk:              commandScenario.CommandRisk,
			RecommendedAction: commandScenario.ActualAction,
			Executed:          commandScenario.Executed,
			Passed:            commandScenario.Passed,
		},
		{
			Surface:        "edit_boundary",
			Label:          "Edit Boundary",
			Scenario:       editScenario.ScenarioID,
			ExpectedAction: editScenario.ExpectedAction,
			ActualAction:   editScenario.ActualAction,
			Reason:         editScenario.Reason,
			Class:          editScenario.EditClass,
			Risk:           editScenario.EditRisk,
			Applied:        editScenario.Applied,
			Passed:         editScenario.Passed,
		},
	}
	passed := githubResult.Passed && commandResult.Passed && editResult.Passed
	status := "pass"
	if !passed {
		status = "fail"
	}
	return &ActionBoundaryResult{
		SchemaVersion:       ActionBoundarySchemaVersion,
		Status:              status,
		Passed:              passed,
		FixtureOnly:         true,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		Surfaces:            surfaces,
		Proof: []string{
			"MCP / Secure GitHub denies the fixture private-repo write before upstream execution.",
			"Command Boundary classifies the fixture git push as a repository mutation and does not execute it.",
			"Edit Boundary classifies the fixture .env patch as secret-bearing and does not apply it.",
		},
		Limitations: []string{
			"Fixture mode does not prove live GitHub App conformance.",
			"Command Boundary governs only commands routed through Boundary.",
			"Edit Boundary governs only proposed edit envelopes routed through Boundary.",
			"Direct shell, direct upstream MCP access, and direct file edits remain deployment bypasses unless operators remove those paths.",
		},
	}, nil
}

func WriteActionBoundaryJSON(w io.Writer, result *ActionBoundaryResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteActionBoundaryText(w io.Writer, result *ActionBoundaryResult) error {
	// Plain-output entry point (no colorizer) — preserves the exact bytes the
	// evidence bundle embeds. The demo CLI uses WriteActionBoundaryTextColor.
	return WriteActionBoundaryTextColor(w, result, nil)
}

// WriteActionBoundaryTextColor renders the cross-surface report, styling
// verdicts and pass/fail tokens through c (nil renders plain).
func WriteActionBoundaryTextColor(w io.Writer, result *ActionBoundaryResult, c *Colorizer) error {
	if result == nil {
		return fmt.Errorf("action boundary result is required")
	}
	fmt.Fprintln(w, c.Bold("Action Boundary demo"))
	fmt.Fprintf(w, "status: %s\n", result.Status)
	fmt.Fprintf(w, "fixture-only: %t\n", result.FixtureOnly)
	fmt.Fprintln(w, "credentials: none")
	fmt.Fprintln(w, "network: none")
	fmt.Fprintln(w, "live mutation: none")
	for _, surface := range result.Surfaces {
		fmt.Fprintf(w, "\nSurface: %s\n", c.Bold(surface.Label))
		fmt.Fprintf(w, "scenario: %s\n", surface.Scenario)
		if surface.Command != "" {
			fmt.Fprintf(w, "command: %s\n", surface.Command)
		}
		if surface.Class != "" {
			fmt.Fprintf(w, "class: %s\n", surface.Class)
		}
		if surface.Risk != "" {
			fmt.Fprintf(w, "risk: %s\n", surface.Risk)
		}
		if surface.RecommendedAction != "" {
			fmt.Fprintf(w, "recommended action: %s\n", surface.RecommendedAction)
		}
		fmt.Fprintf(w, "expected action: %s\n", displayAction(surface.ExpectedAction))
		fmt.Fprintf(w, "actual action: %s\n", c.Verdict(displayAction(surface.ActualAction)))
		fmt.Fprintf(w, "reason: %s\n", surface.Reason)
		if surface.Surface == "mcp_secure_github" {
			fmt.Fprintf(w, "upstream_called=%t\n", surface.UpstreamCalled)
			fmt.Fprintf(w, "read_upstream_called=%t\n", surface.ReadUpstreamCalled)
		}
		if surface.Surface == "command_boundary" {
			fmt.Fprintf(w, "executed=%t\n", surface.Executed)
		}
		if surface.Surface == "edit_boundary" {
			fmt.Fprintf(w, "applied=%t\n", surface.Applied)
		}
		fmt.Fprintf(w, "result: %s\n", colorPassFail(c, surface.Passed))
	}
	fmt.Fprintln(w, "\n"+c.Bold("What this proves:"))
	for _, proof := range result.Proof {
		fmt.Fprintf(w, "- %s\n", proof)
	}
	fmt.Fprintln(w, "\n"+c.Bold("What this does not prove:"))
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "- %s\n", limitation)
	}
	return nil
}

// colorPassFail styles a pass/fail token: green for pass, red for fail. A nil
// colorizer returns the plain "pass"/"fail" string.
func colorPassFail(c *Colorizer, passed bool) string {
	if passed {
		return c.Pass(passFail(passed))
	}
	return c.Fail(passFail(passed))
}

func WriteActionBoundaryMarkdown(w io.Writer, result *ActionBoundaryResult) error {
	if result == nil {
		return fmt.Errorf("action boundary result is required")
	}
	fmt.Fprintln(w, "# Action Boundary Demo")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "- Status: `%s`\n", result.Status)
	fmt.Fprintf(w, "- Fixture only: `%t`\n", result.FixtureOnly)
	fmt.Fprintln(w, "- Credentials: `none`")
	fmt.Fprintln(w, "- Network: `none`")
	fmt.Fprintln(w, "- Live mutation: `none`")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Surfaces")
	fmt.Fprintln(w)
	for _, surface := range result.Surfaces {
		fmt.Fprintf(w, "### %s\n\n", surface.Label)
		fmt.Fprintf(w, "- Scenario: `%s`\n", surface.Scenario)
		if surface.Command != "" {
			fmt.Fprintf(w, "- Command: `%s`\n", surface.Command)
		}
		if surface.Class != "" {
			fmt.Fprintf(w, "- Class: `%s`\n", surface.Class)
		}
		if surface.Risk != "" {
			fmt.Fprintf(w, "- Risk: `%s`\n", surface.Risk)
		}
		if surface.RecommendedAction != "" {
			fmt.Fprintf(w, "- Recommended action: `%s`\n", surface.RecommendedAction)
		}
		fmt.Fprintf(w, "- Expected action: `%s`\n", displayAction(surface.ExpectedAction))
		fmt.Fprintf(w, "- Actual action: `%s`\n", displayAction(surface.ActualAction))
		fmt.Fprintf(w, "- Reason: `%s`\n", surface.Reason)
		if surface.Surface == "mcp_secure_github" {
			fmt.Fprintf(w, "- Upstream called: `%t`\n", surface.UpstreamCalled)
		}
		if surface.Surface == "command_boundary" {
			fmt.Fprintf(w, "- Executed: `%t`\n", surface.Executed)
		}
		if surface.Surface == "edit_boundary" {
			fmt.Fprintf(w, "- Applied: `%t`\n", surface.Applied)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "## What This Proves")
	fmt.Fprintln(w)
	for _, proof := range result.Proof {
		fmt.Fprintf(w, "- %s\n", proof)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "## What This Does Not Prove")
	fmt.Fprintln(w)
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "- %s\n", limitation)
	}
	return nil
}

func WriteActionBoundaryDashboard(w io.Writer, result *ActionBoundaryResult) error {
	if result == nil {
		return fmt.Errorf("action boundary result is required")
	}
	fmt.Fprintln(w, "<!doctype html>")
	fmt.Fprintln(w, `<html lang="en">`)
	fmt.Fprintln(w, "<head>")
	fmt.Fprintln(w, `<meta charset="utf-8">`)
	fmt.Fprintln(w, `<meta name="viewport" content="width=device-width, initial-scale=1">`)
	fmt.Fprintln(w, "<title>Boundary Action Boundary Demo</title>")
	fmt.Fprintln(w, `<style>body{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;margin:2rem;line-height:1.45;color:#111827;background:#f9fafb}main{max-width:980px;margin:auto}.surface{border:1px solid #d1d5db;border-radius:8px;padding:1rem;margin:1rem 0;background:#fff}.status{font-weight:700}.pass{color:#047857}.fail{color:#b91c1c}.mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}</style>`)
	fmt.Fprintln(w, "</head>")
	fmt.Fprintln(w, "<body><main>")
	fmt.Fprintln(w, "<h1>Boundary Action Boundary Demo</h1>")
	fmt.Fprintf(w, `<p class="status %s">status: %s</p>`+"\n", passFail(result.Passed), html.EscapeString(result.Status))
	fmt.Fprintln(w, "<p>No credentials. No network. No live mutation.</p>")
	for _, surface := range result.Surfaces {
		fmt.Fprintln(w, `<section class="surface">`)
		fmt.Fprintf(w, "<h2>%s</h2>\n", html.EscapeString(surface.Label))
		fmt.Fprintf(w, `<p class="mono">scenario: %s</p>`+"\n", html.EscapeString(surface.Scenario))
		if surface.Command != "" {
			fmt.Fprintf(w, `<p class="mono">command: %s</p>`+"\n", html.EscapeString(surface.Command))
		}
		if surface.Class != "" {
			fmt.Fprintf(w, `<p>class: <span class="mono">%s</span></p>`+"\n", html.EscapeString(surface.Class))
		}
		if surface.Risk != "" {
			fmt.Fprintf(w, `<p>risk: <span class="mono">%s</span></p>`+"\n", html.EscapeString(surface.Risk))
		}
		fmt.Fprintf(w, `<p>actual action: <span class="mono">%s</span></p>`+"\n", html.EscapeString(displayAction(surface.ActualAction)))
		fmt.Fprintf(w, `<p>reason: <span class="mono">%s</span></p>`+"\n", html.EscapeString(surface.Reason))
		fmt.Fprintln(w, "</section>")
	}
	fmt.Fprintln(w, "<h2>What This Does Not Prove</h2><ul>")
	for _, limitation := range result.Limitations {
		fmt.Fprintf(w, "<li>%s</li>\n", html.EscapeString(limitation))
	}
	fmt.Fprintln(w, "</ul>")
	fmt.Fprintln(w, "</main></body></html>")
	return nil
}

func findScenario(result *redteam.RunResult, id string) (redteam.ScenarioResult, bool) {
	if result == nil {
		return redteam.ScenarioResult{}, false
	}
	for _, scenario := range result.Results {
		if scenario.ScenarioID == id {
			return scenario, true
		}
	}
	return redteam.ScenarioResult{}, false
}

func displayAction(action string) string {
	trimmed := strings.TrimSpace(action)
	switch strings.ToLower(trimmed) {
	case "allow", "deny":
		return strings.ToUpper(trimmed)
	default:
		return trimmed
	}
}

func passFail(passed bool) string {
	if passed {
		return "pass"
	}
	return "fail"
}
