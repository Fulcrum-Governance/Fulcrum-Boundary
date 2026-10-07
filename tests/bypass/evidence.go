//go:build bypass

package bypass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Evidence is the machine-readable record of one topology probe run. CI
// uploads it as a workflow artifact; it is never committed.
type Evidence struct {
	TopologyID      string        `json:"topology_id"`
	BoundaryVersion string        `json:"boundary_version"`
	ImageDigests    []string      `json:"image_digests"`
	Timestamp       string        `json:"timestamp"`
	Probes          []ProbeReport `json:"probes"`
}

// ProbeReport is one probe's outcome: what was expected, what was observed,
// and the raw exec results that produced the verdict.
type ProbeReport struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Expected    string       `json:"expected"`
	Observed    string       `json:"observed"`
	Passed      bool         `json:"passed"`
	Steps       []ExecResult `json:"steps"`
}

// WriteEvidence writes evidence JSON plus a markdown summary under dir and
// returns the two paths.
func WriteEvidence(t *testing.T, dir string, e Evidence) (jsonPath, mdPath string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create evidence dir: %v", err)
	}
	jsonPath = filepath.Join(dir, e.TopologyID+".json")
	mdPath = filepath.Join(dir, e.TopologyID+".md")

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		t.Fatalf("write evidence json: %v", err)
	}
	if err := os.WriteFile(mdPath, []byte(evidenceMarkdown(e)), 0o644); err != nil {
		t.Fatalf("write evidence markdown: %v", err)
	}
	return jsonPath, mdPath
}

func evidenceMarkdown(e Evidence) string {
	var md bytes.Buffer
	fmt.Fprintf(&md, "# Bypass-resistance evidence: %s\n\n", e.TopologyID)
	fmt.Fprintf(&md, "- timestamp: %s\n", e.Timestamp)
	fmt.Fprintf(&md, "- image ids: %s\n", strings.Join(e.ImageDigests, ", "))
	fmt.Fprintf(&md, "- boundary version:\n\n```\n%s\n```\n\n", e.BoundaryVersion)
	md.WriteString("Scope: these outcomes were observed in the named reference topology only; they are not a claim about any other deployment.\n\n")
	md.WriteString("| Probe | Expected | Observed | Result |\n|---|---|---|---|\n")
	for _, p := range e.Probes {
		result := "FAIL"
		if p.Passed {
			result = "pass"
		}
		fmt.Fprintf(&md, "| %s | %s | %s | %s |\n", p.Name, p.Expected, p.Observed, result)
	}
	md.WriteString("\n## Raw probe output\n")
	for _, p := range e.Probes {
		fmt.Fprintf(&md, "\n### %s\n\n%s\n", p.Name, p.Description)
		for i, step := range p.Steps {
			fmt.Fprintf(&md, "\nstep %d argv: `%s` (exit %d)\n\nstdout:\n```\n%s```\nstderr:\n```\n%s```\n",
				i+1, strings.Join(step.Argv, " "), step.ExitCode, fenced(step.Stdout), fenced(step.Stderr))
		}
	}
	return md.String()
}

func fenced(s string) string {
	if s == "" {
		return "(empty)\n"
	}
	if strings.Contains(s, "```") {
		return s // rare: avoid nesting fences by leaving raw
	}
	if !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}
