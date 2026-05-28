package cli_output_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionTextOutput(t *testing.T) {
	output, code := runBoundary("version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d output=%s", code, output)
	}
	for _, want := range []string{
		"Fulcrum Boundary ",
		"commit:",
		"build_date:",
		"go: go",
		"module: github.com/fulcrum-governance/fulcrum-boundary",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "0.2.0-dev") {
		t.Fatalf("version output includes stale development version:\n%s", output)
	}
}

func TestVersionJSONOutput(t *testing.T) {
	output, code := runBoundary("version", "--json")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d output=%s", code, output)
	}
	var payload struct {
		SchemaVersion string `json:"schema_version"`
		Version       string `json:"version"`
		Commit        string `json:"commit"`
		BuildDate     string `json:"build_date"`
		GoVersion     string `json:"go_version"`
		Module        string `json:"module"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("version JSON did not parse: %v\n%s", err, output)
	}
	if payload.SchemaVersion != "boundary.version.v1" {
		t.Fatalf("schema_version = %q", payload.SchemaVersion)
	}
	if payload.Version == "" || payload.Commit == "" || payload.BuildDate == "" {
		t.Fatalf("metadata fields must not be empty: %+v", payload)
	}
	if !strings.HasPrefix(payload.GoVersion, "go") {
		t.Fatalf("go_version = %q", payload.GoVersion)
	}
	if payload.Module != "github.com/fulcrum-governance/fulcrum-boundary" {
		t.Fatalf("module = %q", payload.Module)
	}
}

func TestVersionHelpOutput(t *testing.T) {
	output, code := runBoundary("version", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d output=%s", code, output)
	}
	for _, want := range []string{
		"Print Boundary version and build metadata.",
		"boundary version --json",
		"Missing build metadata is reported as unknown",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}
