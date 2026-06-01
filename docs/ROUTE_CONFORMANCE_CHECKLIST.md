# Route Conformance Checklist

Boundary governs an action only when the route is forced through Boundary.
Direct access to the same tool is a bypass unless deployment topology removes
that path. This page is a **documented checklist** — not code — for two
questions a reviewer or operator must answer for each route:

1. Does the route implement the ten governance lifecycle steps, or formally
   delegate the steps it does not own?
2. Has the route earned the maturity label it carries
   (`experimental` / `preview` / `production`)?

A passing checklist confirms a route is forced through Boundary in your
deployment and that its lifecycle is accounted for. It does **not** prove that
no other path to the same tool exists; that is a property of your topology, not
of this checklist.

The machine-readable truth lives in `adapters/<adapter>/readiness.yaml` and is
gate-enforced by
[tests/adapter_conformance](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/tests/adapter_conformance).
This checklist is derived from
[ADAPTER_READINESS_MATRIX.md](ADAPTER_READINESS_MATRIX.md), which remains the
canonical per-adapter source. Where this page and a `readiness.yaml` ever
disagree, the `readiness.yaml` and the matrix win.

---

## 1. Identify the surface and its maturity

| Surface | Status | Routed scope |
| --- | --- | --- |
| MCP adapter | Production | MCP tool calls forced through Boundary. The first production route. |
| Command Boundary | Delivered preview | Project command paths routed through Boundary. |
| Edit Boundary | Delivered preview | Edit envelopes routed through Boundary. |
| Secure GitHub | Preview | The tested write-after-taint fixture path. |
| CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A | Preview | See `adapters/<name>/readiness.yaml`. |

Preview does not mean production. Do not treat a preview surface as a production
route. Per-adapter maturity is declared in `adapters/<adapter>/readiness.yaml`
and summarized in [ADAPTER_READINESS_MATRIX.md](ADAPTER_READINESS_MATRIX.md).

---

## 2. Per-route lifecycle conformance checklist (the ten steps)

Each route MUST account for all ten lifecycle steps from the readiness matrix. A
step is satisfied when it is `implemented` (the adapter does it), `delegated` (a
named owner does it under a named contract), or `not_applicable` (the protocol
cannot express it, recorded explicitly). A `stub` is NOT a satisfied step.
`delegated` and `not_applicable` both require a named owner or reason — silent
absence does not pass.

Step states (from the matrix): `implemented`, `delegated`, `not_applicable`,
`stub`.

| # | Step | What to verify for THIS route | Satisfied when |
|---|---|---|---|
| 1 | `parse` | The transport payload is converted into a `GovernanceRequest`; malformed input is rejected, not silently passed. | `implemented` — every route must own parse. A route that cannot parse cannot govern. |
| 2 | `identify` | Agent, tenant, and trace identity are populated from transport context where the protocol carries them. | `implemented`, or `not_applicable` with a recorded reason if the transport carries no identity. |
| 3 | `evaluate` | The request is passed through `governance.Pipeline` (all four stages). | `implemented`, or `delegated` to `governance.Pipeline` under the adapter contract. |
| 4 | `deny` | A blocked request returns a transport-shaped denial **without** forwarding to the tool. | `implemented` — a route that cannot shape a denial cannot enforce. |
| 5 | `forward` | Allowed requests reach the tool through the governed path only, never a side channel. | `implemented`, or `delegated` to the host when the host owns the downstream call. |
| 6 | `inspect` | Tool responses are examined where the protocol allows it. | `implemented`, or `not_applicable` for fire-and-forget protocols that return nothing inspectable. |
| 7 | `metadata` | Governance verdict metadata is attached to the response where the protocol allows it. | `implemented`, or `not_applicable` / `delegated` when the protocol cannot carry response metadata. |
| 8 | `record` | A structured decision record is emitted for the verdict. | `implemented`, or `delegated` to `governance.AuditPublisher` under [DECISION_RECORDS.md](DECISION_RECORDS.md). |
| 9 | `bypass_proof` | The deployment is shown to have no direct tool path around Boundary. | `delegated` to deployment topology under a named contract — this is a deployment property, not adapter code (see Section 4). |
| 10 | `fail_closed` | On governance errors the route denies rather than passing through. | `implemented` — a fault (crashed backend, unreachable interceptor) MUST default to deny, distinct from a policy that decides allow. |

### Fill-in block for one route

For the route under review, copy this block and record each step from its
`readiness.yaml`:

```text
Route: <adapter or profile>
Status: <experimental | preview | production>   Target: <…>
  1 parse         : <implemented | delegated | not_applicable | stub>
  2 identify      : …
  3 evaluate      : …   (owner if delegated)
  4 deny          : …
  5 forward       : …   (owner if delegated)
  6 inspect       : …
  7 metadata      : …
  8 record        : …   (owner if delegated — governance.AuditPublisher)
  9 bypass_proof  : …   (owner if delegated — deployment topology)
 10 fail_closed   : …
Key gap (blocking higher maturity): <BND-…-NNN + one line>
```

### Current per-route lifecycle states

Derived from [ADAPTER_READINESS_MATRIX.md](ADAPTER_READINESS_MATRIX.md) and the
eight `adapters/*/readiness.yaml` declarations. `impl` = `implemented`,
`deleg` = `delegated`.

| Route | Status | Target | parse | identify | evaluate | deny | forward | inspect | metadata | record | bypass_proof | fail_closed |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| mcp | production | production | impl | impl | impl | impl | impl | impl | impl | deleg | deleg | impl |
| cli | preview | preview | impl | impl | deleg | impl | impl | impl | impl | deleg | deleg | impl |
| codeexec | preview | preview | impl | impl | deleg | impl | impl | impl | impl | deleg | deleg | impl |
| grpc | preview | preview | impl | impl | impl | impl | deleg | impl | impl | deleg | deleg | impl |
| managedagents | preview | production | impl | impl | impl | impl | impl | impl | impl | deleg | deleg | impl |
| webhook | preview | preview | impl | impl | impl | impl | deleg | impl | deleg | deleg | deleg | impl |
| a2a | preview | preview | impl | impl | deleg | impl | impl | impl | impl | deleg | deleg | impl |
| securegithub | preview | preview | impl | impl | deleg | impl | impl | impl | impl | deleg | deleg | impl |

`record` and `bypass_proof` are delegated on every route by design: records are
owned by `governance.AuditPublisher`, and bypass proof is owned by deployment
topology. Neither delegation is a defect; both are named contracts.

---

## 3. Graduation criteria: preview route → production

A preview route does not become production by adding documentation. It becomes
production when the lifecycle is fully accounted for AND the maturity evidence
exists. Per the matrix maturity taxonomy:

- `experimental` — `parse` implemented; other steps may be stubbed.
- `preview` — `parse`, `evaluate`, `deny`, and `record` are implemented or
  explicitly delegated; forwarding may be host-delegated.
- `production` — all ten steps implemented or formally delegated, **with
  integration tests, bypass proof, and fail-mode tests.**

### Production graduation checklist (every route)

A preview route reaches production only when ALL of the following hold:

1. **All ten lifecycle steps** are `implemented`, `delegated` (named owner and
   named contract), or `not_applicable` (recorded reason). No `stub` remains.
2. **Integration tests** exercise the full governed lifecycle end to end, not
   only unit-level parse or classify tests. The suite path is listed in the
   route's `readiness.yaml` `evidence.tests` and exists on disk.
3. **Fail-mode tests** prove the route denies on governance faults (the
   `fail_closed` step is `implemented` and tested), distinct from a
   policy-driven allow.
4. **Bypass proof** for the deployment exists under a named contract: evidence
   that the governed path is the **sole** path to the tool in the target
   topology. This is the gate that keeps most previews below production — a
   deployment artifact, not adapter code.
5. **`status` is raised to `production` in `readiness.yaml` only after 1–4**, so
   that `tests/adapter_conformance` (which fails a production claim without
   conformance evidence) stays green.
6. **Public copy is reconciled** in the same change: the readiness matrix, the
   claims ledger (`claims/boundary_claims.yaml`), `README.md`, and
   `RELEASE_TRUTH_PUBLIC.md` must all agree. Until then the route stays labeled
   preview everywhere.

A preview label is not a smaller production label. A route MUST NOT be described
as production until the items above are recorded.

### Per-route blocking gap (what production is waiting on)

From each route's `readiness.yaml` `gaps` and the matrix `Key gap` column:

| Route | Blocking gap ID | What production is waiting on |
|---|---|---|
| cli | BND-CLI-002 | Deployment evidence that the Boundary wrapper is the sole path to command execution. |
| codeexec | BND-CODE-001 | A real, named, tested execution boundary with integration tests and bypass proof. The current path is not a named execution boundary until that evidence exists. |
| grpc | BND-GRPC-001 | Deployment bypass evidence; streaming workloads need per-message governance lifecycle tests. |
| managedagents | BND-MAPROD-001 | A live upstream Managed Agents conformance run with operator-owned credentials. Target is `production`; status stays `preview` until this exists. |
| webhook | BND-WEB-001 | Deployment evidence that execution webhooks are the sole downstream action path; informational webhooks remain post-execution audit only. |
| a2a | BND-A2A-002 | Live protocol conformance and deployment bypass evidence. |
| securegithub | BND-GH-002 | Deployment bypass proof that agents cannot reach the direct GitHub API, direct upstream GitHub MCP, SSH, or other credentialed write paths outside Boundary. The opt-in live conformance harness proves the configured read and denied-write no-mutation path; it does not prove deployment bypass resistance. Secure GitHub is not production. |

### Command Boundary graduation plan (routed command paths only)

Command Boundary is a delivered **preview** surface, governed by
[command-boundary/PREVIEW_CLAIMS.md](command-boundary/PREVIEW_CLAIMS.md)
(`BND-CLAIM-CMD-001`). Its scope is commands routed through `boundary command
run`, `boundary shell`, or project-local shims — and nothing else. The
graduation plan below stays inside that routed scope on purpose.

**In scope for graduation (routed command paths):**

1. **Classifier coverage** — `boundary command classify` classifies commands
   without executing them, across the risk-class corpus, with redteam fixtures
   that assert both the verdict and `executed == false`.
2. **Routed-run enforcement** — `boundary command run` denies blocked commands
   before execution; allowed commands execute exactly once; secret-looking
   arguments are redacted in the decision record.
3. **Shim integrity** — project-local shims do not mutate global shell startup
   files, and the bypass model is documented.
4. **Routed bypass evidence** — deployment evidence that, for the protected
   project or workflow, the Boundary route is the relevant command path. This is
   the production gate for routed command governance.
5. **Reconciled copy** — public copy keeps the "preview" and "when routed
   through Boundary" qualifiers until 1–4 are recorded, and release-truth
   reconciliation states what Command Boundary proves and does not prove.

**Explicitly OUT of scope for any Command Boundary graduation claim.** These are
not paths Command Boundary routes, so production maturity on routed paths does
not extend to them: direct shell execution, global shells, CI jobs, SSH
sessions, cron jobs, editor-embedded terminals, and arbitrary command paths
outside Boundary. Command Boundary does not control your shell, does not protect
direct shell access, and does not control CI or remote SSH by default. Reaching
production on routed command paths does not change that — direct and un-routed
command execution stays outside the boundary unless deployment topology removes
those paths.

### Edit Boundary scope note (for completeness)

Edit Boundary is likewise a delivered **preview** surface
([edit-boundary/PREVIEW_CLAIMS.md](edit-boundary/PREVIEW_CLAIMS.md),
`BND-CLAIM-EDIT-001`). It applies only to proposed file mutations routed through
a Boundary edit envelope. It does not govern direct editor writes, does not
control all file writes, and does not protect direct editor writes; direct
filesystem writes, shell redirection, direct `git apply`, and unwrapped IDE APIs
are outside the envelope. Filesystem sandboxing is not claimed. Any future
graduation stays inside the routed-edit-envelope scope.

---

## 4. Caveat table: "route governed" vs "system globally controlled"

Passing the lifecycle checklist means a **route** is governed. It does **not**
mean the **system** is globally controlled. The distinction is deployment
topology: Boundary sees only what is forced through it. Each path below is a
**bypass** — outside Boundary — **unless** the deployment topology removes that
path so the governed route is the only way to reach the tool.

| Path | Route-governed? | Globally controlled? | Caveat |
|---|---|---|---|
| Agent → governed route → tool | Yes | Only if no other path to the tool exists | The path Boundary governs. The decision record proves Boundary decided; it does not prove no other path was taken. |
| Direct use of a binary already on `PATH` | No | No | A binary invoked directly does not pass through `boundary command run` or a shim. It is outside Boundary unless the environment routes that binary through the wrapper or project-local shims. |
| Direct shell access | No | No | Boundary does not control your shell and does not protect direct shell access. A shell that does not route through Boundary is a bypass. |
| CI / CD job execution | No | No | Boundary does not control CI jobs by default. A CI runner executing commands directly is outside Boundary unless the pipeline routes them through Boundary. |
| SSH / remote session execution | No | No | Boundary does not control remote SSH by default. Commands run over SSH are outside Boundary unless the remote environment routes them through Boundary. |
| Editor-embedded terminal | No | No | A terminal embedded in an editor executes commands directly. It is a bypass unless that terminal is configured to route through Boundary. |
| Direct editor write / direct filesystem write | No | No | Edit Boundary governs only mutations routed through a Boundary edit envelope. Direct editor writes and direct filesystem writes are outside the envelope; filesystem sandboxing is not claimed. |
| Direct GitHub API / direct upstream GitHub MCP / credentialed write outside Boundary | No | No | Secure GitHub is preview and governs supported GitHub actions routed through Boundary. It does not fully secure GitHub; a direct GitHub API call, a direct upstream GitHub MCP path, or another credentialed write path is a bypass until deployment bypass proof exists. |

### Reading the table

- **"Route-governed: Yes"** means Boundary evaluated and decided for that path.
  It is necessary, not sufficient, for system-wide control.
- **"Globally controlled"** is true only when the deployment removes every direct
  path to the tool — the `bypass_proof` step, owned by deployment topology, not
  by adapter code.
- A passing lifecycle checklist plus a missing `bypass_proof` means: the route is
  governed, the system is not globally controlled. State both.

---

## 5. Confirm Boundary is deciding and the record is intact

- [ ] `boundary doctor --json` (optionally `--surface <surface>`) describes the
      route and its bypass caveats. A passing doctor run reports readiness, not
      deployment proof.
- [ ] A governed request emits a structured decision record with the expected
      `action`. See [DECISION_RECORDS.md](DECISION_RECORDS.md).
- [ ] The record's `decision_mode` is `deterministic` or `classified`. Boundary
      does not emit `proved` decisions itself.
- [ ] `boundary verify-record <record.json>` returns `record verification: ok`.
- [ ] Where you control the inputs, bind the record to its policy bundle,
      request, and build digest with `--policies`, `--request`, and
      `--binary-digest`. Bare verification confirms internal hash-consistency
      only, not that the request and policy match what actually ran. See
      [RECEIPTS.md](RECEIPTS.md).

---

## What this checklist does not prove

- It does not prove a deployment removed every bypass path; that is a topology
  property you must enforce and audit yourself (`bypass_proof`).
- It does not prove live upstream conformance for preview surfaces.
- It does not prove the verdict was *correct* — only that the route decided and
  recorded. A record proves Boundary decided, not that execution was
  independently prevented.
- It does not turn a preview route into production. Maturity is the Section 3
  evidence set, not lifecycle coverage alone.
- A self-reported `upstream_called=false` or `executed=false` is an adapter
  signal from the adapter's own control flow, not a verified property of the
  decision record.

---

## Related references

- [ADAPTER_READINESS_MATRIX.md](ADAPTER_READINESS_MATRIX.md) — canonical
  per-adapter lifecycle and maturity (gate-enforced).
- `adapters/<adapter>/readiness.yaml` — machine-readable per-route declaration.
- [tests/adapter_conformance](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/tests/adapter_conformance) — the gate that fails on missing steps or unproven production claims.
- [command-boundary/PREVIEW_CLAIMS.md](command-boundary/PREVIEW_CLAIMS.md) and
  [edit-boundary/PREVIEW_CLAIMS.md](edit-boundary/PREVIEW_CLAIMS.md) — the
  routed-only scope for the two delivered preview surfaces.
- [DECISION_RECORDS.md](DECISION_RECORDS.md) and [RECEIPTS.md](RECEIPTS.md) —
  what the `record` step emits and how to verify it.
- [DOCTOR.md](DOCTOR.md) — routed-surface readiness and bypass caveats.
