# Boundary Evidence Verify

`boundary evidence verify` checks a local evidence bundle: it re-reads the
manifest, re-hashes every artifact, and confirms the human summary references
each one. It is the integrity check for a bundle produced by
`boundary evidence bundle` (see `EVIDENCE_BUNDLE.md`).

```bash
boundary evidence verify boundary-evidence
boundary evidence verify boundary-evidence --json
```

The machine-readable output uses schema `boundary.evidence_verify.v1`.

## Developer Walkthrough

This walkthrough continues the evidence step of the canonical first-run path
(`docs/CLI_REFERENCE.md`). It runs on a clean checkout, needs no credentials,
no cloud account, and performs no live mutation.

### 1. Verify a bundle

```bash
boundary evidence verify boundary-evidence
```

Real output for the 8-artifact `--include-demo` bundle (`./bin/boundary`,
exit 0):

```text
Boundary evidence verify
status: pass
bundle: boundary-evidence
artifacts: 8
verified_artifacts: 8
parsed_records: 0
- PASS manifest_exists: manifest.json is present
- PASS manifest_parse: manifest parses as JSON
- PASS manifest_schema: boundary.evidence_bundle.v1
- PASS artifact:demo/action-boundary.json: sha256:0fd2a61ef527b2859ae9db979ace6bac177f33be87e627b3e8fa4ce878d1ab6c
- PASS artifact:demo/action-boundary.txt: sha256:c13485db68dd390f91151e307bff4b10d576aa0910546806d2b16160f7bf529c
- PASS artifact:doctor.json: sha256:5f117d842bce6ffd5067a53ef4bbb27734266bce7ddf63a4347fb5ae16a190a5
- PASS artifact:selftest.json: sha256:4c2f610517348aca19977eb429793104c99562378b8c4fa4db9d1d7c922ef877
- PASS artifact:selftest.txt: sha256:fe80108540b33e0f062a14239d5fa65178d744d247ab7335d972a678b3b17621
- PASS artifact:summary.md: sha256:b04c0b7c5549402ea12570c27200994a2b6a87dab03925bf265515a5c4934242
- PASS artifact:version.json: sha256:e73f2ebb2f14c72a3c43b2bb5595f804f7080991a48ca0f05eff71eaa0afbf26
- PASS artifact:version.txt: sha256:7c1da5593dbd2c1c385444d47c93c3d239eac9f2456ef69935c9380a35cce477
- PASS fixture_output:version: claimed fixture-safe output is present
- PASS fixture_output:selftest: claimed fixture-safe output is present
- PASS fixture_output:doctor: claimed fixture-safe output is present
- PASS fixture_output:action_boundary_demo: claimed fixture-safe output is present
- PASS summary_references: summary references all manifest artifacts
```

`parsed_records: 0` is expected: the default and `--include-demo` bundles
contain no decision record, so there is nothing to parse on that line. The
bundle and the receipt path are separate subsystems — for receipts, verify a
decision record with `boundary verify-record <record.json>` (see `RECEIPTS.md`).

### 2. Read the machine-readable result

```bash
boundary evidence verify boundary-evidence --json
```

The payload uses schema `boundary.evidence_verify.v1`:

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | string | Constant `boundary.evidence_verify.v1`. |
| `status` | string | `pass` when every check passes, otherwise `fail`. |
| `bundle` | string | Absolute path of the verified bundle directory. |
| `manifest_schema` | string | The `schema_version` read from the bundle manifest. |
| `artifact_count` | int | Number of artifacts listed in the manifest. |
| `verified_artifacts` | int | Number of artifacts that passed existence, size, hash, and (where declared) schema checks. |
| `parsed_records` | int | Count of decision records parsed from `decision_record` artifacts (`0` when none are present). |
| `checks` | object[] | Ordered `{name, status, detail}` entries — one per individual check. |

## Verification Checks

Verification checks, in order:

- `manifest.json` exists and parses
- manifest schema is `boundary.evidence_bundle.v1`
- every manifest artifact exists at its recorded relative path
- every artifact size matches `size_bytes`
- every artifact SHA-256 hash matches `sha256` (recomputed and compared)
- JSON artifacts with a declared `schema_version` carry that schema
- decision record artifacts are parseable when present
- `summary.md` references every manifest artifact path
- claimed `fixture_safe_outputs` are present as artifact kinds

### How the SHA-256 and size checks work

For each manifest artifact, verify resolves the recorded relative `path` under
the bundle root, confirms the file exists, compares the on-disk byte size to
`size_bytes`, then recomputes the SHA-256 hash and compares it to the recorded
`sha256` (lowercase hex, `sha256:` prefixed). A mismatch on size or hash marks
that artifact's check `fail` and flips overall `status` to `fail`; the artifact
is not counted in `verified_artifacts`. A passing artifact emits an
`artifact:<path>` check whose detail is the matched hash.

### How the summary-reference check works

Verify reads the manifest `summary` file (`summary.md`) and confirms the
summary text contains every artifact `path`. The `summary_references` check
fails and lists any artifact paths that are missing from the summary, so the
human-readable summary cannot silently drift from the hashed artifact set.

## Failure Behavior

Verification returns a non-zero exit code when any check fails. With `--json`,
the command still emits the verification payload so automation can inspect the
failed checks; the failing check's `detail` states the expected versus observed
value (for example, the expected versus computed hash).

## Claim Boundary

Evidence verification proves bundle integrity and parseability. Phrased the way
the claim ledger phrases it (BND-CLAIM-UTIL-004): evidence bundles summarize
local artifacts and fixture-safe outputs; they do not prove production
deployment safety by themselves.

Verification does not prove that a deployment removed bypass paths, that a live
upstream service was governed, or that every possible agent action was
protected. It does not prove production route enforcement and it does not prove
live enforcement: a green `verify` means the recorded artifacts are intact and
internally consistent, not that any action was blocked at runtime.

| Property | Verified by `evidence verify`? | Notes |
|---|---|---|
| Manifest is present and parses as the bundle schema | Yes | `manifest_exists` + `manifest_parse` + `manifest_schema`. |
| Each artifact exists, matches size, and matches SHA-256 | Yes | Recomputed per artifact; mismatches fail the run. |
| Declared JSON schemas and decision-record parseability hold | Yes (when present) | Schema checked when `schema_version` is set; records parsed when a `decision_record` artifact exists. |
| Summary references every artifact | Yes | `summary_references`. |
| Production deployment is safe | No | Verify does not prove production safety. |
| Every route is protected | No | Verify does not prove every route is protected. |
| Live enforcement occurred | No | Verify does not prove live enforcement; it checks recorded artifacts, not runtime blocking. |
| Deployment bypasses are closed | No | Verify does not close deployment bypasses; routed-only enforcement is unchanged. |

For a receipt-grade decision record (request, policy bundle, and decision
hashes), use `boundary verify-record <record.json>`; that path is documented in
`RECEIPTS.md` and is independent of `evidence verify`.
