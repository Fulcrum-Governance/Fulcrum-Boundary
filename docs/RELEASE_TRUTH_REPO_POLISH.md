# Release Truth: Repo Polish And Vendor-Neutral Surface

Date: 2026-05-27

Audited base commit: `00ee273582aa7d275d04c37d78e40e6fc25cf117`

Branch: `main`

Release tag: `v0.3.0`

## Summary

This report reconciles the public Boundary repository surface after the
vendor-neutral cleanup, README/docs polish, docs-site skeleton, CLI output
polish, and repository metadata pass.

Final public posture:

- Fulcrum Boundary is the action boundary for MCP-native agents.
- The first-run path uses fixture-only, no-credential checks.
- Secure GitHub remains preview and fixture-backed.
- External MCP inventory ingest is vendor-neutral.
- Repo presentation guidance is documented without fake badges or fake
  adoption signals.
- Public install examples use the repeatable `@v0.3.0` release tag; `@latest`
  resolves to `v0.3.0`.
- The public Go install path requires Go 1.25+.
- GitHub Action examples use `@v0.3.0` for repeatable CI behavior.

## Test Commands

| Command | Result |
|---|---|
| Legacy named-vendor codename grep | Pass: zero matches |
| Legacy named-vendor spaced-codename grep | Pass: zero matches |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `./scripts/docs-build.sh` | Pass |
| `make release-check` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |
| `GOPROXY=direct GOBIN="$tmp/bin" go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0` | Pass |
| `GOPROXY=https://proxy.golang.org,direct go list -m -json github.com/fulcrum-governance/fulcrum-boundary@latest` | Pass: resolves to `v0.3.0` |
| `GOPROXY=https://proxy.golang.org,direct GOBIN="$tmp/bin" go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest` | Pass |
| `"$tmp/bin/boundary" selftest` | Pass |
| `"$tmp/bin/boundary" demo github-lethal-trifecta` | Pass |

`make release-check` also ran:

- `go test ./... -count=1 -timeout 5m`
- `cd adapters/grpc && go test ./... -count=1 -timeout 5m`
- `go test ./tests/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1 -timeout 5m`
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`
- `go run ./cmd/boundary verify-record --help`
- `go run ./cmd/boundary selftest`
- `go run ./cmd/boundary demo github-lethal-trifecta`

## No-Vendor-Reference Check

Status: pass.

The forbidden vendor source terms are absent from public repo content, and the
public vendor-reference guard passes.

## README First-Run Status

Status: pass with `@v0.3.0`.

README now uses:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0
boundary selftest
boundary demo github-lethal-trifecta
```

The `@v0.3.0` install was verified with `GOPROXY=direct` from a clean temporary
`GOBIN`. The installed `boundary` binary passed both `selftest` and
`demo github-lethal-trifecta` without credentials, network calls, or live
mutation.

`@latest` was also verified through `proxy.golang.org` and resolves to
`v0.3.0`. README keeps `@v0.3.0` as the primary copy/paste command for
repeatability.

The public Go install path requires Go 1.25+.

## Docs-Site Status

Status: buildable repository artifact.

`./scripts/docs-build.sh` passed and built the MkDocs site locally. Publication
depends on repository GitHub Pages settings and the `Docs` workflow completing
on `main`.

## GitHub Action Status

Status: delivered as repo-local CI audit/reporting.

The MCP audit action documentation states that the action audits repository MCP
configs, emits Markdown and optional SARIF reports, and does not provide runtime
protection unless the relevant tool calls are routed through Boundary.

Action examples use `@v0.3.0` for repeatable CI behavior. SARIF upload examples
include `contents: read` and `security-events: write` permissions.

The docs do not claim a Marketplace release, package distribution, or runtime
enforcement from the action.

## External Inventory Ingest Wording Status

Status: vendor-neutral.

Approved external ingest copy:

> Boundary can ingest external MCP inventory NDJSON and map recognizable MCP
> records into Boundary inventory records.

The docs say that Boundary does not depend on, shell out to, import, endorse, or
claim compatibility with any named third-party scanner.

## Repo Presentation Checklist

| Item | Status |
|---|---|
| Repository description documented | Pass |
| Live GitHub description updated | Pass |
| Repository topics documented | Pass |
| Live GitHub topics updated | Pass |
| Fake badge guard documented | Pass |
| README badges limited to real CI, Go Reference, Go Report Card, and license signals | Pass |
| Social preview source committed | Pass: `docs/assets/social-preview.svg` |
| Social preview upload caveat documented | Pass |
| First screenshot/GIF plan documented | Pass |

Approved repository description:

> The action boundary for MCP-native agents. See what your AI tools can do;
> block what they should not.

Approved topics:

- `mcp`
- `model-context-protocol`
- `ai-agents`
- `agent-security`
- `agent-governance`
- `mcp-security`
- `golang`
- `security-tools`
- `developer-tools`

## Approved Copy

Fulcrum Boundary is the action boundary for MCP-native agents. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary.

## Forbidden Copy

Do not use these as public capability claims:

- legacy named-vendor codename
- legacy named-vendor style variant
- official third-party scanner integration
- claims of universal prompt-injection defense
- production Secure GitHub
- claims that every adapter is production
- generated policies are production-complete
- dashboard monitoring
- Boundary protects tools that bypass Boundary

The exact legacy vendor codename terms are intentionally omitted from this
repository artifact so the required zero-match guard remains machine-enforced.
The other phrases may appear only in claim-control, language-control,
historical, or explicit limitation context.

## Remaining Work

- Upload a PNG social preview manually if GitHub repository settings reject the
  repo-owned SVG source.
- Record the first terminal screenshot or GIF using the final `@v0.3.0`
  install command.
