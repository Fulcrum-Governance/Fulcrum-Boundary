package evidence_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

type manifestPayload struct {
	SchemaVersion       string `json:"schema_version"`
	Source              string `json:"source"`
	Output              string `json:"output"`
	Summary             string `json:"summary"`
	IncludeDemo         bool   `json:"include_demo"`
	RequiresCredentials bool   `json:"requires_credentials"`
	RequiresNetwork     bool   `json:"requires_network"`
	MutatesLiveSystems  bool   `json:"mutates_live_systems"`
	FixtureSafeOutputs  []string
	Artifacts           []struct {
		Path          string `json:"path"`
		Kind          string `json:"kind"`
		SHA256        string `json:"sha256"`
		SizeBytes     int64  `json:"size_bytes"`
		SchemaVersion string `json:"schema_version"`
	} `json:"artifacts"`
}

func TestEvidenceBundleDefault(t *testing.T) {
	out := filepath.Join(t.TempDir(), "evidence")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "bundle", "--out", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("bundle exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	manifest := parseManifest(t, stdout.Bytes())
	if manifest.SchemaVersion != "boundary.evidence_bundle.v1" {
		t.Fatalf("unexpected manifest schema: %#v", manifest)
	}
	if manifest.IncludeDemo {
		t.Fatalf("default bundle should not include demo: %#v", manifest)
	}
	if manifest.RequiresCredentials || manifest.RequiresNetwork || manifest.MutatesLiveSystems {
		t.Fatalf("bundle must be fixture-safe: %#v", manifest)
	}
	for _, want := range []string{
		"manifest.json",
		"summary.md",
		"version.json",
		"version.txt",
		"selftest.json",
		"selftest.txt",
		"doctor.json",
	} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Fatalf("missing %s: %v", want, err)
		}
	}
	requireArtifact(t, manifest, "version.json", "version", "boundary.version.v1")
	requireArtifact(t, manifest, "selftest.json", "selftest", "boundary.selftest.v1")
	requireArtifact(t, manifest, "doctor.json", "doctor", "boundary.doctor.v1")
	requireArtifact(t, manifest, "summary.md", "summary", "")
}

func TestEvidenceBundleIncludeDemo(t *testing.T) {
	out := filepath.Join(t.TempDir(), "evidence")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "bundle", "--include-demo", "--out", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("bundle include demo exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	manifest := parseManifest(t, stdout.Bytes())
	if !manifest.IncludeDemo {
		t.Fatalf("include_demo not reflected in manifest: %#v", manifest)
	}
	requireArtifact(t, manifest, "demo/action-boundary.json", "action_boundary_demo", "boundary.demo.action_boundary.v1")
	requireArtifact(t, manifest, "demo/action-boundary.txt", "action_boundary_demo_text", "")
	body, err := os.ReadFile(filepath.Join(out, "demo", "action-boundary.txt"))
	if err != nil {
		t.Fatalf("read action-boundary text: %v", err)
	}
	if !strings.Contains(string(body), "upstream_called=false") || !strings.Contains(string(body), "executed=false") {
		t.Fatalf("demo text missing no-mutation evidence:\n%s", string(body))
	}
}

func TestEvidenceBundleCopiesSourceArtifacts(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, ".boundary")
	if err := os.MkdirAll(filepath.Join(source, "decision-records"), 0o700); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	record := `{"record_id":"rec_test","action":"deny","decision_hash":"sha256:test"}`
	if err := os.WriteFile(filepath.Join(source, "decision-records", "record.json"), []byte(record), 0o600); err != nil {
		t.Fatalf("write source record: %v", err)
	}
	out := filepath.Join(root, "evidence")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"evidence", "bundle", "--from", source, "--out", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("bundle source exit = %d, stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	manifest := parseManifest(t, stdout.Bytes())
	requireArtifact(t, manifest, "artifacts/decision-records/record.json", "decision_record", "")
	if _, err := os.Stat(filepath.Join(out, "artifacts", "decision-records", "record.json")); err != nil {
		t.Fatalf("copied source record missing: %v", err)
	}
}

func TestEvidenceBundleHelp(t *testing.T) {
	for _, args := range [][]string{
		{"evidence", "--help"},
		{"evidence", "bundle", "--help"},
	} {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := boundarycli.Run(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit = %d, stdout=%s stderr=%s", args, code, stdout.String(), stderr.String())
		}
		output := stdout.String() + stderr.String()
		for _, want := range []string{"evidence bundle", "no credentials", "no network"} {
			if !strings.Contains(output, want) {
				t.Fatalf("%v help missing %q:\n%s", args, want, output)
			}
		}
	}
}

func parseManifest(t *testing.T, data []byte) manifestPayload {
	t.Helper()
	var manifest manifestPayload
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v\n%s", err, string(data))
	}
	return manifest
}

func requireArtifact(t *testing.T, manifest manifestPayload, path, kind, schema string) {
	t.Helper()
	for _, artifact := range manifest.Artifacts {
		if artifact.Path == path {
			if artifact.Kind != kind || artifact.SchemaVersion != schema {
				t.Fatalf("artifact %s mismatch: %#v", path, artifact)
			}
			if !strings.HasPrefix(artifact.SHA256, "sha256:") || artifact.SizeBytes <= 0 {
				t.Fatalf("artifact %s missing hash/size: %#v", path, artifact)
			}
			return
		}
	}
	t.Fatalf("artifact %s not found in manifest: %#v", path, manifest.Artifacts)
}
