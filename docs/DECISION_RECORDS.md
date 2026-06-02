# Decision Records

Every governed verdict produces a structured decision record. Fulcrum Boundary
emits one structured JSON decision record for every governed request, regardless
of outcome (`allow`, `deny`, `warn`, `escalate`, or `require_approval`), plus a
variant for inputs that are rejected before a governed request can be built.

This page is the versioned field reference for the **structured decision record**
(Tier A, `BND-CLAIM-002`). It is the record's content layer: the fields that
describe what Boundary decided and why.

A decision record becomes **receipt-grade** (Tier B, `BND-CLAIM-005`) when it
also carries the stable request, policy-bundle, and decision hashes that
`boundary verify-record` recomputes. Those hash fields, the verification
walkthrough, and the tamper-detection behavior are documented in
[`docs/RECEIPTS.md`](RECEIPTS.md). Same record type (`DecisionRecordV1`); the
receipt-grade tier is the hash-bearing subset of the fields below.

## The two-tier record model

| Tier | Claim | What it adds | Where it is documented |
| --- | --- | --- | --- |
| A — structured decision record | `BND-CLAIM-002` | The verdict, reason, decision mode, matched rule, and identity/context fields below. Emitted for every governed verdict. | This page. |
| B — receipt-grade record | `BND-CLAIM-005` | The `request_hash`, `policy_bundle_hash`, and `decision_hash` fields that make after-the-fact alteration detectable by recomputation via `boundary verify-record`. | [`docs/RECEIPTS.md`](RECEIPTS.md). |

When you inspect a record, the tier you are looking at depends on which fields
are populated. A Tier A record always carries the content fields; it is also a
Tier B record when the three stable hashes are present. The schema is the same
in both tiers — there is no separate "receipt" type.

## Schema version

Records declare `schema_version`. Two values exist, and both are supported:

- `"1"` — the original record shape, with no route-context fields
  (`DecisionRecordSchemaVersion` in `governance/receipt_schema.go`).
- `"2"` — a strictly additive evolution that appends the route-context fields
  (`adapter_id`, `route_id`, `topology_profile`, `execution_claim`)
  documented under [Route-context fields](#route-context-fields-schema_version-2)
  below (`DecisionRecordSchemaV2`).

V2 is a strict superset of V1: a V1 record is simply a V2 record without the
route-context fields. A record is emitted as `"2"` only when at least one
route-context field is populated; otherwise it is emitted as `"1"` and is
byte-for-byte identical to a pre-V2 record, so existing V1 records and their
`decision_hash` values remain valid unchanged. `boundary verify-record` accepts
`schema_version` of `"1"` or `"2"` and rejects any other value; the `decision_hash`
is recomputed over the same record fields for either version. When the record
shape changes in a way that is not strictly additive, the schema version is
incremented again and this reference is updated in the same change.

## Example

A committed, inspectable example lives at
[`docs/examples/decision-record.example.json`](examples/decision-record.example.json).
The record below is an illustrative `allow` outcome and is not a substitute for
the committed example:

```json
{
  "schema_version": "1",
  "event_type": "governance_decision",
  "record_id": "rec_a1b2c3d4e5f6",
  "timestamp": "2026-05-31T18:24:07Z",
  "boundary_version": "0.8.0",
  "adapter": "mcp",
  "agent_id": "demo-agent",
  "tenant_id": "demo",
  "trace_id": "trace-7f3a",
  "tool": "query",
  "action": "allow",
  "reason": "read-only SELECT permitted by policy",
  "decision_mode": "deterministic",
  "matched_rule": "allow-select",
  "policy_file": "postgres.yaml",
  "policy_bundle_hash": "sha256:<policy-bundle-hash>",
  "request_hash": "sha256:<canonical-request-hash>",
  "decision_hash": "sha256:<canonical-record-hash>",
  "trust_score": 1,
  "trust_state": "trusted"
}
```

## Field reference (schema_version 1)

Field names and Go types are taken from `DecisionRecordV1`
(`governance/receipt_schema.go`). "Required" fields are always serialized;
"Optional" fields use `omitempty` and appear only when set. Fields marked
**(Tier B)** are the hash fields covered in [`docs/RECEIPTS.md`](RECEIPTS.md).

| JSON field | Type | Required / Optional | Meaning |
| --- | --- | --- | --- |
| `schema_version` | string | Required | Record schema version: `"1"` (no route-context) or `"2"` (route-context present). |
| `event_type` | string | Optional | `governance_decision` for a governed verdict (default); `trust_transition` for a trust-state change; `parse_rejected` for an input rejected before a governed request was built. |
| `record_id` | string | Required | Derived identifier: `rec_` plus the first 12 hex characters of `decision_hash`. It is derived from the record, not an independent input. |
| `timestamp` | string (RFC 3339) | Required | UTC time the record was emitted. |
| `boundary_version` | string | Optional | Build/version string of the Boundary that emitted the record. |
| `boundary_build_digest` | string | Optional | Build digest of the Boundary binary, when one is supplied. Checked by `boundary verify-record --binary-digest`. **(Tier B input.)** |
| `adapter` | string | Optional | Transport that carried the request: `mcp`, `cli`, `code_exec`, `grpc`, `a2a`, `webhook`, or `managed_agents`. |
| `agent_id` | string | Optional | Agent identity, when supplied. |
| `tenant_id` | string | Optional | Tenant identity, when supplied. |
| `trace_id` | string | Optional | Caller-supplied trace correlation ID. |
| `tool` | string | Optional | Name of the tool being governed. |
| `action` | string | Required | The verdict: `allow`, `deny`, `warn`, `escalate`, or `require_approval`. |
| `reason` | string | Optional | Human-readable rationale for the verdict. |
| `decision_mode` | string | Optional | Epistemic label for how the verdict was reached. Boundary emits `deterministic` or `classified`. See the decision-mode note below. |
| `matched_rule` | string | Optional | The static policy rule that drove the verdict, when one matched. |
| `policy_file` | string | Optional | The YAML file that supplied the matched rule. |
| `policy_bundle_hash` | string | Optional | Stable hash of the canonical policy bundle. **(Tier B — [`docs/RECEIPTS.md`](RECEIPTS.md).)** |
| `request_hash` | string | Optional | Stable hash of the canonical governed request. **(Tier B — [`docs/RECEIPTS.md`](RECEIPTS.md).)** |
| `raw_shape_hash` | string | Optional | Hash of the raw input shape. Present on parse rejections, in place of `request_hash`, where no governed request was built. |
| `decision_hash` | string | Required | Stable hash of the record itself. **(Tier B — [`docs/RECEIPTS.md`](RECEIPTS.md).)** |
| `trust_score` | number | Required | Trust score for the request (`0.0` default; `0.5` while a trust state is `Evaluating`). |
| `trust_state` | string | Optional | Trust state string, e.g. `trusted`. |
| `signature` | string | Optional | Operator-attached signature. Empty by default; Boundary's default path does not sign records. See [`docs/RECEIPTS.md`](RECEIPTS.md). |
| `signature_key_id` | string | Optional | Key ID for an operator-attached signature, when one is present. |
| `adapter_id` | string | Optional | **(schema_version 2)** Name of the adapter that parsed and routed the request. Descriptive only. |
| `route_id` | string | Optional | **(schema_version 2)** The specific governed route the request traveled (transport plus tool). Descriptive only. |
| `topology_profile` | string | Optional | **(schema_version 2)** The named deployment posture asserted at emission. Asserted, not attested — the field does not verify that the running deployment matches the named posture. |
| `execution_claim` | object | Optional | **(schema_version 2)** The adapter's structured execution self-report (`upstream_called`, `executed`, and a `source` label). Self-report, not corroborated — recording it does not make it independently verified. |

### Field provenance

The record is assembled from an internal `AuditEvent`, which the pipeline
populates from the live governed request and the pipeline's decision. The
content fields above come from that evaluation. The two captured-at-startup
fields — `policy_bundle_hash` and `boundary_build_digest` — come from pipeline
configuration; `request_hash` is computed per request. This is why a record can
carry a `policy_bundle_hash` that reflects the policy bundle the pipeline was
configured with rather than a per-request value.

### Route-context fields (schema_version 2)

A `schema_version "2"` record adds four route-context fields that describe the
governed route the request traveled. They are structured forms of context the
adapter already knows. They are **descriptive context, not attestation**: they
are covered by `decision_hash` (so altering one is detectable by
`boundary verify-record`), but recording them does not make the deployment
posture verified or the adapter self-report independently corroborated.

| Field | What it records | What it does not do |
| --- | --- | --- |
| `adapter_id` | The adapter that parsed and routed the request. | It describes the adapter; it does not prove the adapter was the only path to the tool. |
| `route_id` | The specific governed route (transport plus tool). | It names the routed path; it does not prove no unrouted path to the same tool exists. |
| `topology_profile` | The named deployment posture asserted at emission. | **Asserted, not attested.** The field does not verify that the running deployment matches the named posture; a record can assert a posture the deployment does not actually have. |
| `execution_claim` | The adapter's structured execution self-report — `upstream_called`, `executed`, and a `source` label. | **Self-report, not corroborated.** Recording it explicitly does not make it independently verified; nothing in the hashed record corroborates it. It is the same self-attested adapter signal as a loose `upstream_called` flag, now carried in the record. |

A pipeline emits `execution_claim` as absent for the records it writes itself,
because the pipeline decides **before** execution — a pre-execution record makes
no execution self-report. Adapters or demos that proxy an upstream call attach
`execution_claim` from their own control flow, and it remains a self-report.

A committed, inspectable V2 example with every route-context field populated
lives at
[`docs/examples/decision-record-v2.example.json`](examples/decision-record-v2.example.json)
and verifies with `boundary verify-record docs/examples/decision-record-v2.example.json`.

```json
{
  "schema_version": "2",
  "action": "deny",
  "reason": "write-after-taint",
  "adapter_id": "securegithub",
  "route_id": "mcp:github.create_or_update_file",
  "topology_profile": "single-tenant-routed",
  "execution_claim": { "upstream_called": false, "executed": false, "source": "securegithub" },
  "decision_hash": "sha256:<canonical-record-hash>"
}
```

### Decision-mode note

`decision_mode` records how the verdict was reached, not whether it was correct.
Boundary emits only `deterministic` (static-policy outcomes) and `classified`.
Boundary does **not** emit `proved`; the `proved` and `human_approved` modes
originate upstream in the Fulcrum family and must not be implied by a Boundary
record. See [`docs/PROOF_BOUNDARY.md`](PROOF_BOUNDARY.md).

## Parse-rejection records

When a payload cannot be parsed into a governed request, Boundary emits a record
with `event_type: parse_rejected`, `action: deny`, `decision_mode:
deterministic`, and a `raw_shape_hash` (a hash of the trimmed raw bytes) in
place of `request_hash`. This record shows that Boundary observed and rejected an
input shape even though no governed tool request was built. It is not evidence
that a downstream tool was reached or not reached.

## What this does not prove

The structured decision record is a record of what Boundary **decided**. Read it
with these limits in mind. The hash-bearing receipt tier and its own limits are
covered in [`docs/RECEIPTS.md`](RECEIPTS.md).

- **It is not cryptographic proof of a verdict.** A decision record states the
  verdict and reason; on its own it does not cryptographically attest that the
  verdict was correct or that any particular party produced it. The integrity
  and tamper-detection properties — and their limits — belong to the
  receipt-grade tier in [`docs/RECEIPTS.md`](RECEIPTS.md), not to the content
  fields here.
- **It is not runtime enforcement.** A record that an action was decided `deny`
  is not evidence that execution was prevented. Enforcement holds only for routes
  that are forced through Boundary; direct access to the same tool is a bypass
  that a record cannot see. The record is a decision artifact, not proof that the
  action was blocked at runtime.
- **`upstream_called=false` is an adapter self-report.** Historically these
  flags lived only on adapter and demo result structures. A `schema_version "2"`
  record can now carry them inside the `execution_claim` field, but that does not
  change what they are: a component reporting on itself, not an independently
  observed network fact, with nothing in the record corroborating them. Treat
  `execution_claim.upstream_called=false` as a self-attested adapter signal, the
  same as the loose flag.
- **Route-context is descriptive, not attestation.** `adapter_id`, `route_id`,
  and `topology_profile` describe the route a request traveled and the posture
  asserted at emission. They do not verify that the deployment matches the
  asserted `topology_profile`, and they do not prove that no unrouted path to the
  same tool exists. Recording route-context extends tamper-detection to those
  fields; it does not add attestation or authenticity.

## Locating a written record

Every record-emitting command prints a uniform pair of lines so the record is
easy to find and hand to `boundary verify-record`:

- `decision record id: rec_...` — the record's `record_id`. Always an
  identifier, never a path. Printed by the proof-lane demos, `boundary redteam`,
  and the command/edit boundary surfaces.
- `decision record path: <path>` — the on-disk location a record (or `.jsonl`
  record log) was written to. Always a filesystem path, never an id. Printed only
  when a file was actually written; an in-memory-only run prints no path line.

The two proof-lane demos write their record file under `--out` at a predictable,
per-demo `*-artifacts/decision-records.jsonl` location, so the
"find -> verify" step is copy-paste:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
# -> decision record path: <dir>/github-lethal-trifecta-artifacts/decision-records.jsonl
boundary demo command-secret-exfil --out demo.txt
# -> decision record path: <dir>/command-secret-exfil-artifacts/decision-records.jsonl
```

A `.jsonl` file holds one record per line. `boundary verify-record` takes a
single record object, so split out one line before verifying (see
[`docs/examples/README.md`](examples/README.md) for the full walkthrough). The
printed `decision record path:` is local-only: it names a file Boundary wrote, it
is not a network location and it does not prove the action was enforced.

## Reading a record

To read a stored record without verifying it, use `boundary explain
<record.json>`. It prints a human-readable account of the decision-defining
fields, the route-context fields for a `schema_version "2"` record, each stable
hash and what it covers, and a fixed limitation footer; `--json` emits a stable
`boundary.explain.v1` object. `explain` is read-only: it renders the record and
does **not** recompute any hash, so it does not verify the record and does not
prove the verdict was correct or that the action was enforced. Verification is a
separate command — see below.

```bash
boundary explain docs/examples/decision-record.example.json
boundary explain --json docs/examples/decision-record-v2.example.json
```

## Verification

To check a stored record against its request, policy bundle, and build digest —
and to understand exactly what the request, policy-bundle, and decision hashes
cover — use `boundary verify-record` as documented in
[`docs/RECEIPTS.md`](RECEIPTS.md).

## Replaying a record

To reproduce a recorded *decision* locally, use `boundary replay <record.json>
--request <request.json> --policies <dir>`. A record carries `request_hash` but
not the request body, so replay takes the record plus the canonical
`GovernanceRequest` JSON that was recorded and the operator's policy directory.
Replay recomputes `request_hash` from the supplied request (confirming it is
replaying *the recorded request*), recomputes `policy_bundle_hash` from
`--policies` when the record carries one (confirming it is replaying against *the
recorded policy bundle*), rebuilds the request, re-evaluates it against the
supplied static policy bundle in a hermetic in-process configuration with no
audit side effects, and compares
the decision-defining fields — `action`, `reason`, `decision_mode`,
`matched_rule`, and `policy_file` where present, **not `action` alone**. It exits
non-zero on any decision-field mismatch, a `request_hash` mismatch, or a
`policy_bundle_hash` mismatch; `--json` emits a stable `boundary.replay.v1`
object.

```bash
boundary replay docs/examples/decision-record-replay.example.json \
  --request docs/examples/replay-request.example.json \
  --policies docs/examples/replay-policies/
```

`replay` reproduces the *decision*, not enforcement. A reproduced `deny` is
**not** evidence the action was blocked; replay does **not** prove that no
upstream bytes moved; it reproduces the decision only for routed requests; and a
match does **not** prove the original verdict was correct — only that the same
inputs reproduce the same decision. Replay re-evaluates the supplied static
policy bundle only: a decision that originated from an interceptor (for example
the Postgres AST classifier) or from trust state — not from the policy bundle —
does **not** reproduce, and replay reports a mismatch rather than a false
reproduction. No upstream tool is called and nothing is mutated.
