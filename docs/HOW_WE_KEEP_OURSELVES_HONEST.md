# How We Keep Ourselves Honest

This page explains the mechanical honesty system built into this repository's
release process. It is aimed at an outside skeptic who wants to know whether
the claims in the README and docs are trustworthy, and why.

The short version: every public claim is bound to a named test and a named doc
that must exist on disk, a controlled-language lint rejects specific overclaim
phrases from public docs, and each production-labeled route must pass a readiness
checklist before it can carry that label. None of this runs on trust. It runs on
`go test ./claims/...` and `make release-check`, which fail the build if the
evidence is absent or the language is wrong.

Clone the repo and run `make release-check` yourself. The gate is not a
document. It is an exit code.

---

## 1. The Claims Ledger Gate

Every public claim lives in `claims/boundary_claims.yaml`. The gate in
`claims/claims_test.go` parses that file and enforces the following rules on
every run:

**`delivered` claims** must list at least one test path and at least one doc
path. Both paths must exist on disk. If a path is missing, the test fails and
the release is blocked.

**`partial` claims** must list at least one structured gap with a build-task ID
matching `BND-[A-Z0-9]+-[0-9]{3}`, a description, and a spec reference. A gap
without those fields fails the build.

**`false` claims** — things the repo explicitly disavows — must not appear
verbatim in `README.md`. If they do, the build fails. This is the mechanism
that enforces, for example, that "Boundary is a SQL firewall" does not appear
in the README.

The status vocabulary (`delivered`, `partial`, `planned`, `false`) means what
it says. A claim in `delivered` status without a real test file on disk cannot
exist in this repo. A claim in `partial` status cannot be presented in external
communications without its gaps.

The human-readable rendering of the ledger is at
[docs/CLAIMS_LEDGER.md](CLAIMS_LEDGER.md).

---

## 2. The Language Lint Gate

`claims/language_lint_test.go` scans public documentation for controlled
overclaim phrases. Scanned paths include `README.md`, `CHANGELOG.md`,
`docs/*.md`, `docs/adapters/*.md`, `docs/firewall/*.md`, `docs/secure-mcp/*.md`,
`docs/policies/*.md`, and `docs/deployment/*.md`.

A phrase on the banned list causes the test to fail — **unless** the line that
contains it is negated or limitation-framed. The allowed framing words are: `not`,
`do not`, `does not`, `must not`, `avoid`, `false`, `forbidden`, `prohibited`,
`without`, `unless`, `until`.

This is why the "What It Does Not Prove" tables in the docs are written the way
they are. The hedging is not stylistic. It is the mechanism by which those
sentences pass the lint.

### Controlled phrases (sourced directly from `claims/language_lint_test.go`)

The following phrase categories are controlled. The build does not allow any of
them to appear on a non-negated, non-limitation-framed line in any scanned
public doc. The exact string each rule matches is listed after the rule name.
(This page passes the lint because each phrase below appears on a line that
contains `does not allow`.)

- **generic platform lead** (headline-only): the build does not allow `AI governance platform` in a heading line.
- **SQL firewall overclaim**: the build does not allow `SQL firewall` or `prevents all SQL injection`.
- **universal prompt-injection overclaim**: the build does not allow `prevents all prompt injection` or `universal prompt-injection prevention`.
- **universal agent safety overclaim**: the build does not allow `universal agent safety`.
- **runtime proof overclaim**: the build does not allow `proved decision` or `proved decisions`.
- **secure sandbox overclaim**: the build does not allow `secure sandbox` or `secure sandboxing` without a real, named, tested caveat.
- **adapter maturity overclaim**: the build does not allow `all adapters production`, `six production adapters`, or `seven production adapters`.
- **unverified competitive claim**: the build does not allow `no other tool does this` or `no one else detects this`.
- **GitHub production overclaim**: the build does not allow `fully secures GitHub`, `production GitHub security`, or `detects every malicious issue`.

A small set of control files are exempt from the scan because they must be able
to reference the controlled vocabulary to define it. Those files are
`docs/CLAIMS_LEDGER.md`, `docs/COPY_RULES.md`, `docs/LANGUAGE_SYSTEM.md`,
`docs/LEXICON.md`, `docs/BOUNDARY_PRODUCT_PRIMITIVES.md`, `docs/BOUNDARY_SPEC.md`,
`docs/internal/LAUNCH_TRUTH_FREEZE.md`, and
`docs/internal/RELEASE_TRUTH_RECONCILIATION.md`.

---

## 3. Adapter Readiness: What Production Means

Every transport adapter under `adapters/` carries a `readiness.yaml` file. The
conformance test in `tests/adapter_conformance/adapter_readiness_test.go`
enforces the following rules for any adapter labeled `production`:

- It must declare evidence tests (no empty evidence list).
- No lifecycle step (`parse`, `identify`, `evaluate`, `deny`, `forward`,
  `inspect`, `metadata`, `record`, `bypass_proof`, `fail_closed`) may be in
  `stub` state.
- The `bypass_proof` step must be either `implemented` or formally `delegated`
  to a named owner with a named contract document that exists on disk.
- It must declare at least one fail-closed transport.

The conformance test also verifies that the README and the adapter readiness
matrix list every adapter's maturity label and every lifecycle step. An adapter
that has not satisfied these requirements cannot carry the `production` label
without breaking the build.

Today, one adapter carries `production` status: `adapters/mcp`. All other
adapter packages carry `preview` status and are labeled as such in the README
and readiness matrix.

---

## 4. The Release Gate

`make release-check` runs `scripts/release-check.sh`, which executes the
following in sequence and exits non-zero on any failure:

1. Vendor and internal-artifact assertions (no internal material on public
   surfaces).
2. `go vet` on the root module and the `adapters/grpc` nested module.
3. `go test ./...` on the root, the `adapters/grpc` module, and `tests/`.
4. `go test ./claims/...` — the ledger gate and the language lint gate.
5. Live `boundary` CLI invocations: `verify`, `verify-record --help`, the
   policy-as-code corpus (`test`), `version`, `selftest`, both demos,
   `doctor --json`, and `evidence bundle`/`verify`.

A release-truth change is not done until this passes in full.

---

## 5. What This System Does Not Prove

The honesty system proves that the repo's claims are grounded in named tests and
docs, and that controlled language does not appear on non-negated lines. It does
not prove:

- That governed routes are the only paths to a tool in any given deployment.
  Direct access to the same tool is a bypass unless deployment topology removes
  that path.
- That preview adapters meet production deployment requirements. Their gaps are
  listed in the ledger and readiness files.
- That decision records carry cryptographic attestation or independent
  corroboration of execution outcomes. Records are hash-verifiable for the
  fields they cover; that is what "hash-verifiable" means here, nothing more.
- That any runtime enforcement guarantee holds beyond routed paths.

---

The authoritative claims ledger is at [docs/CLAIMS_LEDGER.md](CLAIMS_LEDGER.md).
