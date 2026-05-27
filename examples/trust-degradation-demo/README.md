# Trust Degradation Demo

Run:

```bash
boundary demo trust-degradation
```

The demo uses the standalone trust backend. It starts `demo-agent` in a healthy
state, sends one allowed query, then sends repeated blocked queries until
Boundary isolates the agent. Once isolated, even otherwise safe protected tool
calls are denied before execution.
