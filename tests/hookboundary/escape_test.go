package hookboundary_test

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHookEscapeClasses(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantVerdict string
		isBypass    bool // document whether this is an enforced escape or a bypass
	}{
		// Enforced nested shells
		{"sh -c", "sh -c 'rm -rf /'", "deny", false},
		{"bash -c", "bash -c 'rm -rf /'", "deny", false},
		{"eval (escalates to ask)", "eval 'rm -rf /'", "ask", false},
		{"command substitution", "echo $(rm -rf /)", "deny", false},
		{"backticks", "echo `rm -rf /`", "deny", false},

		// Interpreters
		{"python -c", `python -c "import os; os.system('rm -rf /')"`, "ask", true},
		{"node -e", `node -e "require('child_process').execSync('rm -rf /')"`, "ask", true},
		{"perl -e", `perl -e "system('rm -rf /')"`, "ask", true},

		// Indirection
		{"env", "env FOO=bar rm -rf /", "deny", false},
		{"xargs", "echo / | xargs rm -rf", "deny", false},
		{"find -exec", "find . -name '*' -exec rm -rf {} \\;", "deny", false},

		// Command chaining and pipes
		{"pipe", "echo / | rm -rf", "deny", false},
		{"and chain", "echo hi && rm -rf /", "deny", false},
		{"or chain", "false || rm -rf /", "deny", false},
		{"semicolon chain", "echo hi; rm -rf /", "deny", false},

		// Ask triggers (Undecomposable)
		{"here-docs", "cat <<EOF\nrm -rf /\nEOF", "ask", false},

		// Inline aliases/functions definition
		{"alias definition", "alias r=rm; r -rf /", "ask", false}, // alias definition is ask, usage is not blocked unless name matches

		// Absolute path binaries
		{"absolute binary /bin/rm", "/bin/rm -rf /", "deny", false},
		{"absolute binary /usr/bin/rm", "/usr/bin/rm -rf /", "deny", false},
		{"absolute binary ./rm", "./rm -rf /", "ask", false}, // Unknown absolute path
		{"absolute binary /tmp/x/ls", "/tmp/x/ls", "ask", false},
		{"absolute binary path traversal /usr/bin/../../tmp/x/cat", "/usr/bin/../../tmp/x/cat", "ask", false}, // Traversal is cleaned and rejected
		{"absolute binary trick /usr/bin/./rm", "/usr/bin/./rm -rf /", "deny", false},                         // dot is cleaned and denied like rm
		{"absolute binary trick /usr/bin//rm", "/usr/bin//rm -rf /", "deny", false},                           // duplicate slash is cleaned and denied like rm
		{"absolute binary trick /usr/bin/subdir/rm", "/usr/bin/subdir/rm -rf /", "ask", false},                // Not directly inside bin dirs

		// Writes to Boundary binary/config/policy (Self-protection)
		{"write to boundary config", "echo 'x' > .claude/settings.json", "deny", false},
		{"write to hook script", "echo 'x' > .claude/hooks/pretooluse.sh", "deny", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			eventBytes, err := json.Marshal(map[string]any{
				"tool_name": "Bash",
				"tool_input": map[string]string{
					"command": tt.command,
				},
			})
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}

			code, stdout, stderr := runHook(t, dir, string(eventBytes))

			if code != 0 {
				t.Fatalf("exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
			}

			decision := decodeDecision(t, stdout)
			got := decision.HookSpecificOutput.PermissionDecision

			if got != tt.wantVerdict {
				t.Errorf("command %q: permissionDecision = %q, want %q\nReason: %s",
					tt.command, got, tt.wantVerdict, decision.reason())
			}
		})
	}

	// Also test an unmatched tool class as a documented bypass
	t.Run("unmatched tool class", func(t *testing.T) {
		dir := t.TempDir()

		eventBytes, err := json.Marshal(map[string]any{
			"tool_name": "UnknownTool",
			"tool_input": map[string]string{
				"command": "rm -rf /",
			},
		})
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}

		code, stdout, _ := runHook(t, dir, string(eventBytes))

		if code != 0 {
			t.Fatalf("exit = %d, want 0", code)
		}

		if strings.TrimSpace(stdout) != "" {
			t.Errorf("expected empty stdout for un-matched tool class, got: %s", stdout)
		}
	})
}
