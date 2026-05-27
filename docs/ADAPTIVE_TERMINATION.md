# Adaptive Termination

Adaptive termination isolates an agent automatically when repeated governed
failures drive trust below the configured threshold.

| State | Score band | Boundary behavior |
|---|---:|---|
| Active / `TRUSTED` | `score >= 0.6` | Normal governed operation. |
| Degraded / `EVALUATING` | `0.3 <= score < 0.6` | Standalone mode returns `require_approval`; kernel deployments can route this to Foundry escalation. |
| Isolated / `ISOLATED` | `score < 0.3` | Deny all protected tool calls before execution. |
| Terminated / `TERMINATED` | operator-set | Deny all calls until a new identity or explicit reset is used. |

Denied actions increment beta. Denied actions in the degraded state increment
beta faster, so repeated unsafe attempts quickly isolate the agent. Allowed
actions increment alpha and can recover an agent that has not crossed into
manual-isolation territory.

Run the local demo:

```bash
boundary demo trust-degradation
```

The demo starts a seeded standalone trust backend, sends one safe action, then
repeated blocked actions, and prints the trust trajectory until isolation.
