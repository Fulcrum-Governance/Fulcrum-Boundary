package doctor_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestDoctorDefaultTextIsLocalOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"Boundary doctor",
		"status: pass",
		"credentials: none",
		"network: none",
		"live mutation: none",
		"Environment diagnostics:",
		"Go toolchain",
		"cgo / C toolchain",
		"go install PATH",
		"Surface: MCP",
		"Surface: Command Boundary",
		"Surface: Edit Boundary",
		"Bypass caveats:",
		"Direct upstream MCP server access is outside Boundary",
		"Direct shell, scripts, cron, SSH, and CI jobs are bypasses",
		"Direct editor writes, direct filesystem mutation, and direct git apply are bypasses",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{"upstream reachable", "listen port available", "Postgres upstream"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("doctor output kept old gateway prerequisite wording %q:\n%s", forbidden, output)
		}
	}
}

func TestDoctorJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor json exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var payload struct {
		SchemaVersion       string `json:"schema_version"`
		Status              string `json:"status"`
		RequiresCredentials bool   `json:"requires_credentials"`
		RequiresNetwork     bool   `json:"requires_network"`
		MutatesLiveSystems  bool   `json:"mutates_live_systems"`
		Environment         []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"environment"`
		Surfaces []struct {
			Surface       string   `json:"surface"`
			Status        string   `json:"status"`
			Checks        []any    `json:"checks"`
			BypassCaveats []string `json:"bypass_caveats"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("parse doctor json: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != "boundary.doctor.v1" || payload.Status != "pass" {
		t.Fatalf("unexpected doctor identity: %#v", payload)
	}
	if payload.RequiresCredentials || payload.RequiresNetwork || payload.MutatesLiveSystems {
		t.Fatalf("doctor must not need credentials, network, or live mutation: %#v", payload)
	}
	if len(payload.Environment) < 3 {
		t.Fatalf("expected first-run environment diagnostics, got %#v", payload.Environment)
	}
	for _, want := range []string{"Go toolchain", "cgo / C toolchain", "go install PATH"} {
		var found bool
		for _, check := range payload.Environment {
			if check.Name == want && check.Status != "" && check.Detail != "" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("environment diagnostics missing %q: %#v", want, payload.Environment)
		}
	}
	if len(payload.Surfaces) != 3 {
		t.Fatalf("expected all three surfaces, got %d: %#v", len(payload.Surfaces), payload.Surfaces)
	}
	for _, surface := range payload.Surfaces {
		if surface.Status == "" || len(surface.Checks) == 0 || len(surface.BypassCaveats) == 0 {
			t.Fatalf("surface missing diagnostics: %#v", surface)
		}
	}
}

func TestDoctorReportRedactsLocalPaths(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor", "--report"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor report exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, `"report_redacted": true`) {
		t.Fatalf("doctor report did not mark redaction:\n%s", output)
	}
	if strings.Contains(output, wd) {
		t.Fatalf("doctor report leaked working directory %q:\n%s", wd, output)
	}
	if !strings.Contains(output, `"project_root": "<redacted>"`) {
		t.Fatalf("doctor report did not redact project_root:\n%s", output)
	}
	if !strings.Contains(output, `"environment"`) || !strings.Contains(output, `"surfaces"`) {
		t.Fatalf("doctor report missing diagnostic sections:\n%s", output)
	}
}

func TestDoctorSurfaceFilter(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor", "--surface", "command"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor surface exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Surface: Command Boundary") {
		t.Fatalf("command surface missing:\n%s", output)
	}
	for _, forbidden := range []string{"Surface: MCP", "Surface: Edit Boundary"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("surface filter included %q:\n%s", forbidden, output)
		}
	}
}

func TestDoctorRejectsUnknownSurface(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor", "--surface", "network"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected unknown surface failure, stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown doctor surface") {
		t.Fatalf("stderr missing unknown surface error: %s", stderr.String())
	}
}

func TestDoctorHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"doctor", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("doctor help exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	output := stdout.String() + stderr.String()
	for _, want := range []string{
		"Check local routed-surface diagnostics",
		"boundary doctor --surface mcp",
		"boundary doctor --json",
		"boundary doctor --report",
		"does not prove production deployment protection",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}
