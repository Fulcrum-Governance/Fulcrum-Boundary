# Release Truth: v0.6.1 Postrelease Smoke

Date: 2026-05-28

Verification time: `2026-05-28T12:38:47Z`

Release tag: `v0.6.1`

Release commit: `64fca7f9612e8396bf90dd7463b97291c096f7c9`

GitHub release: <https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.6.1>

## Summary

The v0.6.1 tag and GitHub release are published. Fresh external install and
smoke verification passed for both the repeatable tag path and public
`@latest` resolution.

The release truth remains unchanged:

- MCP remains production.
- Secure GitHub remains preview.
- Command Boundary remains preview and routed-path-only.
- Edit Boundary remains preview and routed-edit-envelope-only.
- v0.6.1 adds utility, diagnostic, demo, and evidence-bundle surfaces. It does
  not add a new governed action surface and does not upgrade any preview
  surface to production.

## Release Publication

| Check | Result |
|---|---|
| `gh release view v0.6.1 --json tagName,targetCommitish,url,name,isDraft,isPrerelease,publishedAt` | pass |
| `git ls-remote --tags origin refs/tags/v0.6.1` | pass |

Release metadata:

- `tagName`: `v0.6.1`
- `targetCommitish`: `64fca7f9612e8396bf90dd7463b97291c096f7c9`
- `url`: <https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.6.1>
- `isDraft`: `false`
- `isPrerelease`: `false`
- `publishedAt`: `2026-05-28T12:37:11Z`

Remote tag metadata:

```text
64fca7f9612e8396bf90dd7463b97291c096f7c9 refs/tags/v0.6.1
```

## Direct Tag Install Smoke

Command:

```bash
GOBIN="$TMPDIR_SMOKE/bin" GOPROXY=direct \
  go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.1
"$TMPDIR_SMOKE/bin/boundary" version
"$TMPDIR_SMOKE/bin/boundary" selftest
"$TMPDIR_SMOKE/bin/boundary" demo github-lethal-trifecta
"$TMPDIR_SMOKE/bin/boundary" demo action-boundary
"$TMPDIR_SMOKE/bin/boundary" doctor --json
"$TMPDIR_SMOKE/bin/boundary" evidence bundle --include-demo --out "$TMPDIR_SMOKE/evidence"
"$TMPDIR_SMOKE/bin/boundary" evidence verify "$TMPDIR_SMOKE/evidence"
```

Result: pass.

Observed utility output:

- `boundary version` reported `Fulcrum Boundary v0.6.1`.
- `boundary selftest` reported `status: pass`.
- `boundary demo github-lethal-trifecta` reported `status: pass`,
  `actual action: DENY`, `reason: lethal_trifecta_detected`, and
  `upstream_called=false`.
- `boundary demo action-boundary` reported `status: pass` across MCP / Secure
  GitHub, Command Boundary, and Edit Boundary fixture paths.
- `boundary doctor --json` emitted `boundary.doctor.v1` diagnostics and route
  caveats without credentials, network calls, or live mutation.
- `boundary evidence bundle --include-demo` created a bundle with 8 artifacts.
- `boundary evidence verify` verified all 8 artifacts and reported
  `status: pass`.

## Public Proxy And Latest Smoke

Command:

```bash
GOPROXY=https://proxy.golang.org,direct \
  go list -m -json github.com/fulcrum-governance/fulcrum-boundary@latest
```

Result: pass.

Observed output:

```json
{
  "Path": "github.com/fulcrum-governance/fulcrum-boundary",
  "Version": "v0.6.1",
  "Query": "latest",
  "Time": "2026-05-28T12:34:33Z",
  "GoVersion": "1.25.0"
}
```

Command:

```bash
GOBIN="$TMPDIR_LATEST/bin" GOPROXY=https://proxy.golang.org,direct \
  go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest
"$TMPDIR_LATEST/bin/boundary" version
```

Result: pass.

Observed output:

```text
Fulcrum Boundary v0.6.1
module: github.com/fulcrum-governance/fulcrum-boundary
```

## Claims Boundary

This postrelease smoke proves that the published v0.6.1 tag can be installed
and that the fixture-safe release utility path runs successfully from a fresh
temporary install.

It does not prove:

- production route enforcement,
- production Secure GitHub,
- production Command Boundary,
- production Edit Boundary,
- universal attack prevention,
- cryptographic release provenance,
- hosted deployment health.

