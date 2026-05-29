# Fulcrum Boundary

The action boundary for MCP-native agents.

Your agent is about to touch a real system. Boundary decides before the tool
executes, records the verdict, and governs only routes forced through Boundary.

## Try It In One Minute

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.1
boundary selftest
boundary demo github-lethal-trifecta
```

No credentials. No live GitHub calls. No real mutations.

![Boundary demo walkthrough](assets/boundary-demo-walkthrough.svg)

Static walkthrough of the fixture-safe GitHub write-after-taint demo.

## Current Release Truth

| Surface | Status | Limit |
|---|---|---|
| MCP adapter | Production | Governs MCP routes forced through Boundary. |
| Secure GitHub | Preview | Fixture proof and opt-in conformance do not close deployment bypasses. |
| Command Boundary | Delivered preview | Routed command paths only. |
| Edit Boundary | Delivered preview | Routed edit envelopes only. |
| Policy generation | Starter policy utility | Requires operator review. |
| Dashboard | Local artifact visibility | Not hosted monitoring. |

## Start Here

| Need | Page |
|---|---|
| Run the local path | [Quickstart](quickstart.md) |
| Understand the flagship fixture | [Demo](demo.md) |
| Learn the action-boundary model | [Concepts](concepts/action-boundary.md) |
| Check public claim status | [Claims](reference/claims.md) |
| Verify release utilities | [Release Utilities](reference/release-utilities.md) |
