# How We Keep Ourselves Honest

Canonical repository reference:
[docs/HOW_WE_KEEP_OURSELVES_HONEST.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/HOW_WE_KEEP_OURSELVES_HONEST.md)

Every public claim in this repo is bound to named test paths and doc paths that
must exist on disk. A language lint rejects specific overclaim phrases from
public docs unless they are negated or limitation-framed. Production-labeled
routes must pass a readiness checklist. None of this is policy — it is a build
gate enforced by `go test ./claims/...` and `make release-check`.

Key mechanisms:

- **Claims ledger gate** (`claims/claims_test.go`): every `delivered` claim
  requires at least one test path and one doc path that exist on disk; every
  `partial` claim must list structured gaps; every `false` claim must not appear
  in `README.md`.
- **Language lint gate** (`claims/language_lint_test.go`): controlled overclaim
  phrases (`SQL firewall`, `proves all prompt injection`, `proved decisions`,
  `secure sandbox`, `all adapters production`, `fully secures GitHub`, and
  others) fail the build on any non-negated, non-limitation-framed line in
  scanned public docs.
- **Adapter readiness gate** (`tests/adapter_conformance/`): a `production`
  label requires non-stub lifecycle steps, a `bypass_proof` step that is
  `implemented` or formally delegated, at least one fail-closed transport, and
  test evidence on disk.

Clone the repo and run `make release-check` to verify this yourself.

Related references:
[Claims](claims.md) ·
[Route Conformance](route-conformance.md) ·
[Claims Ledger](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLAIMS_LEDGER.md)
