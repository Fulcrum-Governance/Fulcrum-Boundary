# Final Public Release Truth

Date: 2026-09-02

Branch: `main`

Current release: `v0.13.1`

Current release date: `2026-09-02`

Published release: `v0.13.1`

## Published v0.13.1 release

The annotated root tag object `aa32b87ceb4f82e6b4b1525168923aadcd3a93a8`
(`refs/tags/v0.13.1`) and annotated nested-module tag object
`ea3b9b79a2d895ad563be3eba9a4b6742bd48dd6`
(`refs/tags/verify-witnessed/v0.13.1`) both peel to the approved release commit
`8a5762888be8404f8a4a0e64a2ad6206667b71b6`.

The [GitHub release](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.13.1)
is published (not draft or prerelease) with 23 assets: six static archives,
six static SPDX SBOMs, four native-cgo archives, four native-cgo SPDX SBOMs,
`SHA256SUMS`, `SHA256SUMS-cgo`, and the fixture-safe
`boundary-evidence.tar.gz`. It was published by the natural tag-triggered
[release workflow run 33656858833](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/runs/33656858833)
(`push` event, 8/8 jobs successful). All 23 assets match their GitHub API
SHA-256 digests; both checksum manifests verify; and all ten SPDX files parse
and pass structural checks. All 22 configured GitHub build-provenance
attestations verify against `release.yml@refs/tags/v0.13.1` at source digest
`8a57628`. `SHA256SUMS-cgo` is the one expected unattested asset, matching the
configured workflow: attestation coverage is 22 artifacts, not all 23 assets.

`ghcr.io/fulcrum-governance/boundary:v0.13.1` and `:latest` resolve to the same
multi-architecture index,
`sha256:86d21b48baba1700dd79eb7d4026b972aba0792ee73adc21ff5f42038c7d0741`.
That index provides linux/amd64
(`sha256:8cd53330844b01b9769b763ed80989fdc0c7f0a32817428b4e6bffc885bbd7be`)
and linux/arm64
(`sha256:b1542d1b6910a78247170eb9c99fce9c05690aea3a277fa430824216bc8dd5bd`)
variants for version `v0.13.1` and the approved revision. The Homebrew cask
advanced to tap commit `5c3492b29fed9028db3c198779d5af56f66fff28`
(`Casks/boundary.rb`, version 0.13.1); all four cask SHA-256 values equal the
corresponding release-manifest entries.

The published Go modules resolve to the same approved release commit through
their actual tag refs. The root module
`github.com/fulcrum-governance/fulcrum-boundary@v0.13.1` resolves through
`refs/tags/v0.13.1` with module sum
`h1:1X2uXHNAOroHO9aLVNWsELPpuwfqqGdVYJYAox0mcbg=` and `go.mod` sum
`h1:C8i1oWLA1qfjwEHzu9fgMNKk2EnEIU4rhMu4AZNfPNw=`. The exact case-sensitive
nested module
`github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed@v0.13.1`
resolves through `refs/tags/verify-witnessed/v0.13.1` with module sum
`h1:Ea7VTGK+NgOtwJqTnDgU9Dk+4fPZ2h6e5H5E011zvW8=` and `go.mod` sum
`h1:XxhbOvnpQ4R5Z413N/bQAuNq427ZzqErOvjaxaLLSk8=`.

`v0.13.1` is a patch: its only product-behavior fix over `v0.13.0` is the
installer checksum-selector repair. The tagged release delta also contains
the accompanying documentation and release-metadata reconciliation across
four commits and 30 files. The published `v0.13.0` installer's
direct-release lane selected its `SHA256SUMS` entry with an unanchored match
that also captured the archive's `.spdx.json` SBOM line, so checksum
verification failed on the never-downloaded SBOM and the installer refused to
install — fail-closed, nothing written. The immutable `v0.13.0` tag retains
that defect; `v0.13.1` ships the repair. It selects the manifest entry by the
complete filename field,
requires exactly one exact entry (zero or duplicates still refuse), and is
pinned by hermetic direct-release regression tests. The Homebrew lane was
unaffected.

The founder-controlled immutable public-install smoke passed on 2026-09-02.
The public raw `main` and immutable `v0.13.1` installers were byte-identical
at SHA-256
`c4bf951e9849e1a48142adb35d55bb2e67f1fe4bd9dfc3b1d68d8a71e27a6a23`.
With Homebrew and existing Boundary binaries excluded, the installer selected
`boundary_0.13.1_darwin_arm64_static-nocgo.tar.gz` through the direct-release
lane. The selector found exactly one full filename-field match despite two
prefix matches including the SBOM; the archive matched `SHA256SUMS` at
`bfc5603bd3080a6b1b0365ebad2f22c87e5b384cab817c644c02a84b0235218f`.
The installed binary matched the binary extracted from that manifest-verified
archive at
`db33d702d9ed51e74699aa6650188cd0e58f158bd3353e954e121cde9bb38334`,
reported `v0.13.1` at the approved release commit, and passed `version`,
`selftest`, `hook --help`, and the fixture-only
`demo github-lethal-trifecta`. The demo denied before upstream execution.
Receipt-driven uninstall removed the binary and receipt, the disposable root
was removed, and pre/post manifests of the monitored real-environment Boundary
surfaces were byte-identical. An independent read-only review returned
`SMOKE EVIDENCE APPROVED`.

That smoke validates only the published direct-release Apple Silicon static
install path and those local fitness commands. It does not validate Homebrew
installation, plugin or marketplace availability, routed deployment coverage,
authenticity, execution proof, or protection of bypass paths.

Marketplace availability was validated separately on 2026-09-02. The public
[`Fulcrum-Governance/boundary-plugins`](https://github.com/Fulcrum-Governance/boundary-plugins)
repository was created at Tony-authored root commit
`d233fbdb8a37693898cdd9974baa3d01c6ab1f78` with exactly the approved
`README.md` and `.claude-plugin/marketplace.json` copied byte-for-byte from
this repository. Their SHA-256 values are
`1274fd85c37c9289cb2b1b9dbad54032194c71a53fd9b2c604201f2f5aa23492`
and `735dfad47cb402346393b06a3e2f6bd4e204e18f4174417ce4733bc665e7d1ab`.
In a fresh isolated `CLAUDE_CONFIG_DIR`, Claude Code `2.1.258` validated the
public marketplace manifest, added it with the slash command's CLI equivalent,
`claude plugin marketplace add Fulcrum-Governance/boundary-plugins --scope user`,
and installed it with
`claude plugin install boundary@boundary-plugins --scope user --yes`. The
installed plugin reported version `0.13.1`, four skills, two hooks, and source
commit
`8a5762888be8404f8a4a0e64a2ad6206667b71b6`; its installed manifest also
validated. This proves the standalone repository's public discovery,
resolution, installation, and manifest shape. It does not prove hook routing
coverage, protection of bypass paths, receipt authenticity or correctness,
execution, Homebrew installation, or acceptance by a community marketplace.

Install from the published versioned surfaces:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.13.1
go install github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed@v0.13.1
docker run --rm ghcr.io/fulcrum-governance/boundary:v0.13.1 selftest
```

`v0.13.1` changes no Boundary enforcement behavior, classifier, hook semantics,
adapter maturity, or claim status:
Session Receipts and the Claude Code hook first shipped in `v0.13.0`, records
remain hash-verifiable for covered-field integrity only, the hook remains
routed-only, and Boundary does not emit `proved` decisions. The public
`boundary-plugins` repository distributes the exact release-pinned marketplace
package described above. Repository publication and CLI validation do not
constitute community-marketplace acceptance or any broader launch claim.

## Published baseline before v0.13.1: v0.13.0

The published `v0.13.0` record is preserved below exactly as reconciled at its
own publication. Nothing in the v0.13.1 section above rewrites it.

## Published v0.13.0 release

The annotated root tag object `c420c4b8043f52de05899b9c5ca5b68cc1307f07`
(`refs/tags/v0.13.0`) and annotated nested-module tag object
`302334ba71732f4aee0df9034f2ecdf9b02e4ed8`
(`refs/tags/verify-witnessed/v0.13.0`) both peel to the approved release commit
`8abd10be7c3f3e5a3727bb73acd0a84811431d3b`.

The [GitHub release](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.13.0)
is published (not draft or prerelease) with 23 assets: six static archives,
six static SPDX SBOMs, four native-cgo archives, four native-cgo SPDX SBOMs,
`SHA256SUMS`, `SHA256SUMS-cgo`, and the fixture-safe
`boundary-evidence.tar.gz`. It was published by the tag-triggered
[release workflow run 33350936936](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/runs/33350936936)
(natural `push` trigger, 8/8 jobs successful). All 23 assets match their
GitHub API SHA-256 digests; `SHA256SUMS` verifies the twelve static files and
`SHA256SUMS-cgo` verifies the four native-cgo archives. All 22 configured
GitHub build-provenance attestations verify against
`release.yml@refs/tags/v0.13.0` at source digest `8abd10b`; `SHA256SUMS-cgo`
is the one expected un-attested asset, matching the documented v0.12.0
workflow behavior. Attestation coverage is 22 configured artifacts, not all
23 assets.

`ghcr.io/fulcrum-governance/boundary:v0.13.0` and `:latest` resolve to
`sha256:cccd3943939dca5b370e6f388797dc3e32f290b562fa680651d0be40a6ba125c`.
That index provides linux/amd64
(`sha256:a56b3f630b41466df3ffc2cfbba57e05fee42aff0af119908e9e1ce6e766b0e9`)
and linux/arm64
(`sha256:96c87ce158b2900c0d749a34de82be6eb523e013aa8505690000db9f290d6e40`)
variants whose labels record version `v0.13.0` and the approved revision. The
Homebrew cask advanced to tap commit
`55396e40e5df1458c2461e02f70140a284da7694` (`Casks/boundary.rb`, version
0.13.0); all four cask SHA-256 values equal the corresponding `SHA256SUMS`
entries. Install/upgrade smoke of the published channels on a user machine is
not yet claimed and is recorded separately when it happens.

Install the root command from the published lowercase root module path:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.13.0
```

Install the nested verifier with this exact, case-sensitive module path:

```bash
go install github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed@v0.13.0
```

Both resolve on the public Go proxy to the approved release commit — the root
module via `refs/tags/v0.13.0` and the nested module via
`refs/tags/verify-witnessed/v0.13.0`. The nested verifier remains
source-distributed only: no verifier-specific archive, container, Homebrew
package, or version flag.

The v0.13.0 release is the Session Receipts and Claude Code hook release:
`boundary hook pretooluse` (a binary-native Claude Code `PreToolUse` boundary
that decides routed `Bash`/`Shell` and `Edit`/`Write`/`MultiEdit`/
`NotebookEdit` tool calls before execution and persists a canonical
`DecisionRecordV1` first), compound-command decomposition in Command Boundary,
governance control-surface denials on both routed lanes, and the Claude Code
plugin, installer, skills, and hook operator surfaces
(`BND-CLAIM-HOOK-001/002/003`, `BND-CLAIM-CMD-003`). The hook is routed-only:
a tool the matcher does not list, an MCP tool, a tool a subprocess runs on its
own, and shell use outside Claude Code are bypasses and are not governed.
Records remain hash-verifiable for covered-field integrity only — integrity,
not authenticity, correctness, or execution proof. Command Boundary and Edit
Boundary — the classifiers the hook routes into — remain delivered previews.
The release does not change adapter maturity or any claim status, and
Boundary does not emit `proved` decisions. The `boundary-plugins` marketplace
repository does not exist and no marketplace submission has been made; the
marketplace scaffold in this repository remains unpublished.

## Published baseline before v0.13.0: v0.12.0

The published `v0.12.0` record is preserved below exactly as reconciled at
its own publication. Nothing in the v0.13.0 section above rewrites it.

## Published v0.12.0 release

The annotated root tag object `38ff2360c3f321646578cb3afd368a3c7cc2e98d`
(`v0.12.0^{}`) and annotated nested-module tag object
`67d49cf9dc6a523b741f06a7cc06d6fa57ce921e`
(`verify-witnessed/v0.12.0^{}`) both peel to the approved release commit
`a2fd0b81fdec312f6b676a96e1fce45b661f00cf`.

The [GitHub release](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.12.0)
is published (not draft or prerelease) with 23 assets: six static archives,
four native-cgo archives, ten SPDX SBOMs, `SHA256SUMS`, `SHA256SUMS-cgo`, and
the fixture-safe `boundary-evidence.tar.gz`. Both checksum manifests verify;
all ten SPDX files parse and pass structural checks; and the evidence archive
is readable. GitHub build-provenance attestations verify for the configured
static archives and SBOMs, `SHA256SUMS`, and native-cgo archives and SBOMs.
The workflow does not create an attestation for `SHA256SUMS-cgo`.

`ghcr.io/fulcrum-governance/boundary:v0.12.0` and `:latest` resolve to
`sha256:557605c59ec9a9aa65c87d2e36ad7c1cd59ffe71791b18d37f4d28a293e4cb84`.
That image provides linux/amd64 and linux/arm64 variants whose labels record
version `v0.12.0` and the approved revision; its offline `version` reports that
revision and `selftest` passes. The Homebrew cask at tap commit
`8a61399d7dcff10dcabf73286d4545762601330a` is `Casks/boundary.rb` (there is no
`Formula/boundary.rb`); strict online cask audit, release-archive hashes, a real
cask install, version readback, and selftest all passed.

Install the root command from the published lowercase root module path:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.12.0
```

That source installation passes `selftest`, but reports `commit: unknown`; do
not use it to infer the release commit. The release archives and container do
report the approved revision.

Install the nested verifier with this exact, case-sensitive module path:

```bash
go install github.com/Fulcrum-Governance/Fulcrum-Boundary/verify-witnessed@v0.12.0
```

The nested verifier is source-distributed only: it has no verifier-specific
archive, container, Homebrew package, or version flag. In a credential-cleared,
no-network run against release-tag fixtures, the valid bundle passed all 12
checks with exit 0 and the `leaf-tamper` fixture exited 1 with
`inclusion_proof` failing. This is offline integrity checking of the supplied
bundle, not proof of external or independent witness operation. Current
witnesses remain Fulcrum-operated on separate infrastructure; the verifier also
does not prove upstream correctness, completeness, or immutability.

The v0.12.0 release adds the relocated Apache-2.0 verifier; the Secure GitHub
bypass-proof ladder and narrow stateful official-MCP route (both still preview);
the proof-receipt sidecar; the wired-witness vocabulary reconciliation; per-host
install docs; the permanent Windows static-only stance; SBOM and provenance
wiring; kernel escalation await mode and pipeline seam; and the Go 1.26.5
toolchain/security refresh. It does not change adapter maturity; Boundary does
not emit `proved` decisions.

GitHub Pages at https://fulcrum-governance.github.io/Fulcrum-Boundary/ returns
200 and its latest Docs workflow is green. Published navigation is limited to
the routes the site exposes; this report does not imply separate release-truth
or install Pages routes.

## Published baseline before v0.12.0: v0.11.0

This report reconciles the public Boundary release surface for the published
`v0.11.0` release — the verifiable-records-breadth release. `v0.10.1` (the
first binary release) survives as the prior history tag; its full
reconciliation is archived at
[`docs/internal/RELEASE_TRUTH_V0101.md`](./internal/RELEASE_TRUTH_V0101.md).
`v0.10.0` remains a valid source-install tag with no release assets (history
in `CHANGELOG.md` and `docs/releases/v0.10.0.md`).

`v0.11.0` packages, on top of the `v0.10.1` surface: standalone TypeScript
and Rust decision-record verifiers, each recomputing `decision_hash` via a
stock RFC 8785 implementation and pinned to Go by the committed
conformance-vector corpus enforced in CI — a record now verifies in Go,
Python, TypeScript, or Rust (always that enumerated list); opt-in Ed25519
record signing, off by default — `boundary serve --receipt-seed FILE` enables
it (failing closed before the listener opens on a bad seed) and
`boundary verify-record --verify-signature --public-key <hex|file>` checks
the signature over the recomputed `decision_hash`, failing closed; signing
never changes `decision_hash`, a valid signature attests who signed the
record and not the verdict or execution, key custody is the operator's, and
the non-Go verifiers remain integrity-only; shell completions, rich help and
`--json` verbs; three native fuzz targets in the required CI set; a hermetic
serve-boot test and RESP codec coverage; and the adapter production-bar
contributor doc.

`v0.10.1` had packaged three lanes on top of the `v0.9.0` surface:

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

Neither `v0.10.1` nor `v0.11.0` adds a new governed action surface, adds a
transport adapter, or upgrades any preview surface to production.

The final public truth is:

- MCP remains the production adapter path.
- Secure GitHub remains preview, with fixture proof plus an opt-in live GitHub
  App conformance harness.
- Command Boundary remains delivered preview and routed-path-only.
- Edit Boundary remains delivered preview and routed-edit-envelope-only.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A remain preview.
- Decision records are hash-verifiable: unkeyed SHA-256 over RFC 8785
  canonical bytes — integrity, not authenticity. As of `v0.11.0`, Ed25519
  signing is opt-in and off by default; a signature attests who signed the
  record, not the verdict or execution, and signed-by-default is never
  claimed.
- `boundary explain` renders a decision record read-only; it does not
  re-verify hashes or prove the verdict was correct or enforced.
- `boundary replay` reproduces the recorded decision for routed requests; it
  does not prove enforcement or that no upstream bytes moved.
- `boundary test` reports policy verdicts for routed request fixtures against
  local policy bundles; it does not prove production route enforcement,
  deployment bypass resistance, or verdict correctness beyond supplied
  fixtures.

## New in v0.12.0

- **Witnessed-log verifier** (`BND-CLAIM-WITNESS-001`, delivered): `verify-witnessed/`
  relocated from Fulcrum's private repository — public, Apache-2.0,
  independently-buildable, zero private-code imports (machine-enforced by its
  own `independence_test.go`). Checks decision-record integrity, manifest,
  source-hash-to-leaf, inclusion proof, tenant binding, STH signature, and
  witness cosignatures offline, against locally supplied Ed25519 public keys.
  Closes the code/license gate only (ADR-041); witnesses today remain
  Fulcrum-operated, not independently operated. See `verify-witnessed/README.md`.

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

Post-tag verification for `v0.13.0` (recorded 2026-08-31 UTC): the natural
tag-push release run 33350936936 completed with all 8 jobs successful
(release-target validation, the release-check gate, static matrix + release
publish, all four native-cgo builds, and the combined cgo checksums); all 23
release assets were downloaded and digest-verified; both checksum manifests
verified; 22 configured attestations verified (`SHA256SUMS-cgo` expectedly
un-attested); the container index and both runtime-platform digests were
inspected; the Homebrew cask advanced at tap commit `55396e4` with matching
hashes; and both Go module paths resolved on the public proxy to the release
commit.

Historical verification: 2026-06-11 at the `v0.11.0` tag commit `a394488` (`make
release-check` exit 0 on the release-prep branch at the same tree, and the
release workflow's `release-check` gate job passed before publish).

`make release-check` runs the root suite, the gRPC nested module suite, the
`tests/` and `claims/` suites, policy verification, receipt verification help,
`boundary selftest`, both fixture demos, `boundary version`,
`boundary doctor --json`, `boundary evidence bundle --include-demo`,
`boundary evidence verify`, and the policy-as-code corpus.

Post-tag verification for `v0.11.0` (recorded 2026-06-11): all 7 release
jobs green; 13 release assets published (5-platform static archives, 4
`_cgo` archives, `SHA256SUMS`, `SHA256SUMS-cgo`, evidence bundle); the
Homebrew formula update landed in `fulcrum-governance/homebrew-tap`
(commit `12080b771`, "boundary v0.11.0"); `brew upgrade boundary` moved a
real machine from `v0.10.1` to `v0.11.0` end-to-end — the first exercise of
the formula update path — and `boundary version` reports `v0.11.0` at
`a394488` with `boundary selftest` passing. The `v0.10.1` post-tag evidence
is archived in
[`docs/internal/RELEASE_TRUTH_V0101.md`](./internal/RELEASE_TRUTH_V0101.md);
prior evidence remains in `docs/internal/`.

## README First-Run Status

README presents install before architecture, leading with the binary
channels:

```bash
brew install fulcrum-governance/tap/boundary
boundary selftest
boundary demo github-lethal-trifecta
boundary demo tamper-evidence
```

The `v0.11.0` source path was:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
```

All demos remain credential-free and perform no live calls or real mutations.
`v0.10.1` ships six demos: `action-boundary`, `postgres`,
`github-lethal-trifecta`, `command-secret-exfil`, `tamper-evidence`, and
`trust-degradation`.

## Claims Status

The `v0.9.0` claims table in
[`docs/internal/RELEASE_TRUTH_V090.md`](./internal/RELEASE_TRUTH_V090.md)
remains accurate for the carried-over claims; no carried claim changed status
in `v0.10.x`. New claims recorded since `v0.9.0`:

| Claim | Status | Release truth |
| --- | --- | --- |
| BND-CLAIM-BUILD-001 | delivered (v0.10.1) | Static `CGO_ENABLED=0` builds are supported; the Postgres AST classifier is unavailable in them, routed SQL classifies `UNKNOWN` and is denied fail-closed, and the static build never allows SQL the cgo build would deny. |
| BND-CLAIM-DIST-001 | delivered (v0.10.1) | Prebuilt distribution channels (release archives + checksums, Homebrew tap, container image, `go install`) publish from the tag-gated pipeline for `v0.10.1` and later; releases up to and including `v0.10.0` shipped source-only. |
| BND-CLAIM-REC-002 | delivered (v0.10.1) | Decision-record hashes are computed over RFC 8785 (JCS) canonical bytes; the conformance statement is record-scoped and is not a whole-product standards claim. |
| BND-CLAIM-VERIFY-002 | delivered (v0.10.1) | A standalone non-Go (Python) verifier recomputes a record's hash and detects a one-field forgery, pinned to Go by the shared conformance-vector corpus. |
| BND-CLAIM-VERIFY-003 | delivered (v0.11.0) | TypeScript verifier, vector-pinned; integrity only, not authenticity. |
| BND-CLAIM-VERIFY-004 | delivered (v0.11.0) | Rust verifier, vector-pinned (including the ECMAScript float round-trip vector); integrity only, not authenticity. |
| BND-CLAIM-SIGN-001 | delivered (v0.11.0) | Opt-in Ed25519 record signing, off by default; a signature attests the signer, not the verdict or execution; signing never changes decision_hash. |

## Feature Status

The `v0.9.0` feature table in
[`docs/internal/RELEASE_TRUTH_V090.md`](./internal/RELEASE_TRUTH_V090.md)
carries forward unchanged. Added in `v0.11.0`:

| Feature | Status | Release truth |
| --- | --- | --- |
| TypeScript standalone verifier | delivered | `verifiers/typescript/` recomputes `decision_hash` via the stock `canonicalize` package; vector-pinned to Go; integrity only. |
| Rust standalone verifier | delivered | `verifiers/rust/` recomputes `decision_hash` via the stock `serde_jcs` crate; vector-pinned to Go; integrity only. |
| Opt-in Ed25519 record signing | delivered | Off by default. `PipelineConfig.ReceiptSigner` / `serve --receipt-seed` sign emitted records (parse rejections included); `verify-record --verify-signature` fails closed; signing never changes `decision_hash`; a signature attests the signer, not the verdict. |
| Shell completions | delivered | `boundary completion bash\|zsh\|fish`, generated from the binary's command table; static scripts. |
| Fuzz targets in required CI | delivered | Record canonicalization round-trip, policy parse, and SQL classifier fuzz targets; seeds run on every `go test ./...`; the CI fuzz job is in the `ci-ok` required set. |
| Serve-boot + RESP test depth | delivered | Hermetic black-box `boundary serve` boot test asserting the governed deny before upstream; RESP codec unit tests for the kernel trust path. |
| Adapter production bar doc | delivered | `docs/ADAPTER_PRODUCTION_BAR.md`: the preview-to-production contract; the bar earns a label, not a guarantee. |

Added in `v0.10.1`:

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
[`docs/internal/RELEASE_TRUTH_V090.md`](./internal/RELEASE_TRUTH_V090.md).

## User-Install Status

The `v0.11.0` published install channels were:

```bash
brew install fulcrum-governance/tap/boundary          # static build
docker pull ghcr.io/fulcrum-governance/boundary:v0.11.0
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
```

Plus release archives with `SHA256SUMS` / `SHA256SUMS-cgo` manifests. Releases
up to and including `v0.10.0` shipped source-only. The published formula and
archives
install the static build; the `_cgo` archives and the source build carry the
full SQL classifier (Go 1.25+ and a C toolchain required for source).

## GitHub Action Ref Status

The v0.11.0 MCP audit action examples used:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.11.0
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

Secure GitHub maps a deployment onto an L0-L3 bypass-proof ladder. Routed live
conformance earns L1 (the denied write-after-taint no-mutation proof). An
operator-attested bypass-proof packet that denies every direct GitHub path earns
L2, the internal production-candidate gate. L2 does not make Secure GitHub
production and does not prove the deployment is bypass-proof; the packet records
and classifies what the operator attested.

Command Boundary remains preview. Direct shell access, CI jobs, and SSH
sessions remain outside Command Boundary unless routed through Boundary command
wrappers or project-local shims.

Edit Boundary remains preview. Direct editor writes, direct filesystem writes,
direct `git apply`, shell redirection, IDE saves, CI jobs, and arbitrary
processes remain outside Boundary unless routed through Boundary edit
envelopes.

Fulcrum Boundary v0.11.0 widens record verification: a decision record
verifies in Go, Python, TypeScript, or Rust — always that enumerated list —
with each standalone verifier pinned to the Go implementation by the
committed conformance-vector corpus, enforced in CI; the verifiers check
integrity, not authenticity. Ed25519 signing is available opt-in and off by
default: a signature attests who signed the record, not the verdict or
execution, key custody is the operator's, and the non-Go verifiers do not
check signatures. v0.11.0 adds no new governed action surface and upgrades
no preview surface to production.

Boundary's decision record can carry a proof-receipt sidecar bound by
decision_hash; attaching it does not change decision_hash and does not add a `proved`
decision mode. The proof-receipt sidecar is checker-validated: it carries the
wired witness for the budget and static-privilege invariants. Boundary's runtime
behavior corresponds to a machine-checked equilibrium analysis (a Nash equilibrium
and an exact price-of-anarchy bound) upstream in Fulcrum-Proofs; that
correspondence is a design constraint, not a runtime certificate, and trust
termination is a circuit-transition consistency check, not a per-decision
termination proof.

## Forbidden Release Language

Do not use these as public capability claims:

- Do not claim universal prompt-injection prevention.
- Do not claim production Secure GitHub.
- Do not claim Secure GitHub fully secures GitHub.
- Do not claim live conformance proves deployment bypass resistance.
- Do not use "production-candidate" as public copy; it is an internal planning
  word and Secure GitHub stays preview until release truth changes.
- Do not claim the Secure GitHub bypass-proof packet proves the deployment is
  bypass-proof; it records operator attestations and routed evidence and
  classifies the ladder level.
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
  is always enumerated: Go, Python, TypeScript, Rust.
- Do not claim signed receipts or signing by default; signing is opt-in, off
  by default, and attests the signer rather than the verdict.
- Do not claim Boundary aligns with or implements
  `draft-sharif-agent-audit-trail`; the shared per-record algorithms and the
  unimplemented `prev_hash` session chain must be stated together or not at
  all.
- Do not claim Boundary's equilibrium correspondence to enforcement is a runtime certificate; the correspondence is design-only and the wired witness covers only budget and static-privilege invariants.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Docs Checked

This 2026-06-11 revision verified, against the `v0.11.0` tag (`a394488`):

- `README.md` (install channels, first-run, forge-the-receipt language)
- `docs/INSTALL.md` (channel availability, `@v0.11.0` targets)
- `CHANGELOG.md` (`[0.10.0]`/`[0.10.1]` history, `[Unreleased]` content)
- `docs/releases/v0.10.0.md` and `docs/releases/v0.10.1.md`
- `claims/boundary_claims.yaml` (claim diff `v0.10.1..v0.11.0`)
- `verifiers/python/README.md`
- `docs/SIGNING.md`
- `docs/releases/v0.11.0.md`
- `docs/CLI_REFERENCE.md`
- Release assets, tap formula, and container image (post-tag evidence above)

The `v0.9.0` reconciliation's full docs-checked list is preserved in
[`docs/internal/RELEASE_TRUTH_V090.md`](./internal/RELEASE_TRUTH_V090.md).

## Drift Fixed

- Updated active public truth from `v0.10.1` to the published `v0.11.0`
  release; archived the prior active truth to
  `docs/internal/RELEASE_TRUTH_V0101.md` (relative links corrected for the
  directory move).
- Collapsed the "Shipped On Main, Unreleased" section: `main` and the
  `v0.11.0` tag are identical at this reconciliation, so the TypeScript/Rust
  verifiers and opt-in Ed25519 signing moved from fenced main-status to
  released claims (`BND-CLAIM-VERIFY-003`, `BND-CLAIM-VERIFY-004`,
  `BND-CLAIM-SIGN-001` — delivered, v0.11.0).
- Activated the previously pre-approved release language: enumerated
  four-language verification and opt-in, off-by-default signing with the
  attests-the-signer caveat.
- Recorded the `v0.11.0` post-tag evidence: 7/7 release jobs, 13 assets, the
  tap formula update, and the first end-to-end `brew upgrade` exercise of the
  formula update path.
- Updated install, container, and GitHub Action examples to `@v0.11.0`.
- Kept the standards position unchanged and precise: shared per-record
  algorithms with `draft-sharif-agent-audit-trail-00`, unimplemented session
  chain, no alignment claim.
