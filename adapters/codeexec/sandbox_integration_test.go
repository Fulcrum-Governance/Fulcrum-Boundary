package codeexec_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/codeexec"
	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// sandboxIntegrationImage is the image the container sandbox runs in these
// tests. It is pre-pulled by the CI job and by ensureSandboxImage so per-test
// wall-clock timeouts measure execution, not pull time.
const sandboxIntegrationImage = "python:3.11-slim"

// requireSandboxRuntime returns a usable OCI container runtime binary
// (docker or podman). It skips with a clear message when no runtime exists —
// except in CI (CI=true), where a missing runtime is a hard failure: these
// tests are the production-boundary evidence and must actually run there.
func requireSandboxRuntime(t *testing.T) string {
	t.Helper()
	runtime, err := codeexec.DetectContainerRuntime()
	if err == nil {
		if cmdErr := exec.Command(runtime, "info").Run(); cmdErr == nil {
			return runtime
		} else {
			err = cmdErr
		}
	}
	if os.Getenv("CI") == "true" {
		t.Fatalf("container runtime unavailable in CI (want docker or podman daemon): %v", err)
	}
	t.Skipf("skipping sandbox integration test: no usable container runtime (docker/podman): %v", err)
	return ""
}

var (
	pullOnce  sync.Once
	pullError error
)

// ensureSandboxImage pulls the test image once per test binary so that the
// per-execution timeouts in the tests below cover execution, not image pulls.
func ensureSandboxImage(t *testing.T, runtime string) {
	t.Helper()
	pullOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if exec.CommandContext(ctx, runtime, "image", "inspect", sandboxIntegrationImage).Run() == nil {
			return
		}
		pullError = exec.CommandContext(ctx, runtime, "pull", sandboxIntegrationImage).Run()
	})
	if pullError != nil {
		t.Fatalf("failed to pull sandbox image %q: %v", sandboxIntegrationImage, pullError)
	}
}

func newContainerSandbox(t *testing.T, timeout time.Duration) codeexec.Sandbox {
	t.Helper()
	s, err := codeexec.NewSandbox(codeexec.SandboxConfig{
		Type:       codeexec.SandboxTypeContainer,
		Production: true,
		Image:      sandboxIntegrationImage,
		Timeout:    timeout,
	})
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	return s
}

// countSandboxContainers reports how many containers with the codeexec
// sandbox name prefix currently exist on the runtime, created or not.
func countSandboxContainers(t *testing.T, runtime string) int {
	t.Helper()
	out, err := exec.Command(runtime, "ps", "-aq", "--filter", "name=fulcrum-codeexec-").Output()
	if err != nil {
		t.Fatalf("listing sandbox containers: %v", err)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func pythonRequest(code string) *governance.GovernanceRequest {
	return &governance.GovernanceRequest{
		RequestID: "req-it",
		Transport: governance.TransportCodeExec,
		ToolName:  "code_exec",
		AgentID:   "agent-it",
		TenantID:  "tenant-it",
		Language:  "python",
		Code:      code,
	}
}

// A governance-denied request must never start a container: the Boundary
// decision runs before the sandbox runner is invoked.
func TestSandboxIntegration_DeniedCodeNeverStartsContainer(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 15*time.Second)
	adapter := codeexec.NewAdapterWithExecutor("tenant-it", sandbox, sandbox.Boundary())
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: []governance.StaticPolicyRule{{
			Name:   "deny-code-exec",
			Tool:   "code_exec",
			Action: "deny",
			Reason: "code execution blocked by static policy",
		}},
	}, nil, nil, &collectingAuditPublisher{})

	before := countSandboxContainers(t, runtime)
	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "with open('/tmp/should_not_exist', 'w') as f: f.write('x')",
		Language: "python",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if resp.ExitCode != 126 || resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("expected denied CodeExec response, got %+v", resp)
	}
	if after := countSandboxContainers(t, runtime); after != before {
		t.Fatalf("denied request changed sandbox container count: before=%d after=%d", before, after)
	}
}

// An allowed request runs inside the container and returns its output.
func TestSandboxIntegration_AllowedCodeRunsAndReturnsOutput(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 20*time.Second)
	adapter := codeexec.NewAdapterWithExecutor("tenant-it", sandbox, sandbox.Boundary())
	pipeline := governance.NewPipeline(governance.PipelineConfig{}, nil, nil, &collectingAuditPublisher{})

	resp, err := adapter.GovernCode(context.Background(), codeexec.CodeExecInput{
		Code:     "print('hello from governed sandbox')",
		Language: "python",
		AgentID:  "agent-it",
	}, pipeline)
	if err != nil {
		t.Fatalf("GovernCode: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", resp.ExitCode, resp.Metadata["stderr"])
	}
	if !strings.Contains(string(resp.Content), "hello from governed sandbox") {
		t.Fatalf("expected sandbox output, got %q", string(resp.Content))
	}
	if resp.Metadata["x-fulcrum-action"] != "allow" {
		t.Fatalf("governance metadata missing: %+v", resp.Metadata)
	}
	if resp.Metadata["codeexec_boundary_kind"] != "container" || resp.Metadata["codeexec_secure_sandbox"] != "true" {
		t.Fatalf("container boundary metadata missing: %+v", resp.Metadata)
	}
}

// The sandbox has no network: outbound connections must fail inside it.
func TestSandboxIntegration_NetworkAccessFails(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 20*time.Second)
	resp, err := sandbox.Execute(context.Background(), pythonRequest(`
import urllib.request
try:
    urllib.request.urlopen("http://example.com", timeout=3)
    print("network_ok")
except Exception:
    print("network_blocked")
`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := string(resp.Content) + resp.Metadata["stderr"]
	if strings.Contains(out, "network_ok") || !strings.Contains(out, "network_blocked") {
		t.Fatalf("expected outbound network to be blocked, got stdout=%q stderr=%q", string(resp.Content), resp.Metadata["stderr"])
	}
}

// The root filesystem is read-only and the process is non-root: writes outside
// /tmp must fail; writes inside the tmpfs /tmp must succeed.
func TestSandboxIntegration_WriteOutsideTmpFails(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 20*time.Second)
	resp, err := sandbox.Execute(context.Background(), pythonRequest(`
try:
    with open("/etc/pwned", "w") as f:
        f.write("x")
    print("etc_write_ok")
except (PermissionError, OSError):
    print("etc_write_blocked")
try:
    with open("/tmp/allowed", "w") as f:
        f.write("x")
    print("tmp_write_ok")
except OSError:
    print("tmp_write_blocked")
`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := string(resp.Content)
	if !strings.Contains(out, "etc_write_blocked") {
		t.Fatalf("expected write outside /tmp to be blocked, got %q (stderr=%q)", out, resp.Metadata["stderr"])
	}
	if !strings.Contains(out, "tmp_write_ok") {
		t.Fatalf("expected /tmp tmpfs write to succeed, got %q (stderr=%q)", out, resp.Metadata["stderr"])
	}
}

// Nothing from the host filesystem is mounted: host paths must not resolve
// inside the sandbox.
func TestSandboxIntegration_ReadingHostPathsFails(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	hostDir := t.TempDir()
	secretPath := filepath.Join(hostDir, "host-secret.txt")
	if err := os.WriteFile(secretPath, []byte("host_secret_value"), 0o600); err != nil {
		t.Fatalf("writing host marker: %v", err)
	}

	sandbox := newContainerSandbox(t, 20*time.Second)
	resp, err := sandbox.Execute(context.Background(), pythonRequest(`
try:
    with open("`+secretPath+`", "r") as f:
        print("host_read:" + f.read())
except FileNotFoundError:
    print("host_path_absent")
except Exception:
    print("host_read_blocked")
`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := string(resp.Content)
	if strings.Contains(out, "host_secret_value") {
		t.Fatalf("sandbox read a host-only file: %q", out)
	}
	if !strings.Contains(out, "host_path_absent") && !strings.Contains(out, "host_read_blocked") {
		t.Fatalf("expected host path to be unreadable, got %q (stderr=%q)", out, resp.Metadata["stderr"])
	}
}

// The pids limit contains a fork bomb: the container cannot spawn unbounded
// processes and the run terminates with a failure.
func TestSandboxIntegration_ForkBombContainedByPidsLimit(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 20*time.Second)
	resp, err := sandbox.Execute(context.Background(), pythonRequest(`
import os
try:
    while True:
        os.fork()
except OSError:
    print("fork_contained")
`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := string(resp.Content) + resp.Metadata["stderr"]
	if !strings.Contains(out, "fork_contained") {
		t.Fatalf("expected fork bomb to hit the pids limit, got stdout=%q stderr=%q", string(resp.Content), resp.Metadata["stderr"])
	}
	if n := countSandboxContainers(t, runtime); n != 0 {
		t.Fatalf("fork bomb left %d sandbox containers behind", n)
	}
}

// An infinite loop is killed when the wall-clock timeout expires, and the
// container is removed rather than left running.
func TestSandboxIntegration_InfiniteLoopKilledAtTimeout(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 3*time.Second)
	start := time.Now()
	resp, err := sandbox.Execute(context.Background(), pythonRequest(`
import time
while True:
    time.sleep(0.1)
`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Metadata["timeout"] != "true" {
		t.Fatalf("expected timeout=true, got %+v", resp.Metadata)
	}
	if resp.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code on timeout, got %+v", resp)
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("timeout kill took too long: %v", elapsed)
	}
	if n := countSandboxContainers(t, runtime); n != 0 {
		t.Fatalf("timed-out execution left %d sandbox containers behind", n)
	}
}

// An image that cannot be resolved is a sandbox start failure: the result is
// a fail-closed deny envelope classified CHECK_INDETERMINATE (ADR-047), never
// a partial execution.
func TestSandboxIntegration_MissingImageFailsClosed(t *testing.T) {
	requireSandboxRuntime(t)

	sandbox, err := codeexec.NewSandbox(codeexec.SandboxConfig{
		Type:       codeexec.SandboxTypeContainer,
		Production: true,
		Image:      "fulcrum-definitely-missing-image:latest",
		Timeout:    30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	resp, err := sandbox.Execute(context.Background(), pythonRequest("print('never runs')"))
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	if resp.ExitCode != 126 || resp.Metadata["check_result"] != "CHECK_INDETERMINATE" {
		t.Fatalf("expected CHECK_INDETERMINATE deny envelope, got %+v", resp)
	}
	if resp.Metadata["failure_category"] != "sandbox_start_failure" {
		t.Fatalf("failure_category = %q, want sandbox_start_failure", resp.Metadata["failure_category"])
	}
	if resp.Metadata["x-fulcrum-action"] != "deny" || resp.Metadata["codeexec_denied"] != "true" {
		t.Fatalf("deny markers missing: %+v", resp.Metadata)
	}
}

// Governed code may itself exit 125 — the same code the runtime CLI reports
// for a failed `run`. The classification must come from runtime-owned
// evidence (the --cidfile plus the CLI's own error output), not the exit
// code: a user-code 125 is an ordinary execution result, never a deny
// envelope.
func TestSandboxIntegration_UserCodeExit125IsNotStartFailure(t *testing.T) {
	runtime := requireSandboxRuntime(t)
	ensureSandboxImage(t, runtime)

	sandbox := newContainerSandbox(t, 20*time.Second)
	resp, err := sandbox.Execute(context.Background(), pythonRequest("import sys\nsys.exit(125)"))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Metadata["codeexec_denied"] == "true" {
		t.Fatalf("user-code exit 125 must not be denied as a start failure: %+v", resp)
	}
	if resp.ExitCode != 125 {
		t.Fatalf("expected user-code exit 125, got %+v", resp)
	}
	if resp.Metadata["timeout"] != "false" {
		t.Fatalf("unexpected timeout flag: %+v", resp.Metadata)
	}
}

// A wall-clock deadline hit before the container exists is a start failure:
// nothing ran, so the outcome must be fail-closed indeterminate rather than a
// normal execution timeout.
func TestSandboxIntegration_StartTimeoutFailsClosed(t *testing.T) {
	requireSandboxRuntime(t)

	sandbox := newContainerSandbox(t, time.Nanosecond)
	resp, err := sandbox.Execute(context.Background(), pythonRequest("print('never runs')"))
	if err != nil {
		t.Fatalf("Execute returned error instead of fail-closed deny envelope: %v", err)
	}
	if resp.ExitCode != 126 || resp.Metadata["check_result"] != "CHECK_INDETERMINATE" {
		t.Fatalf("expected CHECK_INDETERMINATE deny envelope on start timeout, got %+v", resp)
	}
	if resp.Metadata["failure_category"] != "sandbox_start_failure" {
		t.Fatalf("failure_category = %q, want sandbox_start_failure", resp.Metadata["failure_category"])
	}
}

type collectingAuditPublisher struct {
	mu     sync.Mutex
	events []governance.AuditEvent
}

func (p *collectingAuditPublisher) Publish(_ context.Context, event governance.AuditEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *collectingAuditPublisher) Events() []governance.AuditEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]governance.AuditEvent(nil), p.events...)
}
