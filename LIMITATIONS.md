# Limitations

Fulcrum Boundary governs an action only when the route is forced through
Boundary. Direct shell, editor, filesystem, CI, SSH, or API paths outside
Boundary are not governed unless deployment topology removes that direct path.
This is a property of where Boundary sits, not of how common the interception
primitive is: a governed route still does not govern a path that does not pass
through it. Nearly every limitation below follows from this routed-only
constraint.

This page is a summary. The authoritative, per-surface status lives in
[`docs/RELEASE_TRUTH_PUBLIC.md`](docs/RELEASE_TRUTH_PUBLIC.md),
[`docs/ADAPTER_READINESS_MATRIX.md`](docs/ADAPTER_READINESS_MATRIX.md), and the
[`claims/`](claims/) ledger.

## Surface maturity

- MCP is the first and only production route, and only for MCP requests forced
  through Boundary.
- Command Boundary and Edit Boundary are delivered previews for routed command
  paths and routed edit envelopes. They do not control direct shell access or
  direct file writes.
- Secure GitHub is a preview profile, not production. It denies the tested
  write-after-taint fixture before upstream mutation; the fixture proof and the
  opt-in live conformance harness do not close deployment bypasses.
- The remaining adapters (A2A, CLI, CodeExec, gRPC, Managed Agents, Webhook)
  ship as labeled previews.

The pre-execution action boundary itself — decide allow/deny on a routed tool
call before it runs — is now a commodity capability available in general-purpose
agent platforms and cloud gateways. Boundary does not claim the boundary
primitive as a differentiator. What this repo adds on a governed route is the
operator-verifiable artifact of the decision: a hash-checkable decision record
(receipt-grade when it carries the request, policy-bundle, and decision hashes),
a policy-as-code test lane, and decision replay. Pre-execution interception that
is not coupled to a re-checkable per-decision record is not what this repo is
about.

## Decision records

Decision records are receipt-grade when they carry the request, policy-bundle,
and decision hashes: tampering after emission is detectable by recomputation
with `boundary verify-record`. These hashes are unkeyed SHA-256 over canonical
bytes — integrity, not authenticity. They are not cryptographic proof that a
verdict was correct or that it was enforced. Optional Ed25519 signing (off by
default) adds authorship for holders who manage keys —
`boundary verify-record --verify-signature` checks it and fails closed — but a
signature proves only who signed the record, not the verdict, execution, or
key custody (see [`docs/SIGNING.md`](docs/SIGNING.md)).
`upstream_called=false` / `executed=false` are adapter self-reports, not fields
of the hashed record. See [`docs/RECEIPTS.md`](docs/RECEIPTS.md) and
[`docs/DECISION_RECORDS.md`](docs/DECISION_RECORDS.md).

## Claude Code hook

The `boundary hook pretooluse` route decides a Claude Code tool call only when
the hook is wired to intercept that call. The `matcher` in `settings.json`, or
the plugin manifest at `hooks/hooks.json`, is the whole of the boundary: a tool
that is not listed, an MCP tool, a tool a subagent or subprocess runs on its own,
and any shell used outside Claude Code are bypasses and are not governed. Only
`Bash`/`Shell` and `Edit`/`Write`/`MultiEdit`/`NotebookEdit` are routed today;
every other tool is allowed silently and leaves no record, because nothing was
decided. The classifiers behind both routes are delivered previews, so treat
their verdicts as preview-grade.

The host can also switch the route off above the project. Claude Code's
enterprise managed settings carry `disableAllHooks`, under which no `PreToolUse`
hook runs on that machine and nothing reaches Boundary at all, and
`allowManagedHooksOnly`, under which a hook wired outside the managed file may
not run. `boundary hook doctor` reports both when it can read that file, and
lists the bypasses it knows about rather than claiming it is the only thing
routing tool calls.

Hook decision records carry the record limits above: hash-verifiable for
integrity, not authenticity, and not proof the verdict was correct. They are also
not evidence about upstream side effects. `execution_claim` reports
`upstream_called=false` / `executed=false` because this route decides before
execution and never runs the command or writes the file — a self-report about the
hook, not corroboration that nothing else ran. A decision that cannot be recorded
is escalated to the user rather than allowed unrecorded, which is a posture, not
a guarantee that a record exists for every action the agent took.

The governance control surfaces both routes refuse to write are a set of path
shapes, not an inventory of every way governance could be disabled. A hook wired
from a path not in the list, a settings file outside those shapes, or a path an
interpreter builds at runtime is not matched. Command-route matching is
shape-only and does not analyze argument position, so a control surface named as
the source of a write is refused too. See
[`docs/integrations/CLAUDE_CODE_HOOK.md`](docs/integrations/CLAUDE_CODE_HOOK.md).

## Proof-receipt and checker correspondence

Boundary does not emit `proved` decisions. The proof-receipt sidecar
(proof-receipt-v0.1) is a wired witness for the budget and static-privilege
invariants. It is a checker-validated artifact, bound to the decision record by
decision_hash. The trust-termination behavior is expressed as a circuit-transition
consistency check (not a per-decision termination proof). The equilibrium
correspondence — a machine-checked equilibrium analysis (Nash equilibrium and
price-of-anarchy bound) — is a design constraint held upstream in Fulcrum-Proofs,
not a runtime certificate emitted by this repo. This repo emits structured and
receipt-grade decision records verified by recomputation (`boundary verify-record`);
attaching the proof-receipt sidecar does not change the decision mode. See
[`docs/PROOF_BOUNDARY.md`](docs/PROOF_BOUNDARY.md) and
[`docs/PROOF_RECEIPT.md`](docs/PROOF_RECEIPT.md).

## SQL classification

The bundled Postgres support is an AST classifier that labels statements before
policy evaluation. It is not a general SQL firewall and does not prevent all SQL
injection; dialect-specific syntax and semantic analysis are outside it unless a
test explicitly covers them.

The classifier links `pg_query_go` via cgo. Static (`CGO_ENABLED=0`) builds —
the prebuilt `_static-nocgo` release archives, the Homebrew cask, and the
container image — do not carry it: routed SQL classifies as `UNKNOWN` and the
Postgres guard denies it fail-closed instead of classifying it. The static
build never allows SQL the cgo build would deny; use a `_cgo` release archive
or a cgo source build for class-based SQL policy. See
[`docs/INSTALL.md`](docs/INSTALL.md).

## Evidence and diagnostics

`boundary doctor`, `boundary evidence bundle`, and `boundary evidence verify`
are local-only utilities. A passing doctor or a verified evidence bundle does
not prove that every deployment route is protected or that no bytes moved
outside Boundary.
