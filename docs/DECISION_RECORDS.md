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

Records declare `schema_version` and the current value is `"1"`
(`DecisionRecordSchemaVersion` in `governance/receipt_schema.go`).
`boundary verify-record` rejects any other value. When the record shape changes
in a breaking way, the schema version is incremented and this reference is
updated in the same change.

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
  "boundary_version": "0.7.0",
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
| `schema_version` | string | Required | Record schema version. Constant `"1"` in this release. |
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

### Field provenance

The record is assembled from an internal `AuditEvent`, which the pipeline
populates from the live governed request and the pipeline's decision. The
content fields above come from that evaluation. The two captured-at-startup
fields — `policy_bundle_hash` and `boundary_build_digest` — come from pipeline
configuration; `request_hash` is computed per request. This is why a record can
carry a `policy_bundle_hash` that reflects the policy bundle the pipeline was
configured with rather than a per-request value.

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
- **`upstream_called=false` is an adapter self-report, not part of the record.**
  `DecisionRecordV1` has no `upstream_called` field. Flags such as
  `upstream_called` and `executed` live on adapter and demo result structures and
  are set by the adapter from its own control flow. They are a component
  reporting on itself, not an independently observed network fact, and nothing in
  the record corroborates them. Treat `upstream_called=false` as a self-attested
  adapter signal.

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

## Verification

To check a stored record against its request, policy bundle, and build digest —
and to understand exactly what the request, policy-bundle, and decision hashes
cover — use `boundary verify-record` as documented in
[`docs/RECEIPTS.md`](RECEIPTS.md).
