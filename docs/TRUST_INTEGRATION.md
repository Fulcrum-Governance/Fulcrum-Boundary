# Trust Integration

Boundary trust mode connects Stage 1 of the governance pipeline to a production
trust backend. Deployments can run in three modes:

```yaml
trust:
  mode: standalone # standalone | kernel | disabled
  standalone:
    theta: 0.3
    initial_alpha: 1.0
    initial_beta: 1.0
    decay_rate: 0.01
  kernel:
    redis_url: redis://localhost:6379
    ipc_prefix: "agent:"
    timeout_ms: 100
    fail_closed: true
```

## Modes

- `disabled`: trust checks are skipped.
- `standalone`: Boundary keeps an in-process Beta(alpha,beta) evaluator with
  the same update semantics as `fulcrum-trust`: success increments alpha,
  failure increments beta, and partial outcomes increment both by half weight.
- `kernel`: Boundary reads and writes the Redis IPC state used by
  `fulcrum-trust`: `agent:{agent_id}:circuit_state` with integer states
  `0=TRUSTED`, `1=EVALUATING`, `2=ISOLATED`, and `3=TERMINATED`.

Trust store errors fail closed for production protected transports. When
`RequireAgentID` is enabled, protected adapter requests without an agent
identity are denied before execution.

## CLI

```bash
boundary trust show demo-agent
boundary trust show --redis-url redis://localhost:6379 demo-agent
boundary trust reset demo-agent
```

Every decision record carries `trust_score` and `trust_state`. State changes
also emit `event_type=trust_transition` audit records.
