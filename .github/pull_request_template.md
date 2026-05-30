<!--
Thanks for contributing to Fulcrum Boundary. Keep changes scoped to one lane.
Fill in the summary and tick the gates below; PRs that change public language
or a delivered claim must keep the claims ledger and the honesty gates green.
-->

## What changed and why

<!-- One or two sentences. What does this make clearer, safer, or easier to verify? -->

## Verification

Run the local gates and tick what passed:

- [ ] `make release-check` completes without public-surface or release-gate failures
- [ ] `gofmt -l .` prints no paths (run `git ls-files '*.go' | xargs gofmt -l`)
- [ ] `go vet ./...` is clean
- [ ] `go test ./... -count=1 -timeout 5m` passes
- [ ] `go test ./claims/... -count=1` passes (claims ledger + language lint)
- [ ] Updated `claims/boundary_claims.yaml` (and `docs/CLAIMS_LEDGER.md`) if a delivered/partial/false claim changed

## Honesty gates (claim-safety)

Boundary's credibility is its claim discipline. Confirm this PR keeps it:

- [ ] No forbidden capability copy added: no "global shell control", no "all CLI activity protected", no "governs every way an agent can mutate", no "cryptographic proof of verdict", no "signed receipt by default", no "SQL firewall"
- [ ] Runtime decisions stay `deterministic` / `classified` — no `proved` runtime decision is claimed
- [ ] Decision records are described as "hash-verifiable", not signed or cryptographically proven
- [ ] Routed-only doctrine preserved: any new governed surface states that it governs only what routes through Boundary, and names the known bypasses
- [ ] Fixture demos / red-team packs stay fixture-only: no real secrets, no network, no live mutation, nothing executed (`executed == false`)
- [ ] The word "production" is used only for the MCP route; other surfaces stay `delivered` / `delivered-preview` / `preview` / `starter` / `local-only`

## Scope

- [ ] This PR is scoped to one lane and does not add a private-repo dependency
- [ ] For new adapters / extension points / public positioning changes, an issue was opened first to discuss the shape
