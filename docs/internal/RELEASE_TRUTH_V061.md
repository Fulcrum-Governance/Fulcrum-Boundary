# Release Truth: v0.6.1 Utility Train

Date: 2026-05-28

Audited base commit: `cf8e6595829090edfb567c3a6622aa40c40524e3`

Release packaging branch: `release/v06x-utility-consolidation`

Release train: v0.6.x utility consolidation

## Summary

v0.6.1 packages the already-merged utility train into the active public install
and release surface. It does not add a new governed action surface and does not
upgrade any preview surface to production.

Final v0.6.1 truth:

- MCP remains the production adapter path.
- Secure GitHub remains preview.
- Command Boundary remains preview and routed-path-only.
- Edit Boundary remains preview and routed-edit-envelope-only.
- `boundary version` is delivered utility metadata output, not cryptographic
  provenance.
- `boundary demo action-boundary` is fixture-only and does not use credentials,
  network calls, or live mutation.
- `boundary doctor` reports local diagnostics and bypass caveats; it does not
  prove production route protection.
- `boundary evidence bundle` and `boundary evidence verify` create and verify
  local evidence artifacts; they do not prove production deployment safety or
  close deployment bypasses by themselves.
- Active public install and GitHub Action examples use `@v0.6.1` for
  repeatable behavior.

## Version Decision

This train is packaged as `v0.6.1`, not `v0.7.0`, because the shipped scope is
release utility, diagnostics, and evidence packaging over the existing v0.6.0
surface. It adds public CLI commands, but it does not introduce a new governed
action surface, change adapter maturity, or alter the core governance contract.

## Release Packaging

The v0.6.1 packaging pass moves the already-merged utility commands into the
active public install and action examples:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.1
boundary selftest
boundary demo github-lethal-trifecta
```

GitHub Action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.6.1
```

## Test Commands

| Command | Result |
|---|---|
| `make release-check` | pass |
| `make docs-build` | pass |
| `go test ./claims/... -count=1` | pass |
| `go test ./... -count=1 -timeout 5m` | pass |
| `go vet ./...` | pass |
| `git ls-files '*.go' \| xargs gofmt -l` | pass, no output |
| `git diff --check` | pass |

`make release-check` includes `boundary version`, `boundary demo
action-boundary`, `boundary doctor --json`, `boundary evidence bundle
--include-demo`, and `boundary evidence verify` in addition to the existing
tests, policy verification, selftest, and GitHub lethal-trifecta demo.

## Utility Status

| Surface | Status | Truth |
|---|---|---|
| `boundary version` | delivered | Reports local build metadata in text or JSON. Missing metadata is shown as `unknown`. |
| `boundary demo action-boundary` | delivered | Shows fixture-only MCP / Secure GitHub, Command Boundary, and Edit Boundary paths together without live mutation. |
| `boundary doctor` | delivered | Reports local routed-surface readiness and bypass caveats without network calls. |
| `boundary evidence bundle` | delivered | Creates local evidence artifacts with a manifest and SHA-256 hashes. |
| `boundary evidence verify` | delivered | Verifies bundle shape, artifact existence, hash integrity, declared schemas, and summary references. |
| `make release-check` | delivered | Exercises the utility commands as part of the repeatable release gate. |

## Claim Split

- `BND-CLAIM-UTIL-001` is delivered utility metadata output.
- `BND-CLAIM-UTIL-002` is delivered fixture-only action-boundary demo coverage.
- `BND-CLAIM-UTIL-003` is delivered local diagnostics and bypass caveat output.
- `BND-CLAIM-UTIL-004` is delivered local evidence bundle and verify coverage.

Delivered utility claims do not upgrade Secure GitHub, Command Boundary, or
Edit Boundary to production.

## MCP Status Unchanged

MCP remains the production adapter path. v0.6.1 does not change MCP Firewall
claims or MCP adapter maturity.

## Secure GitHub Status Unchanged

Secure GitHub remains preview. v0.6.1 does not change the v0.5.0 opt-in live
conformance harness, and production status still requires deployment bypass
proof and broader live coverage.

## Command Boundary Status Unchanged

Command Boundary remains delivered preview for routed project-local command
paths. Direct shell execution, CI jobs, SSH sessions, and direct file edits
remain outside Command Boundary unless routed through Boundary.

## Edit Boundary Status Unchanged

Edit Boundary remains delivered preview for proposed file mutations routed
through Boundary edit envelopes. Direct editor writes, direct filesystem
writes, shell redirection, direct `git apply`, IDE saves, CI jobs, and arbitrary
process writes remain outside Edit Boundary unless routed through Boundary edit
envelopes.

## Approved v0.6.1 Copy

Use this copy for public docs:

> Boundary v0.6.1 adds local utility commands for version reporting, fixture
> action-boundary demos, routed-surface diagnostics, and evidence bundle
> verification.

Supporting copy:

- Boundary can bundle fixture-safe release evidence for local review.
- Boundary can verify local evidence bundle hashes and summary references.
- Boundary can report routed-surface diagnostics and bypass caveats without
  network calls.

## Forbidden v0.6.1 Copy

Do not use these statements as public capability claims:

- Evidence bundles prove production safety.
- Doctor proves all routes are protected.
- The action-boundary demo proves all attacks are blocked.
- Version output proves cryptographic release provenance.
- v0.6.1 upgrades Secure GitHub, Command Boundary, or Edit Boundary to
  production.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.
