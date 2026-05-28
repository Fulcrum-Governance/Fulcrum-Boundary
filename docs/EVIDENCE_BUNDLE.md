# Boundary Evidence Bundle

`boundary evidence bundle` creates a local evidence directory for review,
handoff, or release verification.

```bash
boundary evidence bundle
boundary evidence bundle --from .boundary --out boundary-evidence
boundary evidence bundle --include-demo --json
```

The command is fixture-safe by default:

- credentials: none
- network: none
- live mutation: none

## What It Includes

Every bundle writes:

- `manifest.json` with schema `boundary.evidence_bundle.v1`
- `summary.md`
- `version.json` and `version.txt`
- `selftest.json` and `selftest.txt`
- `doctor.json`
- copied source artifacts from `--from` when the directory exists

With `--include-demo`, the bundle also writes:

- `demo/action-boundary.json`
- `demo/action-boundary.txt`

The manifest records each artifact path, kind, size, and SHA-256 hash.

## What It Proves

The bundle proves that Boundary can collect local release evidence, include
fixture-safe command outputs, and hash the artifacts it collected.

## What It Does Not Prove

An evidence bundle does not prove production route enforcement, live upstream
conformance, shell control, filesystem sandboxing, or protection for actions
that bypass Boundary.

Use `boundary evidence verify <bundle-dir>` to check bundle integrity.
