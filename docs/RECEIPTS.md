# Receipt-Grade Decision Records

Fulcrum Boundary decision records are **receipt-grade** when they carry stable
hashes for the governed request, the policy bundle, and the emitted decision.
These hashes let you detect after-the-fact tampering by recomputing them with
`boundary verify-record`. The signature fields are schema-supported but optional;
unsigned records can still be checked against their hashes.

This page documents the **receipt-grade record** (Tier B, `BND-CLAIM-005`): the
hashes, what each hash covers, the `boundary verify-record` walkthrough, and the
tamper-detection behavior.

The record's content fields — the verdict, reason, decision mode, matched rule,
and identity/context fields — and the full field reference live in
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md), which documents the structured
decision record (Tier A, `BND-CLAIM-002`). The record type is the same
(`DecisionRecordV1`); the receipt-grade tier is the hash-bearing subset of those
fields. When you inspect a record, the Tier A page tells you which record you are
looking at, field by field; this page tells you what its hashes prove and what
they do not.

A committed, inspectable example record lives at
[`docs/examples/decision-record.example.json`](examples/decision-record.example.json).

## The three hashes

All hashes are SHA-256, lowercase hex, prefixed `sha256:`.

| Field | What it covers |
| --- | --- |
| `request_hash` | The canonical governed request. At verify time, `boundary verify-record --request request.json` round-trips the supplied file through canonical JSON before hashing, so key ordering and whitespace do not change the digest; it matches when the file is the canonical JSON of the request that was governed. |
| `policy_bundle_hash` | The canonical policy bundle. Each `.yaml`/`.yml` file in the policy directory is normalized from YAML to canonical JSON; the normalized documents are sorted and hashed together. File modification time, directory order, and file metadata are excluded, and symlinks and non-YAML files are skipped. |
| `decision_hash` | The record itself. Computed over the record's canonical JSON with `record_id`, `decision_hash`, `signature`, and `signature_key_id` blanked first, so the hash covers the record's content fields and is self-excluding and signature-excluding. `record_id` is then derived from this hash. |

`raw_shape_hash` is a related hash used on parse rejections. Unlike the three
hashes above, it is taken directly over the trimmed raw input bytes rather than
over canonical JSON. See the parse-rejection note in
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

## Verification

Use `boundary verify-record` against a single JSON decision record (one
`DecisionRecordV1` object, not a multi-record log):

```bash
boundary verify-record \
  --request request.json \
  --policies examples/mcp-postgres-gateway/policies \
  --binary-digest sha256:<build-digest> \
  record.json
```

The command reads the record and runs these checks **in order**, stopping at the
first mismatch:

1. `schema_version` must equal `"1"`.
2. `request_hash` — only when `--request` is set: recompute the canonical
   request hash from the supplied file and compare it to `request_hash`.
3. `policy_bundle_hash` — only when `--policies` is set: recompute the policy
   bundle hash from the directory and compare it to `policy_bundle_hash`.
4. `boundary_build_digest` — only when `--binary-digest` is set: compare the
   supplied digest to `boundary_build_digest`.
5. `decision_hash` — always: recompute the canonical record hash and compare it
   to `decision_hash`.

On success the command prints `record verification: ok` and the `record_id` and
exits `0`. Any mismatch prints `record verification failed: <which> mismatch`
and exits `1`.

The example record at
[`docs/examples/decision-record.example.json`](examples/decision-record.example.json)
can be checked with no cross-check flags:

```bash
boundary verify-record docs/examples/decision-record.example.json
```

With no `--request`, `--policies`, or `--binary-digest`, verification confirms
only `schema_version` and `decision_hash` self-consistency — that the record has
not been altered since emission. It does not, by itself, bind the record to the
request, the policy bundle, or the build that actually ran; supplying the three
flags is what adds those bindings.

## Tamper detection

Each check is what trips when the corresponding input is altered:

- Edit any content field of the record (for example, change `action`, `reason`,
  `tool`, or `trust_score`) and the recomputed `decision_hash` no longer matches
  the stored one, so check 5 fails. Edits to `record_id`, `decision_hash`,
  `signature`, or `signature_key_id` are not caught by check 5 because those
  four fields are blanked before hashing; `record_id` is display-only and is not
  compared against anything.
- Substitute a different request body via `--request` and check 2 fails.
- Change or replace any policy YAML in `--policies` (content, not file metadata
  or order) and check 3 fails.
- Verify against a different build digest and check 4 fails.

## Hash inputs

Policy hashes are computed from YAML content after YAML-to-JSON normalization.
File modification time, directory order, and file metadata are not part of the
hash, and symlinks and non-YAML files are skipped — a policy delivered by a
symlink or a non-YAML mechanism is outside the bundle hash. Request hashes are
computed from canonical JSON, so key ordering does not change the digest.

Malformed requests that cannot enter pipeline evaluation emit
`event_type=parse_rejected` records with `raw_shape_hash`. These records show
that Boundary observed and rejected an input shape even though no governed tool
request was built.

## Signature fields

The schema includes `signature` and `signature_key_id` fields for operators that
attach their own signing layer. These fields are empty by default; Boundary's
default decision-record path does not sign records. `boundary verify-record`
does **not** verify signatures — it validates the stable hashes that make
tampering detectable. Signing is an operator-attached layer, not a shipped
default.

## What this does not prove

These limits are load-bearing. State them whenever the receipt model is
described. The content-layer limits are in
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

- **The hashes are not cryptographic proof of a verdict.** They are unkeyed
  SHA-256 over canonical bytes. They show that a record is internally consistent
  and unmodified since emission, and that it corresponds to a given request,
  policy bundle, and build — they do not prove the verdict was correct, and they
  do not cryptographically attest who produced it. Anyone who recomputes a record
  from altered inputs produces a new, internally valid `decision_hash`: integrity
  is not authenticity. Authenticity requires the optional signature layer, which
  is off by default and not checked by `boundary verify-record`.
- **A receipt is not runtime enforcement.** A record that an action was decided
  `deny` is not evidence that execution was prevented. Enforcement holds only for
  routes forced through Boundary; direct access to the same tool is a bypass the
  record cannot see. The record is a decision artifact, not an execution-control
  proof.
- **`upstream_called=false` is an adapter self-report, not part of the record.**
  `DecisionRecordV1` has no `upstream_called` field. Flags such as
  `upstream_called` and `executed` live on adapter and demo result structures and
  are set by the adapter from its own control flow. They are a component
  reporting on itself, not an independently observed network fact; nothing in the
  hashed record corroborates them. Treat `upstream_called=false` as a
  self-attested adapter signal, not a verifiable property of the record.
- **Verification with no flags is weak.** Without `--request`, `--policies`, and
  `--binary-digest`, `boundary verify-record` confirms only `schema_version` and
  `decision_hash` self-consistency. Binding the record to the actual request,
  policy bundle, and build requires all three flags.
- **Hash scope exclusions can matter.** `policy_bundle_hash` ignores file
  modification time, directory order, and metadata, and skips symlinks and
  non-YAML files. `decision_hash` excludes `record_id`, `decision_hash`,
  `signature`, and `signature_key_id`, so changes to those four are not caught by
  the decision hash.
