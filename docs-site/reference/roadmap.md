# Boundary Roadmap

Canonical repository reference:
[docs/BOUNDARY_ROADMAP.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/BOUNDARY_ROADMAP.md)

A developer-facing roadmap organized around one question: how far can you trust
the decision record a verdict leaves behind? It separates what is shipped from
what is planned, and never presents a planned item as current.

The **shipped baseline** is the current release: the `DecisionRecordV1`
structured decision record (`schema_version "1"`), `boundary verify-record`
receipt-grade verification, `boundary doctor`, `boundary evidence bundle` /
`verify`, and the two proof-lane demos.

The **planned** phases are forward-looking and are not in the current release.
Phase 0A (Trust the Record / Evidence UX) scopes route-context record fields, a
read-side `boundary explain`, and a local `boundary replay`. Phase 0B scopes
deeper `doctor` environment diagnostics, redacted report output, and clearer
README and demo hierarchy. Phase 1 describes a deferred, local, fixture-only
policy-as-code test lane. None of these are shipped, and the page states its
non-goals explicitly: no signing, no cryptographic proof of the verdict, no
topology attestation, and no independent proof that no upstream bytes moved.

Related references:
[CLI](cli.md) ·
[Route Conformance](route-conformance.md) ·
[Release Utilities](release-utilities.md) ·
[Claims](claims.md)
