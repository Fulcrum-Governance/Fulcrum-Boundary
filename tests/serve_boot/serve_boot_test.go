// Package serve_boot_test is a hermetic black-box test that builds the real
// boundary binary, starts `boundary serve` against a minimal deny policy and a
// live HTTP MCP upstream stub, and asserts that a governed deny surfaces as a
// JSON-RPC -32001 error before the upstream is reached.
package serve_boot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// buildBoundary compiles the boundary binary into a temp dir, mirroring the
// convention used by tests/actions/mcp_audit_fixture_test.go.
func buildBoundary(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// tests/serve_boot/ -> repo root (two levels up)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bin := filepath.Join(t.TempDir(), "boundary")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/boundary")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build boundary: %v\n%s", err, string(output))
	}
	return bin
}

// freePort returns a free localhost port by opening a listener, noting its
// address, closing it, and returning the port string ready for --listen.
// There is a brief TOCTOU window but it is acceptable for a loopback test.
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// writeDenyPolicy writes a single-rule static policy YAML that denies the tool
// "blocked_tool" to policyDir.
func writeDenyPolicy(t *testing.T, policyDir string) {
	t.Helper()
	policy := `name: boot-test-deny
version: "1"
rules:
  - name: deny-blocked-tool
    tool: blocked_tool
    action: deny
    reason: boot_test_deny
`
	if err := os.WriteFile(filepath.Join(policyDir, "deny.yaml"), []byte(policy), 0o600); err != nil {
		t.Fatalf("write deny policy: %v", err)
	}
}

// pollTCP retries a TCP connect to addr until it succeeds or deadline is
// exceeded.
func pollTCP(addr string, deadline time.Time) error {
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s to accept connections", addr)
}

// TestServeBoot_DenyBeforeUpstream builds the real binary, starts boundary
// serve with a deny policy and an httptest upstream stub, polls until the
// port accepts connections, then asserts:
//   - POST of a tool call that matches the deny rule returns a JSON-RPC -32001
//     error ("governance denied") without ever reaching the upstream.
//   - POST of an unblocked tool call is forwarded to the upstream and returns
//     the upstream's response.
func TestServeBoot_DenyBeforeUpstream(t *testing.T) {
	bin := buildBoundary(t)
	policyDir := t.TempDir()
	writeDenyPolicy(t, policyDir)
	addr := freePort(t)

	// --- upstream stub -------------------------------------------------
	var upstreamCalls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		id := req["id"]
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  map[string]any{"content": "stub-ok"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	// --- start boundary serve ------------------------------------------
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin,
		"serve",
		"--listen", addr,
		"--policies", policyDir,
		"--upstream", upstream.URL,
	)
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		t.Fatalf("start boundary serve: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// --- poll until accepting ------------------------------------------
	deadline := time.Now().Add(10 * time.Second)
	if err := pollTCP(addr, deadline); err != nil {
		t.Fatalf("boundary serve did not accept connections within 10s; stderr=%s", stderrBuf.String())
	}

	baseURL := "http://" + addr

	// --- denied call ---------------------------------------------------
	deniedBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"blocked_tool","arguments":{}}}`
	deniedResp := postMCP(t, baseURL, deniedBody)

	t.Logf("deny response: %s", deniedResp)

	var deniedObj map[string]any
	if err := json.Unmarshal(deniedResp, &deniedObj); err != nil {
		t.Fatalf("deny response is not valid JSON: %v; body=%s", err, deniedResp)
	}
	errField, hasErr := deniedObj["error"]
	if !hasErr || errField == nil {
		t.Fatalf("expected JSON-RPC error for denied tool, got: %s", deniedResp)
	}
	errMap, ok := errField.(map[string]any)
	if !ok {
		t.Fatalf("error field is not an object: %s", deniedResp)
	}
	code, _ := errMap["code"].(float64)
	msg, _ := errMap["message"].(string)
	if code != -32001 {
		t.Fatalf("expected error code -32001 (governance denied), got %.0f; message=%q body=%s", code, msg, deniedResp)
	}
	if msg != "governance denied" {
		t.Fatalf("expected message %q, got %q; body=%s", "governance denied", msg, deniedResp)
	}
	if upstreamCalls != 0 {
		t.Fatalf("denied request reached upstream %d time(s)", upstreamCalls)
	}
	t.Logf("PASS: deny asserted — code=%.0f message=%q upstream_calls=%d", code, msg, upstreamCalls)

	// --- allowed call --------------------------------------------------
	allowedBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"allowed_tool","arguments":{}}}`
	allowedResp := postMCP(t, baseURL, allowedBody)

	t.Logf("allow response: %s", allowedResp)

	var allowedObj map[string]any
	if err := json.Unmarshal(allowedResp, &allowedObj); err != nil {
		t.Fatalf("allow response is not valid JSON: %v; body=%s", err, allowedResp)
	}
	if _, hasError := allowedObj["error"]; hasError {
		t.Fatalf("allowed tool call returned an error: %s", allowedResp)
	}
	if upstreamCalls != 1 {
		t.Fatalf("allowed request: upstream call count = %d, want 1", upstreamCalls)
	}
	t.Logf("PASS: allow forwarded — upstream_calls=%d", upstreamCalls)

	// Verify the listening banner reached stderr (shows the server booted)
	if !strings.Contains(stderrBuf.String(), "listening on") {
		t.Logf("warn: stderr banner not captured (may be flushed after test); stderr=%s", stderrBuf.String())
	}
}

// postMCP posts a JSON-RPC body to the gateway and returns the response body.
func postMCP(t *testing.T, base, body string) []byte {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", base, err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read response: %v", err)
	}
	return buf.Bytes()
}
