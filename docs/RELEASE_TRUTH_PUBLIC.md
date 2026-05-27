# Final Public Release Truth

Date: 2026-05-27

Audited code commit SHA: `1ad95e3ba9a0ab168dd78d4153dd568b16e7e4b2`

Branch: `codex/2026-05-27-final-public-truth`

## Summary

This report reconciles the public Boundary release surface after the final
public hardening train landed: public README/copy polish, `boundary selftest`,
the GitHub lethal-trifecta fixture demo, NDJSON inventory records, external
inventory ingest, the repo-local MCP audit GitHub Action, and install/release
workflow polish.

The final public truth is:

- MCP remains the only production adapter.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub remain
  preview adapter/profile surfaces.
- Secure GitHub is the flagship preview profile and is fixture-backed.
- Generated policies are starter policies for operator review.
- Dashboard output is local-only artifact visibility, not hosted monitoring.
- External inventory ingest is Boundary-owned MCP inventory mapping, not an
  official third-party scanner integration or compatibility claim.
- The GitHub Action is repo-local CI audit/reporting only.
- Boundary governs routed tools. Tools that bypass Boundary are outside the
  governed route.
- The public Go install path requires Go 1.25+.
- Public action examples use `@main` until a post-rename action tag exists.

## Test Commands

| Command | Result |
|---|---|
| `make release-check` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |
| `go run ./cmd/boundary selftest` | Pass: all ten selftest checks passed without credentials, network mutation, or live mutation |
| `go run ./cmd/boundary demo github-lethal-trifecta` | Pass: `actual action: DENY`, `reason: lethal_trifecta_detected`, `upstream_called=false` |
| `go run ./cmd/boundary inventory ingest --file fixtures/external-inventory/external-mcp-inventory.ndjson --source external-mcp --summary` | Pass: complete snapshot, 3 records read, 1 MCP config, 1 MCP server |
| `GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@1ad95e3ba9a0ab168dd78d4153dd568b16e7e4b2` | Pass: installed binary ran `boundary selftest` successfully |
| `GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest` | Fails until a post-rename release tag supersedes `v0.2.0`, which still declares the old module path. Public install examples use `@main` until that tag exists. |

## README First-Run Status

README presents the first-run path before architecture:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@main
boundary selftest
```

It also gives a source-checkout path:

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary
go run ./cmd/boundary selftest
```

The five-minute demo section now starts with:

```bash
boundary demo github-lethal-trifecta
```

## Claims Status

| Claim | Status | Final release truth |
|---|---|---|
| BND-CLAIM-001 | delivered | MCP Safety Gateway requests are governed before execution only when the route passes through Boundary. |
| BND-CLAIM-002 | delivered | Structured decision records are emitted for governed verdicts. |
| BND-CLAIM-003 | partial | Boundary ships one production MCP adapter and seven preview adapter/profile packages tracked per adapter. |
| BND-CLAIM-004 | false | Boundary is not a SQL firewall. |
| BND-CLAIM-005 | delivered | Receipt-grade decision records are hash-verifiable; signed receipts are not implied by default. |
| BND-CLAIM-006 | delivered | Production MCP JSON-RPC proxy adapter remains supported. |
| BND-CLAIM-007 | partial | Managed Agents remains preview until live upstream conformance is recorded. |
| BND-CLAIM-008 | delivered | Postgres AST guard is statement classification before PolicyEval, not universal SQL protection. |
| BND-CLAIM-009 | delivered | Trust integration and adaptive termination remain scoped to protected adapters. |
| BND-CLAIM-010 | delivered | Standalone and kernel integration contracts remain contract surfaces. |
| BND-CLAIM-011 | delivered | Local MCP config inventory is read-only and classification-only. |
| BND-CLAIM-012 | delivered | Risk graphs and generated policies are starter/operator-review surfaces. |
| BND-CLAIM-013 | delivered | Install/uninstall and descriptor locks are local, reversible, and receipt-backed. |
| BND-CLAIM-014 | delivered | Redteam packs are fixture-only and do not use live secrets or live mutation. |
| BND-CLAIM-015 | delivered | Secure GitHub is a preview fixture profile for write-after-taint denial before upstream GitHub mutation. |
| BND-CLAIM-016 | delivered | Dashboard is local-only visibility over local artifacts. |
| BND-CLAIM-017 | delivered | GitHub Action audits repo-local MCP configs and emits Markdown/SARIF reports. |

## Feature Status

| Feature | Status | Release truth |
|---|---|---|
| `boundary selftest` | delivered | No-credential local smoke test over inventory, risk graph, starter policies, descriptor drift, redteam, Secure GitHub live fail-closed behavior, and decision records. |
| `boundary demo github-lethal-trifecta` | delivered | Fixture-only demo of write-after-taint denial before upstream GitHub mutation. |
| Inventory JSON/Markdown/SARIF | delivered | Local MCP inventory reporting surfaces. |
| Inventory NDJSON | delivered | Versioned record stream for tool ingestion. |
| External inventory ingest | delivered | Boundary, generic, and external MCP inventory NDJSON mapping. |
| GitHub Action MCP audit | delivered | Repo-local MCP config audit with Markdown and optional SARIF. |
| Install/release workflow | delivered | `make selftest`, `make demo-github`, `make release-check`, and `docs/INSTALL.md`. |
| Local dashboard | delivered | Local-only artifact view. |

## Adapter And Profile Status

| Adapter/Profile | Status | Release truth |
|---|---|---|
| MCP | production | Production JSON-RPC MCP proxy path with lifecycle tests; deployment bypass proof remains an operator topology requirement. |
| CLI | preview | Governed wrapper execution works; production requires sole-wrapper deployment evidence. |
| CodeExec | preview | Policy-gated execution works; secure sandboxing is not claimed without a real named boundary. |
| gRPC | preview | Unary lifecycle works with governance trailers; streaming workloads remain preview. |
| Managed Agents | preview | Preview proxy and conformance harness exist; production requires live upstream conformance. |
| Webhook | preview | Informational and execution modes are split; production requires sole-path deployment evidence. |
| A2A | preview | Governed lifecycle exists against a documented snapshot; production requires live protocol conformance. |
| Secure GitHub | preview | Fixture-backed Secure MCP profile; production requires live GitHub App conformance and deployment bypass evidence. |

## User-Install Status

The documented install path is:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@main
```

Requires Go 1.25+.

`@main` follows the current public default branch while the first post-rename
release tag is pending. `@latest` currently resolves to `v0.2.0`, which still
declares the old module path, so `@latest` must not be used in public first-run
copy until a post-rename tag is cut.

No Homebrew, package-manager, or hosted distribution channel is claimed.

## GitHub Action Ref Status

The MCP audit action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@main
```

Use `@main` until a post-rename action tag exists. SARIF upload examples must
include `contents: read` and `security-events: write` permissions.

## External Inventory Ingest Status

External ingest maps Boundary, generic, and external MCP inventory NDJSON into
Boundary inventory records. The `external-mcp` source name selects a tested
mapping mode for the local fixture:

```bash
go run ./cmd/boundary inventory ingest \
  --file fixtures/external-inventory/external-mcp-inventory.ndjson \
  --source external-mcp --summary
```

Boundary does not shell out to, import, depend on, endorse, or claim
compatibility with any named third-party scanner.

## Remaining Preview Or Partial Work

- Secure GitHub remains preview until live GitHub App conformance and deployment
  bypass evidence are recorded.
- Managed Agents remains preview until live upstream conformance evidence is
  recorded with operator-owned credentials.
- CodeExec remains preview until a real named sandbox boundary is implemented,
  tested, and documented.
- gRPC streaming workloads remain preview until per-message governance is
  implemented and tested.
- Webhook execution mode remains preview until sole-path deployment evidence is
  recorded.
- A2A remains preview until live protocol conformance and deployment bypass
  evidence are recorded.
- Full GitHub MCP tool catalog coverage remains deferred.

## Approved Release Language

Fulcrum Boundary is the action boundary for MCP-native agents. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary.

The flagship preview profile is Secure GitHub MCP: a fixture-backed GitHub path
showing write-after-taint denial before private-repo mutation.

## Forbidden Release Language

Do not use these as public capability claims:

- Do not claim universal prompt-injection prevention.
- Do not claim production Secure GitHub.
- Do not claim official named third-party scanner integration or compatibility.
- Do not claim all adapters production.
- Do not claim generated policies are production-complete.
- Do not claim dashboard monitoring.
- Do not claim Boundary protects tools that bypass Boundary.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Docs Checked

- `README.md`
- `docs/INSTALL.md`
- `docs/CLAIMS_LEDGER.md`
- `claims/boundary_claims.yaml`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/LAUNCH_TRUTH_FREEZE.md`
- `docs/RELEASE_TRUTH_FIREWALL_SECUREGITHUB.md`
- `docs/firewall/EXTERNAL_INVENTORY_INGEST.md`
- `docs/firewall/GITHUB_ACTION.md`
- `docs/firewall/DASHBOARD.md`
- `docs/firewall/RISK_GRAPH_POLICY_GENERATION.md`
- `docs/DEMO_GITHUB_LETHAL_TRIFECTA.md`
- `docs/PUBLIC_RELEASE_COPY.md`
- `docs/LANGUAGE_SYSTEM.md`
- `docs/COPY_RULES.md`
- `CHANGELOG.md`

## Drift Found

- README's five-minute demo described the manual walkthrough but did not put the
  new one-command `boundary demo github-lethal-trifecta` path first.
- `docs/RELEASE_TRUTH_FIREWALL_SECUREGITHUB.md` still said the optional
  repo-scanning GitHub Action was split to a follow-up package, which is stale
  now that the repo-local MCP audit action has landed.
- `docs/LAUNCH_TRUTH_FREEZE.md` did not yet name the final public hardening
  train after NDJSON ingest, GitHub Action, and install/release polish.

## Drift Fixed

- Put `boundary demo github-lethal-trifecta` at the start of README's
  five-minute demo path.
- Added this final public truth report.
- Added a superseding note to the earlier Firewall + Secure GitHub truth report
  and replaced the stale GitHub Action follow-up line.
- Added a final public release truth section to `docs/LAUNCH_TRUTH_FREEZE.md`.
- Added a changelog entry for this final public truth report.
