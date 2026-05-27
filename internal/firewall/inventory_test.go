package firewall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInventoryDiscoversConfigsAndClassifiesGitHub(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeMCPConfig(t, filepath.Join(root, ".mcp.json"), `{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github", "--token", "ghp_secret"],
      "env": {"GITHUB_TOKEN": "ghp_secret"}
    },
    "filesystem": {
      "command": "mcp-filesystem",
      "args": ["/tmp"]
    }
  }
}`)
	claudePath := filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	writeMCPConfig(t, claudePath, `{
  "mcpServers": {
    "postgres": {
      "command": "postgres-mcp",
      "env": {"DATABASE_URL": "postgres://user:pass@example/db"}
    }
  }
}`)

	inventory, err := BuildInventory(DiscoverOptions{
		Root:            root,
		Home:            home,
		IncludeDefaults: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if inventory.Summary.ConfigFiles != 2 {
		t.Fatalf("config files = %d, want 2", inventory.Summary.ConfigFiles)
	}
	if inventory.Summary.Servers != 3 {
		t.Fatalf("servers = %d, want 3", inventory.Summary.Servers)
	}
	if inventory.Summary.GitHubServers != 1 {
		t.Fatalf("github servers = %d, want 1", inventory.Summary.GitHubServers)
	}
	if inventory.Summary.HighRiskServers != 3 {
		t.Fatalf("high-risk servers = %d, want 3", inventory.Summary.HighRiskServers)
	}

	github := findServer(t, inventory, "github")
	if github.HighestRisk != "W2" {
		t.Fatalf("github highest risk = %s, want W2", github.HighestRisk)
	}
	if !hasCapability(github, "create_or_update_file", "W1") {
		t.Fatalf("github capabilities missing create_or_update_file W1: %+v", github.Capabilities)
	}
	if !hasCapability(github, "merge_pull_request", "W2") {
		t.Fatalf("github capabilities missing merge_pull_request W2: %+v", github.Capabilities)
	}
	if !stringSliceContains(github.EnvKeys, "GITHUB_TOKEN") {
		t.Fatalf("github env keys missing GITHUB_TOKEN: %+v", github.EnvKeys)
	}
	if strings.Contains(strings.Join(github.Args, " "), "ghp_secret") {
		t.Fatalf("secret-like CLI arg was not redacted: %+v", github.Args)
	}
	body, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "postgres://user:pass") || strings.Contains(string(body), "ghp_secret") {
		t.Fatalf("inventory leaked secret material: %s", string(body))
	}
}

func TestRenderInventoryFormats(t *testing.T) {
	root := t.TempDir()
	writeMCPConfig(t, filepath.Join(root, "mcp.json"), `{
  "servers": {
    "github": {
      "command": "github-mcp"
    }
  }
}`)
	inventory, err := BuildInventory(DiscoverOptions{Root: root, IncludeDefaults: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, format := range []string{"json", "markdown", "sarif"} {
		body, err := RenderInventory(inventory, format)
		if err != nil {
			t.Fatalf("render %s: %v", format, err)
		}
		text := string(body)
		switch format {
		case "json":
			if !strings.Contains(text, `"schema_version": "boundary.firewall.inventory.v1"`) {
				t.Fatalf("json report missing schema: %s", text)
			}
		case "markdown":
			if !strings.Contains(text, "create_or_update_file:W1") {
				t.Fatalf("markdown report missing W1 capability: %s", text)
			}
		case "sarif":
			if !strings.Contains(text, `"version": "2.1.0"`) || !strings.Contains(text, "boundary.mcp.high-risk") {
				t.Fatalf("sarif report missing expected fields: %s", text)
			}
		}
	}
}

func findServer(t *testing.T, inventory Inventory, name string) Server {
	t.Helper()
	for _, server := range inventory.Servers {
		if server.Name == name {
			return server
		}
	}
	t.Fatalf("server %q not found in %+v", name, inventory.Servers)
	return Server{}
}

func hasCapability(server Server, name, class string) bool {
	for _, capability := range server.Capabilities {
		if capability.Name == name && capability.Class == class {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeMCPConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
