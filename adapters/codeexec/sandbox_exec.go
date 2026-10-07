package codeexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/google/uuid"
)

// SandboxType selects the named runtime boundary used for execution.
type SandboxType string

const (
	// SandboxTypeContainer runs code inside a hardened OCI container managed
	// by the local container runtime (docker or podman).
	SandboxTypeContainer SandboxType = "container"
	// SandboxTypeLocal runs code as a plain local subprocess. It is
	// explicitly non-production: policy-gated execution, not isolation.
	SandboxTypeLocal SandboxType = "local"
)

// Errors returned by NewSandbox for invalid sandbox configuration.
var (
	// ErrLocalSandboxNonProduction rejects the local sandbox in production
	// configuration: a local subprocess is not a named isolation boundary.
	ErrLocalSandboxNonProduction = errors.New("codeexec: local sandbox is explicitly non-production")
	// ErrSandboxImageRequired rejects a container sandbox with no image.
	ErrSandboxImageRequired = errors.New("codeexec: container sandbox requires an image reference")
	// ErrContainerRuntimeUnavailable is returned when neither docker nor
	// podman (nor a configured runtime binary) can be resolved.
	ErrContainerRuntimeUnavailable = errors.New("codeexec: no container runtime (docker/podman) available")
)

// DefaultSandboxTimeout bounds one sandboxed execution when the caller does
// not set SandboxConfig.Timeout.
const DefaultSandboxTimeout = 30 * time.Second

// SandboxConfig configures a named sandbox execution boundary.
type SandboxConfig struct {
	// Type selects the boundary implementation: "container" or "local".
	Type SandboxType
	// Production must be true for production deployments. Type "local"
	// refuses Production=true: a local subprocess is not an isolation
	// boundary.
	Production bool
	// Image is the OCI image reference the container sandbox runs
	// (for example "python:3.11-slim"). Required for Type "container".
	Image string
	// Runtime optionally pins the container runtime binary name or path
	// ("docker", "podman", or an absolute path). Empty auto-detects docker,
	// then podman. Intended for deployments that standardize on one runtime.
	Runtime string
	// Timeout is the wall-clock bound for one execution, including image
	// resolution and container startup. Default: DefaultSandboxTimeout.
	Timeout time.Duration
	// MaxOutputBytes caps captured stdout and stderr. Default: the package
	// output ceiling used for inspection (50 KB).
	MaxOutputBytes int64
	// HostInputDir, when set with MountDestDir, is the only host path mounted
	// into the container, always read-only.
	HostInputDir string
	// MountDestDir is the in-container mount point for HostInputDir.
	MountDestDir string
}

func (c SandboxConfig) normalize() SandboxConfig {
	if c.Timeout <= 0 {
		c.Timeout = DefaultSandboxTimeout
	}
	if c.MaxOutputBytes <= 0 {
		c.MaxOutputBytes = maxSafeOutputSize
	}
	return c
}

// Sandbox is a named runtime isolation boundary for governed code. It
// implements the Executor contract and describes the boundary it provides.
type Sandbox interface {
	Executor
	// Boundary names the isolation boundary; adapters stamp it onto response
	// metadata so operators can verify what executed the request.
	Boundary() ExecutionBoundary
}

// NewSandbox constructs a Sandbox from cfg. It returns an error for unknown
// types, a container config without an image, or a local sandbox configured
// for production use.
func NewSandbox(cfg SandboxConfig) (Sandbox, error) {
	switch cfg.Type {
	case SandboxTypeContainer:
		if strings.TrimSpace(cfg.Image) == "" {
			return nil, ErrSandboxImageRequired
		}
		return &containerSandbox{cfg: cfg.normalize()}, nil
	case SandboxTypeLocal:
		if cfg.Production {
			return nil, ErrLocalSandboxNonProduction
		}
		return &localSandbox{cfg: cfg.normalize()}, nil
	default:
		return nil, fmt.Errorf("codeexec: unknown sandbox type %q", cfg.Type)
	}
}

// DetectContainerRuntime resolves the container runtime binary: the docker or
// podman CLI on PATH. It returns ErrContainerRuntimeUnavailable when neither
// exists.
func DetectContainerRuntime() (string, error) {
	return resolveRuntime("")
}

func resolveRuntime(configured string) (string, error) {
	if configured != "" {
		if path, err := exec.LookPath(configured); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("%w: configured runtime %q not found", ErrContainerRuntimeUnavailable, configured)
	}
	for _, name := range []string{"docker", "podman"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", ErrContainerRuntimeUnavailable
}

// interpreterArgs maps a language to its in-boundary interpreter command.
// Languages without a mapping cannot produce a valid execution result.
func interpreterArgs(language, code string) ([]string, error) {
	switch strings.ToLower(language) {
	case "python":
		return []string{"python3", "-c", code}, nil
	case "javascript", "typescript":
		return []string{"node", "-e", code}, nil
	default:
		return nil, fmt.Errorf("codeexec: no interpreter for language %q", language)
	}
}

// containerSandbox executes code inside a hardened OCI container. The
// hardening contract is fixed: no network, read-only root filesystem, a
// size-limited noexec tmpfs for /tmp, non-root UID, all capabilities dropped,
// no-new-privileges, the runtime's default seccomp profile, IPC isolation,
// pids/memory/CPU limits, and no host mounts beyond an explicit read-only
// input directory.
type containerSandbox struct {
	cfg SandboxConfig
}

// Boundary reports the OCI-container execution boundary.
func (s *containerSandbox) Boundary() ExecutionBoundary {
	return ExecutionBoundary{
		Name:          "oci-container",
		Kind:          "container",
		Description:   "Code executes inside a hardened OCI container: no network, read-only root filesystem, size-limited tmpfs /tmp, non-root UID, all capabilities dropped, no-new-privileges, default seccomp profile, pids/memory/CPU limits, wall-clock timeout.",
		SecureSandbox: true,
	}
}

// containerRunArgs builds the hardened `run` argument set. The flags are the
// security contract — do not relax them without a documented reason and test
// coverage for the relaxed property.
func (s *containerSandbox) containerRunArgs(name string, entrypoint []string) []string {
	args := []string{
		"run",
		"--rm",
		"--name", name,
		"--network", "none",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,nodev,size=64m",
		"--user", "1000:1000",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--ipc", "none",
		"--pids-limit", "64",
		"--memory", "128m",
		"--memory-swap", "128m",
		"--cpus", "1",
	}
	if s.cfg.HostInputDir != "" && s.cfg.MountDestDir != "" {
		args = append(args, "--volume", s.cfg.HostInputDir+":"+s.cfg.MountDestDir+":ro")
	}
	args = append(args, s.cfg.Image)
	args = append(args, entrypoint...)
	return args
}

// Execute runs the request inside a fresh hardened container. Failures that
// prevent a valid execution result — missing runtime, unresolvable image,
// container start failure or start timeout — return a fail-closed deny
// envelope carrying a CHECK_INDETERMINATE-compatible classification (ADR-047;
// the pipeline-level CHECK_INDETERMINATE type is tracked separately under
// FUL-464). Only a completed run produces a normal tool response.
func (s *containerSandbox) Execute(ctx context.Context, req *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	if req == nil || strings.TrimSpace(req.Code) == "" {
		return sandboxDeniedResponse(req, "sandbox_config", "malformed execution request"), nil
	}
	entrypoint, err := interpreterArgs(req.Language, req.Code)
	if err != nil {
		return sandboxDeniedResponse(req, "sandbox_config", err.Error()), nil
	}
	runtime, err := resolveRuntime(s.cfg.Runtime)
	if err != nil {
		return sandboxDeniedResponse(req, "sandbox_runtime_unavailable", "container runtime unavailable"), nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	name := sandboxContainerName(req)
	// The container is force-removed when Execute returns for any reason.
	// --rm alone cannot be relied on: if the CLI is killed at the timeout the
	// daemon-side container keeps running.
	defer removeContainer(runtime, name)

	var stdout, stderr cappedBuffer
	stdout.limit = s.cfg.MaxOutputBytes
	stderr.limit = s.cfg.MaxOutputBytes

	start := time.Now()
	cmd := exec.CommandContext(ctx, runtime, s.containerRunArgs(name, entrypoint)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	timedOut := ctx.Err() != nil
	resp := &governance.ToolResponse{
		Content:     stdout.buf.Bytes(),
		ContentType: "text/plain",
		Duration:    time.Since(start),
		Truncated:   stdout.truncated,
		Metadata: map[string]string{
			"stderr":  stderr.buf.String(),
			"timeout": strconv.FormatBool(timedOut),
		},
	}
	if stderr.truncated {
		resp.Metadata["stderr_truncated"] = "true"
	}

	switch {
	case timedOut:
		// Distinguish "container never started" (a sandbox start failure —
		// the result is indeterminate and must deny) from "started but ran
		// past the deadline" (an ordinary execution timeout).
		if !containerExists(runtime, name) {
			return sandboxDeniedResponse(req, "sandbox_start_failure",
				"sandbox did not start within the execution timeout"), nil
		}
		resp.ExitCode = 124
		return resp, nil
	case runErr == nil:
		resp.ExitCode = 0
		return resp, nil
	default:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			// Docker and podman exit 125 when `run` itself fails: image pull
			// or create errors, daemon failures, invalid flags. That is a
			// sandbox start failure — fail closed rather than report it as a
			// user-code result.
			if exitErr.ExitCode() == 125 {
				return sandboxDeniedResponse(req, "sandbox_start_failure",
					"container failed to start: "+tailLine(stderr.buf.String())), nil
			}
			resp.ExitCode = exitErr.ExitCode()
			return resp, nil
		}
		// The runtime binary could not be executed at all.
		return sandboxDeniedResponse(req, "sandbox_start_failure",
			"container runtime invocation failed"), nil
	}
}

// containerExists reports whether the named container was ever created. It is
// used after a deadline kill to decide whether user code started executing.
func containerExists(runtime, name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, runtime, "inspect", "--type", "container", name).Run() == nil
}

// removeContainer force-removes the named container, tolerating "no such
// container" from a normal --rm teardown.
func removeContainer(runtime, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, runtime, "rm", "-f", name).Run()
}

// sandboxContainerName derives a deterministic-prefix container name so
// cleanup and deployment audits can find sandbox containers.
func sandboxContainerName(req *governance.GovernanceRequest) string {
	id := ""
	if req != nil {
		id = req.RequestID
	}
	if !validNamePart(id) {
		id = uuid.NewString()
	}
	return "fulcrum-codeexec-" + id
}

// validNamePart reports whether s is safe inside an OCI container name.
func validNamePart(s string) bool {
	if s == "" || len(s) > 80 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' {
			return false
		}
	}
	return true
}

// localSandbox executes code as a local subprocess. It is explicitly
// non-production (NewSandbox refuses it under Production=true): it provides
// no isolation boundary, only a governed execution path for development and
// tests.
type localSandbox struct {
	cfg SandboxConfig
}

// Boundary reports the local-process boundary; SecureSandbox stays false.
func (s *localSandbox) Boundary() ExecutionBoundary {
	return LocalProcessBoundary("local-sandbox")
}

// Execute runs the request with the host interpreter under a wall-clock
// timeout. Configuration failures return a fail-closed deny envelope.
func (s *localSandbox) Execute(ctx context.Context, req *governance.GovernanceRequest) (*governance.ToolResponse, error) {
	if req == nil || strings.TrimSpace(req.Code) == "" {
		return sandboxDeniedResponse(req, "sandbox_config", "malformed execution request"), nil
	}
	entrypoint, err := interpreterArgs(req.Language, req.Code)
	if err != nil {
		return sandboxDeniedResponse(req, "sandbox_config", err.Error()), nil
	}
	interpreter, err := exec.LookPath(entrypoint[0])
	if err != nil {
		return sandboxDeniedResponse(req, "sandbox_runtime_unavailable",
			fmt.Sprintf("local interpreter %q unavailable", entrypoint[0])), nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	var stdout, stderr cappedBuffer
	stdout.limit = s.cfg.MaxOutputBytes
	stderr.limit = s.cfg.MaxOutputBytes

	start := time.Now()
	cmd := exec.CommandContext(ctx, interpreter, entrypoint[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	resp := &governance.ToolResponse{
		Content:     stdout.buf.Bytes(),
		ContentType: "text/plain",
		Duration:    time.Since(start),
		Truncated:   stdout.truncated,
		Metadata: map[string]string{
			"stderr":  stderr.buf.String(),
			"timeout": strconv.FormatBool(ctx.Err() != nil),
		},
	}
	if stderr.truncated {
		resp.Metadata["stderr_truncated"] = "true"
	}
	switch {
	case ctx.Err() != nil:
		resp.ExitCode = 124
	case runErr == nil:
		resp.ExitCode = 0
	default:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			resp.ExitCode = exitErr.ExitCode()
		} else {
			return sandboxDeniedResponse(req, "sandbox_start_failure",
				"local interpreter invocation failed"), nil
		}
	}
	return resp, nil
}

// sandboxDeniedResponse builds the fail-closed deny envelope for a sandbox
// outcome that cannot produce a valid execution result. Per ADR-047 the
// outcome is CHECK_INDETERMINATE: neither an allow nor a substantive policy
// denial, and it must block execution. The deny envelope carries the
// classification plus safe context (request/tenant/agent ids, transport,
// enforcement stage, check class, machine-readable failure category — never
// secrets or raw code) until the pipeline-level CHECK_INDETERMINATE decision
// type lands (FUL-464).
func sandboxDeniedResponse(req *governance.GovernanceRequest, category, detail string) *governance.ToolResponse {
	resp := &governance.ToolResponse{
		Content:     []byte("code execution blocked: " + detail + "\n"),
		ContentType: "text/plain",
		ExitCode:    126,
		Metadata: map[string]string{
			"x-fulcrum-action":  "deny",
			"codeexec_denied":   "true",
			"check_result":      "CHECK_INDETERMINATE",
			"check_class":       "sandbox_executor",
			"failure_category":  category,
			"enforcement_stage": "execution",
		},
	}
	if req != nil {
		if req.RequestID != "" {
			resp.Metadata["x-fulcrum-request-id"] = req.RequestID
		}
		if req.TenantID != "" {
			resp.Metadata["x-fulcrum-tenant-id"] = req.TenantID
		}
		if req.AgentID != "" {
			resp.Metadata["x-fulcrum-agent-id"] = req.AgentID
		}
		resp.Metadata["x-fulcrum-transport"] = string(governance.TransportCodeExec)
	}
	return resp
}

// tailLine returns the last non-empty line of s for short diagnostics.
func tailLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	const maxLen = 300
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}

// cappedBuffer is an io.Writer that stores at most limit bytes and reports
// whether input was truncated. It never fails a write, so a noisy child cannot
// grow memory without bound.
type cappedBuffer struct {
	buf       bytes.Buffer
	limit     int64
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	rem := b.limit - int64(b.buf.Len())
	switch {
	case rem <= 0:
		b.truncated = true
	case int64(n) > rem:
		_, _ = b.buf.Write(p[:rem])
		b.truncated = true
	default:
		_, _ = b.buf.Write(p)
	}
	return n, nil
}
