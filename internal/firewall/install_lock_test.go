package firewall

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInstallConfigBacksUpRewritesAndRestoresByteForByte(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "mcp.json")
	original := `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "args": ["--token", "ghp_secret"],
      "env": {"GITHUB_TOKEN": "ghp_secret"},
      "tools": [{"name": "get_issue"}, {"name": "create_or_update_file"}]
    }
  }
}`
	writeFirewallTestFile(t, configPath, original)

	dryRun, err := InstallConfig(InstallOptions{
		ConfigPath: configPath,
		OutDir:     filepath.Join(root, ".boundary", "firewall"),
		Servers:    []string{"github"},
		DryRun:     true,
		Now:        time.Unix(100, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Mutated || dryRun.BackupPath != "" || dryRun.ReceiptPath != "" {
		t.Fatalf("dry-run should not report mutation, backup, or receipt: %+v", dryRun)
	}
	assertFileEquals(t, configPath, original)

	result, err := InstallConfig(InstallOptions{
		ConfigPath: configPath,
		OutDir:     filepath.Join(root, ".boundary", "firewall"),
		Servers:    []string{"github"},
		Now:        time.Unix(100, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Mutated || result.BackupPath == "" || result.ReceiptPath == "" {
		t.Fatalf("install result missing mutation evidence: %+v", result)
	}
	assertFileEquals(t, result.BackupPath, original)
	receiptBody, err := os.ReadFile(result.ReceiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(receiptBody), "ghp_secret") {
		t.Fatalf("install receipt leaked secret-like value: %s", string(receiptBody))
	}
	installed, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	installedText := string(installed)
	for _, want := range []string{`"command": "boundary"`, `"mcp"`, `"proxy"`, `"BOUNDARY_MCP_SERVER": "github"`} {
		if !strings.Contains(installedText, want) {
			t.Fatalf("installed config missing %q: %s", want, installedText)
		}
	}

	restore, err := UninstallConfig(UninstallOptions{ReceiptPath: result.ReceiptPath})
	if err != nil {
		t.Fatal(err)
	}
	if !restore.Restored {
		t.Fatalf("uninstall did not report restore: %+v", restore)
	}
	assertFileEquals(t, configPath, original)
}

func TestUninstallRefusesToClobberPostInstallEdits(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "mcp.json")
	original := `{"mcpServers":{"github":{"command":"github-mcp-server"}}}`
	writeFirewallTestFile(t, configPath, original)

	result, err := InstallConfig(InstallOptions{
		ConfigPath: configPath,
		OutDir:     filepath.Join(root, ".boundary", "firewall"),
		Now:        time.Unix(150, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFirewallTestFile(t, configPath, `{"mcpServers":{"github":{"command":"operator-edited-after-install"}}}`)
	if _, err := UninstallConfig(UninstallOptions{ReceiptPath: result.ReceiptPath}); err == nil {
		t.Fatal("uninstall should refuse to clobber post-install edits without force")
	}
	if _, err := UninstallConfig(UninstallOptions{ReceiptPath: result.ReceiptPath, Force: true}); err != nil {
		t.Fatalf("forced uninstall should restore after explicit operator choice: %v", err)
	}
	assertFileEquals(t, configPath, original)
}

func TestInstallPreservesUnknownFieldsAndUnselectedServers(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "mcp.json")
	body := `{
  "clientMeta": {"keep": true},
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "customTransport": {"keep": true},
      "tools": [{"name": "get_issue"}]
    },
    "filesystem": {
      "command": "filesystem-mcp",
      "customTransport": {"untouched": true}
    }
  }
}`
	writeFirewallTestFile(t, configPath, body)

	if _, err := InstallConfig(InstallOptions{
		ConfigPath: configPath,
		OutDir:     filepath.Join(root, ".boundary", "firewall"),
		Servers:    []string{"github"},
		Now:        time.Unix(160, 0),
	}); err != nil {
		t.Fatal(err)
	}
	installed := string(mustFirewallTestRead(t, configPath))
	for _, want := range []string{`"clientMeta"`, `"customTransport"`, `"filesystem-mcp"`, `"untouched"`} {
		if !strings.Contains(installed, want) {
			t.Fatalf("install dropped unknown or unselected field %q: %s", want, installed)
		}
	}
	if !strings.Contains(installed, `"BOUNDARY_MCP_SERVER": "github"`) {
		t.Fatalf("selected server was not routed: %s", installed)
	}
}

func TestDescriptorLockDetectsDriftAndHonorsChangeMode(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "mcp.json")
	lockPath := filepath.Join(root, ".boundary", "firewall", "locks", "descriptor-lock.json")
	writeFirewallTestFile(t, configPath, `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [
        {
          "name": "get_issue",
          "description": "Read a GitHub issue",
          "inputSchema": {"type": "object", "properties": {"number": {"type": "integer"}}}
        },
        {"name": "create_or_update_file"}
      ]
    }
  }
}`)

	created, err := CreateDescriptorLock(LockOptions{
		ConfigPath: configPath,
		OutPath:    lockPath,
		Servers:    []string{"github"},
		Now:        time.Unix(200, 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.Written || len(created.LockFile.Servers) != 1 {
		t.Fatalf("lock result missing server: %+v", created)
	}
	ok, err := VerifyDescriptorLock(VerifyLockOptions{LockPath: lockPath, OnChange: "deny"})
	if err != nil {
		t.Fatal(err)
	}
	if ok.Status != "ok" || !ok.Allowed {
		t.Fatalf("unchanged lock should pass: %+v", ok)
	}

	writeFirewallTestFile(t, configPath, `{
  "mcpServers": {
    "github": {
      "command": "github-mcp-server",
      "tools": [
        {
          "name": "get_issue",
          "description": "Read a GitHub issue with changed semantics",
          "inputSchema": {"type": "object", "properties": {"number": {"type": "string"}}}
        },
        {"name": "create_or_update_file"}
      ]
    }
  }
}`)
	warn, err := VerifyDescriptorLock(VerifyLockOptions{LockPath: lockPath, OnChange: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	if warn.Status != "drift" || !warn.Allowed || warn.Summary.Changed != 1 {
		t.Fatalf("warn mode should allow recorded drift: %+v", warn)
	}
	deny, err := VerifyDescriptorLock(VerifyLockOptions{LockPath: lockPath, OnChange: "deny"})
	if err != nil {
		t.Fatal(err)
	}
	if deny.Status != "drift" || deny.Allowed || deny.Summary.Changed != 1 {
		t.Fatalf("deny mode should fail closed on drift: %+v", deny)
	}
}

func TestRedactArgsRedactsOpaqueValuesAfterSecretFlags(t *testing.T) {
	got := redactArgs([]string{"--token", "abc123", "--api-key", "opaque", "--safe", "value"})
	joined := strings.Join(got, " ")
	if strings.Contains(joined, "abc123") || strings.Contains(joined, "opaque") {
		t.Fatalf("secret flag values leaked after redaction: %#v", got)
	}
	if !strings.Contains(joined, "value") {
		t.Fatalf("non-secret value should remain visible: %#v", got)
	}
}

func writeFirewallTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileEquals(t *testing.T, path, want string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("file %s mismatch:\nwant: %s\n got: %s", path, want, string(body))
	}
}

func mustFirewallTestRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
