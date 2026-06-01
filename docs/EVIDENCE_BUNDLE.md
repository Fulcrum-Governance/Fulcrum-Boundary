# Boundary Evidence Bundle

`boundary evidence bundle` creates a local evidence directory for review,
handoff, or release verification. It collects fixture-safe command outputs,
hashes every artifact it writes, and records the result in a manifest.

```bash
boundary evidence bundle
boundary evidence bundle --from .boundary --out boundary-evidence
boundary evidence bundle --include-demo --json
```

The command is fixture-safe by default:

- credentials: none
- network: none
- live mutation: none

## Developer Walkthrough

This walkthrough is the evidence step of the canonical first-run path
(`docs/CLI_REFERENCE.md`). It runs on a clean checkout, needs no credentials,
no cloud account, and performs no live mutation.

### 1. Run the bundle

```bash
boundary evidence bundle --include-demo --out boundary-evidence
```

Real output on a clean checkout (`./bin/boundary`, exit 0):

```text
Boundary evidence bundle
status: pass
output: boundary-evidence
manifest: boundary-evidence/manifest.json
artifacts: 8
credentials: none
network: none
live mutation: none
warnings:
- source directory not present; no existing .boundary artifacts copied
```

The `source directory not present` line is a benign, expected warning on a
clean checkout: no `.boundary/` workspace exists yet, so no local source
artifacts are copied in. It is not an error and the status is still `pass`.

### 2. Inspect the manifest

```bash
cat boundary-evidence/manifest.json
```

The manifest is `manifest.json` with schema `boundary.evidence_bundle.v1`. Its
top-level fields are:

| Field | Type | Meaning |
|---|---|---|
| `schema_version` | string | Constant `boundary.evidence_bundle.v1`. |
| `created_at` | string | RFC3339 UTC timestamp when the bundle was written. |
| `source` | string | Absolute path of the `--from` source directory (default `.boundary`). |
| `output` | string | Absolute path of the bundle output directory. |
| `summary` | string | Relative path of the human summary (`summary.md`). |
| `include_demo` | bool | Whether `--include-demo` added the action-boundary demo artifacts. |
| `requires_credentials` | bool | Always `false` for the fixture-safe bundle. |
| `requires_network` | bool | Always `false` for the fixture-safe bundle. |
| `mutates_live_systems` | bool | Always `false` for the fixture-safe bundle. |
| `fixture_safe_outputs` | string[] | Kinds of fixture-safe command output the bundle claims to carry. |
| `artifacts` | object[] | One entry per written file (see below), sorted by `path`. |
| `warnings` | string[] | Benign collection warnings; present only when non-empty. |

Each `artifacts[]` entry records the artifact `path`, `kind`, `size_bytes`,
SHA-256 `sha256` (lowercase hex, `sha256:` prefixed), and an optional
`schema_version` for JSON artifacts that declare one. For example:

```json
{
  "path": "doctor.json",
  "kind": "doctor",
  "sha256": "sha256:5f117d842bce6ffd5067a53ef4bbb27734266bce7ddf63a4347fb5ae16a190a5",
  "size_bytes": 3045,
  "schema_version": "boundary.doctor.v1"
}
```

### 3. Run verify

```bash
boundary evidence verify boundary-evidence
```

Real output (exit 0): `status: pass`, `artifacts: 8 / verified_artifacts: 8`,
with a per-artifact `PASS` line carrying each SHA-256, plus `fixture_output:*`
and `summary_references` PASS lines. See `EVIDENCE_VERIFY.md` for the full
check list and what those checks do and do not prove.

When a demo or evidence step prints a decision-record path, verify that record
separately with `boundary verify-record <record.json>`; see `RECEIPTS.md`. The
default bundle does not contain decision records, so `boundary evidence verify`
reports `parsed_records: 0` — the bundle and the receipt path are two distinct
subsystems (see "What It Does Not Prove" below).

## What It Includes

Every bundle writes:

- `manifest.json` with schema `boundary.evidence_bundle.v1`
- `summary.md`
- `version.json` (schema `boundary.version.v1`) and `version.txt`
- `selftest.json` (schema `boundary.selftest.v1`) and `selftest.txt`
- `doctor.json` (schema `boundary.doctor.v1`)
- copied source artifacts under `artifacts/` from `--from` when the directory exists

With `--include-demo`, the bundle also writes:

- `demo/action-boundary.json` (schema `boundary.demo.action_boundary.v1`)
- `demo/action-boundary.txt`

A clean-checkout `--include-demo` bundle therefore contains exactly **8
artifacts**: `version.{json,txt}`, `selftest.{json,txt}`, `doctor.json`,
`demo/action-boundary.{json,txt}`, and `summary.md` (`manifest.json` itself is
the manifest, not a listed artifact). The artifact list is sorted by `path`.

The `--include-demo` flag pulls the **action-boundary** demo, not the
`github-lethal-trifecta` or `command-secret-exfil` demos, and it does not add
any decision record to the bundle.

The manifest records each artifact path, kind, size, and SHA-256 hash. The
`summary.md` file restates the schema, source, fixture-safe posture, and lists
every artifact path and hash, so the bundle is reviewable without parsing JSON.

## What It Proves

The bundle proves that Boundary can collect local release evidence, include
fixture-safe command outputs, and hash the artifacts it collected. Phrased the
way the claim ledger phrases it (BND-CLAIM-UTIL-004): Boundary creates and
verifies local evidence bundles with manifest hashing and fixture-safe utility
outputs.

## What It Does Not Prove

Evidence bundles summarize local artifacts and fixture-safe outputs; they do
not prove production deployment safety by themselves. An evidence bundle does
not prove production route enforcement, live upstream conformance, shell
control, filesystem sandboxing, or protection for actions that bypass
Boundary. The bundle is a collection-and-hashing artifact, not an
execution-control proof.

| Property | Verified by a bundle? | Notes |
|---|---|---|
| The listed artifacts exist and are byte-stable | Yes (via `evidence verify`) | SHA-256 + size per artifact. |
| Fixture-safe outputs were produced without credentials/network/live mutation | Yes | `requires_*`/`mutates_live_systems` are `false`; the commands run hermetically. |
| Production deployment is safe | No | A bundle does not prove production safety; bypass paths are out of scope. |
| Every route is protected | No | A bundle does not prove every route is protected; routed-only enforcement is unchanged by collecting evidence. |
| Live enforcement occurred | No | A bundle records decisions and outputs; it does not prove live enforcement. |
| Deployment bypasses are closed | No | Evidence bundles do not close deployment bypasses; topology, not the bundle, removes bypass paths. |
| Decision records are present | No (by default) | The default and `--include-demo` bundles carry no decision record; verify reports `parsed_records: 0`. Use `boundary verify-record` for receipts. |

Use `boundary evidence verify <bundle-dir>` to check bundle integrity, and see
`EVIDENCE_VERIFY.md` for exactly what that integrity check covers.
