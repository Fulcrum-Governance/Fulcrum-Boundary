# Release Truth: Repo Polish And Vendor-Neutral Surface

Date: 2026-05-27

Audited base commit: `33d31bb89dad7799dfd13078e666dae23e525962`

Branch: `main`

Current release tag: `v0.4.0`

## Summary

This report reconciles the public Boundary repository surface after the
vendor-neutral cleanup, README/docs polish, docs-site skeleton, CLI output
polish, repository metadata pass, and v0.4.0 Command Boundary release
packaging.

Current public posture:

- Fulcrum Boundary is the action boundary for MCP-native agents.
- The first-run path uses fixture-only, no-credential checks.
- Secure GitHub remains preview and fixture-backed.
- Command Boundary is preview and applies only to routed project-local command
  paths.
- External MCP inventory ingest is vendor-neutral.
- Repo presentation guidance is documented without fake badges or fake adoption
  signals.
- Public install examples use the repeatable `@v0.4.0` release tag; `@latest`
  resolves to `v0.4.0`.
- The public Go install path requires Go 1.25+.
- GitHub Action examples use `@v0.4.0` for repeatable CI behavior.

## Test Commands

| Command | Result |
| --- | --- |
| Legacy named-vendor codename grep | Pass: zero matches |
| Legacy named-vendor spaced-codename grep | Pass: zero matches |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `make docs-build` | Pass |
| `make release-check` | Pass |
| `go test ./internal/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Command -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |
| `GOPROXY=direct GOBIN="$tmp/bin" go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0` | Pass |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | Pass: resolves to `v0.4.0` |
| `"$tmp/bin/boundary" selftest` | Pass |
| `"$tmp/bin/boundary" demo github-lethal-trifecta` | Pass |
| `"$tmp/bin/boundary" command classify -- git push origin main` | Pass |

`make release-check` also ran:

- `./scripts/assert-no-public-vendor-refs.sh`
- `go vet ./...`
- `go vet ./...` in `adapters/grpc`
- `go test ./... -count=1 -timeout 5m`
- `go test ./... -count=1 -timeout 5m` in `adapters/grpc`
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

Status: pass with `@v0.4.0`.

README now uses:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
boundary selftest
boundary demo github-lethal-trifecta
```

The `@v0.4.0` install was verified with `GOPROXY=direct` from a clean temporary
`GOBIN`. The installed `boundary` binary passed `selftest`,
`demo github-lethal-trifecta`, and `command classify -- git push origin main`
without credentials, network mutation, or live mutation.

`@latest` was also verified through `proxy.golang.org` and resolves to
`v0.4.0`. README keeps `@v0.4.0` as the primary copy/paste command for
repeatability.

The public Go install path requires Go 1.25+.

## Docs-Site Status

Status: buildable repository artifact.

`make docs-build` passed and built the MkDocs site locally. Publication depends
on repository GitHub Pages settings and the `Docs` workflow completing on
`main`.

## GitHub Action Status

Status: delivered as repo-local CI audit/reporting.

The MCP audit action documentation states that the action audits repository MCP
configs, emits Markdown and optional SARIF reports, and does not provide runtime
protection unless the relevant tool calls are routed through Boundary.

Action examples use `@v0.4.0` for repeatable CI behavior. SARIF upload examples
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

## Command Boundary Status

Status: delivered preview.

Approved Command Boundary copy:

> Fulcrum Boundary v0.4.0 adds Command Boundary preview: project-local command
> classification and wrapper-routed command governance through
> `boundary command run`, `boundary shell`, and project-local shims.

Required caveat:

> Command Boundary is preview. Direct shell access, CI jobs, SSH sessions, and
> direct file edits remain outside Boundary unless they are routed through
> Boundary.

Command Boundary does not claim shell sandboxing, global shell control,
production command governance, direct file-edit governance, SSH control, CI
control, or universal coding-agent safety.

## Repo Presentation Checklist

| Item | Status |
| --- | --- |
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

Fulcrum Boundary v0.4.0 adds Command Boundary preview: project-local command
classification and wrapper-routed command governance through
`boundary command run`, `boundary shell`, and project-local shims.

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
- Boundary controls all shell commands
- Boundary protects direct shell access
- Boundary prevents every overeager agent action
- Boundary provides production command governance
- Boundary governs direct file edits outside routed command paths

The exact legacy vendor codename terms are intentionally omitted from this
repository artifact so the required zero-match guard remains machine-enforced.
The other phrases may appear only in claim-control, language-control,
historical, or explicit limitation context.

## Remaining Work

- Upload a PNG social preview manually if GitHub repository settings reject the
  repo-owned SVG source.
- Record the first terminal screenshot or GIF using the final `@v0.4.0`
  install command.
- Keep Command Boundary preview-scoped until deployment evidence shows Boundary
  is the relevant command path for a protected project or workflow.
- Keep Secure GitHub preview-scoped until deployment bypass proof exists.
- Design v0.6 Filesystem/Edit Boundary for direct file-write governance.
