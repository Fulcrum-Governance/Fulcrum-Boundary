# Boundary Roadmap

Canonical repository reference:
[docs/BOUNDARY_ROADMAP.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/BOUNDARY_ROADMAP.md)

A developer-facing roadmap organized around one question: how far can you trust
the decision record a verdict leaves behind? It separates what is in the
current `v0.11.0` release, what remains planned, and which caveats still apply.

The **shipped baseline** includes the `DecisionRecordV1` structured decision
record (`schema_version "1"`), `boundary verify-record` receipt-grade
verification, `boundary doctor`, `boundary evidence bundle` / `verify`, and
the two proof-lane demos.

**Shipped in `v0.8.0` and still included:** Phase 0A (Trust the Record /
Evidence UX) added route-context record fields (`DecisionRecordV2`,
`schema_version "2"`), a read-side `boundary explain`, and a local
`boundary replay`. `boundary replay` reproduces the *decision*, not the
absence of upstream side effects.

**Shipped in `v0.9.0` and still included:** Phase 1 added `boundary test`, a
local, fixture-only policy-as-code test runner over local YAML policy bundles
and request fixtures. The historical `@v0.8.0` install does not include it.

Phase 0B has shipped in the current release: deeper `doctor` environment
diagnostics, redacted report output, and clearer README and demo hierarchy.
The page states its non-goals explicitly: no signing, no cryptographic proof
of the verdict, no topology attestation, and no independent proof that no
upstream bytes moved.
`boundary test` has its own local-only caveat: it reports policy verdicts for
routed request fixtures only and does not prove production route enforcement or
deployment bypass resistance.

Related references:
[CLI](cli.md) ·
[Policy Testing](policy-testing.md) ·
[Route Conformance](route-conformance.md) ·
[Release Utilities](release-utilities.md) ·
[Claims](claims.md)
