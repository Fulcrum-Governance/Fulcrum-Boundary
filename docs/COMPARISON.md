# Where Boundary Fits

Your agent is about to touch a real system. Several kinds of tooling stand
near that moment. They answer different questions, and most teams will run
more than one. This page places Boundary among them by shape.

The emerging name for this layer is **pre-action authorization**: deciding
whether a specific agent tool call may run, before it runs. Boundary's name
for its shape is the **action boundary** — in plain words, a deterministic
permission layer for routed agent tool calls that leaves a hash-verifiable
record of every verdict it returns.

One rule governs this page: comparisons are framed by **category, never by
vendor**. Boundary's public copy names no third-party tools (the
vendor-neutral copy rule in [docs/BOUNDARY_SPEC.md](./BOUNDARY_SPEC.md)).
Categories below are described by their common shape; individual tools vary,
and "typically" means exactly that.

## The Map

| Shape | What it does | The question it answers | What the shape typically does not give you |
|---|---|---|---|
| **Scanners** | Static analysis of MCP servers, tool manifests, and client configs, before deployment. | "Is this server trustworthy enough to wire in?" | A verdict on the call that is about to run. A clean scan is a pre-deploy fact; it decides nothing at runtime. |
| **Gateways / proxies** | Route, observe, and filter agent traffic; often infrastructure- or control-plane-shaped. | "Should this traffic pass, and what passed?" | An operator-verifiable per-decision record, a policy-as-code test lane, or decision replay. Authorization at the route is not the same as a re-checkable verdict per action. |
| **Guardrail / content libraries** | Classify prompts, outputs, and other content in-process, usually probabilistically. | "Does this content look unsafe?" | A deterministic verdict on an action, or evidence that survives outside the agent process. Content layer, not action layer. |
| **Authorization engines** | Evaluate policy when the integrator's code asks; mature policy-as-code testing. | "Given these attributes, is this permitted?" | The enforcement itself — the integrator builds the interception — and agent-route awareness: hash-verifiable per-decision records of agent tool calls. |
| **Action boundary (Boundary)** | Decides `allow / deny / warn / escalate / require-approval` per call, **before execution**, on the routed path; emits a hash-verifiable decision record; pairs the route with a policy-as-code test lane and decision replay. | "Should this specific call run right now — and can the verdict be checked afterward?" | Anything off the route. Boundary governs only routes forced through it; direct access to the same tool is a bypass unless deployment topology removes that path. |

These shapes compose. Scan a server before wiring it in. Keep the gateway
your platform team runs. Keep content checks where they earn their place.
Boundary's slot is the action moment: the per-call verdict, and the record it
leaves behind.

## The Verifiable Decision Loop

The differentiator of the action-boundary shape is not any single stage. It
is that the same decision is testable before enforcement, recorded at
enforcement, and checkable after enforcement. Boundary ships that loop end to
end:

1. **Test.** `boundary test` runs operator-authored policy-as-code cases
   against local policy bundles and exits non-zero on a verdict mismatch —
   built for CI ([docs/POLICY_TESTING.md](./POLICY_TESTING.md)). Local-only:
   it reports policy verdicts for routed request fixtures, and passing tests
   do not prove production route enforcement or bypass resistance.
2. **Enforce.** The pipeline returns `allow / deny / warn / escalate /
   require-approval` before the tool executes; an errored trust check or
   interceptor is a deny, not a pass. A dry-run mode evaluates and records
   the real verdict while returning the permissive outcome, so a policy can
   be tuned before it enforces ([ARCHITECTURE.md](../ARCHITECTURE.md)).
3. **Record.** Every governed verdict emits a structured decision record
   ([docs/DECISION_RECORDS.md](./DECISION_RECORDS.md)). Where configured, the
   record is receipt-grade — carrying request, policy-bundle, and decision
   hashes ([docs/RECEIPTS.md](./RECEIPTS.md)).
4. **Verify.** `boundary verify-record` recomputes the hashes from the
   record's own fields; an edited verdict fails recomputation. The hashes are
   unkeyed SHA-256 over canonical bytes — integrity, not authenticity, and
   not proof that a verdict was correct or that it was enforced.
5. **Replay.** `boundary replay` re-evaluates the recorded request against
   the recorded policy bundle and fails closed on any mismatch in the
   decision-defining fields. It reproduces the decision, not enforcement.

The loop runs locally from a single binary. The standalone path delivers all
of it without connecting to a control plane or hosted service
([docs/STANDALONE_VS_KERNEL.md](./STANDALONE_VS_KERNEL.md)), and there is no
model in the decision path: verdicts come from policy evaluation over the
proposed action — deterministic code, not probabilistic classification.

## The Honest Scope: Routed-Only

Boundary governs an action only when the route is forced through Boundary.
Direct shell, editor, filesystem, CI, SSH, or API paths outside Boundary are
not governed unless deployment topology removes that direct path. That
constraint is physics for every interception shape on this page — gateway,
in-process wrapper, or boundary — not a Boundary-specific weakness. The
difference is posture: Boundary states it up front and ships a
[route conformance checklist](./ROUTE_CONFORMANCE_CHECKLIST.md) for
confirming a route is actually forced before relying on its verdicts.
[LIMITATIONS.md](../LIMITATIONS.md) carries the constraint in full.

Maturity follows the same discipline. **MCP is the production route** — the
word "production" is reserved for it. **Command Boundary and Edit Boundary
are delivered previews**, governing routed command paths and routed edit
envelopes only. Every other adapter ships as a labeled preview
([docs/ADAPTER_READINESS_MATRIX.md](./ADAPTER_READINESS_MATRIX.md)).

## What This Page Does Not Claim

- It does not claim that any specific tool lacks any capability. Categories
  are described by their common shape; individual tools vary.
- It does not claim Boundary replaces the other shapes. Boundary does not
  scan servers for trustworthiness, does not classify content, and is not a
  general-purpose authorization engine for non-agent traffic. The layers
  compose.
- It does not claim decision records are signed, or that a record proves the
  action was blocked. Records are hash-verifiable: tampering after emission
  is detectable by recomputation — integrity, not authenticity.
- It does not claim protection for non-routed paths, production status for
  any surface other than the MCP route, or that passing policy tests prove
  production route enforcement.

## Related

- [docs/CLAIMS_LEDGER.md](./CLAIMS_LEDGER.md) — every public claim, its
  status, and its test and doc evidence paths.
- [LIMITATIONS.md](../LIMITATIONS.md) — the routed-only constraint and
  per-surface limits in full.
- [docs/GOVERN_MCP_SERVER.md](./GOVERN_MCP_SERVER.md) — put Boundary in front
  of an MCP client, trigger a denial, read the record, uninstall.

## Verify It Yourself

The same discipline that gates the README gates this page: from a source
checkout, `go test ./claims/...` runs the claims ledger and the
public-language lint over the public docs — including this file — and CI
fails the build if the words outrun the evidence. Every delivered claim links
a test that runs in CI — clone it and run `make release-check` yourself.
