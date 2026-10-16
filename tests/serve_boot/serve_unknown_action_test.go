package serve_boot_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeBoot_RejectsUnknownPolicyAction(t *testing.T) {
	bin := buildBoundary(t)
	for _, tc := range []struct {
		name, policy, wantErr string
	}{
		{
			name: "legacy shape",
			policy: `name: observe-echo
version: "1"
rules:
  - name: observe-echo
    tool: echo
    action: observe
`,
			wantErr: `observe.yaml: rules[0] "observe-echo": unsupported action "observe" (want one of: allow, deny, warn, audit, escalate, require_approval)`,
		},
		{
			name: "v1 schema",
			policy: `schema_version: "1"
policy:
  name: observe-echo
  version: "1"
  rules:
    - name: observe-echo
      tool: echo
      action: observe
`,
			wantErr: `observe.yaml: policy.rules[0]: unsupported action "observe" (want one of: allow, deny, warn, audit, escalate, require_approval)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "observe.yaml"), []byte(tc.policy), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "serve", "--listen", freePort(t), "--policies", dir, "--upstream", "http://127.0.0.1:1/mcp")
			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("serve with action observe: err=%v output=%s, want exit 1", err, out)
			}
			if !strings.Contains(string(out), tc.wantErr) {
				t.Fatalf("serve output = %q, want it to contain %q", out, tc.wantErr)
			}
		})
	}
}
