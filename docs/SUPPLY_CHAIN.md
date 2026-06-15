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

- **SPDX SBOM per static archive.** [`.goreleaser.yaml`](../.goreleaser.yaml)
  runs [syft](https://github.com/anchore/syft) over each
  `*_static-nocgo.{tar.gz,zip}` archive and attaches a matching
  `*.spdx.json` (SPDX 2.3) listing the Go module dependencies compiled into the
  binary.
- **Build-provenance attestation for release artifacts.**
  [`.github/workflows/release.yml`](../.github/workflows/release.yml) uses
  [`actions/attest-build-provenance`](https://github.com/actions/attest-build-provenance)
  to record a signed provenance attestation (via GitHub's OIDC identity) for the
  static archives, their SBOMs, the `SHA256SUMS` manifest, and each native-cgo
  archive. The attestation binds an artifact's digest to the workflow,
  repository, commit, and runner that produced it.

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

## Honest scope (status)

- **Verified now:** SPDX SBOM generation for the static archives is exercised by
  `goreleaser release --snapshot --clean --skip=publish,docker` (six archives,
  six SBOMs) and pinned by a wiring test in `tests/supplychain/`.
- **Wired, takes effect at the next tagged release:** build-provenance
  attestation runs only on a `v*` tag (it needs the release workflow's OIDC
  token), and the cgo-archive SBOM is not yet generated. These are not claimed as
  a shipped release capability until the first tag after they land —
  see `BND-CLAIM-DIST-002` (`partial`) in
  [CLAIMS_LEDGER.md](./CLAIMS_LEDGER.md) and its gap `BND-DIST-002`.
- This page describes distribution supply-chain metadata only. It makes no claim
  about upstream package, model, or MCP-server supply chains (see
  `docs/STANDARDS_MAPPING.md`, ASI04).
