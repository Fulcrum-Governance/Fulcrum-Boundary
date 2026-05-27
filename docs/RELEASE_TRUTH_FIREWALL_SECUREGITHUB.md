# Firewall + Secure GitHub Release Truth

Date: 2026-05-27

Audited base commit SHA: `87f336a342ed645d1a6c02aed9f707bcc405c5cf`

Branch: `codex/2026-05-27-firewall-securegithub-truth`

Note: This report records the earlier Firewall + Secure GitHub release-train
state. The final public release state after NDJSON ingest, GitHub Action,
install polish, and release-check work is recorded in
[`docs/RELEASE_TRUTH_PUBLIC.md`](./RELEASE_TRUTH_PUBLIC.md).

## Summary

This reconciliation checks the Boundary release surface after the MCP Firewall
and Secure GitHub release-train PRs landed. It covers public README copy,
claims, readiness, launch docs, demo docs, and verification commands.

The release truth is consistent:

- MCP remains the only production adapter.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub remain
  preview adapter/profile surfaces.
- MCP Firewall discovery, graph, policy generation, install/uninstall,
  descriptor lock, redteam, and local dashboard surfaces have delivered claims
  only where tests and docs support them.
- Secure GitHub is a preview fixture profile for write-after-taint denial
  before upstream GitHub mutation.
- Secure GitHub does not claim live GitHub App conformance.
- Secure GitHub does not claim production bypass resistance.
- The dashboard remains local-only visibility, not hosted monitoring.
- Generated policies remain starter policies requiring operator review.
- Fixture redteam evidence remains fixture evidence, not universal attack
  prevention.

## Test Commands

| Command | Result |
|---|---|
| `go test ./... -count=1 -timeout 5m` | Pass |
| `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)` | Pass |
| `go test ./tests/... -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1 -timeout 5m` | Pass |
| `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies` | Pass: `policy files: 1`, `rules: 5`, `warnings: 0` |
| `go run ./cmd/boundary verify-record --help` | Pass |
| `go run ./cmd/boundary inventory --help` | Pass |
| `go run ./cmd/boundary redteam --help` | Pass |

## Adapter And Profile Maturity

| Adapter/Profile | Status | Release truth |
|---|---|---|
| MCP | production | Production JSON-RPC MCP proxy path with lifecycle tests. Deployment bypass proof remains an operator topology requirement. |
| CLI | preview | Governed wrapper execution works. Production requires evidence that the wrapper is the sole command path. |
| CodeExec | preview | Policy-gated execution works. Secure sandboxing is not claimed without a real named boundary. |
| gRPC | preview | Unary interceptor lifecycle works with governance trailers. Streaming workloads remain preview. |
| Managed Agents | preview | Preview proxy lifecycle and conformance harness exist. Production requires live upstream conformance. |
| Webhook | preview | Informational audit and execution pre-approval modes are split. Production requires sole-path deployment evidence. |
| A2A | preview | Governed lifecycle exists against a documented snapshot. Production requires live protocol conformance and deployment evidence. |
| Secure GitHub | preview | Fixture-backed Secure MCP profile denies tested private-repo write-after-taint paths before upstream. Production requires live GitHub App conformance and bypass evidence. |

## Claims Status

| Claim | Status | Reconciled result |
|---|---|---|
| BND-CLAIM-001 | delivered | Still scoped to MCP Safety Gateway requests routed through Boundary. |
| BND-CLAIM-002 | delivered | Structured decision-record language remains separate from receipt-grade language. |
| BND-CLAIM-003 | partial | Adapter/package breadth remains partial because only MCP is production. |
| BND-CLAIM-004 | false | SQL firewall language remains prohibited outside claims-control or historical context. |
| BND-CLAIM-005 | delivered | Receipt-grade language remains tied to request, policy bundle, and decision hashes. |
| BND-CLAIM-006 | delivered | Production MCP adapter claim remains supported. |
| BND-CLAIM-007 | partial | Managed Agents remains preview until live upstream conformance is recorded. |
| BND-CLAIM-008 | delivered | Postgres AST guard claim remains bounded to statement classification before PolicyEval. |
| BND-CLAIM-009 | delivered | Trust integration and adaptive termination remain scoped to protected adapters. |
| BND-CLAIM-010 | delivered | Standalone and kernel contracts remain contract claims, not proved-decision claims. |
| BND-CLAIM-011 | delivered | Local MCP config inventory remains read-only and classification-only. |
| BND-CLAIM-012 | delivered | Risk graphs and generated policies remain starter/operator-review surfaces. |
| BND-CLAIM-013 | delivered | Install/uninstall and descriptor lock claims remain local and reversible. |
| BND-CLAIM-014 | delivered | Redteam claim remains fixture-only with no live mutation. |
| BND-CLAIM-015 | delivered | Secure GitHub claim remains preview fixture write-after-taint denial before upstream. |
| BND-CLAIM-016 | delivered | Dashboard claim remains local-only artifact visibility. |

## Docs Checked

- `README.md`
- `CHANGELOG.md`
- `claims/boundary_claims.yaml`
- `docs/CLAIMS_LEDGER.md`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/LAUNCH_TRUTH_FREEZE.md`
- `docs/DEMO_SCRIPT.md`
- `docs/YC_DEMO_NARRATIVE.md`
- `docs/LAUNCH_README.md`
- `docs/SCREENSHOT_SCRIPT.md`
- `docs/firewall/DISCOVERY_INVENTORY.md`
- `docs/firewall/RISK_GRAPH_POLICY_GENERATION.md`
- `docs/firewall/INSTALL_LOCK.md`
- `docs/firewall/REDTEAM.md`
- `docs/firewall/DASHBOARD.md`
- `docs/secure-mcp/GITHUB.md`
- `docs/secure-mcp/GITHUB_REDTEAM.md`
- `docs/deployment/secure-github-bypass-proofing.md`

## Drift Found

- The prior reconciliation report predated the Firewall + Secure GitHub
  release-train claims and only listed claims through BND-CLAIM-010.
- `docs/LAUNCH_TRUTH_FREEZE.md` did not yet have a dated section for the final
  Firewall + Secure GitHub release-train truth.
- The README opening paragraph used broader "production AI agents" language
  where the release evidence is better stated as AI agents using privileged
  tools routed through Boundary.

## Drift Fixed

- Added this Firewall + Secure GitHub release truth report.
- Added a dated Firewall + Secure GitHub section to
  `docs/LAUNCH_TRUTH_FREEZE.md`.
- Tightened the README opening description to the concrete action-boundary
  surface without broad production-agent language.
- Added a changelog entry for this final reconciliation artifact.

## Remaining Preview Or Gated Work

- Secure GitHub remains preview until live GitHub App conformance and deployment
  bypass evidence are recorded.
- Managed Agents remains preview until live upstream conformance evidence is
  recorded.
- CodeExec remains preview until a real named sandbox boundary is implemented,
  tested, and documented.
- gRPC streaming workloads remain preview until per-message governance is
  implemented and tested.
- Webhook execution mode remains preview until sole-path deployment evidence is
  recorded.
- A2A remains preview until live protocol conformance and deployment bypass
  evidence are recorded.
- The repo-local MCP audit GitHub Action is delivered as CI audit/reporting
  only; it does not install Boundary or provide runtime protection.
- Full GitHub MCP tool catalog coverage remains deferred.
