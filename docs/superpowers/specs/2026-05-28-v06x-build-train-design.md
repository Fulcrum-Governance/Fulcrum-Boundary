# Fulcrum Boundary v0.6.x Build Train Design

Date: 2026-05-28
Source spec: `/Users/td/Documents/Next_Spec0.6High.md`
Design status: approved in thread before implementation

## Purpose

Fulcrum Boundary v0.6.0 is already shipped. The next build train should not add
another governed action surface. It should make the current product easier to
prove, diagnose, and show:

```text
version -> demo -> diagnose routes -> bundle evidence -> verify evidence
```

The current action surfaces remain:

- MCP Firewall: production.
- Secure GitHub: preview, with fixture and live no-mutation conformance paths.
- Command Boundary: preview.
- Filesystem/Edit Boundary: preview.

No command in this train may upgrade preview surfaces to production, weaken
claims tests, require credentials by default, make live network calls by
default, perform real mutations in demos, or change the public release tag until
an explicit packaging step.

## Sequencing

The implementation should proceed as a sequential train from clean `main`.
Most specs stay on their own branches so failures and review scope remain
small. The evidence bundle and evidence verify specs are intentionally combined
into one evidence branch because they share a manifest schema, artifact hashing
contract, and test fixtures.

Recommended branch sequence:

1. `feat/version-command`
2. `feat/action-boundary-demo`
3. `feat/doctor-command`
4. `feat/evidence-bundle-verify`
5. `claims/v061-utility-claims`
6. `release/check-action-boundary`
7. final consolidation and packaging branch when the train stops

Each branch should be merged before the next branch starts unless a branch is
purely downstream of already-merged artifacts. Claims should land only after
the commands, docs, and tests they cite exist.

## Command Design

### `boundary version`

Add a dedicated version surface with:

- `boundary version`
- `boundary version --json`
- `boundary version --help`

Implementation boundaries:

- Put metadata resolution in `internal/versioninfo`.
- Keep CLI formatting in `internal/boundarycli/version.go`.
- Support ldflags for `Version`, `Commit`, and `BuildDate`.
- Fall back to Go build info when possible.
- Use `unknown` for absent metadata.
- Never fail solely because release metadata is missing.

Text output should include version, commit, build date, Go version, and module.
JSON output should use schema `boundary.version.v1`.

### `boundary demo action-boundary`

Add one fixture-only demo that shows the three current action surfaces together:

- MCP / Secure GitHub: poisoned issue to private repo write, expected and
  actual `DENY`, reason `lethal_trifecta_detected`, `upstream_called=false`.
- Command Boundary: `git push origin main`, class `C3`, risk `HIGH`,
  recommended action `require_approval`, `executed=false`.
- Edit Boundary: `edit-env-secret`, class `E4`, risk `CRITICAL`, actual
  `DENY`, `applied=false`.

Supported forms:

- `boundary demo action-boundary`
- `boundary demo action-boundary --json`
- `boundary demo action-boundary --markdown --out demo.md`
- `boundary demo action-boundary --dashboard --out .boundary/action-boundary-demo`
- `boundary demo action-boundary --help`

The demo must be no-credential, no-network, and no-real-mutation by default.
It should compose existing demo, command, and edit fixtures rather than create
a fourth surface or a parallel governance model.

### `boundary doctor`

Refit the existing doctor command into local route and surface diagnostics:

- `boundary doctor`
- `boundary doctor --surface mcp`
- `boundary doctor --surface command`
- `boundary doctor --surface edit`
- `boundary doctor --json`
- `boundary doctor --help`

The doctor must not mutate state, require credentials, or make live network
calls. It should report local readiness and bypass caveats for each surface.
The old gateway-prerequisite behavior that dials upstream services should not
remain the default behavior of this command.

### `boundary evidence bundle` and `boundary evidence verify`

Add a shared `internal/evidence` package for evidence creation and verification.
Combining bundle and verify keeps the manifest schema and hash contract in one
coherent implementation lane.

Bundle command:

- `boundary evidence bundle`
- `boundary evidence bundle --from .boundary`
- `boundary evidence bundle --out boundary-evidence`
- `boundary evidence bundle --include-demo`
- `boundary evidence bundle --json`
- `boundary evidence bundle --help`

Verify command:

- `boundary evidence verify boundary-evidence`
- `boundary evidence verify boundary-evidence --json`
- `boundary evidence verify --help`

Bundle outputs should include a manifest, summary, version output, optional
fixture-safe selftest/demo/doctor outputs, and copied artifacts where present.
The manifest schema is `boundary.evidence_bundle.v1` and must hash artifacts
with SHA-256.

Verification should prove manifest parseability, artifact existence, SHA-256
matches, schema expectations, fixture-safe run outputs when claimed, parseable
records, and summary references to included artifacts. Verification output uses
schema `boundary.evidence_verify.v1`.

## Claims And Release Checks

After the utility commands exist, add minimal delivered utility claims:

- `BND-CLAIM-UTIL-001`: version metadata through `boundary version`.
- `BND-CLAIM-UTIL-002`: unified fixture-only action-boundary demo across MCP,
  Command Boundary, and Edit Boundary.
- `BND-CLAIM-UTIL-003`: local route and surface diagnostics without mutation.
- `BND-CLAIM-UTIL-004`: local evidence bundle generation and verification for
  fixture-safe Boundary runs.

Forbidden language remains:

- evidence proves production safety;
- doctor proves all routes protected;
- demo proves all attacks blocked;
- version proves cryptographic release provenance.

The release check should add the new utility commands only after they are
implemented and tested:

- `go run ./cmd/boundary version`
- `go run ./cmd/boundary demo action-boundary`
- `go run ./cmd/boundary doctor --json`
- bundle and verify in a temporary directory

No live credentials or network access should be required for release check.

## Testing Strategy

Each branch should run the tests named in the source spec for its scope. The
shared minimum for utility branches is:

```bash
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

Additional targeted tests:

- version: `go test ./internal/versioninfo/...` and
  `go test ./tests/cli_output/... -run Version`.
- action demo: `go test ./tests/demo/... -run ActionBoundary`.
- doctor: `go test ./tests/doctor/...`.
- evidence: `go test ./tests/evidence/...`.

Before any release packaging branch, run the full release check and the new
utility command smoke tests from the updated script.

## Non-Goals

- No new action surface beyond MCP, Command Boundary, and Edit Boundary.
- No production promotion for Secure GitHub, Command Boundary, or Edit Boundary.
- No broad README expansion before final consolidation.
- No public tag change until packaging is explicit.
- No live-network or credential-dependent default demos.
- No weakening or bypassing claims validation.

