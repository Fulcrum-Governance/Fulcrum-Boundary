# Boundary Roadmap

Canonical repository reference:
[docs/BOUNDARY_ROADMAP.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/BOUNDARY_ROADMAP.md)

A developer-facing roadmap organized around one question: how far can you trust
the decision record a verdict leaves behind? It separates what is in the
`v0.9.0` release, what remains planned, and which caveats still apply.

The **shipped baseline** in the current `v0.9.0` release is the `DecisionRecordV1`
structured decision record (`schema_version "1"`), `boundary verify-record`
receipt-grade verification, `boundary doctor`, `boundary evidence bundle` /
`verify`, and the two proof-lane demos.

**Shipped in `v0.8.0` and included in `v0.9.0`:** Phase 0A (Trust the Record /
Evidence UX) adds route-context record fields (`DecisionRecordV2`,
`schema_version "2"`), a read-side `boundary explain`, and a local
`boundary replay`. The `@v0.9.0` install includes them. `boundary replay`
reproduces the *decision*, not the absence of upstream side effects.

**In the `v0.9.0` release:** Phase 1 adds `boundary test`, a local,
fixture-only policy-as-code test runner over local YAML policy bundles and
request fixtures. The `@v0.9.0` install includes it; the historical `@v0.8.0`
install does not.

Phase 0B source-main work has landed after `v0.9.0`, but it is not in the
tagged `@v0.9.0` install: deeper `doctor` environment diagnostics, redacted
report output, and clearer README and demo hierarchy. The page states its
non-goals explicitly: no signing, no cryptographic proof of the verdict, no
topology attestation, and no independent proof that no upstream bytes moved.
`boundary test` has its own local-only caveat: it reports policy verdicts for
routed request fixtures only and does not prove production route enforcement or
deployment bypass resistance.

Related references:
[CLI](cli.md) ·
[Policy Testing](policy-testing.md) ·
[Route Conformance](route-conformance.md) ·
[Release Utilities](release-utilities.md) ·
[Claims](claims.md)
