# Standards and Incident Mapping

This page maps Boundary's delivered capabilities to the frameworks a security
engineer is likely already holding: the OWASP Top 10 for Agentic Applications,
the NSA's MCP security design considerations, the first joint Five Eyes
guidance on agentic AI, the EU AI Act's logging and human-oversight articles,
SOC 2 evidence collection, and the public 2025 incident classes that put
pre-execution control on those checklists.

Two ground rules govern every row on this page:

1. **Routed-only.** Boundary governs an action only when the route is forced
   through Boundary. A direct path to the same tool — a shell with
   credentials, a direct API client, an unrouted MCP connection — is a bypass
   unless deployment topology removes that path. The caveat is repeated per
   incident below because it is the honest limit of every mapping on this
   page. See [LIMITATIONS.md](../LIMITATIONS.md).
2. **Delivered claims only.** Every mapping cites claim IDs from the
   build-gated [claims ledger](./CLAIMS_LEDGER.md) (machine-readable source:
   [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml)).
   `delivered` claims must carry test and doc evidence that exists on disk or
   the build fails; `partial` claims are labeled as such and carry named gaps.
   Nothing on this page relies on a planned capability.

This page is an architecture mapping. It is **not** a compliance
certification, an audit opinion, or a guarantee that any deployment meets any
framework. Whether a specific deployment satisfies a control is determined by
that deployment's topology, policies, and assessment — not by this table.

Maturity vocabulary follows the README release-truth table: the MCP adapter is
the production route; Command Boundary and Edit Boundary are delivered
previews for routed command paths and routed edit envelopes; Secure GitHub and
the remaining adapters are labeled previews. Per-adapter status lives in
[ADAPTER_READINESS_MATRIX.md](./ADAPTER_READINESS_MATRIX.md).

## 1. OWASP Top 10 for Agentic Applications → pipeline stage

The OWASP GenAI Security Project published the Top 10 for Agentic
Applications (ASI01–ASI10, 2026 edition) in December 2025. Boundary's
four-stage evaluation pipeline ([ARCHITECTURE.md](../ARCHITECTURE.md)) —
Stage 1 trust check, Stage 2 static policies, Stage 3 domain interceptors,
Stage 4 policy evaluation — maps onto three of the ten codes. The other seven
are out of scope and listed as such below. A mapping here means Boundary
addresses the **routed** form of the risk at the named stage; it never means
the risk is eliminated.

| ASI code | Risk | Boundary stage / surface | Maturity | Ledger claims |
|---|---|---|---|---|
| ASI01 | Agent Goal Hijack | Stage 1 trust check + adaptive termination; taint-aware write-after-taint denial of the consequential action | Trust integration: delivered for protected adapters. Taint-aware Secure GitHub profile: preview, fixture-proof | `BND-CLAIM-009` (delivered), `BND-CLAIM-015` (delivered, preview surface), `BND-CLAIM-014` (delivered) |
| ASI02 | Tool Misuse | Stage 2 static policies + Stage 3 interceptors (including the Postgres AST guard) + Stage 4 policy evaluation, before the tool runs | Production on the routed MCP path; policy-as-code testing is local-only | `BND-CLAIM-001` (delivered), `BND-CLAIM-006` (delivered), `BND-CLAIM-008` (delivered), `BND-CLAIM-BUILD-001` (delivered), `BND-CLAIM-TEST-001` (delivered) |
| ASI05 | Unexpected Code Execution | Command Boundary: routed commands classified and decided before execution; CodeExec adapter for routed code payloads | Delivered preview, routed command paths and envelopes only; CodeExec adapter is preview | `BND-CLAIM-CMD-001` (delivered, preview surface), `BND-CLAIM-CMD-002` (delivered), `BND-CLAIM-003` (partial) |

### ASI01 — Agent Goal Hijack

Boundary has no LLM in the loop and does not inspect model content, so it
does not detect a hijacked goal in a prompt and does not prevent all prompt
injection. Its lane is the consequence of a hijack: the moment a steered
agent attempts a consequential tool call, that routed call is evaluated
before execution.

- **Stage 1 trust check** (`BND-CLAIM-009`, delivered): isolated or
  terminated agents are denied before the request proceeds; trust-backend
  faults fail closed; repeated protected-tool violations can isolate an agent
  before later protected calls execute
  ([TRUST_INTEGRATION.md](./TRUST_INTEGRATION.md),
  [ADAPTIVE_TERMINATION.md](./ADAPTIVE_TERMINATION.md)).
- **Write-after-taint denial** (`BND-CLAIM-015` delivered as a preview
  surface; `BND-CLAIM-014`, delivered): once untrusted GitHub content has
  entered the session, a later protected private-repository mutation is
  denied before the upstream call. This is the fixture-proof Secure GitHub
  preview profile ([secure-mcp/GITHUB.md](./secure-mcp/GITHUB.md)), exercised
  by the redteam packs and the demo in section 3.

Not covered within ASI01: detection of the injection itself, and any goal
hijack whose consequential action never crosses a Boundary route.

### ASI02 — Tool Misuse

Every routed tool call is evaluated before execution (`BND-CLAIM-001`,
delivered), and the MCP JSON-RPC proxy adapter is the production route
(`BND-CLAIM-006`, delivered). Static deny rules pin protected tools at
Stage 2; interceptors apply domain logic at Stage 3; the policy engine
evaluates conditions at Stage 4 and can return deny, warn, escalate, or
require-approval.

The bundled SQL interceptor is the Postgres AST guard (`BND-CLAIM-008`,
delivered): it classifies routed SQL statements by parser AST class before
policy evaluation, and destructive or unparsable SQL fails closed
([policies/POSTGRES.md](./policies/POSTGRES.md)). In static
(`CGO_ENABLED=0`) builds the classifier is unavailable, so every routed SQL
statement classifies as `UNKNOWN` and is denied fail-closed
(`BND-CLAIM-BUILD-001`, delivered) — the capability reduction is
classification, not deny posture. It is a statement classifier, not a SQL
firewall, and it does not prevent all SQL injection.

Policy behavior is regression-testable in CI with `boundary test`
(`BND-CLAIM-TEST-001`, delivered): operator-authored cases assert the
verdict for each fixture request against a local policy bundle, exiting
non-zero on drift ([POLICY_TESTING.md](./POLICY_TESTING.md)). This is a
local, fixture-only assertion runner; it does not prove production route
enforcement.

### ASI05 — Unexpected Code Execution

Command Boundary (`BND-CLAIM-CMD-001`, delivered preview) governs commands
routed through `boundary command run`, `boundary shell`, or project-local
shims: the command is classified and decided before execution, and denied
commands do not execute. Fixture redteam packs (`BND-CLAIM-CMD-002`,
delivered) demonstrate deny and require-approval outcomes for selected
command-risk paths with `executed=false`
([command-boundary/PREVIEW_CLAIMS.md](./command-boundary/PREVIEW_CLAIMS.md)).
The CodeExec adapter for routed code payloads is preview, tracked under
`BND-CLAIM-003` (partial).

Honest limits: this is routed-path governance, not shell sandboxing. It does
not control direct shell access, direct interpreters, CI runners, or SSH
sessions, and it is preview, not production command governance.

### Out of scope: the other seven ASI codes

Boundary makes no coverage claim for these codes. Where an adjacent Boundary
feature exists, it is noted with its limits — adjacency is not coverage.

| ASI code | Risk | Why out of scope |
|---|---|---|
| ASI03 | Identity & Privilege Abuse | Not covered. Decision records carry the requesting agent identity and trust state, but identity issuance, credential custody, and privilege management live outside Boundary. |
| ASI04 | Agentic Supply Chain Vulnerabilities | Not covered. Descriptor lockfiles detect drift in local MCP server descriptors (`BND-CLAIM-013`, delivered), but Boundary claims nothing about upstream package, model, or server supply chains, and a descriptor lock does not prove an upstream server is safe. |
| ASI06 | Memory & Context Poisoning | Not covered. Boundary does not inspect, sanitize, or persist model memory or context; it evaluates the routed tool call that results. |
| ASI07 | Insecure Inter-Agent Communication | Not covered. The preview A2A adapter evaluates routed agent-to-agent task requests, but Boundary provides no transport encryption, signing, or mutual authentication between agents. |
| ASI08 | Cascading Failures | Not covered. Adaptive termination can isolate a repeatedly violating agent on protected routes (`BND-CLAIM-009`), but Boundary claims no systemic cascading-failure containment. |
| ASI09 | Human-Agent Trust Exploitation | Not covered. Human-layer manipulation is outside a pre-execution action boundary. |
| ASI10 | Rogue Agents | Not covered. Boundary evaluates routed requests from agents it can see; it does not discover or contain agents operating outside its routes. |

## 2. Government and framework hooks

### NSA: MCP security design considerations (May 2026)

The NSA Artificial Intelligence Security Center's cybersecurity information
sheet *Model Context Protocol (MCP): Security Design Considerations for
AI-Driven Automation* (May 2026) recommends, among other controls, mediating
MCP traffic through a controlled point, validating tool invocations, and
logging tool invocations with their parameters, identities, and hashes.

| CSI theme | Boundary surface | Maturity | Ledger claims |
|---|---|---|---|
| Mediating proxy in the MCP path | The MCP JSON-RPC proxy adapter: routed requests are decided (allow / deny / warn / escalate / require-approval) before the upstream tool runs | Production, routed MCP paths only | `BND-CLAIM-006`, `BND-CLAIM-001` (both delivered) |
| Per-invocation logging with identities and hashes | A structured decision record for every governed verdict (`BND-CLAIM-002`); receipt-grade records carry request, policy-bundle, and decision hashes and re-verify offline with `boundary verify-record` (`BND-CLAIM-005`); schema_version 2 adds route-context fields covered by `decision_hash` (`BND-CLAIM-REC-001`) | Delivered | `BND-CLAIM-002`, `BND-CLAIM-005`, `BND-CLAIM-REC-001` (all delivered) |
| Know and pin your MCP surface | Read-only inventory of local MCP client configs (`BND-CLAIM-011`); inventory-derived risk graphs and starter policies (`BND-CLAIM-012`); descriptor lockfiles that deny on drift (`BND-CLAIM-013`) | Delivered, local-only; generated policies require operator review | `BND-CLAIM-011`, `BND-CLAIM-012`, `BND-CLAIM-013` (all delivered) |

Honest boundaries: Boundary does not implement MCP message signing or
transport-layer message integrity; its hashes cover its own decision records,
and they are unkeyed SHA-256 over canonical bytes — integrity, not
authenticity ([RECEIPTS.md](./RECEIPTS.md)). The mediating point governs only
traffic forced through it; a client with a direct path to the same MCP server
is a bypass.

### Five Eyes: "accountability opacity" (2026)

The first joint Five Eyes guidance on securing agentic AI systems (2026)
names **accountability opacity** — agentic decisions that are difficult to
trace and reconstruct after the fact — among its top risk categories.
Hash-verifiable decision records are Boundary's direct answer to that named
risk:

- Every governed verdict emits a structured decision record
  (`BND-CLAIM-002`, delivered) carrying the verdict, reason, decision mode,
  matched rule, and identity context
  ([DECISION_RECORDS.md](./DECISION_RECORDS.md)).
- Receipt-grade records are hash-verifiable: `boundary verify-record`
  recomputes the request, policy-bundle, and decision hashes, so tampering
  after emission is detectable by recomputation (`BND-CLAIM-005`,
  delivered). Schema_version 2 extends the hashed surface to route context
  — `adapter_id`, `route_id`, `topology_profile` (`BND-CLAIM-REC-001`,
  delivered).
- `boundary explain` renders a record's fields and what each hash covers
  (`BND-CLAIM-EXPLAIN-001`, delivered); `boundary replay` re-evaluates the
  recorded request against the recorded policy bundle and fails closed on
  any decision-field mismatch (`BND-CLAIM-REPLAY-001`, delivered).
- `boundary evidence bundle` packages those artifacts under a hashed
  manifest for handoff (`BND-CLAIM-UTIL-004`, delivered).

Honest boundaries: records exist for governed verdicts on routed actions
only; signatures are off by default, so hashes provide integrity, not
authenticity or attestation; replay reproduces the decision, not
enforcement; none of this proves that an unrouted action did not happen.

### EU AI Act: Article 12 and Article 14 — architecture fit, not compliance

Article 12 (record-keeping) expects high-risk AI systems to log relevant
events over the system lifetime; Article 14 (human oversight) expects
effective human oversight, including the ability to intervene. The high-risk
obligations are currently scheduled to apply in 2027.

- **Article 12 shape.** A per-verdict structured decision record
  (`BND-CLAIM-002`, delivered) that is hash-verifiable after the fact
  (`BND-CLAIM-005`, delivered), with read-only explanation and fail-closed
  replay lanes (`BND-CLAIM-EXPLAIN-001`, `BND-CLAIM-REPLAY-001`, both
  delivered) is the record-keeping shape Article 12 points at, applied to
  routed agent tool calls.
- **Article 14 shape.** Boundary verdicts are not binary: `warn`,
  `escalate`, and `require_approval` are first-class decisions alongside
  `allow` and `deny`, and all five are assertable against local policy
  bundles with `boundary test` (`BND-CLAIM-TEST-001`, delivered). Kernel
  integration contracts name an explicit escalation seam for routing
  approvals outward (`BND-CLAIM-010`, delivered). The preview Managed Agents
  adapter resolves routine tool confirmations by policy so that human
  attention concentrates on the escalations (`BND-CLAIM-007`, partial: a
  live upstream conformance run is not yet recorded).

Framing this honestly: the above is **architecture fit** for the 2027
obligations, not compliance. Deploying Boundary does not make a system
EU-AI-Act compliant; risk classification, conformity assessment, and the
rest of the obligation set are deployment- and organization-level work that
no component can supply by itself.

### SOC 2: auditor evidence today

`boundary evidence bundle` creates a local evidence directory — version,
selftest, doctor, optional demo outputs, and copied local decision-record
artifacts — and hashes every artifact into a manifest; `boundary evidence
verify` re-checks artifact existence, sizes, SHA-256 hashes, JSON schemas,
and decision-record parseability offline (`BND-CLAIM-UTIL-004`, delivered;
[EVIDENCE_BUNDLE.md](./EVIDENCE_BUNDLE.md),
[EVIDENCE_VERIFY.md](./EVIDENCE_VERIFY.md)). It needs no credentials and no
network, so the package is producible on demand during an audit window.

Honest boundaries: an evidence bundle is evidence *collection*, not a SOC 2
report, attestation, or control in itself. It does not prove production
deployment safety or that every route is protected, and whether it satisfies
a given evidence request is the auditor's determination.

## 3. 2025 incident classes → the routed shape Boundary denies

Public 2025 incidents made pre-execution control concrete. This section is
deliberately vendor-neutral: incidents are described as classes, not named
parties. For each class, the table shows which pipeline stage denies the
**routed** version of the action. One caveat applies to every row and is
restated per incident: **Boundary governs the action only if it is routed
through Boundary; the unrouted path is a bypass** that deployment topology
must remove.

| Incident class (2025) | Routed shape Boundary denies | Pipeline stage | Maturity | Ledger claims |
|---|---|---|---|---|
| Production-database deletion by a coding agent (July 2025) | Destructive SQL (`DROP` / `TRUNCATE` / `DELETE`) arriving as a routed database tool call | Stage 3 Postgres AST guard (destructive and unknown classes fail closed); Stage 2 static deny rules for protected tools | Production on the routed MCP path | `BND-CLAIM-001`, `BND-CLAIM-008`, `BND-CLAIM-BUILD-001` (all delivered) |
| GitHub MCP write-after-taint exfiltration (May 2025) | A protected private-repository mutation issued after untrusted GitHub content entered the session | Adapter-level taint marking + write-after-taint policy denial at policy evaluation, before the upstream call | Preview profile; fixture-proof demo; opt-in live harness | `BND-CLAIM-015`, `BND-CLAIM-014`, `BND-CLAIM-018` (delivered); `BND-CLAIM-019` (partial) |
| Data exposure through over-broad MCP data access (2025) | A routed call to a data-returning tool outside the policy allowlist | Stage 2 static deny / Stage 4 policy conditions; inventory and risk graph identify the exposure paths first | Production on the routed MCP path; discovery is local-only and read-only | `BND-CLAIM-001`, `BND-CLAIM-006`, `BND-CLAIM-011`, `BND-CLAIM-012` (all delivered) |

### Incident class: production-database deletion (July 2025)

A widely reported July 2025 incident: a coding agent deleted a company's
production database during a declared code freeze, then produced fabricated
data afterward. The routed shape of that action is destructive SQL on a
governed database route. When the database tool call passes through
Boundary, the Postgres AST guard classifies the statement before policy
evaluation; destructive and unparsable statements fail closed
(`BND-CLAIM-008`), and the ledger's evidence for `BND-CLAIM-001` is exactly
this shape: `DROP TABLE` requests denied before the downstream handler runs.
Static builds keep the deny posture by classifying all SQL as `UNKNOWN` and
denying fail-closed (`BND-CLAIM-BUILD-001`).

**Routed-only caveat:** Boundary governs this only if the database action is
routed through it. An agent holding direct database credentials, or a client
connected straight to the database or to an unrouted MCP server, is a bypass
that deployment topology must remove.

### Incident class: GitHub MCP write-after-taint exfiltration (May 2025)

In May 2025, security researchers publicly demonstrated a "toxic agent flow"
against a GitHub MCP integration: a malicious issue in a public repository
steered an agent that also had private-repository access into leaking
private data through its own GitHub tools. The published analysis concluded
the pattern cannot be fixed server-side and requires a control at the
agent-system layer — which is the routed shape Boundary's Secure GitHub
preview profile addresses: untrusted GitHub content taints the session, and
a later protected private-repository mutation is denied before any upstream
GitHub call (`BND-CLAIM-015`). The fixture redteam pack asserts the deny
outcome with no real secrets and no live mutation (`BND-CLAIM-014`), and an
opt-in live conformance harness exists for operator-owned GitHub App
credentials (`BND-CLAIM-018`); an operator-owned live conformance run has
not yet been recorded (`BND-CLAIM-019`, partial).

**Worked example (fixture-only).** The flagship demo runs this exact shape
end to end with no credentials, no network calls, and no live mutation:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
```

Expected signal:

```text
expected action: DENY
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

The denial emits a hash-verifiable decision record, and `verify-record`
recomputes its hashes. `upstream_called=false` is an adapter self-report,
not a field of the hashed record, and the demo is a fixture: it does not
prove live GitHub conformance or production bypass resistance
([DEMO_GITHUB_LETHAL_TRIFECTA.md](./DEMO_GITHUB_LETHAL_TRIFECTA.md)).

**Routed-only caveat:** Boundary governs this only if the GitHub MCP traffic
is routed through it. An agent with a direct GitHub token or a direct
connection to an unrouted GitHub MCP server is a bypass that deployment
topology must remove — and Secure GitHub remains a preview profile until
deployment bypass proof exists.

### Incident class: data exposure through MCP data access (2025)

Multiple 2025 disclosures showed MCP integrations exposing data an agent
should not reach — over-broad data-access tools reachable through MCP calls,
and access-scoping bugs in hosted MCP services. The routed shape Boundary
denies is the least-privilege failure: a routed call to a data-returning
tool that policy does not allow is denied at Stage 2 (static rules) or
Stage 4 (policy conditions) before the upstream runs (`BND-CLAIM-001`,
`BND-CLAIM-006`), and every verdict leaves a decision record. Before
enforcement, `boundary inventory` and `boundary graph` identify which
configured MCP servers expose data paths at all (`BND-CLAIM-011`,
`BND-CLAIM-012` — read-only discovery; generated policies are starter
policies requiring operator review).

Honest limit: an authorization bug inside an upstream service is upstream's
flaw; Boundary cannot repair it. What Boundary contributes is deny-by-policy
for routed calls that exceed the allowlist, and a recorded verdict for every
governed call.

**Routed-only caveat:** Boundary governs this only if the data-access call
is routed through it. Direct API keys, direct database connections, or
unrouted MCP clients are bypasses that deployment topology must remove.

## What this mapping does not claim

| Not claimed | Why |
|---|---|
| Compliance with any framework on this page | This is an architecture mapping. Compliance is a property of an assessed deployment, not of a component. |
| Coverage of OWASP ASI codes beyond ASI01, ASI02, ASI05 | The other seven are listed as out of scope above. |
| Prevention of all prompt injection or universal agent safety | Boundary does not inspect model content; it decides routed consequential actions. Fixture packs prove tested deny paths only. |
| A general SQL firewall | The Postgres AST guard classifies statements; it is not a SQL firewall and does not prevent all SQL injection. |
| Governance of non-routed paths | Direct shell, editor, API, and database paths are bypasses unless topology removes them. |
| Signed or attested records by default | Record hashes are unkeyed SHA-256 — integrity, not authenticity. Boundary does not emit `proved` decisions. |
| Production status for preview surfaces | Secure GitHub, Command Boundary, Edit Boundary, and the non-MCP adapters are previews; detection rates are not published, and fixture corpora are known-pattern regression suites, not effectiveness statistics. |

## References

- [CLAIMS_LEDGER.md](./CLAIMS_LEDGER.md) — claim statuses and evidence paths.
- [ADAPTER_READINESS_MATRIX.md](./ADAPTER_READINESS_MATRIX.md) — per-adapter maturity.
- [LIMITATIONS.md](../LIMITATIONS.md) — the routed-only constraint in full.
- [ARCHITECTURE.md](../ARCHITECTURE.md) — the four-stage pipeline.
- [DECISION_RECORDS.md](./DECISION_RECORDS.md) and [RECEIPTS.md](./RECEIPTS.md) — record fields and hash verification.
- [EVIDENCE_BUNDLE.md](./EVIDENCE_BUNDLE.md) and [EVIDENCE_VERIFY.md](./EVIDENCE_VERIFY.md) — evidence packaging.
- [DEMO_GITHUB_LETHAL_TRIFECTA.md](./DEMO_GITHUB_LETHAL_TRIFECTA.md) — the worked incident demo.

External documents referenced (by title; consult the publishers for current
versions): *OWASP Top 10 for Agentic Applications* (OWASP GenAI Security
Project, December 2025); *Model Context Protocol (MCP): Security Design
Considerations for AI-Driven Automation* (NSA Artificial Intelligence
Security Center CSI, May 2026); the joint Five Eyes guidance on securing
agentic AI systems (2026); Regulation (EU) 2024/1689 (EU AI Act), Articles
12 and 14.
