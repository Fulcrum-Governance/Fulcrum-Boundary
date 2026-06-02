# Policy Testing

Canonical repository reference:
[docs/POLICY_TESTING.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/POLICY_TESTING.md)

`boundary test` runs local, fixture-only policy-as-code test cases against
Boundary policy bundles.

```bash
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
```

It is local-only: no credentials, no network calls, and no live mutation. The
JSON envelope is `boundary.test.v1` and includes one result per case plus the
local-safety flags.

Availability note: this is a post-`v0.8.0` command on `main` after the Phase 1
policy-testing lane merges. The `@v0.8.0` install does not include it.

What it does not prove:

- Production route enforcement.
- Deployment bypass resistance.
- That every direct or unrouted path to the same tool has been removed.
- That the verdict is globally correct beyond the supplied fixture and local
  policy bundle.

Related references:
[CLI](cli.md) ·
[Route Conformance](route-conformance.md) ·
[Roadmap](roadmap.md) ·
[Claims](claims.md)
