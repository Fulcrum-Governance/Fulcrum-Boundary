# Final Public Release Truth

Date: 2026-05-27

Audited base commit SHA: `33d31bb89dad7799dfd13078e666dae23e525962`

Branch: `main`

Current release tag: `v0.4.0`

## Summary

This report reconciles the public Boundary release surface after the v0.4.0
Command Boundary packaging and post-tag smoke verification.

The final public truth is:

- MCP remains the production adapter path.
- Secure GitHub remains the flagship preview MCP profile and is fixture-backed.
- Command Boundary is delivered as a preview surface in v0.4.0.
- Command Boundary governs only commands routed through `boundary command run`,
  `boundary shell`, or project-local shims.
- Direct shell access, CI jobs, SSH sessions, cron jobs, editor tasks, arbitrary
  local processes, and direct file edits remain outside Boundary unless they are
  routed through Boundary.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, Secure GitHub, and Command
  Boundary remain preview adapter/profile/surface areas.
- Generated policies are starter policies for operator review.
- Dashboard output is local-only artifact visibility, not hosted monitoring.
- External inventory ingest is Boundary-owned MCP inventory mapping, not an
  official third-party scanner integration or compatibility claim.
- The GitHub Action is repo-local CI audit/reporting only.
- Boundary governs routed tools. Tools that bypass Boundary are outside the
  governed route.
- The public Go install path requires Go 1.25+.
- Public install examples use the repeatable `@v0.4.0` release tag; `@latest`
  resolves to `v0.4.0`.
- Public action examples use `@v0.4.0` for repeatable CI behavior.

## Test Commands

| Command | Result |
| --- | --- |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `make docs-build` | Pass |
| `make release-check` | Pass |
| `go test ./internal/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Command -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |
| `GOPROXY=direct GOBIN="$tmp/bin" go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0` | Pass: installed binary ran `boundary selftest`, `boundary demo github-lethal-trifecta`, and `boundary command classify -- git push origin main` successfully |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | Pass: `@latest` resolves to `v0.4.0` |

`make release-check` also runs the root suite, the gRPC nested module suite,
the test suite, claims tests, policy verification, receipt verification help,
`boundary selftest`, and `boundary demo github-lethal-trifecta`.

## README First-Run Status

README presents the first-run path before architecture:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
boundary selftest
```

It also gives the fixture-only demo path:

```bash
boundary demo github-lethal-trifecta
```

The demo remains credential-free and performs no live GitHub calls or real
mutation.

## Claims Status

| Claim | Status | Final release truth |
| --- | --- | --- |
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
| BND-CLAIM-CMD-001 | delivered | Boundary provides preview project-local command governance for routed commands only. |
| BND-CLAIM-CMD-002 | delivered | Command Boundary redteam packs are fixture-only and perform no live mutation. |

`delivered` Command Boundary claims are delivered preview claims. They do not
upgrade Command Boundary to production command governance.

## Feature Status

| Feature | Status | Release truth |
| --- | --- | --- |
| `boundary selftest` | delivered | No-credential local smoke test over inventory, risk graph, starter policies, descriptor drift, redteam, Secure GitHub live fail-closed behavior, and decision records. |
| `boundary demo github-lethal-trifecta` | delivered | Fixture-only demo of write-after-taint denial before upstream GitHub mutation. |
| Inventory JSON/Markdown/SARIF | delivered | Local MCP inventory reporting surfaces. |
| Inventory NDJSON | delivered | Versioned record stream for tool ingestion. |
| External inventory ingest | delivered | Boundary, generic, and external MCP inventory NDJSON mapping. |
| GitHub Action MCP audit | delivered | Repo-local MCP config audit with Markdown and optional SARIF. |
| Install/release workflow | delivered | `make selftest`, `make demo-github`, `make release-check`, and `docs/INSTALL.md`. |
| Local dashboard | delivered | Local-only artifact view. |
| Command classification | delivered preview | Classifies command argv without execution and redacts secret-looking arguments. |
| Command run wrapper | delivered preview | Evaluates wrapper-routed commands before execution and blocks denied or approval-required commands. |
| Project shims and shell | delivered preview | Routes selected project-local commands through Boundary without global shell mutation. |
| Command redteam packs | delivered preview | Fixture-only cleanup, secret-exfiltration, and repo-mutation packs. |

## Adapter, Profile, And Surface Status

| Adapter/Profile/Surface | Status | Release truth |
| --- | --- | --- |
| MCP | production | Production JSON-RPC MCP proxy path with lifecycle tests; deployment bypass proof remains an operator topology requirement. |
| CLI | preview | Governed wrapper execution works; production requires sole-wrapper deployment evidence. |
| CodeExec | preview | Policy-gated execution works; secure sandboxing is not claimed without a real named boundary. |
| gRPC | preview | Unary lifecycle works with governance trailers; streaming workloads remain preview. |
| Managed Agents | preview | Preview proxy and conformance harness exist; production requires live upstream conformance. |
| Webhook | preview | Informational and execution modes are split; production requires sole-path deployment evidence. |
| A2A | preview | Governed lifecycle exists against a documented snapshot; production requires live protocol conformance. |
| Secure GitHub | preview | Fixture-backed Secure MCP profile; production requires live GitHub App conformance and deployment bypass evidence. |
| Command Boundary | preview | Project-local command governance for commands routed through `boundary command run`, `boundary shell`, or project-local shims. |

## User-Install Status

The documented repeatable install path is:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
```

Requires Go 1.25+.

`@latest` has been verified through `proxy.golang.org` and resolves to
`v0.4.0`. README keeps `@v0.4.0` as the primary copy/paste command for
repeatability.

No Homebrew, package-manager, or hosted distribution channel is claimed.

## GitHub Action Ref Status

The MCP audit action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.4.0
```

Use the release tag for repeatable CI behavior. SARIF upload examples must
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

- Secure GitHub remains preview until deployment bypass evidence is recorded.
  The opt-in live conformance harness covers read-taint evidence and denied
  write-after-taint no-mutation proof for an operator-owned test repository.
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
- Command Boundary remains preview until deployment evidence shows Boundary is
  the relevant command path for a protected project or workflow.
- Direct file edits outside routed command paths remain a v0.6 design gap.
- Full GitHub MCP tool catalog coverage remains deferred.

## Approved Release Language

Fulcrum Boundary is the action boundary for MCP-native agents. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary.

Fulcrum Boundary v0.4.0 adds Command Boundary preview: project-local command
classification and wrapper-routed command governance through
`boundary command run`, `boundary shell`, and project-local shims.

Command Boundary is preview. Direct shell access, CI jobs, SSH sessions, and
direct file edits remain outside Boundary unless they are routed through
Boundary.

## Forbidden Release Language

Do not use these as public capability claims:

- Do not claim universal prompt-injection prevention.
- Do not claim production Secure GitHub.
- Do not claim official named third-party scanner integration or compatibility.
- Do not claim all adapters production.
- Do not claim generated policies are production-complete.
- Do not claim dashboard monitoring.
- Do not claim Boundary protects tools that bypass Boundary.
- Do not claim Boundary controls all shell commands.
- Do not claim Boundary protects direct shell access.
- Do not claim Boundary prevents every overeager agent action.
- Do not claim Boundary provides production command governance.
- Do not claim Boundary governs direct file edits outside routed command paths.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Docs Checked

- `README.md`
- `docs/INSTALL.md`
- `docs/CLI_REFERENCE.md`
- `docs/CLAIMS_LEDGER.md`
- `claims/boundary_claims.yaml`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/RELEASE_TRUTH_REPO_POLISH.md`
- `docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md`
- `docs/RELEASE_TRUTH_V040.md`
- `docs/LAUNCH_TRUTH_FREEZE.md`
- `docs/PUBLIC_RELEASE_COPY.md`
- `docs/command-boundary/`
- `docs-site/`
- `docs/firewall/GITHUB_ACTION.md`
- `actions/mcp-audit/action.yml`
- `CHANGELOG.md`

## Drift Fixed

- Updated active public truth from v0.3.0 to v0.4.0.
- Added Command Boundary preview status to public release truth.
- Added current `@v0.4.0` install and GitHub Action references.
- Preserved historical v0.3.0 references in release notes, changelog history,
  launch-truth history, and planning artifacts.
