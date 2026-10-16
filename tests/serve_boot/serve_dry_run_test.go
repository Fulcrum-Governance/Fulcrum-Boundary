package serve_boot_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// In dry-run a deny is recorded as the real deny with dry_run=true and the
// call still reaches upstream. The mode is reachable from the CLI flag and
// from security.dry_run in the runtime config.
func TestServe_DryRunRecordsTheDenyAndLetsTheCallProceed(t *testing.T) {
	bin := buildBoundary(t)
	for _, tc := range []struct {
		name string
		args func(t *testing.T, policyDir, addr, upstream string) []string
	}{
		{"flag", func(_ *testing.T, policyDir, addr, upstream string) []string {
			return []string{"serve", "--dry-run", "--listen", addr, "--policies", policyDir, "--upstream", upstream}
		}},
		{"config", func(t *testing.T, policyDir, addr, upstream string) []string {
			cfg := fmt.Sprintf("mode: standalone\nserver:\n  listen: %q\n  upstream: %q\nstandalone:\n  policy_dir: %q\nsecurity:\n  dry_run: true\n", addr, upstream, policyDir)
			path := filepath.Join(t.TempDir(), "boundary.yaml")
			if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			return []string{"serve", "--config", path}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policyDir := t.TempDir()
			writeDenyPolicy(t, policyDir)
			addr := freePort(t)

			var upstreamCalls atomic.Int64
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamCalls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"stub-ok"}}`))
			}))
			defer upstream.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, tc.args(t, policyDir, addr, upstream.URL)...)
			cmd.Dir = t.TempDir() // serve writes decision records under its working directory
			var stdout, stderr syncBuffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			if err := pollTCP(addr, time.Now().Add(10*time.Second)); err != nil {
				t.Fatalf("serve did not start: %v; stderr=%s", err, stderr.String())
			}

			resp := postMCP(t, "http://"+addr, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"blocked_tool","arguments":{}}}`)

			if n := upstreamCalls.Load(); n != 1 {
				t.Fatalf("upstream calls = %d, want 1; response=%s", n, resp)
			}
			var body struct {
				Result struct {
					Meta struct {
						Governance struct {
							Action string `json:"action"`
							DryRun bool   `json:"dry_run"`
						} `json:"governance"`
					} `json:"_meta"`
				} `json:"result"`
			}
			if err := json.Unmarshal(resp, &body); err != nil {
				t.Fatalf("response %s: %v", resp, err)
			}
			if g := body.Result.Meta.Governance; g.Action != "allow" || !g.DryRun {
				t.Fatalf("_meta.governance = %+v, want allow with dry_run=true; response=%s", g, resp)
			}

			batch, err := http.Post("http://"+addr, "application/json", strings.NewReader(`[{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"blocked_tool","arguments":{}}}]`))
			if err != nil {
				t.Fatal(err)
			}
			_ = batch.Body.Close()
			if got := batch.Header.Get("X-Governance-Dry-Run"); got != "true" {
				t.Fatalf("batch X-Governance-Dry-Run = %q, want true", got)
			}

			if !auditHas(stdout.String(), "deny", true) {
				t.Fatalf("no audit line with action=deny dry_run=true; stdout=%s", stdout.String())
			}
		})
	}
}

func auditHas(log, action string, dryRun bool) bool {
	sc := bufio.NewScanner(strings.NewReader(log))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var line struct {
			Action string `json:"action"`
			DryRun bool   `json:"dry_run"`
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.Action == action && line.DryRun == dryRun {
			return true
		}
	}
	return false
}
