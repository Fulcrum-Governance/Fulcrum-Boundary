# Boundary Roadmap

Fulcrum Boundary is the action boundary for routed agent tools. This roadmap is
organized around one developer-facing question: **how far can you trust the
record a verdict leaves behind?** Each phase moves the proof lanes from "a
decision happened" toward "you can locate, read, explain, and re-run that
decision yourself" — without changing what Boundary actually governs.

Three rules govern this page:

- **Released means in the `v0.9.0` tag.** The baseline below, all of Phase 0A,
  and Phase 1 (`boundary test`) are released in `v0.9.0` and exercised by tests
  and the two proof-lane demos. The `@v0.9.0` install includes the Phase 0A
  commands, route-context record fields, and the policy-as-code test runner.
- **History stays history.** `v0.8.0` remains the Phase 0A record-trust tag:
  `DecisionRecordV2`, `boundary explain`, `boundary replay`, and record-location
  UX. It does not include `boundary test`.
- **Planned means planned.** Phase 0B remains forward-looking. Nothing in a
  planned section should be read as a delivered capability or a dated
  commitment.

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

## Phase 0A — Trust the Record / Evidence UX (shipped in `v0.8.0`, included in `v0.9.0`)

> **In the `v0.8.0` release and included in `v0.9.0`.** Everything in this
> section is released, exercised by tests, and reflected in the claims ledger.
> The `@v0.9.0` install includes these commands and record fields. Command and
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
in the `@v0.9.0` install.

### `boundary replay <record>`

A command that re-runs the recorded request through the same evaluation path to
reproduce the verdict locally, so a developer can confirm a record's decision is
deterministic and recompute it on their own machine. Replay is a local,
fixture-safe reproduction step. It reproduces the *decision*, not the absence of
upstream side effects. `boundary replay` is a released command (reference:
[`docs/CLI_REFERENCE.md`](CLI_REFERENCE.md) §11), included in the `@v0.9.0`
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

## Phase 0B - Diagnostics & first-impression clarity (unreleased after `v0.9.0`)

> **Not in the `v0.9.0` tag.** This section describes current unreleased Phase
> 0B work and must not be read as a tagged release capability until the next
> release truth update.

Phase 0B sharpens the first-run experience so a developer can tell, quickly,
whether their local toolchain can run Boundary and read its output. The current
unreleased work covers doctor diagnostics, report redaction, and demo
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
> `@v0.9.0` install includes it. The historical `@v0.8.0` install does not.

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

## How to read this roadmap against the claims gate

This repository mechanically checks that public language matches shipped
behavior. To keep that contract intact:

- The Baseline, Phase 0A, and Phase 1 sections describe behavior that is
  released in `v0.9.0`. `explain`, `replay`, `DecisionRecordV2`, the
  route-context fields, and `boundary test` are released in `v0.9.0`, and the
  `@v0.9.0` install includes them.
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
