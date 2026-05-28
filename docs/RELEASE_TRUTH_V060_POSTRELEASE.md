# Release Truth: v0.6.0 Postrelease Smoke

Date: 2026-05-28

Release tag: `v0.6.0`

Tag object SHA: `081679911141a72fd12bef5909110a1f434787cc`

Release commit SHA: `1c7fe0f65275df9a72d43912f2388d108fa24253`

GitHub release:
<https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.6.0>

## Summary

The v0.6.0 tag and GitHub release are public. Post-tag smoke verification
passed from a clean temporary install path.

Final postrelease truth:

- `v0.6.0` is the latest GitHub release.
- `@v0.6.0` installs successfully with `GOPROXY=direct`.
- `@latest` resolves to `v0.6.0` through `https://proxy.golang.org,direct`.
- The installed binary passes `boundary selftest`.
- The installed binary passes `boundary demo github-lethal-trifecta`.
- Command Boundary classify remains available in the tagged install.
- Edit Boundary inspect and fixture redteam packs remain available in the
  tagged install.
- No credentials, live GitHub calls, or real project mutation were used.

## Commands And Results

| Command | Result |
| --- | --- |
| `GOBIN="$tmp/bin" GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.0` | pass |
| `"$tmp/bin/boundary" selftest` | pass |
| `"$tmp/bin/boundary" demo github-lethal-trifecta` | pass |
| `"$tmp/bin/boundary" command classify -- git push origin main` | pass |
| `"$tmp/bin/boundary" edit inspect --help` | pass |
| `"$tmp/bin/boundary" redteam --pack edit-secret-exfil` | pass |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | pass: `github.com/fulcrum-governance/fulcrum-boundary v0.6.0` |
| `GOBIN="$tmp/latest-bin" GOPROXY=https://proxy.golang.org,direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest` | pass |
| `"$tmp/latest-bin/boundary" selftest` | pass |

## Smoke Output Highlights

`boundary selftest`:

- `status: pass`
- `live mutation: none`
- `credentials: none`
- `network: none`

`boundary demo github-lethal-trifecta`:

- `status: pass`
- `expected action: DENY`
- `actual action: DENY`
- `reason: lethal_trifecta_detected`
- `upstream_called=false`
- `read_upstream_called=true`

`boundary command classify -- git push origin main`:

- `Class: C3 repo mutation`
- `Risk: HIGH`
- `Recommended action: require_approval`
- `Reason: external repository mutation`

`boundary redteam --pack edit-secret-exfil`:

- `scenario: edit-env-secret`
- `class: E4`
- `risk: CRITICAL`
- `applied: false`
- `expected: DENY`
- `actual: DENY`
- `result: pass`

## Version Command Note

`boundary` does not currently expose a `version` subcommand. The postrelease
binary proof therefore uses `go list -m ...@latest`, direct `@v0.6.0` install,
public-proxy `@latest` install, and `boundary selftest` from both installed
paths.

## Claim Boundary

v0.6.0 does not upgrade any preview surface to production:

- MCP remains the production adapter path.
- Secure GitHub remains preview.
- Command Boundary remains preview and routed-path-only.
- Edit Boundary remains preview and routed-edit-envelope-only.
- Direct editor writes, direct filesystem writes, shell redirection, direct
  `git apply`, IDE saves, CI jobs, and arbitrary process writes remain outside
  Edit Boundary unless explicitly routed through Boundary edit envelopes.
