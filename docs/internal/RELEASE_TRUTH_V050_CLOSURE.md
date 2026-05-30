# Release Truth: v0.5.0 Closure Sanity

Date: 2026-05-28

Status: passed.

This document closes the v0.5.0 Secure GitHub live conformance preview line
before the v0.6.0 Filesystem/Edit Boundary work begins.

## Release Objects

- Release tag: `v0.5.0`
- Tag object: `a2ed93ecc1b64208434a065ce396cd73a44e1e1d`
- Tag target commit: `3d22efe7e7ce20499a4196e95b86f989c1007651`
- Current `main` commit audited: `09e0017f3dbd34054ceb9defb5d2d9c830c7dc36`
- GitHub release: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.5.0

## Install Truth

Primary install examples in `README.md`, `docs/INSTALL.md`, and `docs-site/`
use:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.5.0
```

`@latest` resolves to `v0.5.0`:

```bash
GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest
```

Result:

```text
github.com/fulcrum-governance/fulcrum-boundary v0.5.0
```

The post-release verification in `docs/RELEASE_TRUTH_V050_POSTRELEASE.md`
records successful direct `@v0.5.0` install, public proxy `@latest` install,
`boundary selftest`, `boundary demo github-lethal-trifecta`, and
`boundary command classify -- git push origin main`.

## Secure GitHub Live Conformance Status

Secure GitHub remains preview. The v0.5.0 line delivered the opt-in live
conformance harness and no-mutation proof path; it did not promote Secure
GitHub to production.

Current truth:

- Live conformance is opt-in with `BOUNDARY_GITHUB_CONFORMANCE=true`.
- Missing GitHub App environment fails closed when live conformance is enabled.
- Without opt-in, conformance commands skip without network calls.
- Denied write-after-taint conformance records `upstream_called=false` and
  `github_mutation_called=false`.
- Operator-owned live conformance evidence has not been recorded unless a
  sanitized transcript evidence hash is present.
- Production still requires deployment bypass proof and broader live coverage.

Claim split:

- `BND-CLAIM-018` is delivered for the opt-in Secure GitHub live conformance
  harness.
- `BND-CLAIM-019` remains partial until an operator-owned live run is executed
  and sanitized evidence is recorded.

## Command Boundary Status

Command Boundary remains delivered preview for routed command paths only.
Direct shell access, CI, SSH, cron, scripts, and processes that do not route
through `boundary command run`, `boundary shell`, or project-local shims remain
outside Boundary.

## Stale Reference Check

Active install surfaces were checked:

```bash
git grep -n '@v0.4.0' README.md docs/INSTALL.md docs-site || true
git grep -n '@main' README.md docs/INSTALL.md docs-site || true
```

Result: no active matches.

Historical references to earlier tags may remain in changelog and release
history documents.

## Verification

Commands run on 2026-05-28:

| Command | Result |
| --- | --- |
| `git grep -n '@v0.4.0' README.md docs/INSTALL.md docs-site || true` | pass, no active matches |
| `git grep -n '@main' README.md docs/INSTALL.md docs-site || true` | pass, no active matches |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | pass, resolved `v0.5.0` |
| `make release-check` | pass |
| `make docs-build` | pass |
| `go test ./claims/... -count=1` | pass |
| `go test ./... -short -count=1 -timeout 5m` | pass |

## Remaining Preview Gates

- Secure GitHub production status requires deployment bypass proof.
- Operator-owned Secure GitHub live conformance requires a sanitized transcript
  evidence hash before it can be claimed as recorded.
- Command Boundary remains preview and routed-path-only.
- Filesystem/Edit Boundary is not part of v0.5.0; it begins in the v0.6.0
  release train.
