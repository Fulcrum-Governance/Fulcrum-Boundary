# Release Truth Reconciliation

Date: 2026-05-27

Audited base commit SHA: `19b7612dd83542ad9138a157d7d5c9a56b436132`

Branch: `release/truth-reconciliation`

## Summary

This reconciliation checked the post-audit Boundary release surface after the
claims audit, Managed Agents harness, A2A lifecycle, CLI, CodeExec, gRPC, and
Webhook PRs landed.

The release truth is consistent:

- MCP is the only production adapter.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A are preview adapters.
- Managed Agents remains preview because no live upstream conformance evidence
  has been recorded.
- A2A remains preview because no live protocol conformance evidence has been
  recorded.
- CodeExec does not claim secure sandboxing without a real named sandbox
  boundary.
- gRPC documents unary governance and streaming limitations.
- Webhook separates informational audit mode from execution pre-approval mode.
- No user adoption or customer deployment evidence is claimed.

## Test Commands

| Command | Result |
|---|---|
| `go test ./... -count=1 -timeout 5m` | Pass |
| `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)` | Pass |
| `go test ./tests/... -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1 -timeout 5m` | Pass |
| `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies` | Pass: `policy files: 1`, `rules: 5`, `warnings: 0` |
| `go run ./cmd/boundary verify-record --help` | Pass |

## Adapter Maturity

| Adapter | Current status | Release truth |
|---|---|---|
| MCP | production | Production JSON-RPC MCP proxy path with lifecycle tests; deployment bypass proof remains an operator topology contract. |
| CLI | preview | Governed wrapper execution works, but production requires evidence that the wrapper is the sole command path. |
| CodeExec | preview | Policy-gated execution lifecycle works; secure sandboxing is not claimed without a real named boundary. |
| gRPC | preview | Unary interceptor lifecycle works with governance trailers; streaming workloads remain preview. |
| Managed Agents | preview | Preview proxy lifecycle and conformance harness exist; production requires live upstream conformance with operator-owned credentials. |
| Webhook | preview | Informational audit and execution pre-approval modes are split; production requires sole-path deployment evidence. |
| A2A | preview | Governed lifecycle exists against a documented snapshot; production requires live protocol conformance and deployment bypass evidence. |

## Claims Status

| Claim | Status | Reconciled result |
|---|---|---|
| BND-CLAIM-001 | delivered | Still scoped to MCP Safety Gateway requests routed through Boundary. |
| BND-CLAIM-002 | delivered | Structured decision-record language remains separate from receipt-grade language. |
| BND-CLAIM-003 | partial | Adapter package claim remains partial because only MCP is production; all other adapters remain preview. |
| BND-CLAIM-004 | false | SQL firewall language remains prohibited outside historical or claims-control context. |
| BND-CLAIM-005 | delivered | Receipt-grade language remains tied to request, policy bundle, and decision hashes. |
| BND-CLAIM-006 | delivered | Production MCP adapter claim remains supported. |
| BND-CLAIM-007 | partial | Managed Agents remains preview until live upstream conformance is recorded. |
| BND-CLAIM-008 | delivered | Postgres AST guard claim remains bounded to statement classification before PolicyEval. |
| BND-CLAIM-009 | delivered | Trust integration and adaptive termination remain scoped to protected adapters and do not replace deployment isolation. |
| BND-CLAIM-010 | delivered | Standalone and kernel contracts remain contract claims, not proved-decision or full-service-connection claims. |

## Docs Checked

- `README.md`
- `CHANGELOG.md`
- `claims/boundary_claims.yaml`
- `docs/CLAIMS_LEDGER.md`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/LAUNCH_TRUTH_FREEZE.md`
- `docs/adapters/CLI.md`
- `docs/adapters/CODEEXEC.md`
- `docs/adapters/GRPC.md`
- `docs/adapters/MANAGED_AGENTS.md`
- `docs/adapters/MANAGED_AGENTS_CONFORMANCE.md`
- `docs/adapters/WEBHOOK.md`
- `docs/adapters/A2A.md`
- `docs/security/FAIL_MODE_MATRIX.md`

## Drift Found

The claims ledger evidence summary for BND-CLAIM-003 was too terse after the
adapter lifecycle PRs. It referenced adapter package tests and A2A lifecycle
tests, but did not name the newer CLI, CodeExec, gRPC, Webhook, A2A, and
Managed Agents evidence classes.

`docs/LAUNCH_TRUTH_FREEZE.md` did not yet have a dated post-audit adapter-state
entry for the final release truth after the selected adapter work.

## Drift Fixed

- Updated `docs/CLAIMS_LEDGER.md` so BND-CLAIM-003 points to the current
  adapter lifecycle and conformance evidence classes.
- Added this reconciliation report.
- Added a dated post-audit adapter-state section to
  `docs/LAUNCH_TRUTH_FREEZE.md`.
- Added a changelog entry for the reconciliation report.

## Remaining Partial Or Preview Claims

- BND-CLAIM-003 remains partial because only MCP is production.
- BND-CLAIM-007 remains partial because Managed Agents lacks live upstream
  conformance evidence.
- CLI remains preview until wrapper sole-path deployment evidence exists.
- CodeExec remains preview until a real named sandbox boundary is implemented,
  tested, and documented.
- gRPC remains preview for streaming workloads until per-message governance is
  implemented and tested.
- Webhook remains preview until execution-mode sole-path deployment evidence
  exists.
- A2A remains preview until live protocol conformance and deployment bypass
  evidence exist.
