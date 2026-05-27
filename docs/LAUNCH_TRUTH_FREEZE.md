# Launch Truth Freeze

This file records the release-facing truth for Fulcrum Boundary v0.2.0. It is a claims boundary for the OSS release surface, not a competitor benchmark or marketing claim registry.

## Product Identity

| Surface | Release value |
|---|---|
| OSS project name | Fulcrum Boundary |
| GitHub repository | `Fulcrum-Governance/Fulcrum-Boundary` |
| Go module path | `github.com/fulcrum-governance/fulcrum-boundary` |
| CLI binary | `boundary` |
| First release campaign | MCP Safety Gateway |
| Primary release claim | Boundary evaluates an agent action before it reaches the privileged tool when the deployment routes that action through Boundary. |

Historical names and repository redirects are intentionally omitted from release-facing docs. Public setup instructions should point to the current module path and repository only.

## Rename History

### 2026-05-27: Fulcrum-Boundary repo-family alignment

- Old repository: `Fulcrum-Governance/Boundary`
- Old Go module path: `github.com/fulcrum-governance/boundary`
- New repository: `Fulcrum-Governance/Fulcrum-Boundary`
- New Go module path: `github.com/fulcrum-governance/fulcrum-boundary`
- Reason: align the Boundary repository with the Fulcrum repo-family naming convention used by `fulcrum-io`, `fulcrum-trust`, and `Fulcrum-Proofs`.
- Go proxy note: `proxy.golang.org` may continue serving cached `@latest` metadata for the previous module path until it re-polls. Commit-pinned and tag-pinned installs avoid that transient cache window.

Verification recorded 2026-05-27:

- `go mod tidy && go build ./...` passed for the root module.
- `go mod tidy && go build ./...` passed for all seven nested modules: `adapters/grpc`, `examples/custom-interceptor`, `examples/http-middleware`, `examples/mcp-proxy`, `examples/rate-limit`, `examples/redis-trust`, and `examples/simple`.
- `env -u GOROOT go test ./... -short -count=1 -timeout 5m` passed across 19 root packages.
- `go vet ./...` passed.
- `git ls-files '*.go' | xargs gofmt -l` returned no files.

## What v0.2.0 Proves

The MCP Safety Gateway demo proves the release spine:

- a safe `SELECT` request is allowed through Boundary
- a destructive demo `DROP TABLE` request is denied before execution
- a direct bypass attempt from the demo agent fails by network topology
- every verdict emits a structured decision record
- the Postgres path uses an AST guard for statement classification
- decision records include verifiable request, policy bundle, and decision hashes

The bypass claim is scoped to the Docker demo topology. Production deployments must enforce the same sole-route constraint with their own infrastructure controls.

## What v0.2.0 Does Not Claim

Fulcrum Boundary v0.2.0 does not claim:

- general SQL firewall coverage
- universal SQL injection prevention
- signed receipts by default
- multi-agent coordination governance
- benchmark superiority
- compliance certification

Receipt-grade means hash-verifiable decision records. Do not imply signatures
are required or enabled by default.

## 2026-05-27: Post-Audit Adapter State

The post-audit adapter work updates the release-facing adapter state without
changing the public release boundary:

- MCP is the only production adapter.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A are preview adapters.
- Managed Agents remains preview until a live upstream conformance run is
  recorded with operator-owned credentials.
- A2A remains preview until live protocol conformance and deployment bypass
  evidence are recorded.
- CodeExec is policy-gated execution unless a real named sandbox boundary is
  implemented, tested, and documented.
- gRPC governs unary RPCs through the interceptor; streaming workloads remain
  preview until per-message governance is implemented and tested.
- Webhook separates informational post-execution audit mode from execution
  pre-approval mode. Only execution mode can deny before forwarding, and it
  requires deployment evidence that Boundary is the sole downstream action path.

The current repo does not record user adoption or customer deployment evidence.
Release language should describe implemented capabilities and operator
deployment requirements, not production adoption.

## 2026-05-27: Firewall + Secure GitHub Release Train

The Firewall + Secure GitHub release train adds a claim-gated local MCP
Firewall surface and a preview Secure GitHub MCP profile without changing the
production adapter boundary:

- MCP remains the only production adapter.
- MCP Firewall inventory is read-only and does not mutate MCP client configs.
- MCP Firewall risk graphs and generated policies are starter/operator-review
  surfaces.
- MCP Firewall install/uninstall is local, explicit, reversible, and does not
  claim protection by itself.
- Descriptor locks verify local descriptor drift. They do not prove an upstream
  MCP server is safe.
- Redteam packs are safe fixture attacks with no real secrets and no live system
  mutation.
- Secure GitHub is a preview fixture profile for write-after-taint denial before
  upstream GitHub mutation.
- Secure GitHub does not claim live GitHub App conformance or production bypass
  resistance.
- The dashboard is local-only visibility over local artifacts. It is not hosted
  monitoring and not runtime protection by itself.

Release-facing demo language should lead with the concrete poisoned GitHub
issue to private-repo mutation attempt, then state the proof boundary: fixture
write-after-taint denial before upstream, with production Secure GitHub gated on
live GitHub App conformance and deployment bypass evidence.

## 2026-05-27: Final Public Boundary Release Truth

The final public hardening train keeps the same claim boundary while making the
developer path and machine-ingest surfaces easier to verify:

- `boundary selftest` is the no-credential local release smoke test.
- `boundary demo github-lethal-trifecta` is the one-command fixture demo for
  the Secure GitHub write-after-taint path.
- `boundary inventory --format ndjson` emits the machine-readable Boundary
  inventory record stream.
- `boundary inventory ingest` maps Boundary, generic, and external MCP
  inventory NDJSON into Boundary inventory records.
- External MCP ingest is not an official named third-party scanner integration
  or compatibility claim.
- The MCP audit GitHub Action scans repo-local MCP configs by default and emits
  Markdown and optional SARIF reports. It is CI audit/reporting only.
- Generated policies remain starter policies for operator review.
- The dashboard remains local-only visibility over local artifacts.
- Secure GitHub remains preview until live GitHub App conformance and
  deployment bypass evidence are recorded.

The final public release report is
[`docs/RELEASE_TRUTH_PUBLIC.md`](./RELEASE_TRUTH_PUBLIC.md).

## Verified Release Surface

| Surface | Status |
|---|---|
| `cmd/boundary/` CLI | Present |
| `examples/mcp-postgres-gateway/` Docker demo | Present |
| YAML policy loading | Present |
| Structured decision records | Present |
| Receipt verification | Present |
| Trust integration | Present |
| Adaptive termination | Present |
| `docs/RECEIPTS.md` | Present |
| `docs/TRUST_INTEGRATION.md` | Present |
| `docs/ADAPTIVE_TERMINATION.md` | Present |
| `docs/DECISION_RECORDS.md` | Present |
| `docs/LIMITATIONS.md` | Present |
| `docs/BOUNDARY_CONDITIONS.md` | Present |
| `docs/THREAT_MODEL.md` | Present |
| `SECURITY.md` | Present |
| `CONTRIBUTING.md` | Present |
| `CHANGELOG.md` v0.2.0 section | Present |

## Language Lock

Use:

- Fulcrum Boundary
- Boundary
- `boundary` CLI
- MCP Safety Gateway
- action boundary
- pre-execution control
- decision record
- receipt-grade decision record

Do not use as public release names:

- Zero-Trust MCP Router
- MCP gateway as the whole project identity
- governance platform as the lead phrase
- signed receipts by default

Adapters change. The boundary does not.
