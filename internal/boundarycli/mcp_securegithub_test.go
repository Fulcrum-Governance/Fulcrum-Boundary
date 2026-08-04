package boundarycli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSecureGitHubMCPHelpAndCredentialReadinessGate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"mcp", "secure-github", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-token-env") || strings.Contains(stderr.String(), "token string") {
		t.Fatalf("help does not expose the environment-only token contract: %s", stderr.String())
	}

	t.Setenv("FUL471_TEST_GITHUB_TOKEN", "")
	stdout.Reset()
	stderr.Reset()
	code := Run([]string{
		"mcp", "secure-github",
		"--source-owner", "public-org", "--source-repo", "issues", "--source-issue", "17",
		"--target-owner", "private-org", "--target-repo", "protected", "--target-branch", "main",
		"--token-env", "FUL471_TEST_GITHUB_TOKEN",
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "NOT READY: SECURE_GITHUB_MCP_TOKEN_MISSING (FUL471_TEST_GITHUB_TOKEN)") {
		t.Fatalf("credential readiness exit/stderr = %d/%q", code, stderr.String())
	}
}

func TestSecureGitHubMCPRejectsNonLoopbackListenAndInsecureUpstream(t *testing.T) {
	base := []string{
		"mcp", "secure-github",
		"--source-owner", "public-org", "--source-repo", "issues", "--source-issue", "17",
		"--target-owner", "private-org", "--target-repo", "protected", "--target-branch", "main",
	}
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "listen", args: []string{"--listen", "0.0.0.0:8080"}, want: "loopback"},
		{name: "upstream", args: []string{"--upstream", "http://github.example/mcp"}, want: "must use HTTPS"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(append(append([]string{}, base...), test.args...), &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("exit/stderr = %d/%q", code, stderr.String())
			}
		})
	}
}
