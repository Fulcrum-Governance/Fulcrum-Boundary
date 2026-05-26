# CODEX Session Log

## 2026-05-26 — Boundary Phase 1 Claims And Adapter Readiness

### Context

- Spec: `/Users/td/ConceptDev/Projects/Fulcrum-codex/.claude/sprint/BOUNDARY_SPEC_SERIES.md`
- Phase: `Phase 1 — SPEC 1 Claims Ledger + Release Truth System` and `SPEC 2 Adapter Completion Standard`
- Branch: `codex/2026-05-26-boundary-phase1-foundation`
- Scope boundary: foundation artifacts only; no adapter runtime behavior changes and no adapter expansion.

### Preflight

- Confirmed work in Boundary repo: `/Users/td/ConceptDev/Projects/Boundary`
- Confirmed branch: `codex/2026-05-26-boundary-phase1-foundation`
- Confirmed working tree was clean before Phase 1 edits.
- Re-read the active spec plus the current release truth and adapter contract docs:
  - `docs/LAUNCH_TRUTH_FREEZE.md`
  - `docs/ADAPTER_CONTRACT.md`
  - `README.md`

### Built

- Added human and machine claims surfaces:
  - `docs/CLAIMS_LEDGER.md`
  - `claims/boundary_claims.yaml`
  - `claims/claims_test.go`
- Added a release checklist that references the claims ledger and readiness gates:
  - `docs/RELEASE_CHECKLIST.md`
- Added adapter lifecycle vocabulary and conformance validation:
  - `governance/adapter_lifecycle.go`
  - `tests/adapter_conformance/adapter_readiness_test.go`
- Added per-adapter readiness declarations for current shipped adapters:
  - `adapters/mcp/readiness.yaml`
  - `adapters/cli/readiness.yaml`
  - `adapters/codeexec/readiness.yaml`
  - `adapters/grpc/readiness.yaml`
  - `adapters/webhook/readiness.yaml`
  - `adapters/a2a/readiness.yaml`
- Added `docs/ADAPTER_READINESS_MATRIX.md` and updated `README.md` so adapters are split by maturity level.
- Updated `docs/ADAPTER_CONTRACT.md` to distinguish five-method interface conformance from ten-step lifecycle readiness.
- Updated `CHANGELOG.md` with unreleased Phase 1 entries.

### Verification

- `env -u GOROOT go test ./claims ./tests/adapter_conformance`: pass
- `env -u GOROOT go test ./... -short`: pass
- `(cd adapters/grpc && env -u GOROOT go test ./...)`: pass
- `env -u GOROOT go vet ./...`: pass
- `(cd adapters/grpc && env -u GOROOT go vet ./...)`: pass
- `git ls-files '*.go' | xargs gofmt -l`: pass, empty output
- `git diff --check`: pass

### Notes For Next Step

- Phase 1 intentionally leaves all adapters below production maturity. Follow-on specs should promote individual adapters only after lifecycle tests, bypass proof, and fail-mode evidence exist.
- The README now has no production adapter row. That is deliberate until a later adapter-specific spec proves production readiness.

## 2026-05-06 — Phase 2 Proof And Citation Cleanup

### Context

- Phase: `Phase 2 — governance-interception-layer proof/citation cleanup`
- Branch: `fix/gil-phase2-proof-citation-cleanup-2026-05-06`
- Upstream sequencing: `Fulcrum-Proofs` PR #20 and PR #21 merged; `fulcrum-trust` Phase 2 framing lane merged; this GIL lane is next in the approved order.

### Preflight

- Confirmed branch: `fix/gil-phase2-proof-citation-cleanup-2026-05-06`
- Verified working tree was clean before edits.
- Re-read the bounded drift surfaces:
  - `README.md`
  - `CONTRIBUTING.md`
  - `CITATION.cff`

### Plan

- Tighten proof-language wording in `README.md` so it stays within the approved correspondence boundary.
- Update `CONTRIBUTING.md` so formal-verification scope points upstream without implying GIL emits `proved` decisions itself.
- Reduce `CITATION.cff` to a software-only citation surface until the companion paper is publicly citation-ready.

### Built

- Updated `README.md` to describe upstream proof integration through correspondence and decision-mode boundaries rather than implying GIL emits `proved` decisions.
- Updated `CONTRIBUTING.md` to keep formal-verification scope upstream in `Fulcrum-Proofs` and outside GIL's direct decision semantics.
- Removed the paper-style `preferred-citation` block from `CITATION.cff` so the repo remains software-citation-only until a public paper citation exists.

### Verification

- `python3` YAML parse for `CITATION.cff`: pass
- `env -u GOROOT go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass
- Edited-surface scan verified no remaining `preprint forthcoming` or `preferred-citation` strings in `README.md`, `CONTRIBUTING.md`, or `CITATION.cff`

### Notes For Next Step

- If this PR lands cleanly, the Fulcrum conductor record should advance the Phase 2 queue from GIL cleanup to the next architect-approved docs-reconstruction gate.

### Notes

- Use `env -u GOROOT go ...` for verification in this repo to avoid the inherited toolchain mismatch recorded on 2026-05-03.

## 2026-05-03 — Four-Repo Style Mirror

### Context

- Spec: `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/yc/codex/PROOFS_AND_MIRROR_SPEC.md`
- Phase: `Phase C — Four-repo style mirror`
- Branch: `style-mirror-2026-05-04`

### Preflight

- Confirmed `main` is up to date with `origin/main`.
- Verified working tree was clean before branching.
- Re-ran GIL baseline with the leaked `GOROOT` override cleared for this repo:
  - `env -u GOROOT go test ./... -short -count=1 -timeout 5m`
  - Result: pass on `aea1a70f3bb39ad6a6a7ddc1a717a7a67c55abf0`

### Plan

- Add the missing public-surface files: `CITATION.cff`, `CODE_OF_CONDUCT.md`.
- Update `README.md` to mirror the four-repo architecture block used across Fulcrum repos.
- Harmonize `CONTRIBUTING.md`, `SECURITY.md`, and `CHANGELOG.md` wording with the public mirror spec.

### Built

- Added `CITATION.cff` with Apache-2.0 metadata and companion-paper citation note.
- Added `CODE_OF_CONDUCT.md` using Contributor Covenant v2.1 wording.
- Updated `README.md` to the shared "Part of the Fulcrum Architecture" layout, including GitHub cross-links to all four repos and public project-doc links.
- Updated `CONTRIBUTING.md`, `SECURITY.md`, and `CHANGELOG.md` to align email/contact language and public-facing wording with the other Fulcrum repos.

### Verification

- `python3` YAML parse for `CITATION.cff`: pass
- `env -u GOROOT go test ./... -short -count=1 -timeout 5m`: pass
- Standard-file check passed for `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `CITATION.cff`, `CODE_OF_CONDUCT.md`, and `CODEX_SESSION_LOG.md`
- README architecture block verified
- Public-surface cleanup scan verified no remaining `agent@fulcrumlayer.io`, `arXiv link TBD`, `external GStack audit`, or `Private` proof-license wording in the edited public docs

### Notes For Next Step

- The blocking failure from the earlier attempt was environmental, not repo-code: this shell inherited `GOROOT=/Users/td/.local/share/mise/installs/go/1.24.1` while `go` resolved to Homebrew `1.26.1`.
- For GIL verification in this session, run Go commands as `env -u GOROOT go ...` so the compiler and stdlib come from the same toolchain.

## 2026-05-09 — gRPC Dependency Refresh Triage

### Context

- Repo is back on `main` after landing the docs cleanup (`#15`) and the safe Redis dependabot bump (`#6`).
- Remaining open maintenance item is dependabot PR `#14`, which upgrades `google.golang.org/grpc` in `adapters/grpc`.

### Findings

- The dependabot patch is not just a library bump; it also rewrites `adapters/grpc/go.mod` from `go 1.24.0` plus `toolchain go1.24.1` to `go 1.25.0`.
- CI on the PR is green, but that toolchain drift is broader than the repo-owned change we actually want to make.

### Plan

- Refresh `google.golang.org/grpc` in `adapters/grpc` on a codex-owned branch while preserving the existing module/toolchain framing unless verification proves a change is required.
- Re-run the adapter module tests and the repo short regression gate with `env -u GOROOT`.

### Built

- Updated `adapters/grpc` from `google.golang.org/grpc v1.80.0` to `v1.81.0`.
- Accepted the generated indirect dependency refresh in the adapter module for `x/net`, `x/sys`, `x/text`, and `genproto/googleapis/rpc`.

### Verification

- `env -u GOROOT go mod tidy` in `adapters/grpc`: pass
- `env -u GOROOT go test ./... -count=1` in `adapters/grpc`: pass
- `env -u GOROOT go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass

### Decision

- The attempted preservation of `go 1.24.0` failed because the upgraded dependency chain now requires `go 1.25.0`.
- This branch therefore keeps the module directive change as a verified requirement of the `grpc@1.81.0` refresh rather than an incidental tool rewrite.

## 2026-05-09 — CI Security Scan Patch-Level Follow-Up

### Context

- PR `#16` came back blocked even though the adapter/module tests and CodeQL lanes were green.
- The failing job was `security scan`, specifically the `govulncheck` step.

### Findings

- The workflow pinned `GOTOOLCHAIN=go1.26.2`, and GitHub Actions reported two standard-library vulnerabilities fixed in `go1.26.3`: `GO-2026-4971` and `GO-2026-4918`.
- The runner also emitted the separate Node 20 deprecation warning for `actions/checkout@v4` and `actions/setup-go@v5`.

### Built

- Updated `.github/workflows/ci.yml` to use `actions/checkout@v6` and `actions/setup-go@v6` for Node 24 runner readiness.
- Bumped the security job `go-version` to `1.26.3`.
- Bumped `GOTOOLCHAIN` for `govulncheck` to `go1.26.3`.

### Notes

- This is an environment/toolchain fix, not a product-code behavior change in the adapters themselves.
- Dependabot PR `#14` remains the narrower fallback, but `#16` should clear once GitHub reruns on the patched workflow.

## 2026-05-09 — gRPC Adapter CI Version Alignment

### Context

- After the security scan fix landed, PR `#16` still failed one remaining lane: `test grpc adapter`.

### Findings

- The failure was a plain CI/version mismatch: `adapters/grpc/go.mod` now requires `go 1.25.0`, but the dedicated `grpc-adapter` job in `.github/workflows/ci.yml` was still pinned to `go-version: 1.24`.
- GitHub Actions failed before test execution with: `go.mod requires go >= 1.25.0 (running go 1.24.13; GOTOOLCHAIN=local)`.

### Built

- Updated the `grpc-adapter` workflow job to use `go-version: 1.25.0`.

### Notes

- Root repo test coverage remains intentionally broader on `1.23` and `1.24`; only the adapter-specific job needed the higher Go version because that module has its own `go.mod`.

## 2026-05-26 — Gate 1 MCP Safety Gateway

### Context

- Branch: `gate1/mcp-safety-gateway` from clean `main` at `6e9a330`.
- Project identity: Fulcrum Boundary, module `github.com/fulcrum-governance/boundary`.
- Scope: Gate 1 proof of control only. No trust integration, receipt verification, benchmarks, or compliance docs.

### Built

- Added `cmd/boundary` and `internal/boundarycli` with `serve`, `demo postgres`, `verify`, `doctor`, and `audit`.
- Added YAML static-policy loading via `governance/yaml_policy.go`.
- Extended static policies with launch-grade field matching on request arguments, including case-insensitive `contains`.
- Extended decision records with `decision_mode`, `matched_rule`, `policy_file`, `gateway_version`, and `trace_id`.
- Added `examples/mcp-postgres-gateway/` with Docker Compose topology, demo policy, and seed Postgres data.
- Added root `Dockerfile`, `Makefile`, `LIMITATIONS.md`, and `docs/DECISION_RECORDS.md`.
- Updated `README.md` to use Fulcrum Boundary naming and show the MCP Safety Gateway quickstart.

### Verification

- `env -u GOROOT go test ./...`: pass.
- `go build ./cmd/boundary`: pass.
- `go run ./cmd/boundary --help`: pass.
- `go run ./cmd/boundary serve --help`: pass.
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `docker compose -f examples/mcp-postgres-gateway/docker-compose.yml config`: pass.
- `make demo`: pass. Safe `SELECT` returned rows, `DROP TABLE` returned HTTP 403 with `matched_rule=block-drop-table`, direct Postgres bypass failed from the frontend-only demo agent, and gateway logs emitted structured decision records.

### Notes

- `make demo` exports `BOUNDARY_DEMO_PORT=18080` by default to avoid local collisions while raw compose defaults to host `8080`.
- The bypass failure observed in the verified run was DNS isolation from the frontend network: `lookup postgres ... no such host`.

## 2026-05-26 — Gate 2 Release Surface

### Context

- Branch: `gate1/mcp-safety-gateway`, continuing PR #21.
- Scope: additive Gate 2 release-surface artifacts and repo cleanliness only. No tag, GitHub Release, Gate 3 content, trust integration, receipt verification, or benchmarks.

### Built

- Added `docs/BOUNDARY_CONDITIONS.md` to define when Boundary protects a tool, when it does not, fail-closed behavior, production topology requirements, and demo-grade SQL policy scope.
- Added `docs/THREAT_MODEL.md` for the MCP Safety Gateway topology, covering bypass, policy circumvention, audit tampering, what the demo proves, and what it does not prove.
- Promoted `CHANGELOG.md` to a v0.2.0 release section while keeping an empty Unreleased section.
- Updated release-surface naming away from old GIL language in public docs, issue templates, package comments, and visible examples.
- Moved the README quickstart above architecture and added the real CI badge.
- Refreshed `docs/LAUNCH_TRUTH_FREEZE.md` so it reflects current v0.2.0 release truth rather than stale Gate 0 branch state.
- Hardened YAML policy loading against symlinked policy files and annotated the directory-scoped read for gosec.
- Tidied the nested gRPC adapter module so CI does not require a `go mod tidy` update.
- Updated GitHub topics to: `ai-agents`, `agent-governance`, `mcp`, `policy-enforcement`, `security`, `golang`.
- Tightened the demo Postgres downstream so it executes only the canned safe SELECT after governance allows it; this keeps the release demo from treating arbitrary agent-provided SQL as an executable downstream query.

### Verification

- `env -u GOROOT go test ./...`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `env -u GOROOT go test ./... -count=1 -race -cover` in `adapters/grpc`: pass.
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass.
- `docker compose -f examples/mcp-postgres-gateway/docker-compose.yml config`: pass.
- `make demo`: pass. Safe `SELECT` allowed, destructive `DROP TABLE` denied, bypass blocked, decision records emitted.
- `git diff --check`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- Release-surface stale-name scan: pass for README, CHANGELOG, docs, issue templates, module files, command code, adapters, examples, and governance package.

### Notes

- GitHub license detection via `gh repo view --json licenseInfo` returned `null`, but `gh api repos/Fulcrum-Governance/Boundary/license` detects Apache-2.0 correctly and the repository has a visible Apache 2.0 `LICENSE` file.
- Local `gosec` reported zero issues after the YAML loader hardening but emitted SSA/toolchain errors against local Go 1.26/nested-module dependencies; GitHub Actions remains the authoritative security-scan check after push.

## 2026-05-26 — Spec 3 MCP Production Adapter

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing the Boundary spec-series execution after Phase 1 was committed as `67ec4d7`.
- Scope: Spec 3 only. Promote MCP from demo adapter to production-grade governed JSON-RPC proxy while keeping non-MCP adapters below production maturity.

### Built

- Added `adapters/mcp/gateway.go`, `forwarder.go`, `identity.go`, `metadata.go`, `response_inspector.go`, and `tools_list_filter.go`.
- Extended the MCP adapter so `ForwardGoverned` forwards allowed JSON-RPC requests to an upstream MCP HTTP server and refuses denied or ungoverned requests.
- Added JSON-RPC lifecycle handling for malformed input, invalid requests, notifications, batches, request ID preservation, governed denial errors, upstream forwarding errors, and governance metadata.
- Added `tools/list` policy filtering so denied tools are removed from discovery responses.
- Updated `boundary serve --upstream` so HTTP(S) upstreams run in MCP proxy mode while Postgres DSNs continue to use the existing demo downstream.
- Added integration coverage in `tests/integration/mcp_gateway_lifecycle_test.go` for allowed-once forwarding, deny-before-upstream, metadata, `tools/list` filtering, batch requests, parse errors, fail-closed pipeline errors, and bypass probing.
- Added `docs/adapters/MCP.md` and updated adapter readiness, claims ledger, README, changelog, adapter contract, and fail-mode docs to mark only MCP as production-ready.

### Verification

- `env -u GOROOT go test ./adapters/mcp ./tests/integration ./claims ./tests/adapter_conformance`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.

### Notes

- SSE and stdio MCP transport variants remain future transport work; this phase establishes the production HTTP JSON-RPC proxy path.
- The Postgres safety demo path remains available through `boundary serve` when `--upstream` is a Postgres DSN.
