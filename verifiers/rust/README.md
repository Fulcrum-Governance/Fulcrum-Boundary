# Boundary decision-record verifier (Rust)

Verify a Fulcrum Boundary **decision record** with no Boundary code on the path.

A decision record is the hash-verifiable artifact Boundary emits when it decides
an action (allow / deny / warn / escalate / require-approval). This directory
ships a small, dependency-light Rust re-implementation of the one check that
makes that record self-checking: recomputing its `decision_hash`.

```bash
cargo build --manifest-path verifiers/rust/Cargo.toml
./verifiers/rust/target/debug/boundary-verify record.json
# or:
cargo run --manifest-path verifiers/rust/Cargo.toml -- record.json
```

Output is one line and an exit code:

```
record verification: ok                                          # exit 0
decision_hash mismatch: got sha256:… want sha256:…              # exit 1
```

Multiple files can be passed; the process exits 1 if any record fails.

## What it checks

The decision record is **record-scoped RFC 8785 / JCS conformant**: its bytes
are hashed in the [RFC 8785 JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785)
form, and the `decision_hash` is the SHA-256 of that canonical form, with four
fields neutralized first (`record_id` and `decision_hash` blanked to `""`,
`signature` and `signature_key_id` dropped) so the hash is self-excluding and
signature-excluding. (This RFC 8785 / JCS statement is scoped to the decision
record specifically; it is not a claim that Boundary as a whole is RFC 8785 / JCS standards-conformant.)

Because that canonical form is a published standard, the `decision_hash` is
reproducible by a stock RFC 8785 implementation. `boundary-verify` does exactly
that using the [`serde_jcs`](https://crates.io/crates/serde_jcs) crate — no
hand-rolled canonicalization:

1. Load the record JSON with `serde_json::Value`.
2. Blank `record_id` and `decision_hash` to `""`; drop `signature` and
   `signature_key_id`.
3. Canonicalize via `serde_jcs::to_vec` (RFC 8785 / JCS, ECMAScript shortest
   round-trip number formatting per §3.2.4).
4. SHA-256, prefix `sha256:`.
5. Compare to the record's stored `decision_hash`.

Both decision-record schema versions (`"1"` and the route-context superset
`"2"`) hash through this same path, so the verifier handles either without
special-casing.

This is the same computation Boundary's Go implementation performs in
[`governance/receipt.go`](../../governance/receipt.go) (`ComputeDecisionHash`).

## Integrity, not authenticity

Recomputing `decision_hash` is an **integrity** check: it detects whether the
covered fields of a record were altered after it was emitted. It is **not** an
**authenticity** check.

- The hash is **unkeyed**. Anyone can edit a record and recompute a new,
  internally consistent hash, so a passing check does not prove **who** produced
  the record.
- This verifier deliberately **excludes** the optional `signature` /
  `signature_key_id` fields from the hash (matching Boundary). Those fields are
  where authorship would be attested; verifying them is out of scope here.
- A passing check is **not** evidence that the governed action was executed or
  prevented — only that this record's covered content is internally consistent.

In short: a successful verification tells you the record has not been tampered
with since emission. It does not tell you the record is authentic, signed, or
that its verdict was correct.

## Dependency tree

Intentionally minimal:

| Crate | Role |
|-------|------|
| `serde_json` | JSON parsing (`serde_json::Value`) |
| `serde_jcs` | RFC 8785 / JCS canonicalization (ECMAScript float formatting via `ryu-js`) |
| `sha2` | SHA-256 digest |
| `hex` | hex encoding of the digest |

## Tests

```bash
cargo test --manifest-path verifiers/rust/Cargo.toml
```

The test suite ports every case from the Python verifier's test file:

- Example record verifies ok (in-process)
- Tampered `action` (deny → allow) is caught (mismatch)
- Tampered `reason` is caught (mismatch)
- Wrong stored `decision_hash` is caught (mismatch)
- Missing `decision_hash` reports the right error
- `signature` / `signature_key_id` fields are excluded from the hash
- All 9 records in the shared conformance corpus
  ([`tests/conformance/testdata/verifier-vectors/`](../../tests/conformance/testdata/verifier-vectors))
  recompute to their committed `decision_hash` values
- Float regression: `v1_float_trust_score.json` (`trust_score` 1/3 →
  `0.3333333333333333`) verifies the ECMAScript number round-trip path

## Scope

The cross-implementation conformance corpus under
[`tests/conformance/testdata/verifier-vectors/`](../../tests/conformance/testdata/verifier-vectors)
is asserted by **both** this Rust verifier and the Go conformance gate
([`tests/conformance/verifier_vectors_test.go`](../../tests/conformance/verifier_vectors_test.go)),
so the three implementations (Go, Python, Rust) are pinned to one shared set of
records and expected hashes.
