# Boundary Examples — Inspectable, Fixture-Safe Artifacts

This directory holds committed example artifacts you can inspect and verify
yourself on a clean checkout. Nothing here needs credentials, a cloud account,
or any live mutation. Every artifact is a genuine `boundary` tool output, not a
hand-authored sample.

| File | What it is | How it was produced |
|---|---|---|
| [`decision-record.example.json`](./decision-record.example.json) | One `DecisionRecordV1` object (schema `"1"`) — the input `boundary verify-record` consumes. | `boundary demo github-lethal-trifecta --json --out <dir>/demo.json`, first record of the emitted `decision-records.jsonl`. |
| [`evidence-manifest.example.json`](./evidence-manifest.example.json) | An **excerpt** of a real evidence-bundle `manifest.json` (schema `boundary.evidence_bundle.v1`). | `boundary evidence bundle --include-demo --out boundary-evidence`, then trimmed (see "About the manifest excerpt"). |

The two files come from **two separate subsystems**. The decision record is a
receipt-grade artifact emitted by the governance pipeline. The evidence manifest
indexes fixture-safe utility outputs (`version`, `selftest`, `doctor`, the
action-boundary demo). An evidence bundle does **not** contain decision records
(`parsed_records: 0`), so it is not an input to `verify-record`. The two are
shown together only to make both shapes inspectable in one place.

## Verify the decision record yourself

The example record is self-verifying: it carries the canonical `decision_hash`
that `boundary verify-record` recomputes and compares. Run this from the repo
root (use `./bin/boundary` after `make build`, or `boundary` if installed):

```bash
./bin/boundary verify-record docs/examples/decision-record.example.json
```

Real output (exit 0):

```text
record verification: ok
record_id: rec_4b68b9d63c69
```

That is a genuine PASS against the committed file. `verify-record` checks two
things intrinsic to the record with no extra inputs: `schema_version == "1"`,
and that the recomputed `decision_hash` equals the stored one. Because both are
derived from the record's own content, the record is verifiable on its own — no
`request.json`, no policy directory, and no build digest are required for this
to pass.

### It actually catches tampering

`decision_hash` is recomputed over the record's content (with `record_id`,
`decision_hash`, and the signature fields blanked first), so editing any content
field is detected. For example, flipping `"action": "deny"` to `"allow"`:

```text
record verification failed: decision_hash mismatch: got sha256:4b68b9d6... want sha256:312c8a10...
```

The verifier exits non-zero (`1`) and names the mismatched check. This is why
the committed example must stay byte-for-byte as emitted: any edit to the record
content would (correctly) fail verification.

## Regenerate the record from the fixture-safe demos

Both proof-lane demos print a uniform record-location pair and write their
record file under `--out`, so the find -> verify step is copy-paste. Every
record-emitting command prints `decision record id: rec_...` and, when a file is
written, `decision record path: <path>` — the path is exactly the file
`verify-record` consumes.

### Lane 1 — github-lethal-trifecta

Run the flagship demo with `--out` so the workspace (and its decision records)
are retained instead of discarded:

```bash
./bin/boundary demo github-lethal-trifecta --json --out /tmp/bnd-demo/demo.json
# stdout prints:
#   decision record id: rec_...
#   decision record path: /tmp/bnd-demo/github-lethal-trifecta-artifacts/decision-records.jsonl
```

That JSONL holds two `DecisionRecordV1` objects, one per line (a routed-redteam
record and a Secure GitHub preview record). `verify-record` takes a single JSON
object, not a JSONL file, so split out one line before verifying:

```bash
sed -n '1p' /tmp/bnd-demo/github-lethal-trifecta-artifacts/decision-records.jsonl > /tmp/bnd-demo/record.json
./bin/boundary verify-record /tmp/bnd-demo/record.json
```

### Lane 2 — command-secret-exfil

The Command Boundary proof lane lands its single decision record under the same
`--out` convention, at a predictable `command-secret-exfil-artifacts` sibling:

```bash
./bin/boundary demo command-secret-exfil --out /tmp/bnd-demo/cmd.txt
# stdout prints:
#   decision record id: rec_...
#   decision record path: /tmp/bnd-demo/command-secret-exfil-artifacts/decision-records.jsonl
sed -n '1p' /tmp/bnd-demo/command-secret-exfil-artifacts/decision-records.jsonl > /tmp/bnd-demo/cmd-record.json
./bin/boundary verify-record /tmp/bnd-demo/cmd-record.json
```

The hashes in your records will differ from the committed example: the timestamp
and the per-run request hash are not fixed across runs. The verdict, matched
rule, and the fact that they self-verify are stable.

> Without `--out`, a demo prints its records to stdout only and persists no file
> — there is no `decision record path:` line because no file was written. Use
> `--out` to keep a file you can verify. (The github-lethal-trifecta demo also
> retains its workspace under `--dashboard`.)

## What the optional cross-check flags do (and why they do not pass here)

`verify-record` accepts three optional flags that bind a record to external
inputs. They are not satisfiable against this fixture record, and the honest
reason is that the demo record carries fixture placeholders, not real bound
values:

- `--binary-digest <digest>` — exact string match against the record's
  `boundary_build_digest`. This record carries the literal `fixture-only` (a
  fixture value, not the real build digest), so only
  `--binary-digest fixture-only` would match. Do not pass a real build digest
  here; it would not match the fixture record.
- `--policies <dir>` — recomputes a policy-bundle hash from a directory and
  compares it to the record's `policy_bundle_hash`. The Secure GitHub demo
  record stores the placeholder string `fixture-secure-github`, which can never
  equal a real directory hash, so this flag fails on the fixture record:

  ```text
  record verification failed: policy_bundle_hash mismatch: got fixture-secure-github want sha256:c22c4753...
  ```

- `--request <file>` — recomputes a request hash from a request body file and
  compares it to the record's `request_hash`. The demo does not export the raw
  governed-request JSON, and the request hash derives from per-run identifiers,
  so there is no committable `request.json` that matches. This flag is not
  satisfiable from the fixture demo artifacts.

Binding a record to its actual request, its actual policy bundle, and its actual
build requires a record whose hashes were computed from those real inputs (for
example, a record emitted on a routed MCP path with the committed policy set
alongside it). The shipped fixture demo records intentionally use placeholders,
so the reproducible, green path here is **bare self-verification** of the record
file, exactly as shown above.

## About the manifest excerpt

[`evidence-manifest.example.json`](./evidence-manifest.example.json) is a real
`boundary evidence bundle --include-demo` manifest with two edits, both noted in
its `_note` field: the `source`/`output` absolute paths were replaced with
placeholders (they are machine-specific on the box that generated them), and the
artifact list was trimmed to 4 of 8 entries. Every `sha256`, `size_bytes`, and
`schema_version` value shown is the genuine emitted value. To produce and verify
a full, unedited bundle yourself:

```bash
./bin/boundary evidence bundle --include-demo --out boundary-evidence
./bin/boundary evidence verify boundary-evidence
```

`evidence verify` recomputes every artifact hash and confirms the summary
references each one. See `../EVIDENCE_BUNDLE.md` and `../EVIDENCE_VERIFY.md` for
the full walkthrough.

## What a verified record does and does not show

A passing `verify-record` confirms the record is internally hash-consistent and
unmodified since it was emitted. Stated against the limits that matter:

- It is **not** cryptographic proof that the verdict was correct. The hashes are
  unkeyed SHA-256 over canonical bytes; they show integrity, not authenticity.
  An optional Ed25519 signature layer exists but is empty by default and is
  **not** checked by `verify-record`.
- It is **not** evidence that execution was prevented at runtime. The record
  shows Boundary *decided* `deny`; enforcement holds only for routes forced
  through Boundary, and a verified record does not by itself prove a route was
  enforced.
- `upstream_called` / `executed` flags are **not** part of the record.
  `DecisionRecordV1` has no `upstream_called` field; those flags live on demo and
  adapter result structs and are an adapter self-report, not a property the
  hashed record independently corroborates.
- `decision_mode` is `deterministic` here. Boundary does **not** emit `proved`
  decisions; a record does not carry formal-proof backing.
- Bare verification (no `--request`/`--policies`/`--binary-digest`) confirms only
  `schema_version` and `decision_hash` self-consistency. It does **not** confirm
  the record matches the request, policy bundle, or build that actually ran;
  that binding requires supplying those flags with real inputs.

For the full receipt model and field-by-field semantics, see `../RECEIPTS.md`
and `../DECISION_RECORDS.md`. The record schema is `../../schemas/decision-record.v1.json`.
