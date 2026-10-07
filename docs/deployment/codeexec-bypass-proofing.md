# CodeExec Bypass Proofing

The CodeExec adapter evaluates a governed code-execution request before it
reaches a runtime, and the `container` sandbox executes allowed code inside a
hardened OCI container. That boundary only holds for routes forced through
it: any path that lets the agent run code without crossing Boundary is a
bypass. This note describes the reference configuration and the controls that
close the bypass paths the tests cannot.

## Reference Configuration

- `codeexec.NewSandbox(SandboxConfig{Type: SandboxTypeContainer, Production: true, Image: <pinned>, ...})`
  wired through `codeexec.NewAdapterWithExecutor`.
- Local docker or podman runtime reachable only by the Boundary process.
- The sandbox hardening profile is fixed by the implementation: `--network
  none`, read-only root filesystem, size-limited `noexec` tmpfs `/tmp`,
  non-root UID, all capabilities dropped, `no-new-privileges`, default
  seccomp profile, `--ipc none`, pids/memory/CPU limits, wall-clock
  timeout, and no host mounts beyond an explicit read-only input directory.

## Required Controls

| Control | Requirement |
|---|---|
| Runtime socket custody | The governed agent must not reach `docker.sock`, the podman socket, or any container runtime API. Direct runtime access is a total bypass of both governance and sandboxing. |
| Sole execution path | No other tool exposed to the agent may execute code outside the governed CodeExec route — no shell/terminal tool, notebook kernel, or second executor. |
| Interpreter custody | Host interpreters (`python3`, `node`, …) must not be directly invocable by the agent. The `local` sandbox type is non-production and refused when `Production: true`. |
| Image pinning | Deployments should pin the image by digest and pre-pull it. The runtime pull path is fail-closed (an unresolvable image denies execution), but pinning removes ambiguity about what ran. |
| Mount surface | Only the operator-declared read-only input directory may be mounted. Do not extend the mount list without updating this contract and the integration tests. |
| Egress containment | `--network none` covers the container. It does not restrict other processes on the host; do not treat it as host egress policy. |

## Verification

`adapters/codeexec/sandbox_integration_test.go` exercises the reference
boundary against a real runtime: denied code never starts a container;
allowed code runs and returns output; network access fails; writes outside
`/tmp` fail while `/tmp` works; host paths are unreadable; a fork bomb is
contained by the pids limit; an infinite loop is killed at the timeout; and
runtime/image/start failures return a `CHECK_INDETERMINATE` deny envelope
(ADR-047 fail-closed semantics on the existing deny path; the pipeline-level
type is tracked under FUL-464). The tests run in CI on ubuntu-latest and skip
with a clear message where no container runtime exists — except under
`CI=true`, where a missing runtime fails.

These tests prove the governed route for the reference configuration. They do
not prove that a given deployment has removed the bypass paths above — that
evidence is deployment-specific and stays delegated to topology controls
(`bypass_proof: delegated` in `readiness.yaml`).

## Non-Claims

This note does not claim the container boundary is bypass-proof, provably
safe, or tamper-proof. It does not protect against kernel exploits,
container-runtime escapes, hardware or side-channel attacks, or
vulnerabilities in the runtime itself — the host kernel is shared. It says
nothing about code-execution paths that never cross Boundary: sessions,
shells, or runtimes outside the governed route are outside this boundary.
