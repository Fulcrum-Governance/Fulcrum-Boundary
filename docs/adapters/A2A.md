# A2A Adapter

Status: preview

The A2A adapter governs Agent2Agent task/message envelopes before forwarding them to a downstream agent. It tracks the snapshot in [A2A_PROTOCOL_SNAPSHOT.md](./A2A_PROTOCOL_SNAPSHOT.md) and remains preview until live protocol conformance evidence exists.

## Lifecycle

| Step | Status | Notes |
|---|---|---|
| parse | implemented | Parses Boundary preview envelopes and minimal A2A JSON-RPC `message/send` or `tasks/send` payloads. |
| identify | implemented | Maps `sender_agent_id` to `AgentID`, `receiver` to target context, task/message IDs to trace context, and tenant from adapter configuration. |
| evaluate | delegated | Calls the shared `governance.Pipeline`. |
| deny | implemented | Returns an A2A-shaped denial and does not call the downstream forwarder. |
| forward | implemented | Allowed tasks are forwarded only through the configured `Forwarder`. |
| inspect | implemented | Downstream responses are inspected for policy-relevant output signals. |
| metadata | implemented | Governance action, reason, request ID, envelope ID, rule, mode, trust score, and inspection concerns are attached where the response shape permits. |
| record | delegated | The shared pipeline emits a structured decision record for every evaluation. |
| bypass_proof | delegated | Deployment topology must make Boundary the path to the downstream A2A agent. |
| fail_closed | implemented | Malformed requests, unknown mandatory fields, and pipeline errors deny or return unsupported fail-closed responses. |

## Bypass Model

A2A governance applies only when the task routes through Boundary before reaching the downstream agent. If the caller can contact the downstream agent directly, Boundary records and enforcement are bypassed.

Multi-hop delegation is governed only at the first Boundary-controlled hop unless every downstream hop also routes through Boundary.

## Limitations

- A2A is evolving; this adapter tracks a dated snapshot instead of claiming full protocol authority.
- The adapter implements a minimal preview envelope plus synchronous JSON-RPC message parsing.
- Streaming, task resubscription, push notifications, full AgentCard negotiation, and artifact schema negotiation are not implemented.
- Output inspection is pattern-based, not semantic analysis.
- Preview status remains until a live protocol conformance run is recorded.
