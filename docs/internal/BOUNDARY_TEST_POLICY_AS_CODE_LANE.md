# Internal: `boundary test` Policy-as-Code Lane (Scoped Next-Lane Brief)

Status: internal planning only. Not a release commitment. Do NOT link this file
from any public document. It is intentionally excluded from the public docs nav
(`mkdocs.yml` builds from `docs-site/`, not `docs/internal/`).

This brief scopes a *future* lane: a `boundary test` subcommand that runs
policy-as-code test cases against the local policy/interceptor surface so
operators can assert, in CI, that their policies produce the verdicts they
expect. It is written so the work can start later without re-deciding scope.

## 1. Goal

Give operators a first-class, repeatable way to test their Boundary policy
bundles the way they test code: a committed corpus of request fixtures, each
with an expected verdict, run by one command that exits non-zero on any
mismatch. Today an operator can hand-run `boundary verify` (policy *parse*
validity) and `boundary redteam` (Boundary-authored fixture packs), but there is
no operator-owned, policy-bundle-scoped assertion runner that says "given THIS
request against MY policies, the decision MUST be DENY/WARN/REQUIRE_APPROVAL."
`boundary test` closes that gap.

Scope discipline (matches `CONTRIBUTING.md` and `CLAUDE.md`): this stays inside
the routed action-boundary lane. It is a **local, fixture-only test runner over
the operator's own policy bundle**. It is not hosted monitoring, not production
policy generation, not control over non-routed commands or files, and not a
runtime-proof claim. It evaluates proposed actions through the existing
`governance.Pipeline` / `policyeval` path and reports the decision; it does not
execute any upstream tool, touch the network, or mutate any live system.

Non-goals (explicitly out of this lane):
- No live upstream calls, no credentials, no real mutation — same posture as the
  existing fixture redteam and demo paths.
- No new governed action surface. `boundary test` only *evaluates* requests that
  already flow through the documented pipeline.
- No deployment bypass proof. Passing tests prove policy verdicts for routed
  requests, not that a deployment removed every direct path to a tool.
- No `proved` decisions. Boundary does not emit `proved` verdicts; this lane does
  not change that.

## 2. Proposed CLI shape

```
boundary test [--path .boundary/tests] [--format json]
```

- `--path .boundary/tests` (default `.boundary/tests`): directory of test-case
  files the operator commits alongside their policy bundle. A test case names a
  policy bundle (directory of YAML policies), a request fixture, and an expected
  decision.
- `--format json` (default human text): emit a stable
  `boundary.test.v1` result object for CI parsing, mirroring the
  `boundary.selftest.v1` / `boundary.doctor.v1` conventions already in the CLI.
- Exit code: `0` when every case matches its expectation; non-zero when any case
  mismatches, when a referenced policy bundle fails to parse, or when a case file
  is malformed. (A parse-rejection case can assert that a *bad* policy bundle is
  rejected — see the corpus below — in which case rejection is the pass
  condition.)

Proposed test-case file shape (one YAML document per case, illustrative):

```yaml
name: deny-write-after-taint
policies: ../policies            # directory of operator YAML policies
request:                         # a GovernanceRequest fixture
  transport: mcp
  tool: github.create_or_update_file
  agent_id: agent-under-test
  context:
    untrusted_input: true
    target_repo_visibility: private
expect:
  action: deny                   # one of: allow | deny | warn | escalate | require_approval
  reason_contains: lethal_trifecta_detected   # optional substring assertion
```

Text output (illustrative):

```
boundary test: .boundary/tests
  [pass] deny-write-after-taint        expect=deny     actual=deny
  [pass] warn-large-result             expect=warn     actual=warn
  [pass] approve-prod-migration        expect=require_approval actual=require_approval
  [pass] reject-malformed-policy       expect=parse_rejection (bundle rejected)
  [fail] allow-readonly-list           expect=allow    actual=deny
status: fail   cases: 5  passed: 4  failed: 1
```

The runner reuses existing seams: it loads YAML policies through the same loader
`boundary verify` uses, builds a `GovernanceRequest` from the fixture, runs it
through `governance.Pipeline` (all four stages) in a hermetic in-process
configuration with no `AuditPublisher` side effects required, and compares the
returned decision to `expect`. No new evaluation logic is introduced.

## 3. Golden corpus (must cover all verdict classes + two edge cases)

The lane ships a committed golden corpus under `tests/` (fixture-only, no live
secrets, `example.invalid` hosts only) that exercises one case per outcome plus
the two structural edges. This corpus is both the feature's own test evidence
and the worked example operators copy.

| Case | Expected | What it asserts |
|---|---|---|
| `allow-readonly-list` | `allow` | A read-only listing request against permissive policy returns allow (no false deny). |
| `deny-write-after-taint` | `deny` | The flagship routed write-after-taint shape returns deny before any upstream step (`reason_contains: lethal_trifecta_detected`). |
| `warn-large-result` | `warn` | A policy `warn` condition (e.g. oversized result) returns warn without blocking. |
| `approve-prod-migration` | `require_approval` | A request matching a require-approval policy returns require_approval, not silent allow. |
| `escalate-…` (optional sibling) | `escalate` | If the operator bundle defines an escalate path, an escalate case asserts it; covered for completeness alongside require_approval. |
| `reject-malformed-policy` | `parse_rejection` | A deliberately malformed policy bundle is **rejected** at load; rejection is the pass condition. This proves the runner fails closed on bad policy input rather than silently passing. |
| `route-bypass-caveat` | documented caveat | A non-assertion case (or an annotation on the corpus README) recording that a passing `boundary test` run proves *policy verdicts for routed requests only*, not that a deployment removed direct/un-routed paths to the tool. This keeps the routed-only caveat attached to the corpus itself. |

The five verdict classes (`allow`, `deny`, `warn`, `require_approval`, plus
`escalate` for completeness) map directly to the pipeline's terminal decisions.
The two edges — parse rejection and the route-bypass caveat — exist so the
corpus can never be read as "tests passed, therefore the system is globally
controlled." Both edges mirror language already in
`docs/ROUTE_CONFORMANCE_CHECKLIST.md`.

## 4. GitHub Actions CI recipe

A repo-local CI recipe operators can copy. It builds the cgo-linked binary
(Postgres AST guard requires a C toolchain; `CGO_ENABLED=0` builds fail), then
runs the operator's committed policy tests. Fixture-only: no credentials, no
network, no live mutation.

```yaml
name: boundary-policy-test
on:
  pull_request:
  push:
    branches: [main]

jobs:
  policy-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - name: Build boundary (cgo on; Postgres AST guard requires a C toolchain)
        run: CGO_ENABLED=1 go build -o bin/boundary ./cmd/boundary
      - name: Run policy-as-code tests
        run: ./bin/boundary test --path .boundary/tests --format json | tee boundary-test.json
      - name: Upload result
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: boundary-test-result
          path: boundary-test.json
```

The job fails the check when `boundary test` exits non-zero (any verdict
mismatch, any unparseable policy bundle, or any malformed case). The
`--format json` output is captured as a build artifact for review. This is
repo-local CI over the operator's own policies; it does not prove production
route enforcement or deployment bypass resistance.

## 5. The NEW claim this lane would require (proposed — not yet in the ledger)

This lane introduces a *new* capability, so it would add exactly one claim to
`claims/boundary_claims.yaml`. It is recorded here as a **proposed** claim only;
it is intentionally NOT added to the ledger now, because the claims gate
(`claims/claims_test.go`) requires a `delivered` claim to reference test paths
and doc paths that exist on disk, and this lane is not built yet.

Proposed claim (illustrative shape, to be added in the same change that builds
the feature):

```yaml
- id: BND-CLAIM-TEST-001            # proposed id; reserve on implementation
  claim: "Boundary runs operator-authored policy-as-code test cases against local policy bundles and reports the verdict for each case without live mutation"
  status: delivered                 # only when tests + docs below exist on disk
  evidence:
    tests:
      - path: tests/test_runner/boundary_test_runner_test.go
        assertion: "boundary test reports allow/deny/warn/require_approval/escalate outcomes, rejects malformed policy bundles, and exits non-zero on any mismatch without live mutation"
    docs:
      - path: docs/POLICY_TESTING.md
        section: "Policy-as-code tests"
  public_language:
    allowed:
      - "Boundary runs operator-authored policy-as-code tests against local policy bundles and reports the verdict for each case."
      - "boundary test is a local, fixture-only policy assertion runner; it does not call upstream tools, the network, or mutate live systems."
    forbidden:
      - "boundary test proves production route enforcement"
      - "boundary test proves a deployment removed every direct tool path"
      - "boundary test proves the verdict was correct, not only that the policy decided it"
  gaps: []
  depends_on: []
```

Notes for whoever picks this up:
- `status: delivered` requires both the named test path and the named doc path to
  exist on disk, or the claims gate fails. Land the test and `docs/POLICY_TESTING.md`
  in the same change.
- The `forbidden` list must keep the routed-only and not-a-proof caveats so the
  language posture matches the rest of the ledger.
- If any part ships incomplete, use `status: partial` with a structured gap id
  (`^BND-[A-Z0-9]+-[0-9]{3}$`) and a `description` + `spec`, per the gate.

## 6. Gates this lane must pass before it can ship

The lane is not done until every existing release gate stays green with the new
surface added. Concretely:

- **Readiness gate.** `boundary test` is a CLI runner over the existing pipeline,
  not a new transport adapter, so it does not add an `adapters/<name>/readiness.yaml`
  entry. If it is ever exposed as a transport, it must add a readiness
  declaration with all ten lifecycle steps and pass `tests/adapter_conformance`.
  Until then, document explicitly that it reuses existing adapter routes and adds
  no new route.
- **Test gate.** `go test ./... -count=1 -timeout 5m` and
  `go test ./claims/... -count=1` pass. The new runner has hermetic tests under
  `tests/` (in-process, `t.TempDir`, no network), consistent with the
  no-`testing.Short()` / no-`t.Parallel()` full-suite convention. Black-box
  attribution means the runner's coverage may read low on the source package;
  use `-coverpkg=./...` for a real figure (see `docs/TESTING.md`).
- **Claims + language gate.** The single new claim is added to
  `claims/boundary_claims.yaml` with existing test + doc paths, and any public
  copy uses negated/controlled framing for the not-a-proof caveats so
  `claims/language_lint_test.go` passes.
- **Doc gate.** `make docs-build` (strict mkdocs) passes. A public reference page
  for `boundary test` is added under `docs/` (canonical) with a `docs-site/`
  stub registered in `mkdocs.yml` nav, matching how `route-conformance` and
  `troubleshooting` are wired. The CLI reference and first-run docs are updated
  only if the command becomes part of the first-run path (it likely should not be
  — first-run stays the two proof lanes).
- **Release gate.** `make release-check` passes end to end, including the live
  `boundary` CLI invocations and the grpc nested module. If `boundary test` is
  added to the release-check CLI sequence, it must run fixture-only with no
  network and no live mutation, like the other invoked commands.
- **Public-surface guard.** `scripts/assert-no-internal-public-artifacts.sh`
  stays green: this brief and all new files avoid internal-planning tokens,
  retired product framing, capture-instruction placeholders, and non-approved
  contact aliases.

When all six hold, the lane ships as a `delivered` local-only utility — the same
maturity bucket as `selftest`, `doctor`, and the evidence commands — and no
preview surface is upgraded to production by adding it.
