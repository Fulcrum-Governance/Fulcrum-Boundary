# Standalone vs Kernel

| Capability | Standalone | Kernel |
|---|---|---|
| External dependencies | None | Redis, Fulcrum API, NATS |
| Policy source | YAML files | Redis-cached Fulcrum policy bundle |
| Trust source | In-process Beta evaluator | fulcrum-trust Redis IPC state |
| Budget enforcement | Optional in-process tracker | Fulcrum atomic budget service |
| Escalation | `require_approval` decision | Foundry escalation subject |
| Audit | Structured JSON via slog | NATS to Fulcrum audit trail |
| Envelope lifecycle | Local generated IDs | Fulcrum envelope subject |
| Proof correspondence | Documentation reference | Documentation reference |

## Standalone

Standalone mode is the zero-dependency OSS path. It is appropriate for demos,
local gateways, and deployments that want Boundary's transport adapters and
receipt-grade decision records without connecting to Fulcrum services.

## Kernel

Kernel mode is the commercial Fulcrum integration path. Boundary keeps making
the pre-execution adapter decision, but it uses Fulcrum-owned services for
policy, trust, cost, budget, escalation, audit, and envelope lifecycle.

Kernel mode fails hard on incomplete configuration. Boundary should not start
with a half-declared kernel connection because that can turn an intended
fail-closed deployment into a silently local one.
