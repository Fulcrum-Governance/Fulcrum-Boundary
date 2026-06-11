# Signing decision records (opt-in)

Boundary decision records carry an unkeyed SHA-256 `decision_hash` over their
RFC 8785 / JCS canonical bytes. That hash is **integrity, not authenticity**: it
detects tampering after emission, but an unkeyed hash does not attest who
produced the record (see [`RECEIPTS.md`](RECEIPTS.md) and
[`DECISION_RECORDS.md`](DECISION_RECORDS.md)).

Optional Ed25519 signing adds authorship **for holders who manage keys**. When a
signer is configured, every emitted decision record carries a detached
`signature` and a `signature_key_id`. Signing is **off by default**: with no
signer configured, records are unsigned and byte-identical to the unsigned path,
and `decision_hash` is unchanged whether or not a record is signed.

## What a signature does and does not establish

A valid signature proves the record was signed by the holder of the named key,
over exactly the content `decision_hash` covers. It does **not**:

- prove the verdict was correct;
- prove the governed action executed or was prevented (Boundary decides
  pre-execution; `executed` / `upstream_called` are adapter self-reports, not
  fields of the hashed record);
- solve key custody — a signature is only as trustworthy as the private key's
  protection, which is the operator's responsibility, not Boundary's.

Signing narrows the integrity-not-authenticity gap **only** for keys the
operator actually manages. It does not make a record tamper-proof and does not
authenticate the deployment topology.

## The signature contract

The signature is computed over the record's `decision_hash` string:

```
signature = "ed25519:" + base64( ed25519.Sign(privkey, []byte(decision_hash)) )
```

`decision_hash` itself is computed with `record_id`, `decision_hash`,
`signature`, and `signature_key_id` blanked first, so carrying a signature never
changes `decision_hash`: a signed record and its unsigned twin hash identically.
`signature_key_id` is a non-secret identifier a verifier uses to select the
public key; it is not itself authenticated by the signature.

## Generating a key

Boundary does not ship a key-generation command. Generate an Ed25519 seed
(32 bytes, 64 hex characters) and derive the public key with standard tooling.

With OpenSSL:

```bash
# 32-byte seed as 64 hex characters; keep this file secret.
openssl rand -hex 32 > boundary-receipt.seed
chmod 600 boundary-receipt.seed

# Derive the 32-byte (64 hex) public key from the seed.
SEED=$(cat boundary-receipt.seed)
printf '302e020100300506032b657004220420%s' "$SEED" | xxd -r -p \
  | openssl pkey -inform DER -pubout -outform DER \
  | tail -c 32 | xxd -p -c 32 > boundary-receipt.pub
```

The seed file is **secret key material**. Store it with `0600` permissions (or
in a secret manager) and never commit it. The public key (`boundary-receipt.pub`)
is not secret and is what verifiers need.

`signature_key_id` defaults to a public-key fingerprint
(`ed25519:` + first 16 hex chars of `SHA-256(pubkey)`) unless you set an explicit
id when constructing the signer.

## Enabling signing

Signing is wired through the governance pipeline as an opt-in seam
(`governance.PipelineConfig.ReceiptSigner`). Construct a signer from the seed
file and pass it when building the pipeline:

```go
signer, err := governance.NewEd25519SignerFromSeedFile("boundary-receipt.seed", "")
if err != nil {
    return err
}
pipeline := governance.NewPipeline(governance.PipelineConfig{
    // ... existing config ...
    ReceiptSigner: signer, // nil (default) leaves records unsigned
}, trust, evaluator, auditor)
```

With the signer set, emitted decision records carry `signature` and
`signature_key_id`. If signing fails for a record, the pipeline publishes that
record **unsigned** rather than with a partial or bogus signature, so an unsigned
record never masquerades as signed.

## Verifying a signature

`boundary verify-record` checks `decision_hash` (and, when given, `request_hash`,
`policy_bundle_hash`, and `boundary_build_digest`) by default and **ignores the
signature fields for integrity** — unsigned records remain the default and verify
without a key. Pass `--verify-signature --public-key <64-hex | file>` to
additionally check the signature over the recomputed `decision_hash`:

```bash
boundary verify-record --verify-signature --public-key boundary-receipt.pub record.json
```

Behavior:

- The integrity checks run first; a tampered covered field fails as a
  `decision_hash mismatch` before the signature is even checked.
- Signature verification **fails closed**: a missing signature, a signature
  without the `ed25519:` prefix, malformed base64, a wrong-length signature, a
  wrong public key, or a key that does not match all return a non-zero exit and
  an error.
- `--verify-signature` requires `--public-key`; omitting the key is an error,
  not a silent pass.

## Scope of the other verifiers

The standalone verifiers shipped or planned alongside the Go binary — the Python
verifier under [`verifiers/python/`](../verifiers/python/), and any TypeScript or
Rust verifier — are **integrity-only**: they recompute `decision_hash` and do
**not** check signatures. They deliberately exclude `signature` and
`signature_key_id` from the hash (mirroring Boundary), so they detect tampering
of covered fields but make no authorship claim. Signature verification today is
the Go `boundary verify-record --verify-signature` path only. These verifiers do
not let you verify a record in any language with authorship attached; the format
is reproducible for integrity, while signature checking is not yet ported.
