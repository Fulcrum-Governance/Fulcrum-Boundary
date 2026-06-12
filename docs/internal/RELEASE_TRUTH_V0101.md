# Final Public Release Truth

Date: 2026-06-11

Branch: `main`

Current release target: `v0.10.1`

## Summary

This report reconciles the public Boundary release surface for the published
`v0.10.1` release — the first release published with prebuilt binaries — and
records, in a separate fenced section, what has merged to `main` since the tag
and is not yet part of any release. `v0.9.0` survives as the prior history
tag. `v0.10.0` exists as a valid source-install tag with no release assets:
its release-pipeline run failed before publishing, and `v0.10.1` is
content-identical plus the pipeline fix (the failure and the fix are recorded
in `CHANGELOG.md` and `docs/releases/v0.10.0.md`).

`v0.10.1` packages three lanes on top of the `v0.9.0` surface:

- **Distribution.** A tag-gated release pipeline publishes static
  (`CGO_ENABLED=0`) archives for macOS/Linux/Windows with checksums, `_cgo`
  full-classifier variants for macOS/Linux, a Homebrew tap formula, a container
  image, and an evidence bundle. In the static build the Postgres AST
  classifier is unavailable: routed SQL classifies as `UNKNOWN` and is denied
  fail-closed, so the static build never allows SQL the cgo build would deny.
- **RFC 8785 canonical decision records.** The canonical preimage behind every
  stable record hash follows RFC 8785 (JCS). This is a pre-1.0 format change:
  records emitted by older builds no longer verify under `v0.10.x` builds. The
  conformance statement is scoped to the decision record;
  it is not a claim that Boundary as a whole is standards-conformant.
- **Independent verification.** A standalone Python verifier
  (`verifiers/python/`) recomputes `decision_hash` with the stock `rfc8785`
  package, pinned to the Go implementation by a committed conformance-vector
  corpus enforced in CI. Verification confirms record integrity over covered
  inputs; it does not prove the verdict was correct, the action executed or
  prevented, or who produced the record.

`v0.10.1` adds no new governed action surface, adds no transport adapter, and
upgrades no preview surface to production.

The final public truth is:

- MCP remains the production adapter path.
- Secure GitHub remains preview, with fixture proof plus an opt-in live GitHub
  App conformance harness.
- Command Boundary remains delivered preview and routed-path-only.
- Edit Boundary remains delivered preview and routed-edit-envelope-only.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A remain preview.
- Decision records are hash-verifiable: unkeyed SHA-256 over RFC 8785
  canonical bytes — integrity, not authenticity. In `v0.10.1`, signing is not
  available; signed receipts are not claimed in any form.
- `boundary explain` renders a decision record read-only; it does not
  re-verify hashes or prove the verdict was correct or enforced.
- `boundary replay` reproduces the recorded decision for routed requests; it
  does not prove enforcement or that no upstream bytes moved.
- `boundary test` reports policy verdicts for routed request fixtures against
  local policy bundles; it does not prove production route enforcement,
  deployment bypass resistance, or verdict correctness beyond supplied
  fixtures.

## Shipped On Main, Unreleased

> Everything in this section merged to `main` after the `v0.10.1` tag and is
> **not part of any released tag**. Do not cite any of it as released
> capability. It is recorded here so downstream truth documents (including the
> Fulcrum-IO claims lock) can reference Boundary's main-branch state with the
> correct status. These items become release truth only when the next tag
> ships and this document is reconciled again.

- **TypeScript and Rust standalone verifiers** (`verifiers/typescript/`,
  `verifiers/rust/`), each recomputing `decision_hash` via a stock RFC 8785
  implementation and pinned to Go by the same conformance vectors, enforced in
  CI. With these, a record verifies in Go, Python, TypeScript, or Rust — that
  enumerated list, never "any language." Ledger: `BND-CLAIM-VERIFY-003`,
  `BND-CLAIM-VERIFY-004` (main).
- **Opt-in Ed25519 record signing**, off by default: a configured signer
  populates `signature`/`signature_key_id` on emitted records (parse
  rejections included), `boundary serve --receipt-seed FILE` enables it at
  serve (failing closed before the listener opens on a bad seed), and
  `boundary verify-record --verify-signature --public-key <hex|file>` checks
  the signature over the recomputed `decision_hash`, failing closed. Signing
  never changes `decision_hash`. A valid signature attests who signed the
  record; it does not prove the verdict was correct, that the action executed
  or was prevented, and it does not solve key custody. The Python, TypeScript,
  and Rust verifiers remain integrity-only and do not check signatures.
  Ledger: `BND-CLAIM-SIGN-001` (main).
- **CLI and docs polish**: `--version`/`-v`, `boundary help <topic>`, rich
  help across previously bare commands, `--json` on `verify`/`verify-record`,
  `boundary completion bash|zsh|fish`, a skeptic FAQ, a complete CLI reference
  command map, and the adapter production-bar contributor doc.
- **Test and robustness depth**: native Go fuzz targets (record
  canonicalization round-trip, policy parse, SQL classifier) wired into the
  required CI set; a hermetic black-box `boundary serve` boot test asserting
  the governed deny before upstream; RESP codec unit tests for the kernel
  trust path.

On the standards question, stated once and precisely: Boundary's per-record
canonicalization (RFC 8785/JCS) and SHA-256 hashing match the per-record
algorithms in `draft-sharif-agent-audit-trail-00`, an individual,
non-WG-adopted Internet-Draft; Boundary does not implement that draft's
defining `prev_hash` inter-record session chain, and Boundary does not claim
alignment with the draft. Any future session-chain support would be a
documented, additive change.

## Test Commands

| Command | Result |
| --- | --- |
| `./scripts/assert-no-public-vendor-refs.sh` | Pass |
| `make docs-build` | Pass |
| `make release-check` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |

Verified 2026-06-11 at `main` commit `f0e3041` (post-tag main; the `v0.10.1`
tag itself shipped with `make release-check` exit 0 at `7ac56b3`, and the
release workflow's `release-check` gate job passed before publish).

`make release-check` runs the root suite, the gRPC nested module suite, the
`tests/` and `claims/` suites, policy verification, receipt verification help,
`boundary selftest`, both fixture demos, `boundary version`,
`boundary doctor --json`, `boundary evidence bundle --include-demo`,
`boundary evidence verify`, and the policy-as-code corpus.

Post-tag verification for `v0.10.1` (recorded 2026-06-11): all 7 release jobs
green; 13 release assets published (5-platform static archives, 4 `_cgo`
archives, `SHA256SUMS`, `SHA256SUMS-cgo`, evidence bundle); the Homebrew
formula landed in `fulcrum-governance/homebrew-tap`; the container image is
publicly pullable at `ghcr.io/fulcrum-governance/boundary:v0.10.1`
(multi-arch manifest verified anonymously); `brew install
fulcrum-governance/tap/boundary` followed by `boundary version` and
`boundary selftest` passed end-to-end on a real machine. Prior post-tag
evidence for `v0.9.0` and earlier remains in `docs/internal/`.

## README First-Run Status

README presents install before architecture, leading with the binary
channels:

```bash
brew install fulcrum-governance/tap/boundary
boundary selftest
boundary demo github-lethal-trifecta
boundary demo tamper-evidence
```

The source path remains:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.10.1
```

All demos remain credential-free and perform no live calls or real mutations.
`v0.10.1` ships six demos: `action-boundary`, `postgres`,
`github-lethal-trifecta`, `command-secret-exfil`, `tamper-evidence`, and
`trust-degradation`.

## Claims Status

The `v0.9.0` claims table in
[`docs/internal/RELEASE_TRUTH_V090.md`](./RELEASE_TRUTH_V090.md)
remains accurate for the carried-over claims; no carried claim changed status
in `v0.10.x`. New claims recorded since `v0.9.0`:

| Claim | Status | Release truth |
| --- | --- | --- |
| BND-CLAIM-BUILD-001 | delivered (v0.10.1) | Static `CGO_ENABLED=0` builds are supported; the Postgres AST classifier is unavailable in them, routed SQL classifies `UNKNOWN` and is denied fail-closed, and the static build never allows SQL the cgo build would deny. |
| BND-CLAIM-DIST-001 | delivered (v0.10.1) | Prebuilt distribution channels (release archives + checksums, Homebrew tap, container image, `go install`) publish from the tag-gated pipeline for `v0.10.1` and later; releases up to and including `v0.10.0` shipped source-only. |
| BND-CLAIM-REC-002 | delivered (v0.10.1) | Decision-record hashes are computed over RFC 8785 (JCS) canonical bytes; the conformance statement is record-scoped and is not a whole-product standards claim. |
| BND-CLAIM-VERIFY-002 | delivered (v0.10.1) | A standalone non-Go (Python) verifier recomputes a record's hash and detects a one-field forgery, pinned to Go by the shared conformance-vector corpus. |
| BND-CLAIM-VERIFY-003 | main, unreleased | TypeScript verifier, vector-pinned. Not part of any released tag. |
| BND-CLAIM-VERIFY-004 | main, unreleased | Rust verifier, vector-pinned. Not part of any released tag. |
| BND-CLAIM-SIGN-001 | main, unreleased | Opt-in Ed25519 record signing, off by default; signature attests the signer, not the verdict. Not part of any released tag. |

## Feature Status

The `v0.9.0` feature table in
[`docs/internal/RELEASE_TRUTH_V090.md`](./RELEASE_TRUTH_V090.md)
carries forward unchanged. Added in `v0.10.1`:

| Feature | Status | Release truth |
| --- | --- | --- |
| Prebuilt release pipeline | delivered | Tag-gated goreleaser publish: static archives + checksums, `_cgo` variants, tap formula, container image, evidence bundle. |
| Static build variant | delivered | `CGO_ENABLED=0` builds work; SQL classification degrades to fail-safe `UNKNOWN`-deny; `_cgo` archives carry the full classifier. |
| RFC 8785 record canonicalization | delivered | Record-scoped JCS canonical preimage behind all stable hashes; committed conformance vectors; older-build records no longer verify (pre-1.0 format change). |
| Python standalone verifier | delivered | `verifiers/python/` recomputes `decision_hash` via the stock `rfc8785` package; integrity only, not authenticity. |
| Cross-language CI verification gate | delivered | CI requires Go and the stock Python implementation to agree on record hashes every run. |
| `boundary demo tamper-evidence` | delivered | Fixture-only forge-the-receipt demo: mutate a recorded verdict, watch `verify-record` refuse it. |
| First-run / quickstart rework | delivered | README and quickstart lead with install + the record-trust loop; uniform record-location output retained. |
| Comparison and standards docs | delivered | "Where Boundary Fits" category comparison and the standards/incident mapping pages, in the limitation-framed register. |
| Claude Code `PreToolUse` hook | delivered preview | Routes hook-delivered tool calls through `boundary command classify` before execution; governs only the calls the hook delivers — routed-only. |

## Adapter, Profile, And Surface Status

Unchanged from `v0.9.0` — no surface changed maturity in `v0.10.x`. MCP is
production; CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, Secure GitHub,
Command Boundary, and Edit Boundary are preview. The full table is in
[`docs/internal/RELEASE_TRUTH_V090.md`](./RELEASE_TRUTH_V090.md).

## User-Install Status

The documented install channels for `v0.10.1` and later:

```bash
brew install fulcrum-governance/tap/boundary          # static build
docker pull ghcr.io/fulcrum-governance/boundary:v0.10.1
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.10.1
```

Plus release archives with `SHA256SUMS` / `SHA256SUMS-cgo` manifests. Releases
up to and including `v0.10.0` shipped source-only. The formula and archives
install the static build; the `_cgo` archives and the source build carry the
full SQL classifier (Go 1.25+ and a C toolchain required for source).

## GitHub Action Ref Status

The MCP audit action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.10.1
```

Use the release tag for repeatable CI behavior. SARIF upload examples must
include `contents: read` and `security-events: write` permissions.

## Approved Release Language

Fulcrum Boundary is the action boundary for routed agent tools. It inventories
local MCP tool paths, renders risk paths, generates starter policies, runs safe
fixture redteams, and denies governed privileged actions before execution when
those actions route through Boundary. MCP is the first production route;
Command Boundary, Edit Boundary, Secure GitHub, and the remaining adapters are
preview.

Fulcrum Boundary v0.10.1 is the first release with prebuilt binaries: a
one-command Homebrew install, release archives with checksums, a container
image, and `go install`. Decision-record hashes are computed over RFC 8785
(JCS) canonical bytes — that statement is scoped to the decision record and
is not a claim that Boundary as a whole is standards-conformant. A record can
be verified with the Go binary or with the standalone Python verifier built on
the stock `rfc8785` package, pinned to the Go implementation by a committed
conformance-vector corpus. The hashes are unkeyed SHA-256: integrity, not
authenticity. Verification confirms the record was not altered after emission;
it does not prove the verdict was correct, the action executed or prevented,
or who produced the record. The fixture demos — including
`boundary demo tamper-evidence`, which forges a recorded verdict and shows
verification refuse it — use no credentials, make no live calls, and perform
no real mutations. v0.10.1 adds no new governed action surface and upgrades no
preview surface to production.

Secure GitHub is preview. Production status still requires deployment bypass
evidence and broader live coverage.

Command Boundary remains preview. Direct shell access, CI jobs, and SSH
sessions remain outside Command Boundary unless routed through Boundary command
wrappers or project-local shims.

Edit Boundary remains preview. Direct editor writes, direct filesystem writes,
direct `git apply`, shell redirection, IDE saves, CI jobs, and arbitrary
processes remain outside Boundary unless routed through Boundary edit
envelopes.

For the next tagged release (when the main-unreleased items above ship), the
pre-approved additions are: records verify in Go, Python, TypeScript, or Rust
(always the enumerated list); and Ed25519 signing is opt-in and off by
default, a signature attests who signed the record and not the verdict or
execution, and key custody is the operator's. Neither sentence may be used as
released-capability language before that tag exists.

## Forbidden Release Language

Do not use these as public capability claims:

- Do not claim universal prompt-injection prevention.
- Do not claim production Secure GitHub.
- Do not claim Secure GitHub fully secures GitHub.
- Do not claim live conformance proves deployment bypass resistance.
- Do not claim all adapters production.
- Do not claim generated policies are production-complete.
- Do not claim dashboard monitoring.
- Do not claim Boundary protects tools that bypass Boundary.
- Do not claim Boundary controls all shell commands.
- Do not claim Boundary protects direct shell access.
- Do not claim Boundary prevents every overeager agent action.
- Do not claim Boundary provides production command governance.
- Do not claim Boundary governs direct file edits outside routed edit envelopes.
- Do not claim Boundary controls all file writes.
- Do not claim Boundary protects direct editor writes.
- Do not claim Boundary provides filesystem sandboxing.
- Do not claim Boundary provides production edit governance.
- Do not claim evidence bundles prove production safety.
- Do not claim doctor proves all routes protected.
- Do not claim the action-boundary demo proves all attacks blocked.
- Do not claim version output proves cryptographic release provenance.
- Do not claim `topology_profile` attests or verifies the deployment.
- Do not claim `execution_claim` independently proves no upstream bytes moved.
- Do not claim `boundary replay` proves enforcement.
- Do not claim `boundary explain` verifies hashes or proves the verdict.
- Do not claim route-context records are cryptographic proof of a runtime verdict.
- Do not claim `boundary test` proves production route enforcement.
- Do not claim `boundary test` proves deployment bypass resistance.
- Do not claim `boundary test` proves a verdict was globally correct beyond the
  supplied fixture and local policy bundle.
- Do not claim records are tamper-proof, immutable, or non-repudiable; the
  approved properties are hash-verifiable and tamper-evident over covered
  inputs.
- Do not claim Boundary as a whole is standards-conformant; the RFC 8785
  statement is record-scoped with the scope on the same line.
- Do not claim a Boundary record verifies in any language; the verifier list
  is always enumerated, and TypeScript/Rust remain unreleased until the next
  tag.
- Do not claim signed receipts or signing by default; in `v0.10.1` signing
  does not exist, and on main it is opt-in, off by default, and attests the
  signer rather than the verdict.
- Do not claim Boundary aligns with or implements
  `draft-sharif-agent-audit-trail`; the shared per-record algorithms and the
  unimplemented `prev_hash` session chain must be stated together or not at
  all.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Docs Checked

This 2026-06-11 revision verified, against the `v0.10.1` tag and `main`
(`f0e3041`):

- `README.md` (install channels, first-run, forge-the-receipt language)
- `docs/INSTALL.md` (channel availability, `@v0.10.1` targets)
- `CHANGELOG.md` (`[0.10.0]`/`[0.10.1]` history, `[Unreleased]` content)
- `docs/releases/v0.10.0.md` and `docs/releases/v0.10.1.md`
- `claims/boundary_claims.yaml` (claim diff `v0.9.0..v0.10.1` and
  `v0.10.1..main`)
- `verifiers/python/README.md`
- `docs/SIGNING.md` (main, unreleased)
- `docs/CLI_REFERENCE.md`
- Release assets, tap formula, and container image (post-tag evidence above)

The `v0.9.0` reconciliation's full docs-checked list is preserved in
[`docs/internal/RELEASE_TRUTH_V090.md`](./RELEASE_TRUTH_V090.md).

## Drift Fixed

- Updated active public truth from `v0.9.0` to the published `v0.10.1`
  release; archived the prior active truth to
  `docs/internal/RELEASE_TRUTH_V090.md`.
- Recorded the `v0.10.0` history honestly: a valid source-install tag whose
  pipeline run failed before publishing assets; `v0.10.1` is
  content-identical plus the fix.
- Recorded the `v0.10.1` post-tag evidence: 7/7 release jobs, 13 assets, tap
  formula, public multi-arch container image, and an end-to-end
  `brew install` + `selftest` pass.
- Recorded the distribution, static-build, RFC 8785 record, and Python
  verifier claims as delivered at `v0.10.1` with their caveats.
- Added the fenced "Shipped On Main, Unreleased" section so downstream truth
  documents can cite Boundary's main-branch state (TypeScript/Rust verifiers,
  opt-in Ed25519 signing, CLI polish, fuzz/boot-test depth) without
  presenting it as released capability.
- Stated the `draft-sharif-agent-audit-trail` position once and precisely:
  shared per-record algorithms, unimplemented session chain, no alignment
  claim.
- Extended the forbidden-language list with the records-era rules:
  tamper-proof/immutable/non-repudiable, blanket standards conformance,
  any-language verification, signed-by-default, and draft-alignment claims.
