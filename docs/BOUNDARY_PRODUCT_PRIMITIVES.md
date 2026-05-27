# Boundary Product Primitives

Boundary is an action boundary for agent tools. These primitives define the
product shape for the Firewall and Secure MCP release train.

## 1. Inventory

Discover local MCP clients, server configs, tools, and advertised
capabilities.

Evidence target:

- `boundary init`
- `boundary inventory`
- Client and config discovery docs

Claim boundary:

- Inventory reports what Boundary can observe from configured clients and
  fixtures.
- Inventory does not prove every local tool path is visible.

## 2. Risk Graph

Show dangerous source-to-sink and source-to-mutation paths.

Evidence target:

- `boundary graph`
- JSON and Mermaid risk path output
- Fixture paths such as `github.issue_body -> private_repo.write`

Claim boundary:

- Risk graph identifies configured and tested path rules.
- It does not prove all possible exploit paths are known.

## 3. Policy Pack

Generate starter action rules from inventory and risk paths.

Evidence target:

- `boundary policy generate --mode balanced`
- Generated policies that pass `boundary verify`

Claim boundary:

- Generated packs are starter policies.
- Operators still own deployment-specific review and tuning.

## 4. Runtime Boundary

Return a verdict before a privileged tool executes.

Evidence target:

- Governed adapter lifecycle tests
- Denied requests never reaching downstream tools
- Fail-closed tests for parse and pipeline errors

Claim boundary:

- Runtime protection applies only to governed routes.
- Direct access to the tool is a bypass path unless deployment topology removes
  it.

## 5. Redteam Pack

Run safe fixture attacks that demonstrate the boundary behavior.

Evidence target:

- `boundary redteam`
- `github-lethal-trifecta` fixture
- Decision records for expected deny outcomes

Claim boundary:

- Fixture attacks prove the tested path.
- Fixture success is not live conformance evidence.

## 6. Descriptor Lock

Detect tool descriptor changes that could change what an agent thinks a tool
does.

Evidence target:

- `boundary lock`
- `boundary verify-lock`
- Descriptor hash tests
- [`docs/firewall/INSTALL_LOCK.md`](./firewall/INSTALL_LOCK.md)

Claim boundary:

- Descriptor lock detects shape changes.
- It does not replace policy evaluation or deployment isolation.

## 7. Decision Record

Emit structured evidence for every governed verdict.

Evidence target:

- Audit publisher tests
- Decision-record docs
- Adapter metadata tests

Claim boundary:

- Basic decision records are structured records of verdicts.
- Receipt-grade verification belongs to the receipt-grade primitive.

## 8. Receipt-Grade Record

Make selected decision records hash-verifiable against the request, policy
bundle, and decision.

Evidence target:

- `boundary verify-record`
- Receipt verification tests
- Receipt docs

Claim boundary:

- Receipt-grade means hash-verifiable.
- It does not mean signed by default.

## 9. Dashboard Or TUI

Give local visibility into decisions, risk paths, policies, install status, and
lock status.

Evidence target:

- `boundary dashboard` or `boundary tui`
- Local-only docs and tests

Claim boundary:

- The first dashboard/TUI is local developer visibility.
- It is not a hosted enterprise console unless that product surface exists.
