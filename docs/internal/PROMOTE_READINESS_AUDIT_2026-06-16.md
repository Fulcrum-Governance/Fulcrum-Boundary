# Promote Readiness Audit - 2026-06-16

## Decision

Boundary is technically ready on branch `codex/2026-06-16-boundary-promote-audit` after the Pages-source release-truth fix in commit `def4805`.

Do not use the public marketing push until the live publication blockers below are cleared:

1. Boundary GitHub Pages is still serving the June 11 build and renders the stale `@v0.9.0` install command. This branch updates the Pages source to `@v0.11.0`; the fix must land on `main` and the Docs workflow must deploy before the GitHub Pages URL is promote-ready.
2. The separate Fulcrum marketing site at `https://fulcrumlayer.io` renders stale copy: "Boundary v0.9.0 release truth". Its source is in the Fulcrum dashboard marketing component, but that checkout was dirty on a Claude-owned branch during this audit, so it was not edited in this Boundary lane.

## Scope

- Boundary repository, default branch, open PR and issue state.
- Boundary docs-site source, strict docs build, claims gate, Go regression gates, and release-check bundle.
- Boundary live GitHub Pages configuration and rendered body.
- Boundary latest release metadata and non-destructive install channels.
- Cross-check of the primary Fulcrum marketing homepage because it is a likely traffic target for the same marketing push.

Out of scope:

- Starting issue `#153`; it remains an explicit follow-up and is not a launch blocker.
- New tags, new releases, preview-surface promotion, or integration-contract changes.
- Editing the separate Fulcrum marketing checkout while it is dirty on another branch.

## Changes Made In This Lane

Commit `def4805` updates the Boundary docs-site source so active install and release-truth copy aligns with `v0.11.0`:

- `docs-site/index.md`
- `docs-site/quickstart.md`
- `docs-site/faq.md`
- `docs-site/reference/troubleshooting.md`
- `docs-site/reference/policy-testing.md`
- `docs-site/reference/cli.md`
- `docs-site/reference/release-utilities.md`
- `docs-site/reference/roadmap.md`

The fix removes active `@v0.9.0` install guidance, corrects the static-vs-cgo story, and preserves routed-only, fixture-only, preview, and local-only caveats.

## Local Gates

| Command | Result | Evidence |
| --- | --- | --- |
| `env -u GOROOT go test ./claims/... -count=1` | pass | `ok .../claims 0.338s` |
| `make docs-build` | pass | MkDocs strict build completed; documentation built in `0.42` seconds. |
| `git diff --check` | pass | No output. |
| `git ls-files '*.go' \| xargs gofmt -l` | pass | No output. |
| `env -u GOROOT go vet ./...` | pass | No output. |
| `env -u GOROOT go test ./... -count=1 -timeout 5m` | pass | Full root package sweep passed, including `tests/actions`, `tests/supplychain`, and `tests/test_runner`. |
| `make release-check` | pass | Public-artifact guard, root and gRPC vet/tests, claims, policy verify, `boundary test`, selftest, demos, doctor, evidence bundle, and evidence verify all passed. |

## Live Repository State

| Surface | Result | Evidence |
| --- | --- | --- |
| Repository metadata | pass | `Fulcrum-Governance/Fulcrum-Boundary`, public, default branch `main`, not archived, not a fork, homepage `https://fulcrumlayer.io`. |
| Open PRs | pass | `gh pr list --state open` returned `[]`. |
| Open issues | pass with known follow-up | Only issue `#153`, "Opt-in literal enforcement of the safe forbidden-phrase subset", remains open and out of scope. |
| Latest main CI | pass | CI run `27602591976` succeeded at `65227ab59b29e27f4423768a1d10f300a9d7bd30`. |
| Latest main CodeQL | pass | CodeQL run `27602591958` succeeded at `65227ab59b29e27f4423768a1d10f300a9d7bd30`. |
| Docs workflow | blocker until deploy | Latest Docs deploy run `27329704545` succeeded on June 11 at `46187527`, before current release/docs updates. |

## Live Pages State

| Surface | Result | Evidence |
| --- | --- | --- |
| Pages configuration | pass | GitHub Pages is public, uses workflow build type, source `main` path `/`, and HTTPS is enforced. |
| Pages HTTP | pass | `https://fulcrum-governance.github.io/Fulcrum-Boundary/` returned HTTP 200. |
| Pages freshness | blocker until deploy | Response `last-modified` was Thu, 11 Jun 2026 06:59:55 GMT. |
| Pages rendered release truth | blocker until deploy | Rendered body still contains `go install .../cmd/boundary@v0.9.0`. |

## Release And Install Channels

| Surface | Result | Evidence |
| --- | --- | --- |
| Latest GitHub release | pass | Latest release is `v0.11.0`, published June 12, not draft, not prerelease, target `main`. |
| Release assets | pass | 13 uploaded assets: evidence bundle, static archives, native-cgo archives, Windows static archives, `SHA256SUMS`, and `SHA256SUMS-cgo`. |
| Go module latest | pass | `go list -m ...@latest` resolved to `github.com/fulcrum-governance/fulcrum-boundary v0.11.0`. |
| Temporary Go install | pass | `go install ...@latest` produced `Fulcrum Boundary v0.11.0`; binary ran `boundary version`. |
| Homebrew | pass | `brew info fulcrum-governance/tap/boundary` reported stable `0.11.0` and the static-build fail-closed SQL caveat. |
| Container image | pass | `docker manifest inspect ghcr.io/fulcrum-governance/boundary:v0.11.0` returned a manifest list for linux amd64 and arm64. |

## Cross-Repo Marketing Surface

| Surface | Result | Evidence |
| --- | --- | --- |
| `https://fulcrumlayer.io` HTTP | pass | Vercel served HTTP 200. |
| `https://fulcrumlayer.io` Boundary copy | blocker | Rendered page includes "Boundary v0.9.0 release truth". |
| Owning source | not edited | The string was found in the Fulcrum dashboard marketing source. That checkout was dirty on `claude/2026-06-16-audit-field-bridge`, so this Boundary lane left it untouched. |

## Blockers Before Push

1. Merge the Boundary docs-site fix and wait for the Docs workflow to deploy; then re-check the Pages body for `@v0.11.0` and confirm no active `@v0.9.0` install command remains.
2. Update and deploy the separate Fulcrum marketing homepage, or avoid routing tonight's traffic there, because it still says "Boundary v0.9.0 release truth".

## Recommended Final Checks After Boundary Merge

```bash
gh run list --workflow Docs --limit 3 --json databaseId,headSha,status,conclusion,createdAt,updatedAt,url
curl -sS -L -D - -o /tmp/boundary-pages-home.html https://fulcrum-governance.github.io/Fulcrum-Boundary/
rg -n "go install|v0\\.9\\.0|v0\\.11\\.0" /tmp/boundary-pages-home.html
```

Expected post-merge state:

- Latest Docs run succeeds on the merge commit.
- GitHub Pages renders `@v0.11.0` as the active install target.
- Any remaining `v0.9.0` mentions are historical availability notes only.

## Sign-Off

Boundary code, release assets, install channels, and local gates are green. Live promotion is not yet fully signed off because public Pages and the separate Fulcrum marketing homepage still show stale Boundary v0.9-era copy. Once those two live surfaces are corrected and rechecked, there are no additional Boundary technical tests recommended before the marketing push.
