# Managed Agents Bypass Proofing

Boundary protects Managed Agents sessions only when it is the sole route that
can send upstream tool confirmations.

## Required Controls

| Control | Requirement |
|---|---|
| API key custody | The upstream Managed Agents API key is stored only in the Boundary deployment. |
| Customer app path | Customer apps call Boundary, not the upstream session events send endpoint. |
| Confirmation routing | Boundary sends every `user.tool_confirmation` event after policy evaluation. |
| Logs | Boundary audit records are retained for every governed `agent.tool_use` and `agent.mcp_tool_use` event. |
| Network egress | Production deployments should restrict customer app egress to Boundary where possible. |

## Verification

Use `managedagents.VerifyBypassConfig` in deployment tests to assert the
credential boundary. A deployment fails the bypass proof when either of these
is true:

- the customer app can access the upstream Managed Agents API key;
- the customer app can send tool confirmations directly to the upstream
  sessions events endpoint.

## Non-Claims

This bypass proof does not claim that Boundary controls Managed Agents sessions
created outside the protected deployment, sessions where another service owns
the upstream API key, or custom tool execution paths that never emit governed
tool-confirmation events.
