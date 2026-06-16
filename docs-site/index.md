# Fulcrum Boundary

The action boundary for routed agent tools.

Your agent is about to touch a real system. Boundary decides before the tool
executes, records the verdict, and governs only routes forced through Boundary.

## Try It In One Minute

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
boundary selftest
boundary demo github-lethal-trifecta
boundary demo command-secret-exfil
```

No credentials. No live calls. No real mutations.

![Boundary denies a GitHub write-after-taint action before upstream execution](assets/github-lethal-trifecta-demo.gif)

A real run of `boundary demo github-lethal-trifecta`: Boundary denies the routed
private-repo mutation before GitHub is touched and emits a hash-verifiable
decision record. The equal-weight Command Boundary lane is
`boundary demo command-secret-exfil`, which denies a routed secret-exfiltration
command before execution. A static
[walkthrough](assets/boundary-demo-walkthrough.svg) is available as a no-JS
fallback for the MCP lane.

## Current Release Truth

| Surface | Status | Limit |
|---|---|---|
| MCP adapter | Production | Governs MCP routes forced through Boundary. |
| Secure GitHub | Preview | Fixture proof and opt-in conformance do not close deployment bypasses. |
| Command Boundary | Delivered preview | Routed command paths only. |
| Edit Boundary | Delivered preview | Routed edit envelopes only. |
| Policy-as-code testing | Local-only | `boundary test` checks routed request fixtures against local policy bundles. |
| Policy generation | Starter policy utility | Requires operator review. |
| Dashboard | Local artifact visibility | Not hosted monitoring. |

## Start Here

| Need | Page |
|---|---|
| Run the local path | [Quickstart](quickstart.md) |
| Run the two proof lanes | [Demos](demo.md) |
| Test policy behavior in CI | [Policy Testing](reference/policy-testing.md) |
| Learn the action-boundary model | [Concepts](concepts/action-boundary.md) |
| Check public claim status | [Claims](reference/claims.md) |
| Verify release utilities | [Release Utilities](reference/release-utilities.md) |
