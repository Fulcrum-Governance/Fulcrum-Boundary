# Release Truth: v0.4.0 Alignment Audit

Date: 2026-05-27

Audited base commit SHA: `33d31bb89dad7799dfd13078e666dae23e525962`

Branch: `release/v040-alignment-audit`

Release tag: `v0.4.0`

Release URL: <https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.4.0>

## Summary

This audit reconciles the active public Boundary documentation after v0.4.0 was
tagged and published. It does not add product behavior or change Command
Boundary claims.

Final v0.4.0 alignment truth:

- v0.4.0 is the current public release.
- Public install examples use `@v0.4.0`.
- `@latest` resolves to `v0.4.0`.
- GitHub Action examples use `@v0.4.0`.
- MCP remains the production adapter path.
- Secure GitHub remains preview and fixture-backed.
- Command Boundary is preview and applies only to routed project-local command
  paths.
- Direct shell access, CI jobs, SSH sessions, and direct file edits remain
  outside Boundary unless they are routed through Boundary.

## Install Verification

Post-tag smoke verification for v0.4.0 recorded:

| Check | Result |
| --- | --- |
| `GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0` | Pass |
| `boundary selftest` | Pass |
| `boundary demo github-lethal-trifecta` | Pass: `actual action: DENY`, `reason: lethal_trifecta_detected`, `upstream_called=false` |
| `boundary command classify -- git push origin main` | Pass: `Class: C3 repo mutation`, `Recommended action: require_approval` |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | Pass: resolved `v0.4.0` |

## Docs Checked

- `README.md`
- `docs/INSTALL.md`
- `docs/CLI_REFERENCE.md`
- `docs/CLAIMS_LEDGER.md`
- `claims/boundary_claims.yaml`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `docs/RELEASE_TRUTH_PUBLIC.md`
- `docs/RELEASE_TRUTH_REPO_POLISH.md`
- `docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md`
- `docs/RELEASE_TRUTH_V040.md`
- `docs/LAUNCH_TRUTH_FREEZE.md`
- `docs/PUBLIC_RELEASE_COPY.md`
- `docs/command-boundary/`
- `docs-site/`
- `CHANGELOG.md`
- `actions/mcp-audit/action.yml`
- `docs/firewall/GITHUB_ACTION.md`

## Stale References Found And Fixed

| Finding | Classification | Resolution |
| --- | --- | --- |
| `docs/RELEASE_TRUTH_PUBLIC.md` still described v0.3.0 as the current public truth. | Active drift | Updated to v0.4.0 and added Command Boundary preview status. |
| `docs/RELEASE_TRUTH_REPO_POLISH.md` still described v0.3.0 install and action examples as current. | Active drift | Updated to v0.4.0 and added Command Boundary preview status. |
| `docs-site/command-boundary/index.md` opened with v0.3.0 before saying v0.4.0. | Cosmetic stale framing | Reworded to lead with v0.4.0 while preserving v0.3.0 as historical context. |

Historical v0.3.0 references remain where appropriate: v0.3.0 release notes,
changelog history, launch-truth history, session log history, and planning
artifacts.

No stale `@main` install examples were found in `README.md`, `docs/INSTALL.md`,
or `docs-site`.

## Command Boundary Claim Status

| Claim | Status | Alignment truth |
| --- | --- | --- |
| BND-CLAIM-CMD-001 | delivered | Delivered preview: project-local command governance applies only to commands routed through `boundary command run`, `boundary shell`, or project-local shims. |
| BND-CLAIM-CMD-002 | delivered | Delivered fixture proof: command redteam packs exercise selected command-risk paths without live mutation. |

Command Boundary remains preview. Delivered preview claims do not imply
production command governance, direct-shell protection, CI/SSH control, shell
sandboxing, or direct file-edit governance.

## Remaining v0.4 Gaps

- Secure GitHub production status still requires live GitHub App conformance and
  deployment bypass evidence.
- Command Boundary production status still requires deployment evidence that
  Boundary is the relevant command path for a protected project or workflow.
- Direct shell access remains outside Boundary unless routed through Boundary.
- CI jobs, SSH sessions, cron jobs, editor tasks, and arbitrary local processes
  remain outside Boundary unless routed through Boundary.
- Direct file edits outside routed command paths remain a v0.5 design gap.
- Full GitHub MCP tool catalog coverage remains deferred.

## Approved Public Copy

Fulcrum Boundary v0.4.0 adds Command Boundary preview: project-local command
classification and wrapper-routed command governance through
`boundary command run`, `boundary shell`, and project-local shims.

Command Boundary is preview. Direct shell access, CI jobs, SSH sessions, and
direct file edits remain outside Boundary unless they are routed through
Boundary.

## Forbidden Public Copy

Do not use these as public capability claims:

- Boundary controls all shell commands.
- Boundary protects direct shell access.
- Boundary prevents every overeager agent action.
- Boundary provides production command governance.
- Boundary governs direct file edits outside routed command paths.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Verification Commands

| Command | Result |
| --- | --- |
| `git grep -n '@v0.3.0' || true` | Pass: remaining matches are historical or planning context. |
| `git grep -n 'v0.3.0' -- ':!docs/releases/*' ':!CHANGELOG.md' || true` | Pass: remaining matches are historical, dependency, or v0.3-to-v0.4 context. |
| `git grep -n '@main' -- README.md docs/INSTALL.md docs-site || true` | Pass: zero matches. |
| `git grep -n -i 'controls all shell' || true` | Pass: matches only forbidden-copy or limitation context. |
| `git grep -n -i 'protects direct shell' || true` | Pass: matches only forbidden-copy or limitation context. |
| `git grep -n -i 'prevents every overeager' || true` | Pass: matches only forbidden-copy or limitation context. |
| `git grep -n -i 'production command governance' || true` | Pass: matches only forbidden-copy or limitation context. |
| `git grep -n -i 'governs direct file edits' || true` | Pass: matches only forbidden-copy or limitation context. |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `make docs-build` | Pass |
| `make release-check` | Pass |
| `go test ./internal/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Command -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |
