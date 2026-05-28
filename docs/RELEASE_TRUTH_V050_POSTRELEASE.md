# Release Truth: v0.5.0 Post-Release Verification

Date: 2026-05-28

Status: passed.

This file records post-tag verification of the v0.5.0 Secure GitHub live
conformance preview package.

## Release Objects

- PR: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/pull/82
- Merge commit: `3d22efe7e7ce20499a4196e95b86f989c1007651`
- Tag: `v0.5.0`
- Tag object: `a2ed93ecc1b64208434a065ce396cd73a44e1e1d`
- Tag target commit: `3d22efe7e7ce20499a4196e95b86f989c1007651`
- GitHub release: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.5.0
- Published: 2026-05-28T07:43:15Z

## Post-Tag Smoke

| Check | Command | Result |
| --- | --- | --- |
| Direct tag install | `GOBIN="$tmp/bin" GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.5.0` | pass |
| Selftest | `"$tmp/bin/boundary" selftest` | pass |
| GitHub lethal-trifecta demo | `"$tmp/bin/boundary" demo github-lethal-trifecta` | pass |
| Command classifier preview | `"$tmp/bin/boundary" command classify -- git push origin main` | pass |
| Public proxy latest | `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | pass, resolved `v0.5.0` |
| Public proxy install | `GOBIN="$tmp/latest-bin" GOPROXY=https://proxy.golang.org,direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest` | pass |

## Smoke Evidence

`boundary selftest` reported `status: pass` with no live mutation,
credentials, or network, including the Secure GitHub live-mode fail-closed
check and the GitHub lethal-trifecta fixture denial.

`boundary demo github-lethal-trifecta` reported:

- `status: pass`
- `fixture-only: true`
- `credentials: none`
- `network: none`
- `live mutation: none`
- `actual action: DENY`
- `reason: lethal_trifecta_detected`
- `upstream_called=false`

`boundary command classify -- git push origin main` reported:

- `Class: C3 repo mutation`
- `Risk: HIGH`
- `Recommended action: require_approval`
- `Reason: external repository mutation`

## Release Truth

v0.5.0 is the Secure GitHub live conformance preview release. Secure GitHub
remains preview until operator-owned live evidence and deployment bypass
evidence are recorded. MCP remains the production adapter. Command Boundary
remains preview and routed-path-only.
