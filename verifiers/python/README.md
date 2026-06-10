# Boundary decision-record verifier (Python)

Verify a Fulcrum Boundary **decision record** with no Boundary code on the path.

A decision record is the hash-verifiable artifact Boundary emits when it decides
an action (allow / deny / warn / escalate / require-approval). This directory
ships a small, dependency-light Python re-implementation of the one check that
makes that record self-checking: recomputing its `decision_hash`.

```bash
pip install rfc8785
python3 boundary_verify.py record.json
```

Output is one line and an exit code:

```
record verification: ok                       # exit 0
decision_hash mismatch: got sha256:… want sha256:…   # exit 1
```

## What it checks

The decision record is **RFC 8785 / JCS conformant**: its bytes are hashed in
the [RFC 8785 JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785)
form, and the `decision_hash` is the SHA-256 of that canonical form, with four
fields neutralized first (`record_id` and `decision_hash` blanked to `""`,
`signature` and `signature_key_id` dropped) so the hash is self-excluding and
signature-excluding. (This RFC 8785 / JCS statement is scoped to the decision
record specifically; it is not a claim that Boundary as a whole is standards-conformant.)

Because that canonical form is a published standard, the `decision_hash` is
reproducible by a stock RFC 8785 implementation. `boundary_verify.py` does
exactly that:

1. Load the record JSON.
2. Blank `record_id` and `decision_hash` to `""`; drop `signature` and
   `signature_key_id`.
3. Canonicalize with the stock `rfc8785` library.
4. SHA-256, prefix `sha256:`.
5. Compare to the record's stored `decision_hash`.

Both decision-record schema versions (`"1"` and the route-context superset
`"2"`) hash through this same path, so the verifier handles either without
special-casing.

This is the same computation Boundary's Go implementation performs in
[`governance/receipt.go`](../../governance/receipt.go) (`ComputeDecisionHash`).
You can therefore verify a record either **with the Go binary or a stock RFC 8785
verifier** like this one.

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

## Scope

Only the Python verifier ships today. The cross-implementation conformance
corpus under
[`tests/conformance/testdata/verifier-vectors/`](../../tests/conformance/testdata/verifier-vectors)
is asserted by **both** this Python verifier and the Go conformance gate
([`tests/conformance/verifier_vectors_test.go`](../../tests/conformance/verifier_vectors_test.go)),
so the two implementations are pinned to one shared set of records and expected
hashes. The Go assertion runs in CI regardless of whether Python is present.

## Tests

```bash
pip install rfc8785
python3 test_boundary_verify.py
```

The test asserts: the committed example record verifies (exit 0); a one-field
forgery (`"action": "deny"` → `"action": "allow"`) is caught (mismatch, exit 1);
and every record in the shared conformance corpus recomputes to its committed
`decision_hash`.
