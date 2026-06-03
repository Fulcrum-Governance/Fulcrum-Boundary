# Release Utilities

Boundary's local utility commands diagnose the routed surface, exercise
fixture-safe examples, package local evidence, verify records, and run local
policy-as-code assertions.

```bash
boundary version
boundary doctor --json
# source builds after v0.9.0:
boundary doctor --report
boundary demo action-boundary
boundary verify-record record.json
boundary test --path tests/fixtures/policy-test/cases
boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
```

Availability note: `boundary test` is included in the `v0.9.0` release. The
`@v0.9.0` install includes it; the historical `@v0.8.0` install does not.
Source builds after `v0.9.0` also include `boundary doctor --report`, which
emits redacted JSON for support threads; the pinned `@v0.9.0` install does not
include that flag until the next release tag.

These commands are local-first. They do not require credentials, do not make
live GitHub calls by default, and do not mutate real systems by default.

## What They Prove

| Command | Proof |
| --- | --- |
| `boundary version` | The binary can report local build metadata, module path, Go runtime, and schema-versioned JSON. |
| `boundary doctor` | Local first-run diagnostics, routed-surface diagnostics, and bypass caveats can be rendered without network calls. |
| `boundary doctor --report` | Source builds after `v0.9.0` can emit the same local diagnostics as redacted JSON for support threads. |
| `boundary demo action-boundary` | Fixture-only MCP / Secure GitHub, Command Boundary, and Edit Boundary paths can be shown together. |
| `boundary verify-record` | A single decision record can be recomputed for internal hash consistency. |
| `boundary test` | Local policy bundles can be evaluated against operator-authored request fixtures with expected verdicts. |
| `boundary evidence bundle` | Local release artifacts can be packaged with a manifest and SHA-256 hashes. |
| `boundary evidence verify` | A local evidence bundle can be checked for manifest shape, artifact existence, hash integrity, and fixture-safe summary references. |

## What They Do Not Prove

- Production route enforcement.
- Deployment bypass resistance.
- Production Secure GitHub, Command Boundary, or Edit Boundary maturity.
- Universal attack prevention.
- Cryptographic release provenance.
- Global verdict correctness beyond the supplied local fixtures and policy
  bundle.

MCP remains the production adapter path. Secure GitHub, Command Boundary, and
Edit Boundary remain preview surfaces.
