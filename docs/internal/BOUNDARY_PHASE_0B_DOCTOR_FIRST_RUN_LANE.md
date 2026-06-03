# Boundary Phase 0B Doctor First-Run Lane

Date: 2026-06-03

## Summary

Phase 0B improves the first-run trust loop without adding a governed action
surface. This slice extends `boundary doctor` so developers can diagnose the
three common install failures directly from the CLI:

- Go toolchain is missing or older than Go 1.25.
- cgo / C-toolchain readiness is missing for the default build.
- `go install` wrote `boundary` somewhere that is not available on `PATH`.

It also adds `boundary doctor --report`, a redacted JSON report intended for
support threads.

## Authority

- Roadmap: `docs/BOUNDARY_ROADMAP.md` Phase 0B.
- Doctor contract: `docs/DOCTOR.md`.
- First-run troubleshooting: `docs/TROUBLESHOOTING.md`.
- Claim boundary: `docs/CLAIMS_LEDGER.md` claim `BND-CLAIM-UTIL-003`.

## Delivered In This Slice

- Additive `environment` diagnostics in `boundary.doctor.v1` JSON output.
- Text output section named `Environment diagnostics:`.
- `boundary doctor --report`, which emits redacted JSON with
  `report_redacted: true` and `project_root: "<redacted>"`.
- Tests that assert the environment diagnostics exist and that report output
  does not leak the local working directory.

## Non-Goals

- No new governed action surface.
- No production deployment proof.
- No remote runtime verification.
- No route-bypass closure claim.
- No release tag in this lane.
- No README/demo visual hierarchy pass; that remains the next Phase 0B follow-up.

## Verification

Required gates before merge:

```bash
go test ./tests/doctor -count=1
go test ./internal/doctor/... ./internal/boundarycli/... ./tests/doctor -count=1
go test ./claims/... -count=1
mkdocs build -s
git diff --check
```

Full release gates remain required before any tag:

```bash
make release-check
make docs-build
```
