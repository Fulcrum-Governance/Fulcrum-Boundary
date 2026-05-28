# Boundary Evidence Verify

`boundary evidence verify` checks a local evidence bundle.

```bash
boundary evidence verify boundary-evidence
boundary evidence verify boundary-evidence --json
```

The machine-readable output uses schema `boundary.evidence_verify.v1`.

## Verification Checks

Verification checks:

- `manifest.json` exists and parses
- manifest schema is `boundary.evidence_bundle.v1`
- every manifest artifact exists
- every artifact size matches
- every artifact SHA-256 hash matches
- JSON artifacts with declared schemas match those schemas
- decision record artifacts are parseable when present
- `summary.md` references every manifest artifact
- claimed fixture-safe outputs are present

## Failure Behavior

Verification returns a non-zero exit code when any check fails. With `--json`,
the command still emits the verification payload so automation can inspect the
failed checks.

## Claim Boundary

Evidence verification proves bundle integrity and parseability. It does not
prove that a deployment removed bypass paths, that a live upstream service was
governed, or that every possible agent action was protected.
