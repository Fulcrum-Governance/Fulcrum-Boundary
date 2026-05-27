# A2A Protocol Snapshot

Date: 2026-05-27

Boundary targets the public Agent2Agent (A2A) v0.3.0 JSON-RPC shape documented at <https://a2a-protocol.org/v0.3.0/specification/> and the upstream project specification at <https://github.com/a2aproject/A2A/blob/main/docs/specification.md>.

This is a preview adapter snapshot. It is intentionally narrow because the protocol continues to evolve and Boundary does not invent missing protocol fields.

## Supported Request Shapes

Boundary supports two inbound shapes:

| Shape | Supported fields |
|---|---|
| Boundary preview envelope | `task_id`, `context_id`, `message_id`, `sender_agent_id`, `receiver`, `action`, `input`, `metadata`, `required_fields` |
| A2A JSON-RPC `message/send` or `tasks/send` | `jsonrpc`, `id`, `method`, `params.message.taskId`, `params.message.contextId`, `params.message.messageId`, `params.message.parts[].text`, `params.message.parts[].data`, `params.metadata`, `params.message.metadata` |

For JSON-RPC requests, Boundary reads `sender_agent_id`, `receiver`, `action`, and optional `required_fields` from merged request/message metadata. Text parts become `input.text` or `input.text_parts`; data parts are copied into `input`.

## Unsupported Fields

Boundary does not claim full A2A server conformance in this preview. These surfaces are not implemented here:

- AgentCard discovery or publication.
- `message/stream` Server-Sent Events.
- `tasks/get`, `tasks/cancel`, push notification configuration, task resubscription, or multi-turn task state storage.
- File parts, binary parts, structured artifact negotiation, or UI extension parts beyond simple data maps.
- Per-hop delegation governance beyond the first Boundary-controlled hop.

## Unknown Mandatory Fields

If a request declares a mandatory field through `required_fields` and Boundary does not explicitly support that field, parsing fails closed with an unsupported A2A response. Supported mandatory fields must also be present, or the request fails closed.

Boundary does not silently pass through unknown mandatory fields.

## Assumptions

- The caller routes the A2A task through Boundary before the downstream agent receives it.
- `sender_agent_id` is the best available agent identity in this preview.
- `receiver` identifies the intended downstream agent or endpoint.
- `action` is the policy-relevant task/tool name.
- Tenant identity comes from adapter configuration, not the public A2A message body.
- Response inspection is pattern-based and cannot prove semantic safety.

## Boundary-Specific Envelope Fields

Boundary-specific fields exist to bridge A2A messages into `governance.GovernanceRequest`:

- `sender_agent_id` maps to `AgentID`.
- `receiver` maps to policy context and forwarding target.
- `action` maps to `ToolName`.
- `task_id`, `context_id`, and `message_id` map to trace context.
- `required_fields` provides fail-closed behavior when a caller needs a field to be understood before forwarding.

These fields are part of Boundary's preview governance envelope and not a claim that the upstream A2A protocol standardizes them exactly as named.
