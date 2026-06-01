# Boundary Roadmap

Canonical repository reference:
[docs/BOUNDARY_ROADMAP.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/BOUNDARY_ROADMAP.md)

A developer-facing roadmap organized around one question: how far can you trust
the decision record a verdict leaves behind? It separates what is shipped on
`main` from what is planned, and never presents a planned item as current.

The **shipped baseline** in the current `v0.7.0` release is the `DecisionRecordV1`
structured decision record (`schema_version "1"`), `boundary verify-record`
receipt-grade verification, `boundary doctor`, `boundary evidence bundle` /
`verify`, and the two proof-lane demos.

**Shipped on `main`, not in the `v0.7.0` release:** Phase 0A (Trust the Record /
Evidence UX) adds route-context record fields (`DecisionRecordV2`,
`schema_version "2"`), a read-side `boundary explain`, and a local
`boundary replay`. These are in the codebase on `main` and exercised by tests,
but the latest tag (`v0.7.0`) predates them, so the `@v0.7.0` install does not
include them; build from source (`make build`) to use them until a release that
includes them is tagged. `boundary replay` reproduces the *decision*, not the
absence of upstream side effects.

The **planned** phases are forward-looking and are not in the codebase. Phase 0B
scopes deeper `doctor` environment diagnostics, redacted report output, and
clearer README and demo hierarchy. Phase 1 describes a deferred, local,
fixture-only policy-as-code test lane. Neither of these is shipped, and the page
states its non-goals explicitly: no signing, no cryptographic proof of the
verdict, no topology attestation, and no independent proof that no upstream bytes
moved.

Related references:
[CLI](cli.md) ·
[Route Conformance](route-conformance.md) ·
[Release Utilities](release-utilities.md) ·
[Claims](claims.md)
