# YC Demo Narrative

## One Sentence

Fulcrum Boundary is the action boundary for MCP-native agents: it shows what
tools an agent can reach, generates starter policies for risky paths, and denies
unsafe privileged actions before execution when those actions route through
Boundary.

## The Concrete Demo

A coding agent reads a poisoned GitHub issue from an untrusted source. That
issue enters the agent context. The agent then tries to write to a private
repository through a GitHub MCP tool. Boundary's Secure GitHub preview profile
sees the tainted context and denies the private-repo mutation before any
upstream GitHub call is made.

This is the wedge:

```text
See what your AI tools can do. Block what they should not.
```

## Why Now

MCP gives agents direct paths to GitHub, filesystems, databases, shell commands,
and messaging tools. Those tool paths are useful, but they also create a new
failure mode: untrusted content can enter the model context and influence a
later privileged action.

Boundary focuses on the moment that matters operationally:

1. the agent is about to call a privileged tool
2. the request has context, identity, policy, trust, and envelope metadata
3. Boundary decides allow or deny before the tool executes
4. every governed verdict is recorded

## What Exists In This Release

Production:

- MCP Safety Gateway for governed MCP JSON-RPC proxying

Delivered Firewall surfaces:

- local MCP config inventory
- inventory-derived risk graphs
- starter firewall policy generation
- reversible local install and uninstall mechanics
- descriptor lock verification
- fixture redteam packs
- local-only dashboard output

Preview profiles and adapters:

- Secure GitHub MCP fixture profile
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A adapters

## Secure GitHub Proof Boundary

The Secure GitHub profile is preview. The current proof is fixture-backed and
uses no live GitHub credentials or real repository mutation.

The fixture proves:

- external GitHub content can mark the session as tainted
- a protected private-repo write after taint is denied
- the denied call does not reach upstream execution
- the decision emits a record with rule and hash evidence

The fixture does not prove:

- live GitHub App conformance
- production bypass resistance
- coverage of the full GitHub MCP catalog
- coverage of every malicious issue, pull request, or prompt-injection pattern

## Demo Talk Track

```text
The old safety story for agents was mostly observation: log what happened,
scan after the fact, or rely on the model to behave.

Boundary moves the control point in front of the tool. In this demo, the agent
reads untrusted GitHub issue content, then attempts a private-repo file write.
Boundary carries the taint metadata into the tool request, matches the
write-after-taint rule, denies the write before GitHub is called, and emits a
decision record.

The product is not claiming universal agent safety. It is a concrete action
boundary for MCP-native tools, with production MCP gateway support and preview
adapter/profile surfaces whose maturity is tracked in the repo.
```

## Evidence Links

- `docs/firewall/DISCOVERY_INVENTORY.md`
- `docs/firewall/RISK_GRAPH_POLICY_GENERATION.md`
- `docs/firewall/INSTALL_LOCK.md`
- `docs/firewall/REDTEAM.md`
- `docs/firewall/DASHBOARD.md`
- `docs/secure-mcp/GITHUB.md`
- `docs/secure-mcp/GITHUB_REDTEAM.md`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/CLAIMS_LEDGER.md`
- `claims/boundary_claims.yaml`

## Claim-Safe External Copy

Long:

```text
Fulcrum Boundary is the action boundary for MCP-native agents. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary. The flagship preview profile is Secure
GitHub MCP: a fixture-backed GitHub path showing write-after-taint denial before
private-repo mutation.
```

Short:

```text
See what your AI tools can do. Block what they should not.
```

YC version:

```text
Coding agents now reach GitHub, databases, filesystems, and messaging tools
through MCP. Fulcrum Boundary sits before those tools and decides allow or deny
before execution when the action routes through Boundary. The demo shows a
poisoned GitHub issue tainting the agent context, followed by a private-repo
write attempt that Boundary denies before any GitHub mutation occurs.
```

