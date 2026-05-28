package boundarycli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func TestSecureGitHubConformanceSkipsWithoutOptIn(t *testing.T) {
	t.Setenv(securegithub.EnvGitHubConformance, "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"secure", "github", "conformance", "read"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: skipped") || !strings.Contains(stdout.String(), "BOUNDARY_GITHUB_CONFORMANCE") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestSecureGitHubConformanceFailsWhenEnabledWithoutRequiredEnv(t *testing.T) {
	t.Setenv(securegithub.EnvGitHubConformance, "true")
	t.Setenv(securegithub.EnvGitHubAppID, "")
	t.Setenv(securegithub.EnvGitHubInstallationID, "")
	t.Setenv(securegithub.EnvGitHubPrivateKeyPath, "")
	t.Setenv(securegithub.EnvGitHubOwner, "")
	t.Setenv(securegithub.EnvGitHubRepo, "")
	t.Setenv(securegithub.EnvGitHubIssueNumber, "")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"secure", "github", "conformance", "read"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected failure when enabled without env, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), securegithub.EnvGitHubAppID) || !strings.Contains(stderr.String(), securegithub.EnvGitHubIssueNumber) {
		t.Fatalf("missing required env names in stderr: %s", stderr.String())
	}
}

func TestSecureGitHubHelpMentionsConformance(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"secure", "github", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{"conformance", "Live conformance is opt-in", "deployment bypass evidence"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
