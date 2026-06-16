# Supply Chain: SBOM & Build Provenance

Boundary's tag-gated release pipeline attaches supply-chain metadata to its
release artifacts so a consumer can answer two questions about a downloaded
binary: **what is in it** (SBOM) and **where it came from** (build provenance).

This is provenance for the **release distribution**. It is distinct from runtime
**decision-record** signing: Boundary does **not** sign decision records by
default (see [PROOF_BOUNDARY.md](./PROOF_BOUNDARY.md) and issue #134), and the
`boundary version` command reports local build metadata only — it does not prove
provenance. You verify provenance with `gh attestation verify`, not with the
binary itself.

## What the pipeline produces

- **SPDX SBOM per release archive.** [`.goreleaser.yaml`](../.goreleaser.yaml)
  runs [syft](https://github.com/anchore/syft) over each
  `*_static-nocgo.{tar.gz,zip}` static archive, and
  [`.github/workflows/release.yml`](../.github/workflows/release.yml) (the
  `cgo-binaries` job) runs syft over each `*_cgo.tar.gz` native-cgo archive. Each
  archive gets a matching `*.spdx.json` (SPDX 2.3) listing the Go module
  dependencies compiled into the binary.
- **Build-provenance attestation for release artifacts.**
  [`.github/workflows/release.yml`](../.github/workflows/release.yml) uses
  [`actions/attest-build-provenance`](https://github.com/actions/attest-build-provenance)
  to record a signed provenance attestation (via GitHub's OIDC identity) for the
  static archives, their SBOMs, the `SHA256SUMS` manifest, and each native-cgo
  archive and its SBOM. The attestation binds an artifact's digest to the
  workflow, repository, commit, and runner that produced it.

## Verifying a downloaded artifact

```bash
# 1. Integrity: checksums (see docs/INSTALL.md)
shasum -a 256 -c SHA256SUMS --ignore-missing

# 2. Provenance: the artifact was built by this repo's release workflow
gh attestation verify boundary_<version>_<os>_<arch>_static-nocgo.tar.gz \
  --repo Fulcrum-Governance/Fulcrum-Boundary

# 3. Contents: read the SBOM attached to the release
cat boundary_<version>_<os>_<arch>_static-nocgo.tar.gz.spdx.json | jq '.packages[].name'
```

The same commands apply to a native-cgo archive — substitute the `_cgo.tar.gz`
suffix (and `SHA256SUMS-cgo` for checksums). One integrity note: the
static-archive SBOMs are listed in `SHA256SUMS`, but the cgo-archive SBOM is
**not** in `SHA256SUMS-cgo` (that manifest covers the archives only) — the cgo
SBOM's integrity is instead covered by its build-provenance attestation, which
binds its digest (`gh attestation verify <the .spdx.json>`).

## Honest scope (status)

- **Verified short of a release:** static-archive SPDX SBOM generation is
  exercised by `goreleaser release --snapshot --clean --skip=publish,docker` (six
  archives, six SBOMs); the cgo-archive SBOM command (`syft … spdx-json`) is
  verified by running syft on a locally-built cgo archive (valid SPDX 2.3). The
  wiring for both is pinned by tests in `tests/supplychain/`.
- **Wired, takes effect at the next tagged release:** build-provenance
  attestation (both archive families) and the native-cgo archive SBOM run only on
  a `v*` tag — the cgo SBOM in the per-runner `cgo-binaries` matrix, attestation
  via the workflow's OIDC token. They are not claimed as a shipped release
  capability until the first tag after they land — see `BND-CLAIM-DIST-002`
  (`partial`) in [CLAIMS_LEDGER.md](./CLAIMS_LEDGER.md) and its gap `BND-DIST-002`.
- This page describes distribution supply-chain metadata only. It makes no claim
  about upstream package, model, or MCP-server supply chains (see
  `docs/STANDARDS_MAPPING.md`, ASI04).
