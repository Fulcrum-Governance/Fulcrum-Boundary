# CLI Adapter

Status: preview

The CLI adapter governs command execution when commands route through the Boundary CLI wrapper. It parses the command, evaluates policy through the shared pipeline, executes through `os/exec` only when allowed, and attaches governance metadata to the response.

## Lifecycle

| Step | Status | Notes |
|---|---|---|
| parse | implemented | Parses a command string into argv pipe segments without invoking a shell. |
| identify | implemented | Maps `agent_id`, `tenant_id`, and generated request identity onto `GovernanceRequest`. |
| evaluate | delegated | Calls the shared `governance.Pipeline`. |
| deny | implemented | Denied commands return a CLI-shaped denial response and never reach the executor. |
| forward | implemented | Allowed commands execute through the configured executor. The default executor uses `os/exec` and never invokes a shell itself. |
| inspect | implemented | Command output is inspected for sensitive-data patterns. |
| metadata | implemented | Governance action, request ID, envelope ID, and policy ID are attached to response metadata. |
| record | delegated | The shared pipeline emits a structured decision record for every evaluation. |
| bypass_proof | delegated | Deployment topology must make the Boundary wrapper the only path to command execution. |
| fail_closed | implemented | Pipeline errors deny before command execution. |

## Bypass Model

Boundary governs CLI commands only when they route through the Boundary wrapper. Direct shell access, local scripts, cron jobs, remote SSH, CI runners, or another process that invokes commands without the wrapper are outside Boundary.

The direct-shell bypass test documents this limitation: a normal shell command can execute without producing a Boundary decision record. That is an honest deployment boundary, not a failure of the wrapper path.

## Production Gate

The adapter stays preview until a deployment provides evidence that the Boundary wrapper is the sole command path for the governed environment. Without that topology, Boundary can prove deny-before-execute only for wrapper-routed commands.
