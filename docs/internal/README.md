# Internal release-process docs

These are **internal release-process records**, not product documentation. They
capture per-version release-truth reconciliation, claims audits, and launch
freezes from Boundary's development history. They are not part of the published
documentation site (`mkdocs.yml` builds from `docs-site/`).

For canonical, product-facing truth, start with:

- [`docs/RELEASE_TRUTH_PUBLIC.md`](../RELEASE_TRUTH_PUBLIC.md) — the current,
  authoritative public release truth.
- [`docs/CLAIMS_LEDGER.md`](../CLAIMS_LEDGER.md) — the human-readable claims
  ledger (machine source: `claims/boundary_claims.yaml`).

Files here are retained because some are referenced as evidence paths by the
claims ledger; treat them as superseded by `RELEASE_TRUTH_PUBLIC.md` for any
current claim.
