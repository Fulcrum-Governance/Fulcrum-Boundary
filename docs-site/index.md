# Fulcrum Boundary

The action boundary for MCP-native agents.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
boundary selftest
boundary demo github-lethal-trifecta
```

Boundary inventories local MCP tool paths, renders risk paths, generates
starter policies, runs safe fixture redteams, and denies governed privileged
actions before execution when those actions route through Boundary.

```mermaid
flowchart LR
  A[Agent intent] --> B[Proposed action]
  B --> C[Boundary]
  C -->|allow| D[Privileged tool executes]
  C -->|deny / escalate / require approval| E[No execution]
  C --> F[Decision record]
```

## Current Release Truth

- MCP is the production adapter.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub are
  preview adapter/profile surfaces.
- Secure GitHub is fixture-backed until live GitHub App conformance and
  deployment bypass evidence are recorded.
- Command Boundary is a preview, routed-path-only command governance surface.
- Generated policies are starter policies for operator review.
- The dashboard is local-only artifact visibility.
- Boundary governs routed tools; direct tool access is outside the governed
  route.

## Pages Status

This docs site is published by GitHub Pages from the `Docs` workflow on `main`.
