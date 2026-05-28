# Release Truth: Repo Polish And Vendor-Neutral Surface

Date: 2026-05-28

Audited base commit: `77474b22616f83aedf18fb6eea284349211aae8d`

Branch: `release/v060-package`

Current release target: `v0.6.0`

## Summary

This report reconciles the public Boundary repository surface after the v0.6.0
Edit Boundary preview packaging pass.

Current public posture:

- Fulcrum Boundary is the action boundary for MCP-native agents.
- The first-run path uses fixture-only, no-credential checks.
- Secure GitHub remains preview.
- Secure GitHub includes fixture proof plus an opt-in live GitHub App
  conformance harness for read-taint and denied-write no-mutation evidence.
- Command Boundary is preview and applies only to routed project-local command
  paths.
- Edit Boundary is preview and applies only to proposed file mutations routed
  through Boundary edit envelopes.
- External MCP inventory ingest is vendor-neutral.
- Repo presentation guidance is documented without fake badges or fake adoption
  signals.
- Public install examples use the repeatable `@v0.6.0` release tag.
- The public Go install path requires Go 1.25+.
- GitHub Action examples use `@v0.6.0` for repeatable CI behavior.

## Test Commands

| Command | Result |
| --- | --- |
| Legacy named-vendor codename grep | Pass: zero matches |
| Legacy named-vendor spaced-codename grep | Pass: zero matches |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `make docs-build` | Pass |
| `make release-check` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |

`make release-check` also runs:

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

Post-tag install smoke is recorded separately in
[`docs/RELEASE_TRUTH_V060_POSTRELEASE.md`](./RELEASE_TRUTH_V060_POSTRELEASE.md).

## No-Vendor-Reference Check

Status: pass.

The forbidden vendor source terms are absent from public repo content, and the
public vendor-reference guard passes.

## README First-Run Status

Status: pass with `@v0.6.0`.

README now uses:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.0
boundary selftest
boundary demo github-lethal-trifecta
```

The public Go install path requires Go 1.25+.

## Docs-Site Status

Status: buildable repository artifact.

`make docs-build` passed and built the MkDocs site locally. Publication depends
on repository GitHub Pages settings and the `Docs` workflow completing on
`main`.

## GitHub Action Status

Status: delivered as repo-local CI audit/reporting.

The MCP audit action documentation states that the action audits repository MCP
configs, emits Markdown and optional SARIF reports, and does not provide
runtime protection unless the relevant tool calls are routed through Boundary.

Action examples use `@v0.6.0` for repeatable CI behavior. SARIF upload examples
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

## Secure GitHub Status

Status: delivered preview harness; still preview overall.

Approved Secure GitHub copy:

> Secure GitHub can use real GitHub App credentials to read real GitHub context,
> mark the session tainted, and deny a protected write-after-taint action before
> any upstream GitHub mutation client call executes.

Required caveat:

> Secure GitHub remains preview. Production status still requires deployment
> bypass evidence and broader live coverage.

The v0.6.0 packaging pass does not claim production Secure GitHub, full GitHub
MCP catalog coverage, universal prompt-injection defense, or deployment bypass
resistance.

## Command Boundary Status

Status: delivered preview.

Approved Command Boundary copy:

> Fulcrum Boundary v0.4.0 added Command Boundary preview: project-local command
> classification and wrapper-routed command governance through
> `boundary command run`, `boundary shell`, and project-local shims.

Required caveat:

> Command Boundary is preview. Direct shell access, CI jobs, and SSH sessions
> remain outside Command Boundary unless routed through Boundary command
> wrappers or project-local shims. Direct file edits are covered only by Edit
> Boundary when they route through edit envelopes.

Command Boundary does not claim shell sandboxing, global shell control,
production command governance, direct file-edit governance, SSH control, CI
control, or universal coding-agent safety.

## Edit Boundary Status

Status: delivered preview.

Approved Edit Boundary copy:

> Boundary provides preview Edit Boundary governance for proposed file
> mutations routed through Boundary edit envelopes.

Required caveat:

> Edit Boundary is preview. Direct editor writes, direct filesystem writes,
> direct `git apply`, shell redirection, IDE saves, CI jobs, and arbitrary
> processes remain outside Boundary unless routed through Boundary edit
> envelopes.

Edit Boundary does not claim direct editor-write protection, arbitrary
filesystem interception, filesystem sandboxing, IDE control, or production edit
governance.

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

## Approved Copy

Fulcrum Boundary is the action boundary for MCP-native agents. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary.

Fulcrum Boundary v0.6.0 packages Edit Boundary preview: proposed file
mutations can be classified and gated before they apply when the mutation is
submitted through a Boundary edit envelope.

## Forbidden Copy

Do not use these as public capability claims:

- legacy named-vendor codename
- legacy named-vendor style variant
- official third-party scanner integration
- claims of universal prompt-injection defense
- production Secure GitHub
- Do not claim Secure GitHub fully secures GitHub
- live conformance proves deployment bypass resistance
- claims that every adapter is production
- generated policies are production-complete
- dashboard monitoring
- Boundary protects tools that bypass Boundary
- Boundary controls all shell commands
- Boundary protects direct shell access
- Boundary prevents every overeager agent action
- Boundary provides production command governance
- Boundary governs direct file edits outside routed edit envelopes
- Boundary controls all file writes
- Boundary protects direct editor writes
- Boundary provides filesystem sandboxing
- Boundary provides production edit governance

The exact legacy vendor codename terms are intentionally omitted from this
repository artifact so the required zero-match guard remains machine-enforced.
The other phrases may appear only in claim-control, language-control,
historical, or explicit limitation context.

## Remaining Work

- Upload a PNG social preview manually if GitHub repository settings reject the
  repo-owned SVG source.
- Record the first terminal screenshot or GIF using the final `@v0.6.0`
  install command.
- Keep Command Boundary preview-scoped until deployment evidence shows Boundary
  is the relevant command path for a protected project or workflow.
- Keep Secure GitHub preview-scoped until deployment bypass proof and broader
  live coverage exist.
- Keep Edit Boundary preview-scoped until deployment evidence shows edit
  proposals are routed through Boundary edit envelopes.
