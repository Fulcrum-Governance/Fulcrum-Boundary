# Boundary Test Golden Corpus

This corpus exercises `boundary test` as a local, fixture-only policy assertion
runner. The case files under `cases/` point at committed policy bundles under
`policies/` and expect one verdict each.

The corpus covers `allow`, `deny`, `warn`, `require_approval`, `escalate`, and
an expected `parse_rejection` for a deliberately malformed policy bundle.

Passing this corpus reports policy verdicts for routed requests only. It does not prove production route enforcement, does not prove a deployment removed direct or unrouted paths to a tool, and does not prove the verdict was globally correct.
