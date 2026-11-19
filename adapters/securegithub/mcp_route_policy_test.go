package securegithub

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// compiledInRules is the route's rule set as it was hard-coded before FUL-772
// moved it into mcp_route_policy.yaml. The YAML must decide exactly as it did.
func compiledInRules() []governance.StaticPolicyRule {
	deny := func(name, tool, reason string, conditions ...governance.StaticPolicyMatch) governance.StaticPolicyRule {
		return governance.StaticPolicyRule{
			Name: name, Tool: tool, Action: "deny", Reason: reason,
			Transport: string(governance.TransportMCP), DecisionMode: governance.DecisionModeDeterministic,
			Conditions: conditions, Metadata: map[string]string{"profile": ProfileID},
		}
	}
	equals := func(field, value string) governance.StaticPolicyMatch {
		return governance.StaticPolicyMatch{Type: "equals", Field: field, Value: value}
	}
	return []governance.StaticPolicyRule{
		deny("secure-github-tool-allowlist", "*", "tool is not exposed by the Secure GitHub route", equals("arguments.tool_allowed", "false")),
		deny("secure-github-source-allowlist", "github."+GitHubIssueReadTool, "source issue is outside the configured untrusted source", equals("arguments.source_allowed", "false")),
		deny("secure-github-sink-allowlist", "github."+GitHubProtectedWriteTool, "target repository or branch is outside the configured protected sink", equals("arguments.sink_allowed", "false")),
		deny("secure-github-write-after-taint", "github."+GitHubProtectedWriteTool, "protected write denied after configured untrusted source read", equals("arguments.tainted", "true")),
		deny("secure-github-read-before-write", "github."+GitHubProtectedWriteTool, "protected write requires a successful configured source read"),
	}
}

func TestMCPRoutePolicyMatchesCompiledInRules(t *testing.T) {
	embedded, err := MCPRoutePolicyRules()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(embedded, compiledInRules()) {
		t.Fatalf("embedded bundle differs from the compiled-in rules:\n got %+v\nwant %+v", embedded, compiledInRules())
	}

	// The seeded file loads to the same rules through the operator loader.
	dir := t.TempDir()
	if err := WriteMCPRoutePolicy(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := governance.LoadStaticPoliciesFromDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := range loaded {
		loaded[i].PolicyFile = ""
	}
	if !reflect.DeepEqual(loaded, compiledInRules()) {
		t.Fatalf("seeded bundle differs from the compiled-in rules:\n got %+v", loaded)
	}
	info, err := os.Stat(filepath.Join(dir, MCPRoutePolicyFile))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("seeded file mode: %v %v, want 0600", info, err)
	}
}

func TestWriteMCPRoutePolicyKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, MCPRoutePolicyFile)
	if err := os.WriteFile(path, []byte("operator edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteMCPRoutePolicy(dir); err != nil {
		t.Fatal(err)
	}
	if body, _ := os.ReadFile(path); string(body) != "operator edit\n" {
		t.Fatalf("existing file overwritten: %q", body)
	}
}
