# Plan: Secure GitHub Live Conformance Preview v0.5

Date: 2026-05-28

Branch: `feat/securegithub-live-conformance-v050`

## Goal

Add an opt-in Secure GitHub live conformance preview harness without changing
the default fixture demo or claiming production GitHub security.

## Scope

- GitHub App JWT and installation-token auth.
- Live GitHub issue read evidence through an operator-owned installation.
- Sanitized live-read transcript.
- Denied write-after-taint no-mutation proof with `upstream_called=false` and
  `github_mutation_called=false`.
- CLI surface under `boundary secure github conformance`.
- Claims, readiness, docs, docs-site, and release truth.

## Non-Goals

- No production Secure GitHub claim.
- No live repository mutation by default.
- No full GitHub MCP catalog claim.
- No universal prompt-injection claim.
- No Filesystem/Edit Boundary implementation.
- No weakening fixture tests.
- No credential or raw transcript storage in the repo.

## Verification

- `go test ./adapters/securegithub/... -count=1 -timeout 5m`
- `go test ./internal/boundarycli/... -count=1 -timeout 5m`
- `go test ./tests/conformance/secure_github/... -count=1 -timeout 5m`
- `go test ./tests/redteam/... -run GitHub -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `make docs-build`
- `make release-check`
- `go test ./... -short -count=1 -timeout 5m`

## Closeout Rule

Update `docs/RELEASE_TRUTH_V050.md` with final pass/fail results before
tagging a v0.5.0 release.

