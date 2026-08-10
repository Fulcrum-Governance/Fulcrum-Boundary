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

- **Published and verified at `v0.12.0`:** all ten SPDX files parse and pass
  structural checks; both checksum manifests verify; and GitHub
  build-provenance attestations verify for configured static archives and SBOMs,
  `SHA256SUMS`, and native-cgo archives and SBOMs. `SHA256SUMS-cgo` is not
  attested because the workflow does not create an attestation for it.
- The build wiring remains pinned by tests in `tests/supplychain/`. It describes
  distribution metadata, not a statement about the contents or provenance of
  upstream packages, models, or MCP servers.
- This page describes distribution supply-chain metadata only. It makes no claim
  about upstream package, model, or MCP-server supply chains (see
  `docs/STANDARDS_MAPPING.md`, ASI04).
