# Webhook Adapter

Status: preview.

The webhook adapter supports two explicit endpoint modes. Endpoint configuration must choose the mode; public docs and claims must not blur them.

## Informational Mode

Informational mode is a post-execution audit path. A webhook reports an action that already happened, Boundary evaluates the payload, and Boundary records the verdict through the shared governance pipeline.

Informational mode:

- cannot deny before execution
- does not forward to a downstream action
- returns an audit result even when the governance verdict is `deny`
- uses `X-Governance-Webhook-Mode: informational`
- uses `X-Governance-Can-Deny: false`

This mode must not be described as pre-execution control.

## Execution Mode

Execution mode is a pre-execution approval path. The caller asks Boundary before the downstream action runs. Boundary evaluates the payload and forwards only when the verdict allows execution.

Execution mode:

- can deny before forwarding
- must not forward denied webhooks
- must not forward if the governance pipeline is unavailable
- uses `X-Governance-Webhook-Mode: execution`
- uses `X-Governance-Can-Deny: true`

Execution webhooks require Boundary to be the sole path to the downstream action. If callers can invoke the downstream action directly, Boundary is bypassable.

## Bypass Model

Informational webhooks are inherently bypassable because the action has already happened before Boundary receives the event.

Execution webhooks are governed only when clients route through the Boundary handler and the downstream action is not reachable through an unguarded path.
