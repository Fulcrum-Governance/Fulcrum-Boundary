# Internal: Phase 0A — Record & Evidence UX Lane (Scoped Execution Brief)

Status: internal planning only. Not a release commitment. Do NOT link this file
from any public document. It is intentionally excluded from the public docs nav
(`mkdocs.yml` builds from `docs-site/`, not `docs/internal/`).

This brief is the buildable execution plan for the **Phase 0A** section of the
public roadmap (`docs/BOUNDARY_ROADMAP.md`). The roadmap states the planned
intent in claim-safe language; this brief fixes the technical contracts and
sequence so implementation can start later without re-deciding scope. Nothing
here is shipped. Each item below becomes a claim only when it lands behind the
full gate set with a `claims/boundary_claims.yaml` update in the **same change**.

## 1. Goal & framing

Phase 0A answers one developer question: **how far can you trust the record a
verdict leaves behind?** It moves the proof lanes from "a decision happened"
toward "I can locate, read, and re-run that decision myself" — a record-UX and
route-context phase, **not** a new enforcement surface.

Hard constraints (carry through every step):

- No new governed action surface, no new transport adapter, no preview surface
  promoted to production. The product frame is unchanged (MCP is the first
  production route; Command Boundary is a delivered preview lane).
- Local-first, fixture-safe, no credentials, no network, no live mutation —
  the same posture as `selftest`, `doctor`, the demos, and the evidence commands.
- The honesty boundary is load-bearing and repeated wherever the record is
  described: hashes prove **integrity, not authenticity**; `upstream_called` /
  `executed` are **adapter self-reports**, not independently corroborated by the
  record; Boundary does not emit `proved` decisions.

## 2. Locked sequencing (dependency order)

The lane ships as four small PRs, each behind the full gate set, in this order.
The order is the de-risking order — each step only depends on the ones before it:

1. **Record-location UX** (no schema change). Lowest risk; unblocks the rest by
   making "where is my record" a stable contract.
2. **`DecisionRecordV2`** (additive route-context schema; V1 stays valid). The
   data-model change everything downstream reads.
3. **`boundary explain <record>`** (read-only). First new command; pure render,
   so the safest new surface. Only needs to *read* a record.
4. **`boundary replay <record>`** (read-only, fixture-safe). Last, because it
   needs a stable record→request reconstruction contract that Steps 1–2 settle.

`boundary test` (policy-as-code, `docs/internal/BOUNDARY_TEST_POLICY_AS_CODE_LANE.md`)
remains the lane **after** this one. It is Phase 1 and is not part of Phase 0A.

Why explain before replay: `explain` only needs to read fields already on the
record. `replay` needs enough to *reconstruct and re-evaluate the request*, which
is a stronger contract. Shipping `explain` first delivers read-side value while
the reconstruction contract is settled.

## 3. Step 1 — Record-location UX (no schema change)

**Problem today.** Records are emitted, but locating them is ad hoc. The README
documents that `boundary demo github-lethal-trifecta --json --out demo.json`
writes `github-lethal-trifecta-artifacts/decision-records.jsonl`; the evidence
bundle copies artifacts under its `--out` dir; `boundary verify-record` consumes
a single record file. There is no uniform "here is the record path / id" signal
across the record-emitting commands.

**Design (minimal; prefer convention over new CLI surface).**

- A uniform output contract: every record-emitting command prints a single
  stable line identifying the emitted record — its `record_id` and the path it
  was written to — using one shape across `demo`, `redteam`, and `evidence`.
- Uniform `--out` semantics across those commands (same flag meaning, same
  on-disk layout: one JSON record file plus, where multiple records are emitted,
  one `.jsonl`).
- Make both proof-lane demos land their record at a documented, predictable path
  so the `find → verify` step is copy-paste, and wire that path into the existing
  `docs/examples/` walkthrough.

**Decide at build time:** whether a thin locator (`boundary records ls` over a
local artifact dir) earns its keep, or whether the printed-path convention is
enough. Default to the convention — do not add a CLI command unless the
convention demonstrably fails, per scope discipline.

**Claim impact.** UX over existing behavior; expected to need **no new claim**
(at most an evidence/doc refresh on `BND-CLAIM-002` / `BND-CLAIM-UTIL-004`). If
any command's behavior changes in a way a claim asserts, update that claim in the
same change.

**Tests / gates.** Hermetic CLI-output tests asserting the path line is stable;
docs refresh to `DECISION_RECORDS.md` / `RECEIPTS.md` / first-run; full gate set.

**Non-goals.** No new record fields; no network; no change to what is hashed.

## 4. Step 2 — `DecisionRecordV2` (additive route-context; V1 stays valid)

**Design as a strictly additive evolution.** The current shipped schema is
`DecisionRecordV1` (`schema_version "1"`, `governance/receipt_schema.go`). V2 adds
route-context fields and bumps `schema_version` to `"2"`; **V1 records remain
fully valid and supported**. A V1 record is simply a V2 record without the
route-context fields.

New fields (all the structured form of context the adapter already knows):

| Field | Meaning | Honesty caveat (must ship with it) |
| --- | --- | --- |
| `adapter_id` | Which adapter parsed and routed the request. | Descriptive only. |
| `route_id` | The specific governed route the request traveled. | Descriptive only. |
| `topology_profile` | The named deployment posture asserted at emission. | **Asserted, not attested** — the field does not verify the deployment matches it. |
| `execution_claim` | Structured form of the adapter's execution self-report (e.g. `upstream_called`, `executed`). | **Self-report, not corroborated** — recording it explicitly does not make it independently verified. |

**Hashing.** `decision_hash` continues to blank `record_id`, `decision_hash`,
`signature`, `signature_key_id` before hashing; the new route-context fields are
**content**, so they are covered by `decision_hash` (tampering with a
route-context field fails verification). `request_hash` and `policy_bundle_hash`
are unchanged. Net effect: V2 extends tamper-detection to the route-context
fields — it does **not** add attestation or authenticity.

**Go shape.** Prefer a single versioned superset struct (the new fields
`omitempty`) with `schema_version` as the discriminator, so `verify-record` and
`explain` share one type and one code path. Emit `"2"` only when route-context is
populated; otherwise emit `"1"` for byte-compatibility with existing tooling.
Validate this choice against the canonical-hashing code before committing to it.

**`verify-record`.** Accept `schema_version ∈ {"1","2"}` (today it rejects
anything ≠ `"1"`); recompute `decision_hash` per the record's own version. Add
tests that a V1 record still verifies, a V2 record verifies, and tampering with a
route-context field fails `decision_hash`.

**Claim.** One new or revised ledger claim for route-context recording, with
`forbidden` language keeping the *asserted-not-attested* and
*self-report-not-corroborated* caveats. `status: delivered` only once the test +
doc paths exist on disk (claims gate).

**Non-goals.** No signing; no attestation; `topology_profile` is asserted only.

## 5. Step 3 — `boundary explain <record>` (read-only)

**Design.** A new read-side subcommand: `boundary explain <record.json>
[--format json]`. Pure read — no evaluation, no network, no mutation. It renders
a human-readable account of a record (V1 or V2): `action`, `reason`,
`decision_mode`, `matched_rule`, `policy_file`, and — for V2 — `adapter_id`,
`route_id`, `topology_profile`, `execution_claim`; the three hashes and exactly
what each covers; and a fixed "what this does not prove" footer (integrity not
authenticity; self-reports not corroborated).

- `--format json` emits a stable `boundary.explain.v1` object, mirroring the
  `boundary.selftest.v1` / `boundary.doctor.v1` conventions.
- `explain` **renders**; it does not re-verify hashes. It may print a one-line
  pointer to run `boundary verify-record` rather than duplicating verification,
  keeping the two commands' responsibilities crisp.

**Claim.** New claim: `explain` renders a decision record. `forbidden` language:
`explain` does not prove the verdict was correct, does not prove enforcement, and
does not verify hashes by itself.

**Tests / gates.** `explain` on the committed `docs/examples/decision-record.example.json`
renders the expected fields for V1, and on a V2 fixture for V2; `--format json`
schema is stable; hermetic. Add a CLI reference entry; do **not** add `explain`
to the first-run path (first-run stays the two proof lanes). Full gate set.

**Non-goals.** No evaluation, no network, no new governed surface.

## 6. Step 4 — `boundary replay <record>` (read-only, fixture-safe)

**The reconstruction contract is the crux.** A V1 record carries `request_hash`
but **not** the full request body, so the record alone cannot rebuild the
`GovernanceRequest`. Replay therefore takes the record **plus the request input**
— the same `--request` file the `verify-record` flow already documents — and the
operator's `--policies` directory:

```
boundary replay <record.json> --request <request.json> --policies <dir> [--format json]
```

**Behavior.** Replay (1) recomputes `request_hash` from the supplied request and
confirms it matches the record (so it is replaying *the recorded request*); (2)
when the record carries a `policy_bundle_hash`, recomputes it from `--policies`
and confirms it matches (so it is replaying against *the recorded policy bundle*,
not a stale or different one); (3) rebuilds the `GovernanceRequest` and runs it
through the same `governance.Pipeline` (all four stages) in a hermetic,
in-process configuration with no `AuditPublisher` side effects; and (4) compares
the **decision-defining fields** of the reproduced verdict against the record —
at minimum `action`, `reason`, `decision_mode`, `matched_rule`, and `policy_file`
where present — **not `action` alone**, because a stale or different bundle can
reach the same `action` through a different rule, reason, or decision mode. Exit
non-zero on any decision-field mismatch, on a `request_hash` mismatch, or on a
`policy_bundle_hash` mismatch. No upstream tool is called; nothing is mutated; no
network.

**Decide at build time:** whether V2 should optionally embed a canonical request
payload so `--request` becomes unnecessary for V2 records. Treat that as an
enhancement, not a prerequisite — the `--request` contract ships first and keeps
replay honest about what it is replaying.

**Claim.** New claim: `replay` reproduces the recorded *decision* locally — the
decision-defining fields (`action`, `reason`, `decision_mode`, `matched_rule`,
`policy_file`) match when the recorded request is re-evaluated against the
recorded policy bundle. `forbidden` language (load-bearing): replay does **not**
prove enforcement, does **not** prove that no upstream bytes moved, reproduces
the decision only for routed requests, and a match does **not** prove the
original verdict was correct — only that the same inputs reproduce the same
decision.

**Tests / gates.** Replay of the example record + its request against the
recorded bundle reproduces the **full** recorded decision (every decision-defining
field), not just `action`. Drift cases must fail closed: same `action` /
different `matched_rule`, same `action` / different `reason`, `policy_bundle_hash`
mismatch (stale or different bundle), and `request_hash` mismatch (different
request). Hermetic; CLI reference entry; full gate set.

**Non-goals (critical).** Replay reproduces the *decision*, not the absence of
upstream side effects, and not enforcement. Routed-only.

## 7. Cross-cutting claim & language discipline

- Steps 2–4 each add **exactly one** ledger claim, `delivered` only when its
  named test path(s) and doc path(s) exist on disk, or `claims/claims_test.go`
  fails. Step 1 likely adds none.
- All public copy uses negated / limitation framing for the not-attestation,
  not-corroboration, and not-proof caveats, so `claims/language_lint_test.go`
  passes. Reuse the wording already in `docs/RECEIPTS.md` and
  `docs/ROUTE_CONFORMANCE_CHECKLIST.md`.
- The public `BOUNDARY_ROADMAP.md` Phase 0A section is the *planned promise*.
  Moving any item into its **Baseline** section happens in the same change that
  ships the item behind gates — never ahead of it.

## 8. PR sequence (one lane, small PRs)

| PR | Scope | New claim? |
| --- | --- | --- |
| 0A-A | Record-location UX: uniform record-path output + `--out` semantics + docs/examples wiring. | No (evidence/doc refresh only). |
| 0A-B | `DecisionRecordV2` additive schema + `verify-record` dual-version support + tests. | Yes — route-context recording. |
| 0A-C | `boundary explain` (read-only) + `--format json` + CLI ref + tests. | Yes — explain. |
| 0A-D | `boundary replay` (read-only, `--request` contract) + CLI ref + tests. | Yes — replay. |

Each PR is independently revertable and ships only when green. `boundary test`
(Phase 1) follows 0A-D.

## 9. Gates every step must pass (same as the repo)

`gofmt -l` (empty), `go vet ./...` (root + the grpc nested module), `go test
./claims/... -count=1` (claims + language lint), `go test ./... -count=1 -timeout
5m` (no `-short` — this repo has no `testing.Short()` guards), `make docs-build`
(strict; any new public page wired into the `docs-site/` nav), and `make
release-check` end to end. New docs must be **staged (`git add`) before** running
`scripts/assert-no-internal-public-artifacts.sh`, because that guard enumerates
via `git ls-files` and only sees tracked or staged files — an unstaged new doc is
not scanned. Use `-coverpkg=./...` for any real coverage figure (black-box
attribution makes source packages read low; see `docs/TESTING.md`).

## 10. Non-goals for the whole lane

- No signing of records or commands.
- No cryptographic proof of the verdict itself.
- No topology attestation — `topology_profile` records an asserted posture and
  does not verify the deployment matches it.
- No independent proof that no upstream bytes moved — `execution_claim` stays an
  adapter self-report; `replay` reproduces the decision, not the absence of
  upstream side effects.
- No new governed action surface, no new transport adapter, no preview surface
  promoted to production, and no hosted, cloud, or control-plane behavior.
