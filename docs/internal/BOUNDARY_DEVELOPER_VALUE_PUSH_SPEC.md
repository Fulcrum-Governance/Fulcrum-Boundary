# Fulcrum Boundary Developer-Value Push Spec

Last updated: 2026-05-31 (Rev 2 — execution-locked)

> Status: historical planning artifact. The push this spec scopes shipped in
> PR #107 (merged to `main`) and is now part of the `v0.8.0` release. The
> version references below (`v0.7.0`) reflect the release truth at authoring
> time; the current release is `v0.8.0`.

This is an internal execution spec for the next Fulcrum Boundary push. Its job is
to turn the current repository from "interesting but hard to evaluate" into a
developer tool that can be installed, run, inspected, and judged in minutes.

The release must lead with the concrete action Boundary governs:

> Before an agent touches a dangerous routed tool, Boundary decides whether that
> action is allowed and emits a hash-verifiable decision record.

Boundary is not a universal agent sandbox. It governs actions that route through
Boundary. Direct paths outside the route remain outside the claim unless the
operator's deployment topology blocks them.

## Execution Decision (Locked 2026-05-31)

This revision locks scope before execution:

- **In scope for this push:** Subgoal 0 (First-Run Trust Loop) and Subgoal 1
  (Evidence You Can Inspect), plus the Subgoal 3 route-conformance work delivered
  as a documented checklist only. These directly remediate the four evaluation
  criticisms, stay inside the current claims ledger, and require no new claim.
- **Deferred to a separately gated next lane:** Subgoal 2 (`boundary test`
  policy-as-code) and Subgoal 5 (Adapter SDK). Each is a new CLI surface and/or a
  new claim, so each gets its own branch, claim review, tests, docs, and release
  gate per `CONTRIBUTING.md` one-lane discipline. A scoped next-lane brief is
  written at `docs/internal/BOUNDARY_TEST_POLICY_AS_CODE_LANE.md`.
- **Publish posture:** land on the execution branch, pass every gate, and open a
  PR to `main`. No tag is cut, no `@`-pin changes, and `main` is not updated
  until human review and merge. Nothing outward-facing happens automatically.

## Why This Push Exists

Recent external criticism focused on four evaluation failures:

- The repository looked sparse and hard to understand.
- The purpose was unclear without reverse-engineering the source.
- The value looked narrow or Fulcrum-only.
- Production readiness and adapter maturity were hard to judge.

The answer should not be louder positioning. The answer should be a better
developer path:

1. Install the CLI.
2. Run local diagnostics.
3. Run the two proof lanes.
4. Inspect the decision record and evidence bundle.
5. See exactly which surfaces are production, preview, local-only, or planned.
6. Copy a policy or CI recipe into a real project without adopting the full
   Fulcrum platform.

## Current Release Truth

Authority for current claims:

- `docs/RELEASE_TRUTH_PUBLIC.md`
- `claims/boundary_claims.yaml`
- `docs/CLAIMS_LEDGER.md`
- `docs/ADAPTER_READINESS_MATRIX.md`
- `README.md`
- `docs/COPY_RULES.md`
- `docs/LANGUAGE_SYSTEM.md`

Current v0.7.0 truth:

- MCP is the first and only production route.
- Command Boundary is a delivered preview for routed project-local command paths.
- Edit Boundary is a delivered preview for routed edit envelopes.
- Secure GitHub is preview, with fixture proof and an opt-in live conformance
  harness.
- The default public demos are fixture-safe: no credentials, no live GitHub
  calls, and no real mutations.
- `boundary doctor` is delivered and reports local diagnostics plus bypass
  caveats. It is not deployment proof.
- `boundary evidence bundle` and `boundary evidence verify` are delivered local
  evidence utilities. Evidence bundles do not prove production deployment
  safety.
- Decision records are hash-verifiable. They are not cryptographic proof of a
  runtime verdict unless and until a signing/attestation system is explicitly
  shipped and claimed.
- `upstream_called=false` means the adapter recorded no upstream call. It does
  not independently prove that no bytes moved outside Boundary.
- Boundary governs routed actions only. Direct shell, editor, filesystem, CI,
  SSH, or API paths outside Boundary are not governed by Boundary.

## External Developer Expectations

The next push should align with current developer and security expectations:

- GitHub README guidance says a repository should quickly explain what the
  project does, why it is useful, how users start, where they get help, and who
  maintains it.
- The current Model Context Protocol specification treats tools as powerful
  external capabilities and calls for explicit user consent, tool safety,
  access controls, privacy controls, and clear security documentation.
- MCP authorization guidance emphasizes least privilege, resource-bound tokens,
  token validation, and avoiding token passthrough.
- OpenAI Agents SDK documentation distinguishes agent-level guardrails from
  per-tool guardrails; tool guardrails are the relevant comparison because they
  run around custom tool invocations.
- OWASP's LLM and agentic AI guidance frames excessive agency, tool misuse,
  identity/privilege abuse, and agentic supply-chain risk as live problems for
  builders.

Sources reviewed:

- https://docs.github.com/en/enterprise-server@3.18/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-readmes
- https://modelcontextprotocol.io/specification
- https://modelcontextprotocol.io/docs/tutorials/security/security_best_practices
- https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization
- https://openai.github.io/openai-agents-python/guardrails/
- https://openai.github.io/openai-agents-python/ref/tool_guardrails/
- https://genai.owasp.org/llmrisk/llm062025-excessive-agency/
- https://genai.owasp.org/resource/owasp-top-10-for-agentic-applications-for-2026/

## Criticism To Remediation Map

| Criticism | Developer-facing answer | Required repo work | Acceptance gate |
| --- | --- | --- | --- |
| Sparse docs | A first-run path that starts with a dangerous action, a verdict, and a record. | Tighten README, add/refresh quickstart, point to CLI reference and examples. | A new developer can find install, selftest, demo, doctor, evidence, and maturity links from the first screen. |
| Unclear purpose | Boundary is the action boundary for routed agent tools. | Keep public copy centered on the dangerous action, the decision, and the record. | Public docs avoid platform abstraction as the first explanation. |
| Fulcrum-only concern | Boundary has standalone local value before any connected mode. | Keep first-run, evidence, policy, and CI flows local-first. | No first-run step requires Fulcrum cloud, credentials, or hosted services. |
| Unclear maturity | Every surface has an explicit status and caveat. | Readiness matrix, claims ledger, README, CLI docs, and release notes agree. | No preview surface is described as production. |
| Reverse-engineering burden | The repo ships runnable proof loops and inspectable artifacts. | Provide copy-paste commands, fixture outputs, record examples, evidence bundle examples, and troubleshooting. | A developer can evaluate Boundary without reading Go source. |

## Status Vocabulary

Use these terms consistently:

- `production`: Full lifecycle, conformance, bypass evidence, fail-mode tests,
  and claims evidence exist. In v0.7.0 this applies to MCP only.
- `delivered`: Shipped behavior with tests and docs, still scoped by the claim.
- `delivered preview`: Shipped behavior that is intentionally not production
  and carries explicit gaps.
- `preview`: Useful surface with documented gaps before production readiness.
- `local-only`: Reads or verifies local artifacts without proving live runtime
  protection.
- `planned`: Roadmap work. Never state as current behavior.
- `non-goal`: Work intentionally outside this push or outside Boundary's claim.

## Developer Value North Star

The next push succeeds when an outside developer can answer five questions
without reverse-engineering the code:

1. What dangerous action is Boundary stopping?
2. What route does the action travel through?
3. What policy or rule caused the verdict?
4. What record or evidence can I inspect after the run?
5. What does this proof not cover?

The release should be judged by developer comprehension, not by adapter count.

## Baseline Developer Path Today

The current repo already has enough pieces for a stronger first-run story. The
next push should make this path explicit:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.7.0

boundary selftest
boundary doctor --json

boundary demo github-lethal-trifecta
boundary demo command-secret-exfil

boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
```

When a demo or evidence artifact provides a decision record path, the docs
should show the verification step:

```bash
boundary verify-record <record.json>
```

This path must remain local-first and credential-free by default.

## Execution Subgoals

### Subgoal 0: First-Run Trust Loop

Goal: a developer understands Boundary's value in under five minutes.

Deliverables:

- README first screen says what Boundary does, why it is useful, and how to run
  it.
- Quickstart presents a single local path: install, selftest, doctor, two proof
  lanes, evidence bundle, evidence verify, and record verification.
- CLI reference mirrors the same path and explains the difference between
  delivered, preview, and local-only commands.
- Troubleshooting page covers Go version, cgo/C toolchain, PATH issues, common
  command failures, and how to read `doctor --json`.
- Every first-run command is fixture-safe unless explicitly marked opt-in.

Acceptance gates:

- README has one copy-paste path a new developer can run.
- The path contains no required credentials, cloud account, hosted Fulcrum
  service, or live mutation.
- The docs explain that a passing doctor is not proof every deployment route is
  protected.
- `make release-check` still exercises `doctor`, `evidence bundle`, and
  `evidence verify`.

### Subgoal 1: Evidence You Can Inspect

Goal: a developer can inspect what Boundary decided and what the evidence proves.

Deliverables:

- A versioned decision-record reference that documents the **two-tier record
  model** so a skeptical developer is not misled by the word "hash-verifiable":
  - the structured decision record (`BND-CLAIM-002`): what every governed verdict
    emits, field by field;
  - the receipt-grade record (`BND-CLAIM-005`): the request, policy-bundle, and
    decision hashes that `boundary verify-record` recomputes, exactly what those
    hashes cover, and the tamper-detection behavior (verification fails after any
    field is altered).
  The reference must state plainly what "hash-verifiable" does **not** prove: it
  is not cryptographic proof of a runtime verdict, it does not prove the verdict
  was enforced, and `upstream_called=false` is an adapter self-report, not
  independent network proof. Document required fields, optional fields, and
  caveats alongside the model.
- A small set of committed example records from fixture-safe runs.
- A record verification walkthrough using `boundary verify-record`.
- Evidence bundle documentation that shows manifest shape, artifact list,
  SHA-256 checks, summary references, and limitations.
- If `boundary explain` or `boundary replay` remain desired commands, mark them
  explicitly as planned until implemented and release-gated.

Acceptance gates:

- Example records pass `boundary verify-record` when the necessary inputs are
  present.
- Evidence bundle docs state what is verified and what is not verified.
- No doc implies evidence bundles prove production route safety.
- No doc implies `upstream_called=false` is independent network proof.

### Subgoal 2: Policy As Code

Goal: teams can write, test, and review Boundary policies like code.

Deliverables:

- A policy-test spec for `.boundary/tests/`.
- A fixture format that includes action input, expected verdict, expected class,
  expected policy/rule, and expected explanation/caveat where relevant.
- A golden corpus covering ALLOW, DENY, WARN, REQUIRE_APPROVAL, parse rejection,
  and route-bypass caveat cases.
- A CI recipe for running policy tests in GitHub Actions.
- Starter policy examples that are clearly labeled as examples requiring
  operator review.
- Error messages that tell the developer what failed, why it failed, and which
  policy or fixture to inspect.

Planned CLI shape:

```bash
boundary test
boundary test --path .boundary/tests
boundary test --format json
```

Acceptance gates:

- `boundary test` is not advertised as current behavior until implemented,
  documented, and release-gated.
- Policy examples do not auto-install from unreviewed sources.
- Generated or starter policies are never described as production-ready without
  operator review.
- CI examples fail closed on malformed policy tests.

### Subgoal 3: Route Conformance

Goal: a second route can move toward production only by proving route control,
not by adding broader language.

Current stance:

- MCP remains the only production route.
- Command Boundary and Edit Boundary remain delivered previews.
- Secure GitHub remains preview until production gaps are closed.

Deliverables:

- A route conformance checklist derived from `docs/ADAPTER_READINESS_MATRIX.md`.
- A Command Boundary graduation plan focused on routed command paths only.
- Bypass tests for direct PATH binary use, direct shell, CI/SSH execution, editor
  terminals, and other known direct paths.
- A fail-closed behavior matrix for parse errors, missing policy, policy load
  errors, unsupported routes, and malformed envelopes.
- A public caveat table that distinguishes "route governed" from "system
  globally controlled."

Acceptance gates:

- No Command Boundary copy claims global shell control.
- No Edit Boundary copy claims direct filesystem or editor-write protection.
- No Secure GitHub copy claims production status until deployment bypass evidence
  and broader live coverage are recorded.
- Adapter status updates require claims-ledger and readiness-matrix changes in
  the same release lane.

### Subgoal 4: Practical Integrations

Goal: developers can try Boundary in real workflows without adopting a hosted
platform first.

Deliverables:

- MCP client setup recipes for local development.
- GitHub Action recipe for repo-local MCP config audit and SARIF output.
- Secure GitHub preview recipe with fixture path first and opt-in live
  conformance second.
- Local evidence workflow for pull requests: run demo, bundle evidence, verify
  evidence, attach summary.
- Optional connected Fulcrum mode design that is explicitly opt-in.

Acceptance gates:

- The default integration path remains local-first.
- No docs imply default cloud sync, hosted monitoring, or automatic upload.
- Any connected mode must show exactly what is sent, what is redacted, and how
  the user approves the action.
- GitHub Action language stays repo-local audit/reporting unless runtime routing
  is actually configured.

### Subgoal 5: Ecosystem After Contracts Stabilize

Goal: extension is safe only after the core contracts are stable enough for
outside contributors.

Deliverables:

- Adapter SDK design with required lifecycle hooks:
  - parse
  - identify
  - evaluate
  - deny
  - forward
  - inspect
  - metadata
  - record
  - bypass_proof
  - fail_closed
- Adapter test harness requirements.
- Policy example contribution rules.
- Provenance rules for contributed adapters and policy packs.
- Control-mapping packs only after policy tests and evidence semantics are
  stable.

Acceptance gates:

- No community policy registry before trust/provenance rules exist.
- No adapter can claim production without readiness-matrix evidence.
- No contributed pack can execute or install automatically from an unreviewed
  source.

## Repo Work Items

This spec should translate into a repo branch with the following likely changes:

- `README.md`: tighten first-run value loop and ensure every proof claim is
  routed, fixture-safe, and status-labeled.
- `docs/CLI_REFERENCE.md`: align first-run command order with README.
- `docs/DOCTOR.md`: ensure local-only caveats are easy to find from the
  first-run path.
- `docs/EVIDENCE_BUNDLE.md` and `docs/EVIDENCE_VERIFY.md`: add a developer
  walkthrough and limitations table if missing.
- `docs/DECISION_RECORDS.md` or successor: provide a stable field reference and
  example records.
- `docs/ADAPTER_READINESS_MATRIX.md`: keep route maturity visible and
  graduation criteria concrete.
- `docs/CLAIMS_LEDGER.md` and `claims/boundary_claims.yaml`: update only when
  behavior and evidence actually change.
- `docs/TESTING.md`: add policy-as-code testing guidance once `boundary test`
  exists.
- `examples/` or `fixtures/`: add inspectable examples only when they are stable
  and covered by tests.

## Release Gates

Before publishing the next push:

```bash
make release-check
make docs-build
git ls-files '*.go' | xargs gofmt -l
go vet ./...
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
```

> Note: this repository has **no `-short` mode** (zero `testing.Short()` guards).
> The canonical suite is `go test ./... -count=1 -timeout 5m` — the same
> invocation `make release-check` runs. An earlier draft listed `-short`, which
> is a no-op here; it is removed.

Additional review gates:

- Public-surface guard passes.
- Claims ledger and release truth agree.
- Adapter readiness and README status labels agree.
- `boundary doctor` and evidence docs keep their local-only limitations.
- Planned commands are not presented as delivered.
- Every new public claim points to test and doc evidence.
- Every first-run example is fixture-safe unless explicitly marked opt-in.

## Language Rules

Use:

- "routed agent tools"
- "governed routed action"
- "decision record"
- "hash-verifiable"
- "fixture-safe"
- "delivered preview"
- "local-only diagnostics"
- "recorded adapter claim"

Avoid as public capability claims:

- "universal agent safety"
- "fully secures GitHub"
- "global shell control"
- "all CLI activity protected"
- "direct filesystem protection"
- "hosted monitoring"
- "cryptographic proof of verdict"
- "production Secure GitHub"
- "production Command Boundary"
- "production Edit Boundary"
- "prevents all prompt injection"
- "governs every way an agent can mutate"

Prefer this pattern:

```text
Dangerous action -> routed system -> Boundary verdict -> decision record -> caveat.
```

Example:

```text
The agent attempted a routed `curl -d @.env ...` exfiltration command. Boundary
classified the action as high-risk, denied it before execution, and emitted a
hash-verifiable decision record. Direct shell paths outside the Boundary route
remain outside this claim.
```

## Permanent Non-Goals

These are not part of the next push:

- Universal agent safety.
- Global shell protection.
- Direct editor or filesystem interception.
- Hosted monitoring.
- Default cloud sync.
- Automatic upload of local evidence.
- Production claims for Command Boundary, Edit Boundary, or Secure GitHub.
- Cryptographic signing or attestation unless separately implemented,
  documented, and release-gated.
- Automatic installation of unreviewed policies, adapters, or community packs.
- Compliance guarantees.

## Definition Of Done

The next push is done when:

- A developer can understand the product from the first README screen.
- A developer can run the local first-run path without credentials.
- The two proof lanes remain fixture-safe and status-labeled.
- Decision records and evidence bundles are easy to inspect.
- Policy-as-code work is either delivered with gates or clearly marked planned.
- Route conformance is the graduation path for previews.
- Connected Fulcrum mode is optional and not required for local value.
- Public language stays inside the release truth, claims ledger, and readiness
  matrix.
- Release, docs, claims, and test gates pass.
