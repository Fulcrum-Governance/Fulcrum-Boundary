# Boundary Roadmap

Fulcrum Boundary is the action boundary for routed agent tools. This roadmap is
organized around one developer-facing question: **how far can you trust the
record a verdict leaves behind?** Each phase moves the proof lanes from "a
decision happened" toward "you can locate, read, explain, and re-run that
decision yourself" — without changing what Boundary actually governs.

Three rules govern this page:

- **Released means in the `v0.11.0` tag.** The baseline below, all of Phase 0A,
  and Phase 1 (`boundary test`) are released in `v0.9.0` and exercised by tests
  and the two proof-lane demos. The current `v0.11.0` install includes the Phase 0A
  commands, route-context record fields, and the policy-as-code test runner.
- **History stays history.** `v0.8.0` remains the Phase 0A record-trust tag:
  `DecisionRecordV2`, `boundary explain`, `boundary replay`, and record-location
  UX. It does not include `boundary test`.
- **Source-main is not release truth.** Phase 0B slices may land on `main`
  before the next tag, but they are not installable from `@v0.11.0` until a
  later release truth update says so. Planned sections still should not be read
  as delivered capabilities or dated commitments.

The product frame does not change across these phases. MCP is the first
production route, not the identity; Command Boundary is a delivered preview lane
that fits the same frame. No preview surface is promoted to production by
anything on this roadmap, and no roadmap item adds a new governed action surface.

---

## Baseline (shipped — current release)

These are the verdict-and-record capabilities you can rely on today. They are
local-first, fixture-safe, and require no account, no cloud, and no live calls.

| Capability | What it does today |
| --- | --- |
| `DecisionRecordV1` (`schema_version "1"`) | Every governed verdict emits one structured JSON decision record — allow, deny, warn, escalate, or require-approval — carrying the verdict, reason, decision mode, and matched rule. Field reference: [`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md). |
| `boundary verify-record` | Recomputes the stable request, policy-bundle, and decision hashes on a receipt-grade record so after-the-fact alteration is detectable by recomputation. Walkthrough: [`docs/RECEIPTS.md`](RECEIPTS.md). |
| `boundary doctor` | Renders local routed-surface diagnostics and bypass caveats, including a `--json` form, without network calls. Reference: [`docs/DOCTOR.md`](DOCTOR.md). |
| `boundary evidence bundle` / `boundary evidence verify` | Packages local release artifacts with a manifest and SHA-256 hashes, then re-checks a bundle for manifest shape, artifact existence, and hash integrity. Reference: [`docs/EVIDENCE_BUNDLE.md`](EVIDENCE_BUNDLE.md). |
| Two proof-lane demos | `boundary demo github-lethal-trifecta` (MCP, the first production route) and `boundary demo command-secret-exfil` (Command Boundary, delivered preview). Each denies a dangerous action and emits a hash-verifiable decision record. |

The two proof lanes today produce these verified shapes:

| Lane | Demo | Denied action | Verified shape |
| --- | --- | --- | --- |
| MCP — the first production route | `boundary demo github-lethal-trifecta` | A write-after-taint GitHub action, denied before upstream | `actual=DENY`, `upstream_called=false`, `reason=lethal_trifecta_detected` |
| Command Boundary — a delivered preview (routed-only) | `boundary demo command-secret-exfil` | A routed secret-exfiltration command, denied before execution | `actual=DENY`, `executed=false`, `class=C6` |

One honesty note carries through every phase below: the `upstream_called=false`
and `executed=false` fields are adapter self-reports of their own control flow.
They are **not** fields of the hashed record and are **not** independently
corroborated by it. Boundary does not emit `proved` decisions itself.

---

## Phase 0A — Trust the Record / Evidence UX (shipped in `v0.8.0`, included in the current `v0.11.0` release)

> **In the `v0.8.0` release and included in the current `v0.11.0` release.** Everything in this
> section is released, exercised by tests, and reflected in the claims ledger.
> The current `v0.11.0` install includes these commands and record fields. Command and
> field reference:
> [`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md) (§§10–11) and the route-context
> section of [`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

Phase 0A makes a decision record easy to *find, read, and reproduce* — closing
the gap between "Boundary emitted a record" and "I can sit down with that record
and understand exactly what happened." It is a record-UX and route-context phase,
not a new enforcement surface.

### Richer route context on the record

`schema_version "2"` records add route-context fields so a record carries where
the decision was made, not only what it decided:

- `adapter_id` — which adapter parsed and routed the request.
- `route_id` — the specific governed route the request traveled.
- `topology_profile` — the named deployment posture the record was produced under.
- `execution_claim` — the structured form of the adapter's own execution
  self-report (the same self-reported control-flow signal as `upstream_called` /
  `executed` today, recorded explicitly rather than as a free field).

Because these fields change the record shape, they arrive as a later schema
version (`DecisionRecordV2`). `DecisionRecordV2` is released as an additive
superset over `DecisionRecordV1`, and both `schema_version "1"` and
`schema_version "2"` records are emitted and verified. `DecisionRecordV1`
(`schema_version "1"`) remains valid and unchanged.

### `boundary explain <record>`

A read-side command that takes an existing decision record and renders a
human-readable account of the verdict — the matched rule, the reason, the
decision mode, and the route context above — so a record can be understood
without reverse-engineering JSON by hand. `boundary explain` is a released
command (reference: [`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md) §10), included
in the current `v0.11.0` install.

### `boundary replay <record>`

A command that re-runs the recorded request through the same evaluation path to
reproduce the verdict locally, so a developer can confirm a record's decision is
deterministic and recompute it on their own machine. Replay is a local,
fixture-safe reproduction step. It reproduces the *decision*, not the absence of
upstream side effects. `boundary replay` is a released command (reference:
[`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md) §11), included in the current `v0.11.0`
install.

### Both proof-lane records are first-class

The records from both proof-lane demos are easy to locate, verify, explain, and
replay end-to-end — so the github-lethal-trifecta and command-secret-exfil
records are a worked example of the full find → verify → explain → replay loop
([`docs/examples/README.md`](examples/README.md)).

### Non-goals for Phase 0A (stated explicitly)

Phase 0A deliberately does **not** add any of the following, and these are not
claimed:

- No signing of records or commands.
- No cryptographic proof of the verdict itself.
- No topology attestation — a `topology_profile` field records an asserted
  posture and does **not** attest or verify that the deployment matches it.
- No independent proof that no upstream bytes moved — the execution self-report
  remains an adapter self-report and is **not** independently corroborated by the
  record. `boundary replay` reproduces the *decision*, not the absence of
  upstream side effects.

---

## Phase 0B — Diagnostics & first-impression clarity (shipped in the current `v0.11.0` release)

> **Shipped in the current `v0.11.0` release.** These Phase 0B slices landed on
> source `main` after `v0.9.0` and are now included in the `v0.11.0` install:
> doctor environment diagnostics, the redacted `--report`, and the clearer
> first-run hierarchy.

Phase 0B sharpens the first-run experience so a developer can tell, quickly,
whether their local toolchain can run Boundary and read its output. The
source-main work covers doctor diagnostics, report redaction, and demo
hierarchy:

- **Deeper `doctor` environment diagnostics** for the Go toolchain, the
  cgo / C-toolchain requirement, and `PATH` / `GOBIN` resolution after
  `go install` — turning today's common first-run failures into named,
  actionable diagnostics.
- **Redacted report output**, so a developer can share a `doctor` report for
  help without leaking local environment detail; the report does not include
  secrets or the raw local project path.
- **Visual hierarchy for the README and demo surfaces**, so the two proof lanes
  and the record they leave read clearly on first impression.

Phase 0B adds diagnostics and presentation only. It does **not** add a new
governed surface and does **not** change any verdict.

---

## Phase 1 — Policy-as-code testing (shipped in `v0.9.0`)

> **In the `v0.9.0` release.** `boundary test` is delivered in `v0.9.0`; the
> current `v0.11.0` install includes it. The historical `@v0.8.0` install does not.

`boundary test` is a **local, fixture-only policy-as-code test runner**. It lets
an operator author request fixtures and expected verdicts against local YAML
policy bundles, evaluates those fixtures through the existing governance
pipeline, and exits non-zero on any mismatch, unexpected policy-load error, or
malformed case. It covers `allow`, `deny`, `warn`, `require_approval`,
`escalate`, and expected policy `parse_rejection` cases.

Scope and limits, stated up front:

- It introduces **no** new governed action surface and **no** new transport
  adapter, so it adds no readiness entry and promotes no preview surface to
  production.
- It emits **no** `proved` decisions. A passing run reports policy verdicts for
  routed request fixtures only. It does **not** prove production route
  enforcement, does **not** prove that a deployment removed every direct or
  unrouted path to a tool, and does **not** prove the verdict was globally
  correct beyond the supplied fixture and local policy bundle.
- It sits in the same local-utility maturity bucket as `selftest`, `doctor`, and
  evidence — a developer-facing local tool, not a hosted or control-plane feature.
- Its committed golden corpus lives under `tests/fixtures/policy-test/`, and the
  canonical operator reference is [`docs/POLICY_TESTING.md`](POLICY_TESTING.md).

---

## Phase P-pos — Positioning surgery (planned)

> **Not shipped. Planned, language-and-claims only.** This phase changes how
> Boundary is *described*, not what it governs. It adds no command, no adapter,
> no record field, and no verdict. It lands behind the same claims and language
> gate as everything else, and the claims ledger is updated in the same change.

Phase P-pos tightens public language so the repository leads with the exact
capability it can stand behind, and hardens the controlled-overclaim list against
a new class of overstatement. The pre-execution boundary primitive is now widely
available from several vendors; this phase stops describing Boundary as the
boundary in the abstract and scopes every headline to the witness-and-record lane
the tests actually exercise.

Concretely:

- **Adopt the exact-conjunction claim.** Where the repository describes its
  differentiated capability, it states the capability as a single conjunction of
  parts that hold together, never as a part standing in for the whole. A claim
  that names only one half of the conjunction is treated as an overclaim and is
  rewritten or removed.
- **Harden the controlled-overclaim list.** New phrases are added to the language
  system (`docs/LANGUAGE_SYSTEM.md`, `docs/LEXICON.md`, `docs/COPY_RULES.md`) and
  to `claims/language_lint_test.go` so they fail the build on any public surface
  unless negated or limitation-framed — including standalone superlatives of the
  "first/only/category-of-one" shape and headline use of the bare gateway
  framing.
- **No new capability is asserted.** This phase cannot move any claim from
  `partial`/`planned` to `delivered`; it only narrows existing language toward
  what is already true.

Non-goals for Phase P-pos: no new governed surface, no preview promoted to
production, no claim upgraded by language change alone.

---

## Phase P-verify — Defensive verification of referenced claims (planned)

> **Not shipped. Planned.** Scope is *internal verification of claims the
> repository repeats*, not a new product surface. Any artifact that lands ships
> behind the full gate, and nothing here emits a `proved` decision or changes a
> verdict.

When the repository references an external formal result — a machine-checked
equilibrium argument, a circuit-breaker termination property, or a
witness-checker invariant — Phase P-verify keeps a compile-checked Lean artifact
that re-derives the specific property being cited, so a referenced result is
corroborated locally rather than asserted on trust. This is a defensive,
citation-hygiene lane: it raises the cost of repeating a claim the repository has
not itself re-checked.

Scope and limits, stated up front:

- It covers only the **named invariants** the public language relies on — for
  example, that a budget guard requires available budget to cover a requested
  cost, and that a requested privilege set is contained in the available set.
- A green Lean build corroborates the **stated property**; it does **not** prove
  any running Boundary deployment satisfies it, and it does **not** make Boundary
  emit `proved` decisions. Boundary does not emit `proved` decisions itself.
- It introduces no new transport adapter and no new governed action surface, so
  it adds no readiness entry.

---

## Phase P-standards — Standards legibility for the decision record (planned)

> **Not shipped. Planned.** This phase makes the *existing* decision record
> legible to external attestation formats. It adds no governed surface and does
> not change a verdict; the record content is unchanged, only its expressibility
> in standard envelopes.

Phase P-standards demonstrates a lossless round-trip of the existing decision
record through interoperable authorization and supply-chain formats, so an
operator can carry a Boundary verdict into the attestation tooling they already
run. The work is a mapping and a round-trip test over the record that ships
today; it earns only the scoped, record-bounded conformance claim the language
gate already permits.

Concretely:

- A documented mapping expresses a decision record as an external
  authorization-decision payload and as a supply-chain attestation statement, and
  a round-trip test confirms the verdict, reason, and decision hashes survive the
  conversion intact.
- The conformance claim stays **scoped to the decision record** — the surface
  whose canonicalization is already RFC 8785/JCS — and is never widened into a
  blanket whole-product conformance claim.
- The execution self-report remains an adapter self-report; expressing the record
  in a standard envelope does **not** independently corroborate that no upstream
  bytes moved.

Non-goals for Phase P-standards: no new governed surface, no signing of records,
no attestation that a deployment matches its asserted topology.

---

## Phase P-witness — Witness-up-the-stack research [INTENT] (gated on Phase P-redteam)

> **Not shipped. [INTENT] / research only.** Nothing in this phase is built,
> installable, or claimed as a capability. It is recorded here so the research
> direction is legible, not as a dated commitment. It **must not** be read as a
> delivered or in-progress feature, and it does **not** ship until Phase
> P-redteam clears the correctness gate below.

Phase P-witness explores moving the per-decision certifying witness *up the
stack* — from a single tool call to a bounded, compiled language-model program —
so that a certificate could attest that an optimized program preserves its
declared invariants across the steps it actually runs. This is conjectured work
on an open problem; the repository asserts no result here.

Stated as intent, with the honesty that governs the rest of this page:

- The target is a **bounded** program shape with declared invariants, not
  arbitrary agent behavior. No certificate is claimed for unbounded reasoning.
- This phase is **blocked** on Phase P-redteam. A per-trace certificate is
  meaningless if a program can do consequential work inside a sub-call or REPL
  that the boundary never observes; that bypass must be characterized and
  contained before any witness-up-the-stack artifact is pursued.
- Until both conditions hold, this remains `[INTENT]`: no command, no adapter, no
  record field, no verdict, and no entry in the claims ledger.

---

## Phase P-redteam — Self-decomposition bypass red-team (planned, correctness gate)

> **Not shipped. Planned, and a hard gate on Phase P-witness.** This is a
> correctness and adversarial-coverage lane, not a new product surface. It adds
> no governed surface and changes no verdict; it characterizes a failure mode and
> raises the bar a witness claim must clear.

Phase P-redteam builds an adversarial corpus for the failure mode where an agent
that decomposes its own task performs consequential work inside a sub-call,
spawned process, or REPL that a pre-execution boundary on the outer call never
sees. The corpus uses synthetic fixtures only (no real secrets; `example.invalid`
hosts), asserts both the intended verdict **and** that the dangerous action did
not execute, and is wired into the existing red-team lane.

Scope and limits:

- It is a **correctness gate**, not a coverage claim. A passing corpus shows the
  characterized bypasses are caught under the modeled topology; it does **not**
  prove every decomposition path is observable, and it does **not** remove the
  routed-only caveat — an unrouted sub-call remains a bypass unless deployment
  topology removes that path.
- Clearing this gate is a **precondition** for any Phase P-witness work; an
  un-contained self-decomposition bypass blocks witness-up-the-stack research.
- It introduces no new transport adapter and no new governed action surface.

---

## Strategy posture

The business posture that sequences these phases is **not** recorded on this
public roadmap and is **not** committed to this repository. It lives only in a
local, uncommitted strategy record, kept outside this repository. This page stays strategy-free by
design; consult that record for posture, and this roadmap for
shipped-versus-planned engineering truth.


---

## How to read this roadmap against the claims gate

This repository mechanically checks that public language matches shipped
behavior. To keep that contract intact:

- The Baseline, Phase 0A, and Phase 1 sections describe behavior that is
  released in `v0.9.0`. `explain`, `replay`, `DecisionRecordV2`, the
  route-context fields, and `boundary test` are released in `v0.9.0`, and the
  current `v0.11.0` install includes them.
- `v0.8.0` remains the historical Phase 0A tag and does not include
  `boundary test`.
- When any planned item lands, it ships behind the same release gates as the rest
  of the repository — tests, the claims and language gate, a strict docs build,
  and the full release check — and the claims ledger is updated in the same
  change. A planned item does **not** become a claim until that happens.

Authoritative current-truth references live alongside this page:
[`docs/CLAIMS_LEDGER.md`](CLAIMS_LEDGER.md),
[`docs/ADAPTER_READINESS_MATRIX.md`](ADAPTER_READINESS_MATRIX.md),
[`docs/RELEASE_TRUTH_PUBLIC.md`](RELEASE_TRUTH_PUBLIC.md), and the CLI surface in
[`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md).
