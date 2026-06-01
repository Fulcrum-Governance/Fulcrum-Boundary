# Route Conformance Checklist

Canonical repository reference:
[docs/ROUTE_CONFORMANCE_CHECKLIST.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/ROUTE_CONFORMANCE_CHECKLIST.md)

A documented, per-route checklist for two questions:

1. Does the route implement the ten governance lifecycle steps (`parse`,
   `identify`, `evaluate`, `deny`, `forward`, `inspect`, `metadata`, `record`,
   `bypass_proof`, `fail_closed`), or formally delegate the steps it does not
   own?
2. Has the route earned the maturity label it carries (`experimental` /
   `preview` / `production`)?

It also records concrete preview-to-production graduation criteria, a Command
Boundary graduation plan scoped to routed command paths only, and a caveat table
that distinguishes a governed route from a globally controlled system.

A passing checklist confirms a route is forced through Boundary in your
deployment and that its lifecycle is accounted for. It does not prove that no
other path to the same tool exists; direct binary use, direct shell, CI or SSH
execution, and editor-embedded terminals are bypasses unless deployment topology
removes those paths. The machine-readable truth lives in
`adapters/<adapter>/readiness.yaml` and the readiness matrix.

Related references:
[Release Utilities](release-utilities.md) ·
[Claims](claims.md) ·
[Adapter Readiness Matrix](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/ADAPTER_READINESS_MATRIX.md)
