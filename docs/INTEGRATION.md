# Fulcrum Integration

Boundary runs in two modes:

- `standalone`: local policy files, in-process trust, in-process budget checks, and JSON decision records.
- `kernel`: Boundary keeps the same pre-execution adapter surface but loads policy, trust, budget, escalation, audit, and envelope signals from Fulcrum services.

## Interface Contracts

The shared seams are defined in `governance/providers.go`.

| Contract | Standalone implementation | Kernel implementation |
|---|---|---|
| `PolicyProvider` | `governance/standalone.FilePolicyProvider` | `governance/kernel.RedisPolicyProvider` |
| `TrustChecker` / `TrustBackend` | `governance.StandaloneTrustBackend` | `governance.RedisTrustBackend` |
| `CostPredictor` | `governance/standalone.StaticCostPredictor` | `governance/kernel.StaticCostPredictor` until the Oracle bridge is wired |
| `BudgetEnforcer` | `governance/standalone.InMemoryBudgetEnforcer` | `governance/kernel.HTTPBudgetEnforcer` |
| `EscalationHandler` | `governance/standalone.RequireApprovalEscalationHandler` | `governance/kernel.NATSEscalationHandler` |
| `AuditPublisher` | `governance.SlogAuditPublisher` | `governance/kernel.NATSAuditPublisher` |
| `EnvelopeManager` | `governance/standalone.LocalEnvelopeManager` | `governance/kernel.NATSEnvelopeManager` |
| `ProofCorrespondence` | static proof map | static proof map |

## Kernel Configuration

Boundary validates configuration before server startup. Kernel mode must name every external seam explicitly:

```yaml
mode: kernel
kernel:
  policy_engine:
    type: redis
    redis_url: redis://localhost:6379
    key_prefix: "fulcrum:policies:"
  trust:
    type: redis_ipc
    redis_url: redis://localhost:6379
    key_prefix: "agent:"
  budget:
    type: api
    endpoint: http://fulcrum-api:8080/api/v1/cost/record
  escalation:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.foundry.escalate
  audit:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.audit.boundary
  envelope:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.envelope
security:
  require_agent_id: true
```

The schema shape is recorded in `config/schema.v1.yaml`.

## Ownership

- `fulcrum-io` owns Policy Engine, Budget API, Foundry escalation, audit storage, and envelope lifecycle.
- `fulcrum-trust` owns the canonical Beta trust model and Redis IPC semantics.
- `Fulcrum-Proofs` owns theorem names and invariant scopes.
- Boundary owns the transport-facing enforcement point and the local interfaces that connect those services to pre-execution decisions.
