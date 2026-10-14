package codeexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
	// ErrSandboxMountConfig rejects a container sandbox whose input mount is
	// misconfigured: a bad source can become a named volume instead of a
	// host bind, and a bad destination can shadow sandbox controls or the
	// interpreter.
	ErrSandboxMountConfig = errors.New("codeexec: invalid sandbox mount configuration")
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
	// HostInputDir, when set with MountDestDir, is the only host path the
	// adapter mounts into the container, always as a read-only bind. It must
	// be an absolute, clean host path free of ':' , ',' and control
	// characters.
	HostInputDir string
	// MountDestDir is the in-container mount point for HostInputDir. It must
	// be an absolute, clean container path that is not "/" and does not
	// equal, contain, or sit inside a protected system directory (/tmp,
	// /usr, /bin, /lib, /etc, /proc, /sys, /dev, /opt — including the
	// directories the interpreter resolves from). A fixed destination such
	// as /input is recommended.
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
		if err := validateMountConfig(cfg.HostInputDir, cfg.MountDestDir); err != nil {
			return nil, err
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

// protectedMountDestDirs are container paths the input mount must never
// shadow: the tmpfs scratch dir, system trees, and the directories the
// interpreter resolves from. A destination equal to, above, or below any of
// these can override the sandbox's own controls or the interpreter binary.
var protectedMountDestDirs = []string{
	"/tmp", "/usr", "/bin", "/lib", "/etc", "/proc", "/sys", "/dev", "/opt",
}

// validateMountConfig fails closed on a partially or unsafely configured
// input mount. HostInputDir and MountDestDir must be set together or not at
// all. Both must be absolute, clean paths free of ':' , ',' and control
// characters — a ':' or ',' inside a --mount/--volume spec splits into extra
// fields, and a non-absolute source is interpreted by the runtime as a named
// volume rather than a host path. The destination must not be "/" or
// overlap a protectedMountDestDirs entry.
func validateMountConfig(hostDir, destDir string) error {
	if (hostDir == "") != (destDir == "") {
		return fmt.Errorf("%w: HostInputDir and MountDestDir must be set together or not at all", ErrSandboxMountConfig)
	}
	if hostDir == "" {
		return nil
	}
	if err := validHostInputDir(hostDir); err != nil {
		return err
	}
	return validMountDestDir(destDir)
}

// validHostInputDir validates the host side of the input bind. Host paths
// use filepath semantics.
func validHostInputDir(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("%w: HostInputDir %q must be an absolute, clean path", ErrSandboxMountConfig, dir)
	}
	if strings.ContainsAny(dir, ":,") || hasControlChars(dir) {
		return fmt.Errorf("%w: HostInputDir %q contains ':' ',' or a control character", ErrSandboxMountConfig, dir)
	}
	return nil
}

// validMountDestDir validates the container side of the input bind.
// Container paths are POSIX, so path (not filepath) semantics apply.
func validMountDestDir(dir string) error {
	if !path.IsAbs(dir) || path.Clean(dir) != dir {
		return fmt.Errorf("%w: MountDestDir %q must be an absolute, clean container path", ErrSandboxMountConfig, dir)
	}
	if strings.ContainsAny(dir, ":,") || hasControlChars(dir) {
		return fmt.Errorf("%w: MountDestDir %q contains ':' ',' or a control character", ErrSandboxMountConfig, dir)
	}
	if dir == "/" {
		return fmt.Errorf("%w: MountDestDir must not be the filesystem root", ErrSandboxMountConfig)
	}
	for _, protected := range protectedMountDestDirs {
		if pathWithin(dir, protected) || pathWithin(protected, dir) {
			return fmt.Errorf("%w: MountDestDir %q must not equal, contain, or sit inside protected path %q", ErrSandboxMountConfig, dir, protected)
		}
	}
	return nil
}

// pathWithin reports whether p equals dir or is a descendant of dir. Both
// arguments must be absolute, clean POSIX paths.
func pathWithin(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}

// hasControlChars reports whether s contains a byte below 0x20 or DEL.
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// interpreterArgs maps a language to its in-boundary interpreter command.
// Languages without a mapping cannot produce a valid execution result and
// fail closed as a sandbox_config deny. "typescript" is deliberately absent:
// the sandbox has no TypeScript runtime contract — `node -e` does not
// transpile TS syntax, so mapping it would surface a runtime syntax error on
// ordinary input instead of a deterministic deny. Restore the mapping only
// alongside a real TS runtime contract.
func interpreterArgs(language, code string) ([]string, error) {
	switch strings.ToLower(language) {
	case "python":
		return []string{"python3", "-c", code}, nil
	case "javascript":
		return []string{"node", "-e", code}, nil
	default:
		return nil, fmt.Errorf("codeexec: no interpreter for language %q", language)
	}
}

// containerSandbox executes code inside a hardened OCI container. The
// hardening contract is fixed for the argv the adapter supplies: no network,
// read-only root filesystem, a size-limited noexec tmpfs for /tmp, non-root
// UID, all capabilities dropped, no-new-privileges, the runtime's default
// seccomp profile, IPC isolation, pids/memory/CPU limits, and no mount flags
// beyond an optional validated read-only bind of the operator input
// directory. Runtime-level defaults outside the adapter's argv (for example
// containers.conf default mounts) are an operator hardening requirement.
type containerSandbox struct {
	cfg SandboxConfig
}

// Boundary reports the OCI-container execution boundary.
func (s *containerSandbox) Boundary() ExecutionBoundary {
	return ExecutionBoundary{
		Name:          "oci-container",
		Kind:          "container",
		Description:   "Code executes inside a hardened OCI container: no network, read-only root filesystem, size-limited tmpfs /tmp, non-root UID, all capabilities dropped, no-new-privileges, default seccomp profile, pids/memory/CPU limits, wall-clock timeout, and only the validated read-only input bind among adapter-supplied mounts.",
		SecureSandbox: true,
	}
}

// containerRunArgs builds the hardened `run` argument set. The flags are the
// security contract — do not relax them without a documented reason and test
// coverage for the relaxed property. --pull never makes the image contract
// explicit: the configured reference must already exist in the runtime's
// local image store, so a governed execution can never block on (or be
// swapped by) a registry pull. --cidfile records the container ID at create
// time so a run-level exit 125 (container never created) can be told apart
// from governed code exiting 125 (container created and ran).
func (s *containerSandbox) containerRunArgs(name, cidPath string, entrypoint []string) []string {
	args := []string{
		"run",
		"--rm",
		"--pull", "never",
		"--cidfile", cidPath,
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
		// --mount (not --volume): the explicit key=value form is unambiguous
		// once validateMountConfig has rejected ':' , ',' and control
		// characters. Mounts are the only volume mechanism the adapter
		// supplies — argv never contains -v, --volume, or --volumes-from.
		args = append(args, "--mount",
			"type=bind,src="+s.cfg.HostInputDir+",dst="+s.cfg.MountDestDir+",readonly")
	}
	// --entrypoint pins argv[0] to the interpreter. Without it an
	// image-defined ENTRYPOINT would receive the interpreter argv as its own
	// arguments and image code would run instead of the governed snippet.
	args = append(args, "--entrypoint", entrypoint[0])
	args = append(args, s.cfg.Image)
	args = append(args, entrypoint[1:]...)
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
	// The runtime CLI writes the container ID to cidPath at create time and
	// only then; its presence is evidence the container started. On exit 125
	// an absent cidfile plus runtime error output marks a run-level failure
	// (image resolution, create errors, daemon faults), not a governed-code
	// exit — see the classification below. The cidfile also proves
	// ownership for cleanup below.
	cidPath := filepath.Join(os.TempDir(), "fulcrum-codeexec-"+uuid.NewString()+".cid")
	defer func() { _ = os.Remove(cidPath) }()

	// The container this execution created is force-removed when Execute
	// returns for any reason. --rm alone cannot be relied on: if the CLI is
	// killed at the timeout the daemon-side container keeps running.
	// Removal is ownership-scoped so a concurrent execution sharing the
	// RequestID can never have its live container killed by this defer.
	defer func() { removeOwnedContainer(runtime, name, cidPath) }()

	var stdout, stderr cappedBuffer
	stdout.limit = s.cfg.MaxOutputBytes
	stderr.limit = s.cfg.MaxOutputBytes

	start := time.Now()
	// #nosec G204 -- the runtime binary is resolved via exec.LookPath, the
	// container name is constrained by validNamePart, and argv is the fixed
	// hardening flag list plus the configured image and interpreter argv; no
	// shell is invoked.
	cmd := exec.CommandContext(ctx, runtime, s.containerRunArgs(name, cidPath, entrypoint)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	// DeadlineExceeded means the sandbox wall-clock timeout fired;
	// context.Canceled means the caller aborted. Both set ctx.Err(), but only
	// the former is an execution timeout — a caller cancel produces no valid
	// execution result and must fail closed, not report a run.
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
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
	case ctx.Err() != nil:
		// The caller canceled without a deadline having fired. No valid
		// execution result exists — fail closed rather than report a killed
		// run as a completed execution.
		return sandboxDeniedResponse(req, "sandbox_canceled",
			"execution aborted: caller canceled the request context"), nil
	case runErr == nil:
		resp.ExitCode = 0
		return resp, nil
	default:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			// Docker and podman exit 125 when `run` itself fails (image
			// resolution or create errors, daemon failures, invalid flags) —
			// but governed code can also exit 125, so the exit code alone
			// cannot classify the failure. Two runtime-owned signals decide:
			// a written --cidfile (the CLI records it only after a
			// successful create) and runtime-CLI error text on stderr. A 125
			// with no cidfile and a runtime error signature is a sandbox
			// start failure — fail closed rather than report it as a
			// user-code result. A 125 after the container was created, or
			// with only governed-code stderr, is an ordinary exit.
			if exitErr.ExitCode() == 125 && !containerCreated(cidPath) && runtimeRunError(stderr.buf.String()) {
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
	// #nosec G204 -- runtime is resolved via exec.LookPath, argv is a fixed
	// inspect command, and name is constrained by validNamePart; no shell is
	// invoked.
	return exec.CommandContext(ctx, runtime, "inspect", "--type", "container", name).Run() == nil
}

// containerCreated reports whether the runtime CLI recorded a container ID in
// the --cidfile at cidPath. Docker and podman write the file only after a
// successful container create, so an absent or empty file proves the run
// failed before user code could start. Note podman removes the cidfile along
// with an --rm container, so an absent file is necessary but not sufficient
// evidence — runtimeRunError supplies the second signal.
func containerCreated(cidPath string) bool {
	info, err := os.Stat(cidPath)
	return err == nil && info.Size() > 0
}

// runtimeRunError reports whether captured stderr carries a container-runtime
// CLI failure signature rather than governed-code output. Docker prefixes its
// own errors with "docker:" and daemon errors with "Error response from
// daemon"; podman reports "Error: ...". Only stderr written by the CLI itself
// matches — the governed interpreter's output is forwarded verbatim and could
// mimic a signature, which would misclassify toward deny (fail closed).
func runtimeRunError(stderr string) bool {
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(line, "docker:") ||
			strings.HasPrefix(line, "error:") ||
			strings.Contains(line, "error response from daemon") ||
			strings.Contains(line, "unable to find image") ||
			strings.Contains(line, "no such image") {
			return true
		}
	}
	return false
}

// removeOwnedContainer force-removes only the container this execution
// created. It prefers the container ID recorded in the run --cidfile, which
// the runtime CLI writes at create time and which identifies exactly this
// run's container. When no cidfile exists — the container was never created,
// or the runtime already removed it — the unique per-execution name is the
// target; it cannot match another execution's container because the name
// carries a per-execution random suffix.
func removeOwnedContainer(runtime, name, cidPath string) {
	target := name
	// #nosec G304 -- cidPath is generated inside Execute as a --cidfile path
	// under os.TempDir() with a random uuid file name; it is never derived
	// from request input.
	if data, err := os.ReadFile(cidPath); err == nil {
		if cid := strings.TrimSpace(string(data)); cid != "" {
			target = cid
		}
	}
	removeContainer(runtime, target)
}

// removeContainer force-removes the named container, tolerating "no such
// container" from a normal --rm teardown.
func removeContainer(runtime, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// #nosec G204 -- runtime is resolved via exec.LookPath, argv is a fixed rm
	// -f command, and name is constrained by validNamePart; no shell is
	// invoked.
	_ = exec.CommandContext(ctx, runtime, "rm", "-f", name).Run()
}

// sandboxContainerName derives the container name for one execution. The
// validated RequestID stays a deterministic prefix so cleanup and deployment
// audits can find sandbox containers, but the trailing random component is
// mandatory: concurrent executions may share a RequestID (retries, resumed
// runs), and an identical name would let one execution's cleanup force-
// remove the other's live container.
func sandboxContainerName(req *governance.GovernanceRequest) string {
	id := ""
	if req != nil {
		id = req.RequestID
	}
	if !validNamePart(id) {
		id = uuid.NewString()
	}
	return "fulcrum-codeexec-" + id + "-" + uuid.NewString()
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
	// #nosec G204 -- the interpreter is resolved via exec.LookPath from the
	// fixed interpreterArgs map and argv is that map's fixed flag plus the
	// governed code string; no shell is invoked.
	cmd := exec.CommandContext(ctx, interpreter, entrypoint[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	// As in the container executor, only DeadlineExceeded is an execution
	// timeout; a caller cancel aborts the run and must fail closed.
	timedOut := errors.Is(ctx.Err(), context.DeadlineExceeded)
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
		resp.ExitCode = 124
	case ctx.Err() != nil:
		return sandboxDeniedResponse(req, "sandbox_canceled",
			"execution aborted: caller canceled the request context"), nil
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
		if req.EnvelopeID != "" {
			resp.Metadata["x-fulcrum-envelope-id"] = req.EnvelopeID
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
