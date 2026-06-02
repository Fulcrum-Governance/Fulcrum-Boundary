package cli_output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
)

func TestHelpOutputUsesIntentionalLanguage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "root",
			args: []string{"--help"},
			want: []string{"Purpose:", "routed tools", "record the verdict"},
		},
		{
			name: "selftest",
			args: []string{"selftest", "--help"},
			want: []string{"Run local no-credential Boundary release checks.", "Common usage:", "not live deployment conformance"},
		},
		{
			name: "demo",
			args: []string{"demo", "--help"},
			want: []string{"Run Boundary demos", "Fixture demos use no credentials", "github-lethal-trifecta"},
		},
		{
			name: "demo github lethal trifecta",
			args: []string{"demo", "github-lethal-trifecta", "--help"},
			want: []string{"fixture-only Secure GitHub denial demo", "no live GitHub mutation", "not live GitHub App conformance"},
		},
		{
			name: "inventory",
			args: []string{"inventory", "--help"},
			want: []string{"Discover MCP configs", "routed tools", "local file inspection"},
		},
		{
			name: "graph",
			args: []string{"graph", "--help"},
			want: []string{"Render inventory-derived MCP risk paths", "Mermaid output", "not proof that a live action occurred"},
		},
		{
			name: "policy generate",
			args: []string{"policy", "generate", "--help"},
			want: []string{"Generate starter policies", "review baseline", "boundary verify --policies"},
		},
		{
			name: "install",
			args: []string{"install", "--help"},
			want: []string{"routed tools execute through Boundary", "Dry runs do not mutate", "deployment bypass"},
		},
		{
			name: "uninstall",
			args: []string{"uninstall", "--help"},
			want: []string{"Restore an MCP config", "restore plan", "install receipt"},
		},
		{
			name: "inventory ingest",
			args: []string{"inventory", "ingest", "--help"},
			want: []string{"external MCP inventory NDJSON", "Boundary does not depend on or endorse", "Partial snapshots"},
		},
		{
			name: "dashboard",
			args: []string{"dashboard", "--help"},
			want: []string{"local-only dashboard", "loopback-only", "not a policy enforcement path"},
		},
		{
			name: "secure github",
			args: []string{"secure", "github", "--help"},
			want: []string{"preview profile", "routed GitHub tools", "no live mutation"},
		},
		{
			name: "test",
			args: []string{"test", "--help"},
			want: []string{"Run local policy-as-code test cases", "no credentials, no network, no live mutation", "does not prove production route enforcement"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output, code := runBoundary(tc.args...)
			if code != 0 {
				t.Fatalf("expected exit 0, got %d output=%s", code, output)
			}
			if strings.Contains(output, "Usage of boundary") {
				t.Fatalf("help output still uses raw flag package header: %s", output)
			}
			assertNoBannedVendorLanguage(t, output)
			for _, want := range tc.want {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q:\n%s", want, output)
				}
			}
		})
	}
}

func TestExternalInventoryHelpDoesNotUseDependencyLanguage(t *testing.T) {
	output, code := runBoundary("inventory", "ingest", "--help")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d output=%s", code, output)
	}
	for _, forbidden := range []string{"integration", "integrates with", "compatible with"} {
		if strings.Contains(strings.ToLower(output), forbidden) {
			t.Fatalf("external inventory help uses forbidden dependency language %q:\n%s", forbidden, output)
		}
	}
}

func runBoundary(args ...string) (string, int) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := boundarycli.Run(args, &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func assertNoBannedVendorLanguage(t *testing.T, output string) {
	t.Helper()
	lowered := strings.ToLower(output)
	joined := "bumble" + "bee"
	spaced := "bumble" + " " + "bee"
	for _, forbidden := range []string{joined, spaced} {
		if strings.Contains(lowered, forbidden) {
			t.Fatalf("output contains banned vendor language %q:\n%s", forbidden, output)
		}
	}
}
