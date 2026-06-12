# Fulcrum Integration

Boundary runs in two modes:

- `standalone`: local policy files, in-process trust, in-process budget checks, and JSON decision records.
- `kernel`: Boundary keeps the same pre-execution adapter surface and **defines integration contracts for all six seams** (policy, trust, budget, escalation, audit, envelope). It validates all six seam configs at startup and currently **wires the trust seam** (Redis IPC via `fulcrum-trust`). The remaining seam configs (policy engine, budget, escalation, audit, envelope) are validated configuration contracts; the binary does not yet consume them — local policies and slog audit remain in effect. Operators will see a startup notice when running in kernel mode.

## Interface Contracts

The shared seams are defined in `governance/providers.go`.

| Contract | Standalone implementation | Kernel implementation |
|---|---|---|
| `PolicyProvider` | `governance/standalone.FilePolicyProvider` | `governance/kernel.RedisPolicyProvider` |
| `TrustChecker` / `TrustBackend` | `governance.StandaloneTrustBackend` | `governance.RedisTrustBackend` |
| `CostPredictor` | `governance/standalone.StaticCostPredictor` | `governance/kernel.StaticCostPredictor` until the Oracle bridge is wired |
| `BudgetEnforcer` | `governance/standalone.InMemoryBudgetEnforcer` | `governance/kernel.HTTPBudgetEnforcer` |
| `EscalationHandler` | `governance/standalone.RequireApprovalEscalationHandler` | `governance/kernel.NATSEscalationHandler` (routing) or `governance/kernel.AwaitingEscalationHandler` (await) — selected by `BundleConfig.Subscriber`; see "Escalation Seam: Routing And Await" below |
| `AuditPublisher` | `governance.SlogAuditPublisher` | `governance/kernel.NATSAuditPublisher` |
| `EnvelopeManager` | `governance/standalone.LocalEnvelopeManager` | `governance/kernel.NATSEnvelopeManager` |
| `ProofCorrespondence` | static proof map | static proof map |

## Kernel Configuration

Boundary validates configuration before server startup. Kernel mode must name every external seam explicitly:

```yaml
mode: kernel
kernel:
  policy_engine:
    type: redis
    redis_url: redis://localhost:6379
    key_prefix: "fulcrum:policies:"
  trust:
    type: redis_ipc
    redis_url: redis://localhost:6379
    key_prefix: "agent:"
  budget:
    type: api
    endpoint: http://fulcrum-api:8080/api/v1/cost/record
  escalation:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.foundry.escalate
  audit:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.audit.boundary
  envelope:
    type: nats
    nats_url: nats://localhost:4222
    subject: fulcrum.envelope
security:
  require_agent_id: true
```

The schema shape is recorded in `config/schema.v1.yaml`.

## Escalation Seam: Routing And Await

The kernel escalation seam (`governance.EscalationHandler`) has two modes, selected in `kernel.NewBundle` by whether `BundleConfig.Subscriber` is set. Both are Go-level `NewBundle` contracts exercised by the integration tests — per the kernel-mode note above, the served binary does not yet consume the kernel escalation seam end-to-end — and both govern only actions routed through Boundary; a direct path to the same tool is not held, escalated, or denied by this seam.

### Routing Mode (`NATSEscalationHandler` — `Subscriber` nil)

Routing mode publishes the escalate envelope `{"request": <GovernanceRequest>, "reason": <string>}` to the escalate subject (`BundleConfig.EscalateSubject`, default `fulcrum.foundry.escalate`) and unconditionally returns a synthetic `escalate` decision. It is advisory routing only: it never waits for a resolution and is never an approval — the decision leaves the pipeline as `escalate`, and any follow-up happens out-of-band. With a nil `Publisher` it skips the publish silently and still returns the synthetic decision.

### Await Mode (`AwaitingEscalationHandler` — `Subscriber` set)

Await mode publishes the same frozen escalate envelope, then blocks for a bounded window awaiting a resolution message on the resolved subject, correlated by `GovernanceRequest.RequestID` (the waiter is registered before the publish, so a resolution that arrives immediately after the publish is not missed). Human-in-the-loop resolution requires a deployed resolver consuming the escalate subject; absent one, every awaited escalation denies when the window expires.

`BundleConfig` fields for await mode:

| Field | Default | Meaning |
|---|---|---|
| `Subscriber` | nil (selects routing mode) | Bare injection seam mirroring `Publisher`: `Subscribe(ctx, subject, handler func([]byte)) (unsubscribe func(), error)`. Boundary ships no NATS implementation in-repo; the deployment provides the transport. |
| `EscalateResolvedSubject` | `fulcrum.foundry.escalate.resolved` | Subject the awaiting handler lazily subscribes to (once, on the first escalation; a failed first subscribe is sticky, and every later escalation faults closed until the process restarts). |
| `EscalateAwaitTimeout` | `120s` when zero | The bounded synchronous hold. Negative values are a `NewBundle` error, validated only in await mode so routing-mode configs are unaffected. |

The 120-second default is a bounded synchronous hold, not an approval-workflow SLA: long enough for an automated or already-recorded resolution to round-trip, short enough that a routed transport is not held open indefinitely, and at expiry the decision denies by default (fail-closed). The intended flow for approvals slower than the window is retry-after-async-approval: the denied call is retried after the approval lands and the retry resolves fast (see the resolver-side contract below) — but the fast retry requires a stable request identity across retries, and `RequestID` stability is adapter-dependent (the pipeline mints a fresh UUID when the adapter supplies none), so do not treat the retry fast path as unconditionally wired.

### Resolved-Message Wire Contract (v0.1, Frozen)

Subject: `fulcrum.foundry.escalate.resolved` (core NATS publish by IO; JetStream-captured on the IO side). Boundary subscribes; Boundary never publishes here. Correlation key: `request_id` == the originating `GovernanceRequest.RequestID`.

JSON object, exactly these fields:

| field | JSON type | required | semantics |
|---|---|---|---|
| `request_id` | string | **required** | Correlation key. MUST equal the `request.request_id` of the escalate envelope this resolves. A message whose `request_id` matches no live waiter is ignored. |
| `status` | string | **required** | One of `approved` \| `denied` \| `expired`. (`pending` MAY appear on the IO stream but is **not a resolution**; Boundary ignores it — see below.) Any other value → message ignored. |
| `reviewer_id` | string | optional | Identity of the human/automation that resolved it. Surfaced in the decision reason when present on `approved`/`denied`. Absent/empty → omitted from the reason. |
| `review_note` | string | optional | Free-text rationale from the reviewer. Surfaced in the decision reason when present. Absent/empty → omitted. |
| `resolved_at` | string (RFC 3339) | optional | When IO recorded the resolution. Advisory/audit only; Boundary does not branch on it. Ignored if absent or unparseable. |

Malformed and unexpected messages are ignored, and the waiter keeps awaiting until its window expires: a body that is not JSON, an empty `request_id` or one matching no live waiter (which also covers resolutions arriving after the window), and a missing or unknown `status` (including `pending`) are all dropped without effect. Unknown extra fields are ignored for forward compatibility. On duplicate resolutions for the same `request_id`, the first delivered wins.

### Outcome Mapping

| Outcome | Action | DecisionMode | Reason |
|---|---|---|---|
| `status: "approved"` | `allow` | `human_approved` | `escalation approved`, plus ` by <reviewer_id>` and `: <review_note>` when present |
| `status: "denied"` | `deny` | `human_approved` | `escalation denied`, plus reviewer/note when present |
| `status: "expired"` | `deny` | `deterministic` | `escalation expired`, plus reviewer/note when present (usually absent) |
| window elapses with no resolution | `deny` | `deterministic` | `escalation timed out after <window> awaiting resolution` |
| fault: publish or subscribe error, duplicate in-flight `request_id`, cancelled context, nil or invalid handler decision | `deny` | `deterministic` | `escalation fault (fail-closed): <detail>` |

`human_approved` appears only when a human verdict was actually relayed (`approved`/`denied`); a mechanical record expiry on the resolver side, a local await timeout, and every fault stay `deterministic`, because `human_approved` there would assert a review that did not happen. The four decision modes (`deterministic`, `classified`, `proved`, `human_approved`) are frozen — the seam adds no mode, and Boundary still does not emit `proved` decisions (`docs/PROOF_BOUNDARY.md`).

The awaiting handler asserts no trust: its returned decision carries `TrustScore: 0` and an empty `TrustState`, because a reviewer attests a verdict, not trust — trust fields are pipeline-owned (stage 1 plus the deferred trust update). The resolved message carries no trust fields and none may be read from it; the pipeline adopts only `Action`, `Reason`, and `DecisionMode` from the handler's decision.

### Pipeline Seam And Dry-Run

`PipelineConfig.Escalation` is the pipeline-side seam. Nil — the default, and always nil on the standalone path — preserves the pre-seam behavior byte-for-byte: the decision is relabeled `escalate`/`classified` and returned with no await. With a handler configured, the pipeline relays the handler's resolved verdict and denies fail-closed (reason prefixed `escalation fault (fail-closed):`) on a handler error, a nil decision, or a returned action outside `allow`/`deny`/`warn`/`escalate`/`require_approval`. Under `PipelineConfig.DryRun` the pipeline does not block on the await: the escalate decision keeps the relabel path and the reason notes that the await was skipped.

### Resolver-Side Contract And Deployment Notes

- IO's ingest is idempotent on `request_id`: a duplicate ingest of an already-resolved escalation re-publishes the existing resolution to the resolved subject, which is what lets a re-escalation of an approved request resolve fast instead of timing out — subject to the request-identity caveat above (`RequestID` stability is adapter-dependent).
- `Subscriber` implementations must core-subscribe to the resolved subject (or bind a consumer to the resolver's existing stream); never declare a new JetStream stream over subjects overlapping `fulcrum.foundry.>` — that collides with the IO resolver's `FULCRUM_FOUNDRY_EVENTS` stream (NATS error 10065). The resolver retains resolutions for 7 days; replay is possible from its stream, but the await path is a core subscribe.
- `AwaitingEscalationHandler.Close()` releases the resolution subscription and refuses further escalations (they fault, and a fault denies fail-closed rather than hangs). The additive `Bundle.Close()` closes the bundle's closeable seams and is a no-op for routing-mode bundles.

## Ownership

- `fulcrum-io` owns Policy Engine, Budget API, Foundry escalation, audit storage, and envelope lifecycle.
- `fulcrum-trust` owns the canonical Beta trust model and Redis IPC semantics.
- `Fulcrum-Proofs` owns theorem names and invariant scopes.
- Boundary owns the transport-facing enforcement point and the local interfaces that connect those services to pre-execution decisions.
