# Release Truth: v0.5.0 Secure GitHub Live Conformance Preview

Date: 2026-05-28

Branch: `feat/securegithub-live-conformance-v050`

Release train: Secure GitHub Live Conformance Preview

## Summary

v0.5.0 is scoped to Secure GitHub live conformance preview. It does not change
MCP production status, Command Boundary preview status, or the fixture-only
v0.3/v0.4 demo path.

Final v0.5.0 truth:

- Secure GitHub remains preview.
- Secure GitHub fixture proof remains available and unchanged.
- Live conformance is opt-in with `BOUNDARY_GITHUB_CONFORMANCE=true`.
- Missing required GitHub App environment fails closed when conformance is
  enabled.
- Without opt-in, conformance commands skip without network calls.
- Live read conformance can read a real GitHub issue through GitHub App
  installation auth and record sanitized taint evidence.
- Denied write-after-taint conformance denies before any GitHub mutation client
  call executes.
- Sanitized transcripts include hashes and booleans, not raw issue content or
  credentials.
- Production status still requires deployment bypass proof.
- Operator-owned live GitHub conformance was not run in this branch because no
  credentials were provided; the opt-in harness, skip path, missing-env
  fail-closed path, sanitized transcript path, and denied-write no-mutation path
  are covered by automated tests.

## Test Commands

| Command | Result |
|---|---|
| `go test ./adapters/securegithub/... -count=1 -timeout 5m` | pass |
| `go test ./internal/boundarycli/... -count=1 -timeout 5m` | pass |
| `go test ./tests/conformance/secure_github/... -count=1 -timeout 5m` | pass |
| `go test ./tests/redteam/... -run GitHub -count=1 -timeout 5m` | pass |
| `go test ./claims/... -count=1` | pass |
| `make docs-build` | pass |
| `make release-check` | pass |
| `go test ./... -short -count=1 -timeout 5m` | pass |
| `golangci-lint run --timeout=5m` | pass, `0 issues` |
| `go run github.com/securego/gosec/v2/cmd/gosec@v2.26.1 ./adapters/securegithub/... ./internal/boundarycli/...` | pass, `0 issues` |
| `CodeQL` | pass after integer-conversion upper-bound fix |

## Secure GitHub Status

| Surface | Status | Truth |
|---|---|---|
| Fixture setup/serve | delivered preview | No credentials, no network calls, no live mutation. |
| Fixture redteam | delivered preview | Demonstrates the tested write-after-taint deny path with fixture data. |
| GitHub App auth | delivered preview | Generates RS256 JWTs and exchanges them for installation tokens at runtime. |
| Live read conformance | delivered preview | Reads a configured real issue and records sanitized taint evidence. |
| Live denied-write conformance | delivered preview | Denies protected write-after-taint before the GitHub mutation client is reached. |
| Live deployment bypass proof | not delivered | Still required before production. |

## MCP Status Unchanged

MCP remains the production adapter path. v0.5.0 does not change MCP Firewall
claims or MCP adapter maturity.

## Command Boundary Status Unchanged

Command Boundary remains delivered preview for routed project-local command
paths. v0.5.0 does not add Filesystem/Edit Boundary or direct file-edit
governance.

## Approved Secure GitHub Copy

Use this copy for public docs:

> Secure GitHub can use real GitHub App credentials to read real GitHub context,
> mark the session tainted, and deny a protected write-after-taint action before
> any upstream GitHub mutation client call executes.

Supporting copy:

- Live conformance is opt-in and skips by default.
- The denied-write conformance path records `upstream_called=false` and
  `github_mutation_called=false`.
- Secure GitHub remains preview until deployment bypass proof exists.

## Forbidden Secure GitHub Copy

Do not use these statements as public capability claims:

- Do not claim Secure GitHub is production.
- Do not claim Boundary fully secures GitHub.
- Do not claim Boundary prevents every malicious issue.
- Do not claim Boundary prevents universal prompt injection.
- Do not claim live conformance proves deployment bypass resistance.
- Do not claim Boundary mutates live repositories during conformance by default.
- Do not claim Boundary covers the full GitHub MCP catalog.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Remaining Limits

- Direct GitHub API access remains outside Boundary unless removed by
  deployment topology.
- Direct upstream GitHub MCP server access remains outside Boundary unless
  removed by deployment topology.
- `gh`, `git`, browser, CI, SSH, or other credentialed paths remain outside
  Secure GitHub unless routed through Boundary.
- The live harness covers configured read and denied write-after-taint evidence,
  not the full GitHub MCP catalog.
- Production requires deployment-specific bypass evidence.
