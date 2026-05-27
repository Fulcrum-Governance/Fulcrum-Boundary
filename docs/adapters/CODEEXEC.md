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

`NewAdapterWithExecutor` wires CodeExec to an operator-provided `Executor` and `ExecutionBoundary`. The boundary metadata must name what actually isolates execution:

- container
- WASM runtime
- Firecracker or another microVM
- OS-level sandbox with documented restrictions
- local process

Only the first four may be described as secure sandboxing when implemented, tested, and documented. A local-process boundary must remain preview and must be described as policy-gated execution, not secure sandboxing.

## Bypass Model

Boundary governs code execution only when code enters through the CodeExec adapter. Direct host execution, notebook kernels, CI scripts, shell access, or any path into the execution runtime that does not pass through Boundary is outside this adapter.

The direct-execution bypass test documents this limitation by performing a host write without invoking Boundary. That is an honest deployment boundary: production use requires topology evidence that the governed executor is the sole code-execution path available to the agent.

## Production Gate

CodeExec stays preview until a real named sandbox boundary is implemented, tested, and documented with:

- deny-before-execute lifecycle evidence
- allowed-code execution evidence inside the named boundary
- timeout, stdout, stderr, exit-code, and output-inspection evidence
- bypass proof showing direct runtime access is unavailable to the governed agent
- transcript or integration evidence suitable for the readiness matrix and claims ledger
