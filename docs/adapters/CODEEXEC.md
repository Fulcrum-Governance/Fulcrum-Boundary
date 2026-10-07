# CodeExec Adapter

Status: preview

The CodeExec adapter governs source-code execution requests before they reach a configured execution boundary. It parses Python, JavaScript, and TypeScript requests, analyzes code for policy-relevant operations, denies unsupported or disallowed behavior before execution, forwards allowed requests only through a configured executor, and attaches governance metadata to the response.

This adapter does not claim secure sandboxing by itself. The default adapter is unconfigured and refuses execution. An embedding runtime must provide a named executor boundary. A local-process executor is policy-gated execution, not a secure sandbox.

## Lifecycle

| Step | Status | Notes |
|---|---|---|
| parse | implemented | Parses code-execution JSON or typed inputs into a `GovernanceRequest`. |
| identify | implemented | Maps `agent_id`, `tenant_id`, generated request ID, language, and sandbox ID into the canonical request. |
| evaluate | delegated | Calls the shared `governance.Pipeline`. |
| deny | implemented | Denied requests and sandbox-policy violations return a CodeExec-shaped denial response and never reach the executor. |
| forward | implemented | Allowed requests are forwarded only through the configured `Executor` boundary. The default executor refuses to run. |
| inspect | implemented | Output size, non-zero exit codes, and sensitive-data patterns are inspected after execution. |
| metadata | implemented | Governance action, request ID, envelope ID, transport, policy/rule metadata, and execution-boundary metadata are attached to responses. |
| record | delegated | The shared pipeline emits a structured decision record for every governance evaluation. |
| bypass_proof | delegated | Deployment topology must make Boundary the only path into the code execution runtime. |
| fail_closed | implemented | Policy pipeline errors deny by default for `code_exec`; missing pipeline or sandbox-policy violations deny before execution. |

## Policy Checks

Boundary analyzes the submitted source before execution and exposes policy signals on the request:

- language: allowed by `SandboxPolicy.AllowedLanguages`
- resource access: required capabilities derived from detected operations
- filesystem behavior: read/write/delete patterns
- network behavior: outbound network patterns
- subprocess behavior: process-spawn and system-call patterns
- obfuscation: base64, dynamic eval/exec, decoded payload, dynamic import, and suspicious encoding/decoding chains

The default sandbox policy allows Python, JavaScript, and TypeScript, allows file reads and environment reads, and denies network, filesystem writes/deletes, subprocesses, restricted imports, eval-like system calls, and obfuscated execution. Unsupported languages or denied capabilities produce a denial response before executor invocation.

## Execution Boundary

`NewAdapterWithExecutor` wires CodeExec to an operator-provided `Executor` and `ExecutionBoundary`. The boundary metadata must name what actually isolates execution.

Boundary ships a `Sandbox` interface (`adapters/codeexec/sandbox_exec.go`) that implements `Executor` and reports its boundary via `Boundary()`. `NewSandbox` constructs it from a `SandboxConfig`:

- `Type: "container"` — the production boundary. Runs each allowed request in a fresh, hardened OCI container on the local container runtime (`docker` or `podman`; `Runtime` may pin one).
- `Type: "local"` — a local subprocess boundary kept for development and tests. `NewSandbox` refuses it when `Production: true`; its `ExecutionBoundary` reports `SecureSandbox: false`.

Only a named isolation boundary — container, WASM runtime, microVM, or OS-level sandbox — may be described as sandboxing when implemented, tested, and documented. A local-process boundary is policy-gated execution, not sandboxing.

## Container Sandbox Contract

The container implementation fixes a hardening profile for every run:

- `--pull never` — the configured image must already exist in the runtime's local image store; a governed execution never pulls
- `--network none` — no inbound or outbound network for the container
- `--read-only` root filesystem, plus a size-limited `noexec,nosuid,nodev` tmpfs at `/tmp` — writes outside `/tmp` fail
- non-root `--user 1000:1000`
- `--cap-drop ALL` and `--security-opt no-new-privileges`
- the runtime's default seccomp profile (not disabled)
- `--ipc none`
- `--pids-limit 64`, `--memory 128m`/`--memory-swap 128m`, `--cpus 1`
- a wall-clock timeout (`SandboxConfig.Timeout`, default 30s) that kills and removes the container
- no host mounts except an optional explicit read-only input directory (`HostInputDir` + `MountDestDir`)

Every container is named `fulcrum-codeexec-<request id>` and force-removed after the run, including when the wall-clock timeout kills the runtime client — a timed-out container is never left running.

**Fail-closed semantics.** When the sandbox cannot produce a valid execution result — container runtime missing, image unresolvable, container start failure, caller cancellation, or the deadline expiring before the container starts — the executor returns a deny envelope (exit code 126, `codeexec_denied=true`) classified `CHECK_INDETERMINATE` with a machine-readable `failure_category` (`sandbox_runtime_unavailable`, `sandbox_start_failure`, `sandbox_config`, `sandbox_canceled`) and safe request context (request/tenant/agent IDs, transport, enforcement stage, check class). This is the ADR-047 indeterminate outcome expressed on the existing deny path; the pipeline-level `CHECK_INDETERMINATE` decision type is tracked under FUL-464. A container that started and then exceeded the timeout is an ordinary execution timeout (`timeout=true`, exit 124), not an indeterminate result; the `timeout` flag is set only on `context.DeadlineExceeded`, so a caller cancellation is never reported as a timeout. A governed-code exit of 125 is reported as an ordinary result: run-level 125s (image or create failures) are told apart by the run `--cidfile` and the runtime CLI's own error output, not by the exit code alone.

**What the sandbox does not protect against.** The container boundary contains accidental breakage and routine host interference. It does not protect against kernel exploits, container-runtime escapes, hardware or side-channel attacks, or vulnerabilities in the runtime itself — the host kernel is shared. Image provenance is an operator responsibility. The sandbox also does not make the governed route unreachable from the outside: see Bypass Model and `docs/deployment/codeexec-bypass-proofing.md`.

**Operator requirements.** The Boundary process needs permission to run containers on the local runtime; the governed agent must not have that access. The sandbox runs `docker run`/`podman run` with `--pull never`, so the configured image must already be present in the runtime's local image store — pre-pull or pin it during deployment or every request fails closed as a start failure. Keep host mounts limited to the explicit read-only input directory.

## Bypass Model

Boundary governs code execution only when code enters through the CodeExec adapter. Direct host execution, notebook kernels, CI scripts, shell access, direct container-runtime access, or any path into the execution runtime that does not pass through Boundary is outside this adapter.

The direct-execution bypass test documents this limitation by performing a host write without invoking Boundary. That is an honest deployment boundary: production use requires topology evidence that the governed executor is the sole code-execution path available to the agent. See `docs/deployment/codeexec-bypass-proofing.md` for the reference-configuration controls.

## Production Gate

CodeExec stays preview until the remaining readiness gap is closed: the named sandbox boundary is implemented and covered by runtime-gated integration tests (`adapters/codeexec/sandbox_integration_test.go`), and deployment bypass evidence must still show that direct runtime access is unavailable to the governed agent — see the bypass-proofing note. Gap BND-CODE-001 remains recorded in `readiness.yaml`; status changes are consolidated separately.
