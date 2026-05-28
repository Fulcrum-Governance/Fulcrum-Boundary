# Fulcrum Boundary v0.6.x Build Train Implementation Plan

> Agent workflow: use `superpowers:executing-plans` or the closest available
> local equivalent. Execute task by task, keep the repo clean between branches,
> and do not start implementation on `main`.

Date: 2026-05-28
Design: `docs/superpowers/specs/2026-05-28-v06x-build-train-design.md`
Source spec: `/Users/td/Documents/Next_Spec0.6High.md`

## Goal

Add the v0.6.x utility train that makes Boundary cohesive and provable without
adding a fourth governed action surface:

```text
boundary version
boundary demo action-boundary
boundary doctor
boundary evidence bundle
boundary evidence verify
release-check coverage
minimal utility claims
```

## Invariants

- MCP remains production.
- Secure GitHub remains preview.
- Command Boundary remains preview.
- Edit Boundary remains preview.
- No command requires credentials, live network access, or real mutation by
  default.
- Every new command supports `--help`.
- Every machine-readable command supports `--json`.
- Claims are added only after the tests and docs they cite exist.

## Task 0: Land Design And Plan

- [ ] Confirm design branch is clean.
- [ ] Commit this plan artifact.
- [ ] Push the branch.
- [ ] Merge the design/plan branch into `main`.
- [ ] Pull `main` and start the implementation train from clean `main`.

## Task 1: Add `boundary version`

- [ ] Create branch `feat/version-command`.
- [ ] Add `internal/versioninfo/version.go` with ldflag fields for version,
  commit, and build date plus build-info fallback.
- [ ] Add `internal/boundarycli/version.go` for text and JSON rendering.
- [ ] Register `version` in the root CLI help and dispatcher.
- [ ] Add `tests/cli_output/version_test.go`.
- [ ] Minimally update `docs/CLI_REFERENCE.md`.
- [ ] Run:

```bash
go test ./internal/versioninfo/... -count=1 -timeout 5m
go test ./tests/cli_output/... -run Version -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 2: Add Unified Action Boundary Demo

- [ ] Create branch `feat/action-boundary-demo`.
- [ ] Add `internal/demo/action_boundary.go`.
- [ ] Add `internal/boundarycli/demo_action_boundary.go`.
- [ ] Register `boundary demo action-boundary`.
- [ ] Reuse existing Secure GitHub, command, and edit fixtures.
- [ ] Support text, `--json`, `--markdown --out`, and
  `--dashboard --out`.
- [ ] Add `tests/demo/action_boundary_demo_test.go`.
- [ ] Add `examples/cli/demo-action-boundary.txt`.
- [ ] Add minimal `docs/DEMO_ACTION_BOUNDARY.md`.
- [ ] Run:

```bash
go test ./tests/demo/... -run ActionBoundary -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 3: Refit `boundary doctor`

- [ ] Create branch `feat/doctor-command`.
- [ ] Add `internal/doctor` package for local-only surface diagnostics.
- [ ] Replace old network/gateway-prerequisite default doctor behavior.
- [ ] Support `--surface mcp|command|edit`, `--json`, and `--help`.
- [ ] Include bypass caveats for each surface.
- [ ] Add `tests/doctor/doctor_test.go`.
- [ ] Add minimal `docs/DOCTOR.md`.
- [ ] Run:

```bash
go test ./tests/doctor/... -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 4: Add Evidence Bundle And Verify

- [ ] Create branch `feat/evidence-bundle-verify`.
- [ ] Add `internal/evidence` package with bundle, manifest, hash, and verify
  logic.
- [ ] Add `internal/boundarycli/evidence.go`.
- [ ] Register `boundary evidence bundle` and `boundary evidence verify`.
- [ ] Support `--include-demo`, `--from`, `--out`, `--json`, and `--help`
  as specified.
- [ ] Use schema `boundary.evidence_bundle.v1` for bundle manifests.
- [ ] Use schema `boundary.evidence_verify.v1` for verify output.
- [ ] Hash artifacts with SHA-256.
- [ ] Add `tests/evidence/bundle_test.go` and `tests/evidence/verify_test.go`.
- [ ] Add minimal `docs/EVIDENCE_BUNDLE.md` and
  `docs/EVIDENCE_VERIFY.md`.
- [ ] Run:

```bash
go test ./tests/evidence/... -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 5: Add Utility Claims

- [ ] Create branch `claims/v061-utility-claims`.
- [ ] Add utility claims `BND-CLAIM-UTIL-001` through
  `BND-CLAIM-UTIL-004` to `claims/boundary_claims.yaml`.
- [ ] Mirror claims in `docs/CLAIMS_LEDGER.md`.
- [ ] Ensure each delivered claim cites at least one existing test path and one
  existing doc path.
- [ ] Preserve forbidden language:
  - evidence proves production safety;
  - doctor proves all routes protected;
  - demo proves all attacks blocked;
  - version proves cryptographic release provenance.
- [ ] Run:

```bash
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 6: Update Release Check

- [ ] Create branch `release/check-action-boundary`.
- [ ] Update `scripts/release-check.sh` to run:

```bash
go run ./cmd/boundary version
go run ./cmd/boundary demo action-boundary
go run ./cmd/boundary doctor --json
tmp="$(mktemp -d)"
go run ./cmd/boundary evidence bundle --include-demo --out "$tmp/evidence"
go run ./cmd/boundary evidence verify "$tmp/evidence"
```

- [ ] Ensure no live credentials or network access are needed.
- [ ] Run:

```bash
make release-check
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

- [ ] Commit, push, merge, and return to clean `main`.

## Task 7: Final Consolidation

- [ ] Create final packaging branch only after the utility train lands.
- [ ] Decide `v0.6.1` vs `v0.7.0` based on actual shipped scope.
- [ ] Update release truth, changelog, docs-site nav, and README minimally.
- [ ] Run full release verification before any tag.
- [ ] Do not change the public release tag until this explicit packaging step.

