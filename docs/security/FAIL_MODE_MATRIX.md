# Transport Fail-Mode Matrix

**Status:** v0.2 (updated for ADR-047 CHECK_INDETERMINATE; earlier v0.1
described the pre-ADR-047 deny/fail-open behavior)
**Source:** PRD-004, ADR-047. Pipeline at `governance/pipeline.go`.
**Audience:** security reviewers, acquirers, on-call engineers.

This document specifies the exact fail-open / fail-closed behavior of the Boundary
governance pipeline for each supported transport and each fault class. It
records what the code **does today**, with source citations. Recommendations
for production defaults are in §3; known gaps are in §6.

**Path conventions:** unqualified `pipeline.go` citations refer to
`governance/pipeline.go`. Adapter citations use the full repo-relative path
(`adapters/<transport>/adapter.go`). Test citations use the full path
(`governance/pipeline_test.go`, `governance/pipeline_coverage_test.go`).

## 0. CHECK_INDETERMINATE (ADR-047)

ADR-047 resolves how the pipeline classifies a required synchronous check that
cannot produce a valid result — a trust lookup, trust update, interceptor run,
policy evaluation, or required identity — on an execution-capable transport:

> The outcome is CHECK_INDETERMINATE. It is neither ALLOW nor a substantive
> policy denial and MUST block execution.

Mechanics (`governance/request.go`, `governance/errors.go`, `pipeline.go`):

- `GovernanceDecision.Action == "check_indeterminate"`
  (`governance.ActionCheckIndeterminate`). `GovernanceDecision.Allowed()`
  returns true only for `"allow"` and `"warn"`, so every execution gate that
  consults `Allowed()` blocks an indeterminate verdict.
- A machine-readable `CheckFailure` travels on the decision, the audit event
  (`AuditEvent.Check`), and the decision record (`DecisionRecordV1.Check`):
  `stage` (enforcement stage: `trust`, `trust_update`, `interceptor`,
  `policy_eval`, `identity`, `config`), `class` (check class: `trust`,
  `policy`, `identity`, `config`), `category` (one of `unavailable`,
  `timeout`, `canceled`, `panic`, `invalid_result`, `missing_config`,
  `missing_identity`, `stale_snapshot`), and a sanitized `cause`. The record
  also carries `request_id`, agent/tenant ids when present, transport,
  `request_hash` (canonical action digest), and `trust_state: "UNKNOWN"` when
  no posture was obtained. No secrets or raw arguments are recorded.
- **Enforcing vs non-enforcing.** A transport in `FailClosedTransports` is
  enforcing: check failure → `check_indeterminate` and the action is not
  executed. A transport explicitly left out of a *non-empty*
  `FailClosedTransports` list is a declared non-enforcing surface (ADR-047's
  can_deny=false): the action may continue and the decision and record carry
  the would-have-blocked `CheckFailure` context rather than an ordinary allow.
- **Evaluator, trust-lookup, interceptor, and trust-update panics** are
  recovered at the pipeline boundary and classified `category: "panic"`.
- **Invalid check results** — a nil evaluator decision, an out-of-vocabulary
  evaluator action, or a blocking interceptor action outside the verdict
  vocabulary — are `category: "invalid_result"`, not implicit allows.
- **Audit delivery failure** never changes the decision. When the configured
  auditor implements `CheckedAuditPublisher` (or panics in `Publish`), the
  pipeline increments `Pipeline.AuditFailures()`, flips `Pipeline.Degraded()`,
  and logs once per failure with structured, identifier-digested fields. A
  successful publish clears `Degraded()`. `AuditFailures()` never decreases,
  so the outage is visible after recovery.
- **Configuration validity.** `PipelineConfig.Validate()` rejects a non-nil
  empty `FailClosedTransports` (`ErrEmptyFailClosedList`) — an empty list is
  not an acceptable production enforcement policy, and emergency bypass would
  require a break-glass mechanism this package does not offer. A pipeline
  built with `RequireAudit: true` and a nil auditor records
  `ErrMissingAuditPublisher`. `NewPipeline` stores any configuration error;
  every `Evaluate` then returns `check_indeterminate` /
  `missing_config` (`stage: "config"`) — an invalid configuration fails
  closed rather than silently failing open. `Pipeline.ConfigError()` exposes
  the stored error.

## 1. Pipeline Fail-Mode Architecture

`Pipeline.Evaluate` runs four stages in sequence and always emits exactly one
audit event via a deferred hook (`pipeline.go`, `Evaluate`). The staging is:

| # | Stage | Location | Error behavior |
|---|-------|----------|----------------|
| 1 | Trust Check | `pipeline.go` | **Required-check failure → CHECK_INDETERMINATE on enforcing transports.** Trust checker error or recovered panic → `check_indeterminate` (category `unavailable`/`timeout`/`canceled`/`panic`), trust posture recorded as `UNKNOWN`. Agent in `ISOLATED`/`TERMINATED` state → substantive deny (unchanged). `RequireAgentID` with a missing AgentID → `check_indeterminate`/`missing_identity` on enforcing transports; on a declared non-enforcing transport the check is recorded and evaluation continues. |
| 2 | Static Policies | `pipeline.go` | **No error path.** Glob matching via `path.Match` discards the match error; malformed patterns are treated as non-matching rather than crashing the pipeline. |
| 3 | Domain Interceptors | `pipeline.go` | **Required-check failure → CHECK_INDETERMINATE on enforcing transports.** Interceptor error or recovered panic → `check_indeterminate`. A blocking result whose action is outside the verdict vocabulary (`deny`, `warn`, `escalate`, `require_approval`) is `invalid_result`. `{Allowed: false}` with a valid action keeps the interceptor's own verdict; empty action defaults to `deny`. |
| 4 | PolicyEval | `pipeline.go` | **Required-check failure → CHECK_INDETERMINATE on enforcing transports.** Evaluator error or recovered panic, a nil decision, or an out-of-vocabulary action → `check_indeterminate`. On a declared non-enforcing transport the default allow stands and `Check` records the would-have-blocked context. |
| 5 | Trust Update (deferred) | `pipeline.go` defer | **Required-check failure → CHECK_INDETERMINATE on enforcing transports.** A `RecordDecision` backend error or recovered panic flips an otherwise-allowed decision to `check_indeterminate` (`stage: "trust_update"`). A successfully-obtained trust posture (e.g. `TRUSTED`) is preserved — it is evidence that was genuinely obtained. |

The Postgres SQL interceptor is a concrete Stage 3 guard: unknown or
unparsable SQL returns `deny`, destructive SQL returns `deny`, administrative
SQL returns `escalate`, and read/write classes continue with `sql_class`
annotations for PolicyEval.

Every decision emitted by `Pipeline.Evaluate` now carries an explicit
`decision_mode` label (see `governance/decision_mode.go`, PRD-002). The
four modes are mutually exclusive:

| Mode | Who sets it | When |
|---|---|---|
| `deterministic` | Boundary pipeline | Default for every stage; static-rule matches, trust outcomes, interceptor outcomes, `ActionDeny`/`ActionWarn`/`ActionRequireApproval`, every CHECK_INDETERMINATE outcome (a local fault is a mechanical outcome, not a relayed resolution), and the no-match default allow. Also the kernel escalation-await seam's mechanical denies — a resolver-side record expiry, a local await timeout, and every escalation fault — because no human verdict was relayed and none is claimed. |
| `classified` | Boundary pipeline | PolicyEval `ActionEscalate` (escalation implies a semantic condition the evaluator could not resolve deterministically). With no `EscalationHandler` configured this is the whole escalate outcome; it also stays as the relabel default when an await handler returns no adoptable mode. |
| `proved` | Upstream Foundry (fulcrum-io) | Set when a Lean 4 invariant has discharged the decision. Boundary itself never emits this mode, and the escalation seam is guarded against adopting it (`isAdoptableEscalationMode`, `governance/pipeline.go`). |
| `human_approved` | Upstream Foundry (fulcrum-io), relayed by the kernel escalation-await seam | Set when a human review resolved an escalated action. Boundary does not originate this mode from its own logic; the kernel `AwaitingEscalationHandler` relays it onto a pipeline decision only for an `approved`→allow or `denied`→deny resolution message from the upstream layer (`governance/kernel/escalation.go`). This is a kernel-mode, routed-only path that requires an injected `Subscriber` and a deployed resolver; with no handler configured (the standalone path, and the default) Boundary never emits it. |

Per-stage citations: the default value is initialized where `decision` is
constructed (`governance/pipeline.go`, in `Evaluate`); the `classified`
override is in the `ActionEscalate` branch of Stage 4 (`governance/pipeline.go`,
`Evaluate`). On that same branch, when `PipelineConfig.Escalation` is set, the
pipeline relays the handler's vetted verdict via `resolveEscalation`
(`governance/pipeline.go`), which is how a relayed `human_approved` (or a
mechanical-deny `deterministic`) can reach a Stage-4 escalate decision; a fault
there denies `deterministic` fail-closed. The decision-mode field is propagated
to the audit event by `emitAudit` (`governance/pipeline.go`), so audit sinks can
aggregate or filter by epistemic confidence level.

The defer-emit hook in `Evaluate` is the only place a decision can be
reshaped:

1. `decision.Duration` is recorded.
2. The deferred trust update runs; a failure marks the decision
   `check_indeterminate`/`trust_update` (enforcing transports).
3. `p.publishAudit(...)` (via `emitAudit`) publishes the **original** action
   to the auditor and accounts for delivery (see §0).
4. **Then** — and only then — if `p.dryRun` and the action is `deny` or
   `check_indeterminate`, the action is rewritten to `allow`,
   `decision.DryRun = true`, and the original reason is prefixed with
   `DRY-RUN would deny:` or `DRY-RUN would block:` respectively.

This ordering guarantees that the audit log always reflects what governance
would have blocked, even when dry-run flips the caller-visible action.

## 2. Fail-Mode Matrix

Fault classes × transports. Each cell is one of:

- **DENY** — pipeline sets `decision.Action = "deny"` (substantive verdict).
- **CHECK_INDETERMINATE (blocks)** — pipeline sets
  `decision.Action = "check_indeterminate"`; `Allowed()` is false so every
  execution gate blocks. Recorded with stage/class/category context.
- **ALLOW + check recorded** — declared non-enforcing transport
  (can_deny=false): the default allow stands and `Check` carries the
  would-have-blocked context.
- **ALLOW** — pipeline leaves the default `decision.Action = "allow"`.
- **PASS** — pipeline is not involved; downstream error is surfaced by the
  caller's runtime unchanged.
- **ERR→caller** — adapter `ParseRequest` returns a Go error; the embedding
  runtime (MCP gateway, agent runtime, sandbox runtime) decides what the caller
  sees. Functionally the tool call does not proceed, which is equivalent to
  deny — but the decision and audit event are **not** produced by the Boundary
  pipeline.
- **HTTP 400 / codes.Internal** — adapter surfaces a protocol-specific
  fail-closed error before pipeline entry.

The pipeline rows below assume enforcing transports (the defaults cover all
seven). The non-enforcing column outcome — allow with recorded check context —
applies only to transports explicitly left out of a non-empty
`FailClosedTransports` list.

| Fault Class | MCP | CLI | Code Exec | gRPC | Managed Agents | A2A | Webhook |
|---|---|---|---|---|---|---|---|
| Trust store unreachable | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE; informational mode records only |
| Trust update (RecordDecision) error | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE; informational mode records only |
| Agent ISOLATED or TERMINATED | DENY | DENY | DENY | DENY | DENY | DENY | DENY |
| AgentID missing with `RequireAgentID` | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) | CHECK_INDETERMINATE (`missing_identity`) |
| Adapter parse failure | JSON-RPC error `adapters/mcp/gateway.go` or ERR→caller from raw adapter use | ERR→caller `adapters/cli/adapter.go` | ERR→caller `adapters/codeexec/adapter.go` | `codes.InvalidArgument` with deny trailers `adapters/grpc/adapter.go` | ERR→caller or deny confirmation from proxy resolver `adapters/managedagents` | ERR→caller `adapters/a2a/adapter.go` | HTTP 400 `adapters/webhook/adapter.go` |
| Interceptor error or panic | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE; informational mode records only |
| PolicyEval error, panic, nil result, or invalid action | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE | CHECK_INDETERMINATE; execution mode 403/not forwarded; informational mode records only |
| Transport left out of a non-empty `FailClosedTransports` (declared non-enforcing) | ALLOW + check recorded | ALLOW + check recorded | ALLOW + check recorded | ALLOW + check recorded | ALLOW + check recorded | ALLOW + check recorded | ALLOW + check recorded |
| Audit publisher outage or panic | Decision unchanged; `Degraded()` true, `AuditFailures()` increments, one structured warn per failure | same | same | same | same | same | same |
| Downstream tool error (5xx / non-zero exit) | PASS through governed proxy response inspection `adapters/mcp/forwarder.go` | PASS `adapters/cli/adapter.go` | PASS `adapters/codeexec/adapter.go` | PASS handler error after allow decision, with governance trailers where the server context permits `adapters/grpc/adapter.go` | PASS through proxied session stream with response inspection `adapters/managedagents/response_inspector.go` | PASS `adapters/a2a/adapter.go` | Execution mode passes downstream response after allow; informational mode never forwards |

**Notes on the matrix:**

- **Adapter parse failure** happens BEFORE the pipeline runs. No audit event
  is emitted by the pipeline in this case, because `Evaluate` is never
  called. The gRPC and webhook adapters embed pipeline invocation in their
  own HTTP/gRPC handlers (`adapters/mcp/gateway.go`,
  `adapters/grpc/adapter.go`, `adapters/webhook/adapter.go`),
  which is why they can map parse errors to protocol-level fail-closed responses
  (JSON-RPC error, `codes.Internal`, HTTP 400). CLI exposes `GovernCommand`
  and CodeExec exposes `GovernCode`, but parse errors still happen before the
  shared pipeline emits a decision record; embedding runtimes are responsible
  for translating those errors into their protocol response. The A2A preview
  adapter also exposes `GovernTask`, which maps malformed or unsupported
  requests to A2A-shaped unsupported responses without forwarding.

- **Agent ISOLATED/TERMINATED remains a substantive DENY**, not
  check_indeterminate: the trust check produced a valid result and the result
  is a policy state.

- **The declared non-enforcing row** is the only cell where the pipeline
  returns ALLOW after a required-check failure — and it carries the
  would-have-blocked `CheckFailure` context on the decision, audit event, and
  record, plus one structured warning log. It exists for ADR-047's explicit
  can_deny=false surfaces (e.g. informational webhooks); it is never selected
  implicitly.

- **Downstream tool error (PASS row)**: the MCP proxy now forwards allowed
  JSON-RPC requests itself and inspects upstream errors. CLI and CodeExec now
  forward through configured executors after an allow decision. Other adapters
  still delegate forwarding to the surrounding runtime (gRPC interceptor chain,
  webhook `Handler()`). The governance
  decision is emitted before forwarding happens, so downstream 5xx or non-zero
  exit does not retroactively change the action in the audit event.

## 3. `FailClosedTransports` Defaults

`PipelineConfig.FailClosedTransports` is `nil` by default, which means Boundary
applies `DefaultFailClosedTransports` — all seven transports (`mcp`,
`managed_agents`, `cli`, `code_exec`, `grpc`, `a2a`, `webhook`). Every
transport the pipeline serves is execution-capable, so the default posture is
enforce everywhere.

A populated (non-empty) list overrides the default: the listed transports
enforce; any transport left out is declared non-enforcing (ADR-047
can_deny=false) and records would-have-blocked context on check failure.

A non-nil **empty** list is invalid — `PipelineConfig.Validate()` returns
`ErrEmptyFailClosedList` and `NewPipeline` records it so every `Evaluate`
returns `check_indeterminate`/`missing_config` rather than silently opting
every transport out of enforcement. There is no fail-open-everywhere opt-out;
emergency bypass would require an explicit, time-bounded, identity-attributed,
audited break-glass mechanism, which this package does not provide.

| Transport | Default | Rationale |
|---|---|---|
| `TransportMCP` | **enforcing** | Model-facing tool surface; silently allowing on a check outage means the governance layer degrades to the pre-Boundary state for agent tool calls. |
| `TransportManagedAgents` | **enforcing** | Hosted-agent tool confirmations are execution gates. A check outage must block by withholding confirmation rather than letting a tool proceed. |
| `TransportCLI` | **enforcing** | Command execution with parsed pipe-chain risk classification (`adapters/cli/classifier.go`). Silently allowing on a check outage drops the high-risk classification results. |
| `TransportCodeExec` | **enforcing** | Arbitrary code execution. A PolicyEval outage that allows-by-default here sidesteps the obfuscation detection categories (`adapters/codeexec/analyzer_python.go`, `analyzer_javascript.go`). |
| `TransportGRPC` | **enforcing** | Unary RPC interceptor (`adapters/grpc/adapter.go`). Internal service surface; enforcing matches the rest of the control plane's default posture. |
| `TransportA2A` | **enforcing** | Preview A2A governed lifecycle. Malformed requests, unknown mandatory fields, and check failures deny or return unsupported fail-closed responses. |
| `TransportWebhook` | **enforcing** | Execution-mode webhooks are an approval gate: `HandlerWithConfig` blocks any decision that is not `Allowed()`, so `check_indeterminate` gets HTTP 403 and is never forwarded. Informational mode is the documented can_deny=false exception — it never forwards regardless and records the verdict for an action that already happened. |

The pipeline exercises this map with `p.failClosed[req.Transport]`
(`pipeline.go`), which is O(1) and never errors on unknown keys.

## 4. DryRun Mode Interaction

DryRun is configured via `PipelineConfig.DryRun`. When
set, the deferred hook applies **after** audit:

1. Pipeline runs all four stages exactly as in production mode.
2. On `return`, the deferred function fires:
   - Stopwatch is stopped.
   - The deferred trust update runs (and can mark the decision
     `check_indeterminate`/`trust_update`).
   - Audit event is published with the **real** action and the `Check`
     context when present.
   - If `dryRun == true` and `decision.Action == "deny"` or
     `"check_indeterminate"`:
     - `decision.DryRun = true`
     - `decision.Reason = "DRY-RUN would deny: <original>"` or
       `"DRY-RUN would block: <original>"` (preserves the
       original reason under a prefix so callers can still reason about why
       the block would have fired).
     - `decision.Action = "allow"` (caller sees allow).

**Implications:**

- Audit logs in dry-run mode contain the ground-truth decision, including
  `check_indeterminate` outcomes and their check context. They are the
  source of truth for "what would governance have blocked if dry-run were
  off?".
- Callers in dry-run mode see the rewritten action. Any SLO measurement that
  reads `decision.Action` from the caller side will under-count blocks; any
  measurement that reads from the audit stream will count correctly.
- DryRun only rewrites `deny`/`check_indeterminate` → `allow`. Actions like
  `escalate`, `warn`,
  and `require_approval` are not touched. This matches
  the semantics of "what would have blocked?" — non-terminal decisions
  would not have blocked.
- DryRun is an explicitly non-enforcing mode. A check failure never selects
  it implicitly, and enabling it does not make an enforcing transport
  non-enforcing in the audit record.
- DryRun does **not** short-circuit any stage. All four stages still run, so
  dry-run has the same latency profile as production.

## 5. Fault-Injection Test Coverage

The ADR-047 failure matrix is covered by table-driven behavioral tests in
`governance/pipeline_indeterminate_test.go`:

| Area | Coverage |
|---|---|
| Every failure category × every transport | `TestPipeline_CheckIndeterminate_FailureMatrix` — unavailable, timeout, canceled, panic (evaluator/trust lookup/trust update), nil evaluator result, unknown evaluator action (`invalid_result`), stale snapshot (`CheckError`), missing identity, trust lookup/update errors — each across all seven transports, asserting blocked action, `Check` fields, recorded audit context, and zero downstream execution |
| Webhook execution mode | `TestPipeline_CheckIndeterminate_WebhookExecutionBlocksByDefault`; handler level: `TestHandlerWithConfig_Execution_CheckIndeterminateDoesNotForward`, `TestHandlerWithConfig_Informational_CheckIndeterminateStillRecords` (`adapters/webhook/adapter_test.go`) |
| Non-enforcing transport | `TestPipeline_CheckIndeterminate_NonEnforcingTransport_RecordsWouldHaveBlocked`, `TestPipeline_EvaluatorError_NonEnforcingTransport_Allows` |
| Empty fail-closed list | `TestPipeline_Config_EmptyFailClosedListRejected`, `TestPipeline_FailClosedTransports_ExplicitEmptySliceIsConfigError` |
| Configured transport combinations | `TestPipeline_Config_ValidatePopulatedListAccepted` plus the per-list rows in the failclosed/evaluator table tests |
| Nil auditor + RequireAudit | `TestPipeline_RequireAudit_NilAuditorIsConfigError` |
| Audit publisher outage/recovery | `TestPipeline_AuditDeliveryFailure_ExposesDegraded`, `TestPipeline_AuditPublisherPanic_DegradedNotFatal` |
| `Degraded()` flip/clear | `TestPipeline_AuditDeliveryFailure_ExposesDegraded` (flips on failure, clears on the next successful publish, failure counter monotonic) |
| Dry-run would-have-blocked | `TestPipeline_DryRun_CheckIndeterminate_RecordsWouldHaveBlocked`, `TestPipeline_DryRun_EnforcingEvaluatorError_Rewritten` |
| Record context | `TestPipeline_CheckIndeterminate_RecordCarriesSafeContext` (request id, tenant/agent, transport, stage, class, category, request_hash; no secrets/raw arguments) |
| Interceptor error / invalid result | `TestPipeline_CheckIndeterminate_InterceptorError`, `TestPipeline_CheckIndeterminate_InvalidInterceptorResult` |
| Trust update failure preserving a prior verdict | `TestPipeline_CheckIndeterminate_TrustUpdateFailure_PreservesDenyVerdict` |

Existing tests that previously asserted `deny` for infrastructure failures now
assert `check_indeterminate` with the ADR-047 rationale inline
(`TestPipeline_TrustError_FailClosed`, `TestPipeline_InterceptorError`,
`TestPipeline_EvaluatorError_EnforcingTransport_CheckIndeterminate`,
`TestPipeline_RequireAgentID_CheckIndeterminateWhenMissing`,
`TestPipeline_TrustRecordError_EnforcingCheckIndeterminate`, the three
decision-mode tests, the CLI/CodeExec/A2A lifecycle tests in `tests/adapters`,
`TestKernelTrustTimeoutFailsClosed` and
`TestManagedAgentsAdapterParsesAndFailsClosed` in `tests/integration`).

## 6. Known Gaps and Recommendations

The A2A stub-level gap was remediated by the preview lifecycle adapter. A2A
still remains below production until live protocol conformance and deployment
bypass evidence are recorded.

- **A2A adapter is preview.** `docs/adapters/A2A_PROTOCOL_SNAPSHOT.md`
  documents the supported A2A JSON-RPC subset and Boundary preview envelope.
  The adapter now implements denial shaping, governed forwarding, response
  inspection, metadata attachment, and fail-closed handling for malformed
  requests, unknown mandatory fields, and pipeline errors. It is not a full
  A2A server and does not implement streaming, task resubscription, push
  notifications, full AgentCard negotiation, or multi-hop governance beyond
  the first Boundary-controlled hop.

- **[RESOLVED — ADR-047] Infrastructure failures were classified as policy
  denies.** Trust lookup, trust update, interceptor, and evaluator failures
  (plus panics and invalid results) now return `check_indeterminate` with a
  machine-readable category on enforcing transports, blocking execution
  without claiming a policy verdict. See §0.

- **[RESOLVED — ADR-047] Webhook failed open by default.**
  `TransportWebhook` was absent from `DefaultFailClosedTransports`, so an
  evaluator error allowed execution-mode webhooks to forward. Webhook is now
  in the default enforcing set; informational mode remains the documented
  non-enforcing exception.

- **[RESOLVED — ADR-047] An explicit empty `FailClosedTransports` silently
  opted every transport out.** The configuration is now rejected
  (`ErrEmptyFailClosedList` via `PipelineConfig.Validate()`); a pipeline
  built with it fails closed at the `config` stage.

- **[RESOLVED — ADR-047] Evaluator/trust panics propagated to the caller.**
  All four check call sites are panic-safe; a recovered panic classifies as
  `category: "panic"`.

- **[RESOLVED — ADR-047] Audit publisher errors were silently absorbed.**
  `CheckedAuditPublisher` reports delivery failure; the pipeline exposes
  `Degraded()`/`AuditFailures()` and logs once per failure. A publisher panic
  is recovered and counted the same way.

- **[RESOLVED in PRD-004R Phase 4] Adapter-level parse failure is now typed.**
  Every adapter's `ParseRequest` returns `*governance.ParseError` on
  failure (`governance/errors.go`). Callers use `errors.As` or the
  convenience helper `governance.IsParseError(err)` to detect parse
  failures and route them through whatever deny-equivalent behavior their
  runtime wants. The underlying cause is preserved via `Unwrap`.
  Protocol-specific surface (HTTP 400 for webhook, `codes.InvalidArgument` for
  gRPC) still differs by design — the three-way split documented in §2 is
  correct for those transports — but MCP/CLI/CodeExec/A2A callers can now
  handle parse failures uniformly via the typed error, instead of
  string-matching adapter-specific messages.

---

*Authored April 17, 2026 per PRD-004 "Transport Fail-Mode Matrix"; updated for
ADR-047 CHECK_INDETERMINATE (FUL-465). Document is an audit of existing
behavior in the Boundary repo at the time of writing. Line citations refer to
the present snapshot; future refactors must update this document alongside the
code change.*
