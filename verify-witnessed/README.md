# Fulcrum witnessed-log verifier (Go)

Verify a Fulcrum witnessed-log bundle offline, with no Fulcrum account,
credential, or network call required.

A witnessed-log bundle is an export produced by Fulcrum's witnessed-log
service: the decision records under audit, a manifest describing the
bundle's contents, the log's signed tree head (STH) for the period those
records were checked into, and any witness cosignatures collected over that
STH. This directory ships `fulcrum-verify-witnessed`: a small,
dependency-light Go command that checks a bundle end to end.

> **Terminology note.** "Witness" here means a party that cryptographically
> cosigns the witnessed log's signed tree head — the `witness_cosignature`
> check below. This is a different artifact from `boundary evidence bundle`
> (see [`docs/EVIDENCE_BUNDLE.md`](../docs/EVIDENCE_BUNDLE.md)), which
> packages local Boundary diagnostics.

`verify-witnessed/` is its own Go module — build and test it from inside
this directory, not via a package path from the repo root, since it's
deliberately excluded from the root module's build graph (the same boundary
as `adapters/grpc`; see the root `CLAUDE.md`).

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary/verify-witnessed
go build -o fulcrum-verify-witnessed .

# try it against the bundle fixtures shipped in this directory:
./fulcrum-verify-witnessed testdata/witnessed-log-v1/bundles/valid \
  --keyring testdata/witnessed-log-v1/public-keys-v1.json
```

Default output is one JSON object per line, one per check, and the process
exits 0 only if every check status is `pass`:

```
{"id":"decision_record_integrity:sha256:04f26fb888aad12b244ed3350a0e6e5cd0d139e438a401f42d6c88ca230ddfc7","status":"pass"}
{"id":"manifest:decisions.jsonl","status":"pass"}
{"id":"manifest:declines.jsonl","status":"pass"}
{"id":"manifest:inclusion-proof-v1.json","status":"pass"}
{"id":"manifest:tree-head-v1.json","status":"pass"}
{"id":"manifest:witness-cosignatures-v1.json","status":"pass"}
{"id":"source_hash_to_leaf","status":"pass"}
{"id":"inclusion_proof","status":"pass"}
{"id":"bundle_tenant_binding","status":"pass"}
{"id":"sth_signature","status":"pass"}
{"id":"witness_cosignature:witness-alpha","status":"pass"}
{"id":"witness_cosignature:witness-beta","status":"pass"}
```
exit 0

Point it at the shipped `leaf-tamper` fixture and the tampered leaf's
inclusion proof no longer holds, and the process exits 1:

```
{"id":"decision_record_integrity:sha256:04f26fb888aad12b244ed3350a0e6e5cd0d139e438a401f42d6c88ca230ddfc7","status":"pass"}
{"id":"manifest:decisions.jsonl","status":"pass"}
{"id":"manifest:declines.jsonl","status":"pass"}
{"id":"manifest:inclusion-proof-v1.json","status":"pass"}
{"id":"manifest:tree-head-v1.json","status":"pass"}
{"id":"manifest:witness-cosignatures-v1.json","status":"pass"}
{"id":"source_hash_to_leaf","status":"pass"}
{"id":"inclusion_proof","status":"fail"}
{"id":"bundle_tenant_binding","status":"pass"}
{"id":"sth_signature","status":"pass"}
{"id":"witness_cosignature:witness-alpha","status":"pass"}
{"id":"witness_cosignature:witness-beta","status":"pass"}
```
exit 1

`--json` emits one envelope instead of one line per check:
`{"schema_version":"witnessed-verifier-results-v1","checks":[...]}`.

There is deliberately **no aggregate `verified` field**, in either output
mode. Read the per-check array: a bundle is fully verified, by your own
policy, only when every check you care about reports `pass`.

## What it checks

Each check reports exactly one of `pass`, `fail`, `missing`, or
`not_present`. `missing`/`not_present` mean a check could not run in the
terms it was configured for (e.g. no key supplied for a cosignature the
bundle carries) — not automatically a failure, but not a pass either; decide
your own acceptance policy before treating a bundle as fully checked.

- **`decision_record_integrity:<decision_hash>`** — recomputes each decision
  record's `decision_hash`, one check per record. This is the same
  record-level integrity check performed by the Go, Python, TypeScript, and
  Rust record verifiers elsewhere in this repository — Go in
  [`governance/receipt.go`](../governance/receipt.go) (`ComputeDecisionHash`),
  the other three under [`verifiers/`](../verifiers).
- **`manifest:<filename>`** — one check per file the bundle's manifest
  lists, confirming each listed entry is present and resolves.
- **`source_hash_to_leaf`** — confirms the hash chain from a decision
  record's source bytes to the log leaf it was checked into.
- **`inclusion_proof`** — confirms the Merkle inclusion proof places that
  leaf under the bundle's signed tree head (STH).
- **`bundle_tenant_binding`** — confirms the bundle is bound to the expected
  tenant, so records from one tenant cannot be substituted into another's
  bundle.
- **`sth_signature`** — confirms Fulcrum's Ed25519 signature over the signed
  tree head, against a key supplied with `--fulcrum-pubkey` or `--keyring`.
- **`witness_cosignature:<witness_id>`** — confirms a witness's Ed25519
  cosignature over that same signed tree head, against a key supplied with
  `--witness-pubkey` or `--keyring`, one check per witness id present.

## What it does NOT check

- **Completeness** — that the bundle contains every record for the period
  it covers, only that the records it does contain check out.
- **Correctness of upstream Fulcrum systems** — that decisions recorded
  were substantively correct, or that the witnessed-log service behaved
  correctly before producing the bundle.
- **Non-repudiation or legal admissibility** — a passing bundle is not, by
  itself, a legal evidentiary instrument.
- **Equal integrity guarantees for every leaf type.**
- **External or independent witnessing** — `witness_cosignature` confirms a
  valid cryptographic cosignature from the configured key. It does **not**
  establish that the witness is operated by an organization independent of
  Fulcrum. As of this writing, Fulcrum operates its own witnesses, on
  infrastructure separate from the systems they witness — that is honestly
  **Fulcrum-operated witnessing on separate infrastructure**, not external
  or independent witnessing.
- **Immutability** — the log and its bundles are **tamper-evident**, not
  immutable: tampering after the fact is detectable, not physically
  prevented.

## How to obtain it

Clone the public Fulcrum-Boundary repository and build from that clone. No
Fulcrum account, credential, or private-repository access is required:

```bash
git clone https://github.com/Fulcrum-Governance/Fulcrum-Boundary.git
cd Fulcrum-Boundary/verify-witnessed
go build -o fulcrum-verify-witnessed .
```

Requires a Go 1.26+ toolchain. Building it pulls in no Boundary-internal
package and no other Fulcrum private code (see "Tests" below for the
command that checks this directly). Once this capability ships in a tagged
release, pin your clone to that tag (`git checkout vX.Y.Z`) to match your
bundle's `exporter.boundary_wire_contract_version` — the same convention
used for the existing decision-record verifiers.

## How to run it

Run against an exported bundle directory with no network access and no
Fulcrum credentials in the environment, using a local key-map file:

```bash
env -i PATH=/usr/bin:/bin ./fulcrum-verify-witnessed ./bundle \
  --keyring ./published-keys.json
```

or with individual keys, each as `ID=KEY`:

```bash
env -i PATH=/usr/bin:/bin ./fulcrum-verify-witnessed ./bundle \
  --fulcrum-pubkey 'ed25519:fulcrum-key-id=ed25519:<base64-public-key>' \
  --witness-pubkey 'ed25519:witness-key-id=ed25519:<base64-public-key>'
```

`env -i` clears the environment before exec and sets only `PATH`. Substitute
the actual key ids and base64 Ed25519 public keys your counterparty
published for their Fulcrum STH signer and each witness they run — or point
`--keyring` at their published `witnessed-public-key-map-v1` file directly.

```
Usage: fulcrum-verify-witnessed DIR [options]

Options:
  --keyring FILE                 local witnessed-public-key-map-v1 file (repeatable)
  --fulcrum-pubkey ID=KEY        trusted Fulcrum STH Ed25519 key (repeatable)
  --witness-pubkey ID=KEY        trusted witness Ed25519 key (repeatable)
  --json                         emit one witnessed-verifier-results-v1 object
  -h, --help                     show this help
```

`--keyring`, `--fulcrum-pubkey`, and `--witness-pubkey` are each repeatable
and may be combined. Exit codes: `0` if every check status is `pass`; `1` on
a runtime error or any `fail` status; `2` on a usage or input error.

## Tests

```bash
go test ./... -count=1
```

The suite spawns the built binary with only `FULCRUM_VERIFY_HELPER`,
`FULCRUM_VERIFY_BUNDLE`, `FULCRUM_VERIFY_KEYRING`, and `HOME` set in its
environment, and asserts none of `DATABASE_URL`, `REDIS_URL`, `NATS_URL`,
`FULCRUM_API_KEY`, `AWS_*`, `GOOGLE_APPLICATION_CREDENTIALS`, `*_PROXY`, or
`GOPROXY` are present — pinning that a real check run needs no Fulcrum
service, credential, or outbound network configuration. Run
`go test ./... -run TestSourceAndDependencyIndependence -v` on its own to
check, directly, that this module imports nothing beyond the standard
library, itself, and `golang.org/x/mod/sumdb/tlog`.
