# Testing

Boundary's test suite is layered, deterministic, and wired to its claims. This
page explains the test architecture, how to add the two kinds of fixtures, the
claims-ledger contract, and one coverage-shape caveat an external reader will
otherwise misread.

## Running the tests

```bash
go test ./... -count=1 -timeout 5m            # the fast path most contributors run
go test ./claims/... -count=1                 # the claims ledger + language lint gate
git ls-files '*.go' | xargs gofmt -l          # prints no paths when clean
go vet ./...
make release-check                            # the full release gate CI enforces
```

There is no `-short` mode: there are **zero `testing.Short()` guards** in the
tree, so `-short` would be a silent no-op. The full suite runs well under five
minutes.

## Policy-as-code tests

`boundary test` runs local policy-as-code cases against local policy bundles:

```bash
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
```

The committed corpus under `tests/fixtures/policy-test/` covers `allow`, `deny`,
`warn`, `require_approval`, `escalate`, and an expected `parse_rejection`.
Failures exit non-zero, so the command is suitable for CI. It is local-only and
fixture-safe: no credentials, no network, and no live mutation. See
[`docs/POLICY_TESTING.md`](./POLICY_TESTING.md) for the case format and caveats.

## Test architecture

Tests are layered, not flat — 492 `func Test` across 107 `_test.go` files in the
main module, plus the `adapters/grpc` nested module.

- **Co-located unit tests** — `*_test.go` next to the code in `governance/`,
  `policyeval/`, `interceptors/`, `adapters/*`, `internal/*`.
- **Black-box integration** — `_test` packages under `tests/` (16+
  subdirectories). Load-bearing fail-closed end-to-end tests include
  `TestMCPGateway_DeniedRequestNeverReachesUpstream`,
  `TestMCPGateway_FailClosedOnPipelineError`,
  `TestMCPBypassProbeFailsWhenDirectPathIsClosed`, and
  `TestStandaloneBundleBootsWithoutExternalDependencies`.
- **Cross-transport parity** — `governance/parity_test.go` asserts identical
  verdicts across MCP / CLI / other transports for one pipeline config.
- **Adapter conformance + readiness** — `tests/adapter_conformance/` validates
  every `adapters/*/readiness.yaml`, requires `production` adapters to carry
  conformance evidence with no `stub` steps, and cross-checks the README and
  `ADAPTER_READINESS_MATRIX.md`.
- **Red-team fixtures** — `internal/redteam/` + `tests/redteam/`: the
  `github-lethal-trifecta` deny pack, the three command packs
  (`command-overeager-cleanup`, `command-repo-mutation`,
  `command-secret-exfil`), and the edit diff fixtures. Each asserts the expected
  `deny` / `require_approval`, asserts nothing executed (`Executed == false`) /
  nothing applied, uses no real secrets and no live mutation, and emits a
  decision record with a `RecordID` + `DecisionHash`.
- **SQL evasion corpus** — `interceptors/sql/evasion_corpus/postgres.yaml` is
  replayed against the AST classifier, with a build-failing floor if the corpus
  shrinks.
- **The claims spine** — `claims/claims_test.go` enforces the ledger per status
  and `claims/language_lint_test.go` lints public language.

### Determinism / flake posture

`-count=1` (no retries), no `t.Parallel()`, and no `//go:build` tags on test
files (every test compiles and runs every time). Patterns are hermetic
(`t.TempDir`, `httptest`, in-process). The only `t.Skip`s are env-gated live
conformance suites (`tests/conformance/secure_github`,
`tests/conformance/managed_agents`) — these are opt-in live testing, not
disabled testing.

## The coverage number, explained once

Aggregate statement coverage reads about **43.8%**, and several
security-critical packages read **0.0%** — `interceptors/sql`,
`governance/kernel`, `adapters/managedagents`, plus `cmd/boundary`,
`internal/doctor`, `internal/evidence`, `governance/standalone`.

**This is a Go black-box-test attribution artifact, not under-testing.** Those
packages are exercised by external `_test`-package suites under `tests/`
(`tests/interceptors/sql_*`, `tests/integration/` kernel tests,
`tests/adapters/` + `tests/conformance/managed_agents`, `tests/cli_output/`,
`tests/doctor/`, `tests/evidence/`). Because those tests live in a *separate*
package, Go attributes the coverage to the external test package, so the source
package reads 0.0%. The headline number therefore **understates** what is
actually exercised; it does **not** mean those packages are untested.

Mapping of the 0.0% source packages to the black-box suite that covers them:

| Source package (reads 0.0%) | Covering black-box suite |
|---|---|
| `interceptors/sql` | `tests/interceptors/sql_*` (+ `evasion_corpus`) |
| `governance/kernel` | `tests/integration/` kernel tests |
| `adapters/managedagents` | `tests/adapters/`, `tests/conformance/managed_agents` |
| `cmd/boundary` | `tests/cli_output/`, end-to-end CLI tests |
| `internal/doctor` | `tests/doctor/` |
| `internal/evidence` | `tests/evidence/` |
| `governance/standalone` | `tests/integration/` standalone bundle tests |

Guidance:

- Do **not** ship a naive coverage badge using 43.8%, and do **not** let it be
  cited as "half untested" — that reading is wrong.
- For a real figure, generate a cross-package profile:
  `go test ./... -coverpkg=./... -coverprofile=cover.out`.
- The well-covered core is real: `interceptors` ~94%, `policyeval` ~86%,
  `adapters/codeexec` ~85%, `adapters/cli` ~81%, `adapters/webhook` ~79%,
  `internal/selftest` ~74%, `adapters/grpc` ~74%.
- Genuinely thin direct spots (distinct from the artifact) worth a targeted unit
  pass — `governance.Pipeline.Evaluate` and `adapters/mcp` — are high-value
  insurance, not a launch blocker.

There is **no fuzzing yet** (zero `func Fuzz`); a seeded `FuzzClassifyPostgres`
wired into CI is a recommended fast-follow, not a current claim.

## The claims-ledger contract

`claims/boundary_claims.yaml` is the machine-readable ledger (rendered for
humans at `docs/CLAIMS_LEDGER.md`). `claims/claims_test.go` fails the build on
any violation:

- Every claim has a unique non-empty `id` and `claim` text.
- `status ∈ {delivered, partial, planned, false}`.
- `delivered` claims must have at least one test path **and** one doc path, and
  every referenced path must exist on disk.
- `partial` claims must list at least one structured gap
  (`^BND-[A-Z0-9]+-[0-9]{3}$`) with a `description` and a `spec` reference.
- `false` claims must **not** appear in `README.md` (the test greps and fails if
  present).

If you change what a delivered claim asserts, update the ledger in the same PR.
`claims/language_lint_test.go` separately lints public docs for controlled
overclaim phrases; limitation framing ("does not", "not", "until", "unless") is
exactly what keeps the "What It Does Not Prove" tables passing.

## Adding a fixture

- **A red-team command/edit fixture** — add a scenario to the relevant pack in
  `internal/redteam/command_packs.go` or `internal/redteam/edit_packs.go` (or a
  diff under `fixtures/editboundary/`), keep it synthetic (no real secret, use
  `example.invalid`), and assert the expected verdict with `Executed == false`.
  See `fixtures/README.md`.
- **A SQL evasion case** — add it to
  `interceptors/sql/evasion_corpus/postgres.yaml`; the replay test will pick it
  up and the corpus floor prevents silent shrinkage.
