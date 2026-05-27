package securegithub_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestBoundarySecureGitHubSetupWritesPreviewProfile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "secure-github")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"secure", "github", "setup", "--out", out, "--owner", "fixture-org", "--repo", "fixture-private-repo"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("setup exit = %d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"status: preview", "fixture mode: true", "live GitHub mutation: none"} {
		if !strings.Contains(output, want) {
			t.Fatalf("setup output missing %q: %s", want, output)
		}
	}
	body, err := os.ReadFile(filepath.Join(out, "secure-github-profile.json"))
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	var profile struct {
		ProfileID          string `json:"profile_id"`
		Status             string `json:"status"`
		FixtureMode        bool   `json:"fixture_mode"`
		LiveGitHubEvidence bool   `json:"live_github_evidence"`
		Owner              string `json:"owner"`
		Repo               string `json:"repo"`
	}
	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("parse profile: %v", err)
	}
	if profile.ProfileID != "secure-github" || profile.Status != "preview" || !profile.FixtureMode || profile.LiveGitHubEvidence {
		t.Fatalf("unexpected profile truth: %#v", profile)
	}
	if profile.Owner != "fixture-org" || profile.Repo != "fixture-private-repo" {
		t.Fatalf("unexpected repo binding: %#v", profile)
	}
}

func TestBoundarySecureGitHubServeDryRunStatesFixturePreview(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"secure", "github", "serve", "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("serve dry-run exit = %d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"profile: secure-github",
		"status: preview",
		"fixture mode: true",
		"one repo per session: true",
		"protected writes after taint: W1,W2 deny before upstream",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("serve dry-run missing %q: %s", want, output)
		}
	}
}

func TestBoundarySecureGitHubServeLiveModeFailsClosed(t *testing.T) {
	var stderr bytes.Buffer
	code := boundarycli.Run([]string{"secure", "github", "serve", "--fixture=false", "--dry-run"}, &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatal("expected live mode to fail in preview profile")
	}
	if !strings.Contains(stderr.String(), "live GitHub App mode is not implemented") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
