package serve_boot_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// serve writes one signed decision record per decision by default, and
// `boundary verify-record` accepts each of them: allow, warn, deny and
// check_indeterminate. --record-dir moves them.
func TestServe_EveryDecisionIsAVerifiableRecord(t *testing.T) {
	bin := buildBoundary(t)
	t.Run("default dir", func(t *testing.T) {
		workDir := t.TempDir()
		assertVerifiableRecords(t, bin, workDir, nil, filepath.Join(workDir, ".boundary", "serve"), filepath.Join(".boundary", "serve", "decision-records.jsonl"))
	})
	t.Run("record-dir override", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "records")
		assertVerifiableRecords(t, bin, t.TempDir(), []string{"--record-dir", dir}, dir, filepath.Join(dir, "decision-records.jsonl"))
	})
}

func assertVerifiableRecords(t *testing.T, bin, workDir string, extraArgs []string, recordDir, wantLog string) {
	t.Helper()
	policyDir := t.TempDir()
	writeDenyPolicy(t, policyDir)
	warn := "name: boot-test-warn\nversion: \"1\"\nrules:\n  - name: warn-tool\n    tool: warn_tool\n    action: warn\n    reason: boot_test_warn\n"
	if err := os.WriteFile(filepath.Join(policyDir, "warn.yaml"), []byte(warn), 0o600); err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	seedFile := filepath.Join(t.TempDir(), "receipt.seed")
	if err := os.WriteFile(seedFile, []byte(hex.EncodeToString(seed)), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKey := hex.EncodeToString(ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey))
	addr := freePort(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":"stub-ok"}}`))
	}))
	defer upstream.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	// Kernel trust against a closed port: a call with an agent id cannot be
	// checked and is check_indeterminate; calls without one skip trust.
	args := append([]string{"serve", "--listen", addr, "--policies", policyDir, "--upstream", upstream.URL,
		"--receipt-seed", seedFile, "--trust-mode", "kernel", "--trust-redis-url", "redis://127.0.0.1:1"}, extraArgs...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = workDir
	var stderr syncBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err := pollTCP(addr, time.Now().Add(10*time.Second)); err != nil {
		t.Fatalf("serve did not start: %v; stderr=%s", err, stderr.String())
	}

	base := "http://" + addr
	call := func(tool, agent string) {
		req, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+tool+`","arguments":{}}}`))
		req.Header.Set("Content-Type", "application/json")
		if agent != "" {
			req.Header.Set("X-Governance-Agent-ID", agent)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
	}
	call("allowed_tool", "")
	call("warn_tool", "")
	call("blocked_tool", "")
	call("allowed_tool", "agent-a")

	files, _ := filepath.Glob(filepath.Join(recordDir, "records", "*.json"))
	var actions []string
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var rec struct {
			Action string `json:"action"`
		}
		if err := json.Unmarshal(body, &rec); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		actions = append(actions, rec.Action)
		out, err := exec.Command(bin, "verify-record", "--policies", policyDir, "--verify-signature", "--public-key", publicKey, f).CombinedOutput()
		if err != nil {
			t.Fatalf("verify-record %s (%s): %v\n%s", filepath.Base(f), rec.Action, err, out)
		}
	}
	sort.Strings(actions)
	if got, want := strings.Join(actions, ","), "allow,check_indeterminate,deny,warn"; got != want {
		t.Fatalf("record actions = %s, want %s; stderr=%s", got, want, stderr.String())
	}
	if !strings.Contains(stderr.String(), "decision record log: "+wantLog) {
		t.Fatalf("startup did not name the record log; stderr=%s", stderr.String())
	}
}

// --no-records and server.records: false write no decision records.
func TestServe_RecordsCanBeTurnedOff(t *testing.T) {
	bin := buildBoundary(t)
	for _, tc := range []struct {
		name string
		args func(t *testing.T, policyDir, addr, upstream string) []string
	}{
		{"flag", func(_ *testing.T, policyDir, addr, upstream string) []string {
			return []string{"serve", "--no-records", "--listen", addr, "--policies", policyDir, "--upstream", upstream}
		}},
		{"config", func(t *testing.T, policyDir, addr, upstream string) []string {
			cfg := "mode: standalone\nserver:\n  listen: " + strconv.Quote(addr) + "\n  upstream: " + strconv.Quote(upstream) + "\n  records: false\nstandalone:\n  policy_dir: " + strconv.Quote(policyDir) + "\n"
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
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
			}))
			defer upstream.Close()

			workDir := t.TempDir()
			cmd := exec.Command(bin, tc.args(t, policyDir, addr, upstream.URL)...)
			cmd.Dir = workDir
			var stderr syncBuffer
			cmd.Stderr = &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
			if err := pollTCP(addr, time.Now().Add(10*time.Second)); err != nil {
				t.Fatalf("serve did not start: %v; stderr=%s", err, stderr.String())
			}
			postMCP(t, "http://"+addr, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"blocked_tool","arguments":{}}}`)

			if _, err := os.Stat(filepath.Join(workDir, ".boundary")); !os.IsNotExist(err) {
				t.Fatalf("records written with records off (stat err=%v); stderr=%s", err, stderr.String())
			}
		})
	}
}
