# Managed Agents Adapter

The Managed Agents adapter is preview until a live upstream conformance run is
recorded with operator-owned credentials. In proxy mode, customer applications
call Boundary, Boundary opens or streams the upstream session, and Boundary
resolves tool-confirmation pauses before they reach the privileged tool path.

## Protocol Mapping

| Managed Agents event | Boundary behavior |
|---|---|
| `agent.tool_use` | Convert to `GovernanceRequest`, evaluate, then send `user.tool_confirmation` with `allow` or `deny`. |
| `agent.mcp_tool_use` | Same as `agent.tool_use`, with the MCP tool name as the governed tool. |
| `session.thread_created` | Track the child thread and optional budget allocation. |
| `session.thread_status_*` | Preserve the event and maintain thread state. |
| `session.status_idle` with `requires_action` | Boundary resolves the referenced pending tool events by policy. |
| Tool-result events | Preserve the stream and inspect results for obvious error or sensitive-output signals. |

The current Anthropic API contract supports `user.tool_confirmation` with
`result: "allow"` or `result: "deny"` and an optional `deny_message`. Boundary
uses that denial path instead of waiting for the hosted session to time out.

## Identity Model

Boundary maps `agent_id` to `GovernanceRequest.AgentID`, `tenant_id` to
`TenantID`, `session_id` to `TraceID`, and `session_thread_id` to both the
budget key suffix and the thread-level state key. The customer application's
authenticated tenant remains authoritative; upstream session fields are
treated as transport context, not proof of tenancy.

## Lifecycle

1. Parse the SSE event into `managedagents.Event`.
2. Identify tenant, agent, session, and thread.
3. Evaluate the event with `governance.Pipeline`.
4. Deny by sending `user.tool_confirmation` with `result: "deny"` and a
   `deny_message`.
5. Allow by sending `user.tool_confirmation` with `result: "allow"`.
6. Track thread-level and session-level budget usage.
7. Track per-thread trust state in standalone mode.
8. Attach governance metadata to proxied events and confirmations.
9. Emit decision records through `governance.AuditPublisher`.
10. Fail closed because `managed_agents` is in the default fail-closed
    transport set.

## Bypass Model

The protected topology is credential based: the customer app must not possess
the upstream Managed Agents API key and must not be able to call the session
events send endpoint directly. Boundary is the sole component allowed to send
tool confirmations upstream.

See
[`docs/deployment/managed-agents-bypass-proofing.md`](../deployment/managed-agents-bypass-proofing.md).

## Live Conformance

The live conformance harness and post-run promotion checklist are documented in
[`MANAGED_AGENTS_CONFORMANCE.md`](./MANAGED_AGENTS_CONFORMANCE.md). The harness
skips cleanly without credentials and validates sanitized transcript evidence
when `BOUNDARY_MA_CONFORMANCE=true` is set.

## Limitations

- The adapter is preview until a live upstream Anthropic Managed Agents
  conformance run is recorded with operator-owned credentials.
- Standalone budget tracking is in-process. Kernel-connected deployments should
  sync budgets to Fulcrum's atomic budget engine.
- Standalone trust tracking is in-process. Kernel-connected deployments should
  sync trust state to the Fulcrum trust bridge.
- Dreaming or self-improvement sessions are governed only at the action
  boundary; model behavior changes are outside this adapter's scope.
