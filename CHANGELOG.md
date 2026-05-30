# Changelog

All notable changes to **Fulcrum Boundary** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.0] - 2026-05-30

First open-source launch release. Launch-prep hardening over the v0.6.x utility
train; no preview surface is upgraded to production.

### Added

- A second user-facing proof lane: `boundary demo command-secret-exfil` denies a
  routed `curl -d @.env …` secret exfiltration before execution (`executed=false`,
  `class=C6`) with a decision record, alongside Lane 1 `boundary demo
  github-lethal-trifecta` (MCP). `boundary redteam --pack command-secret-exfil`
  remains the underlying fixture/evidence path.
- `docs/BOUNDARY_SPEC.md` as the authoritative in-repo launch spec and
  language-control document.
- `docs/TESTING.md` and a pull-request template documenting the test posture and
  no-`testing.Short()` full-suite convention.

### Changed

- README and public copy lead with the routed-agent-tools top-line ("the action
  boundary for routed agent tools") and the two-lane proof spine, replacing the
  narrower MCP-native framing as the identity.
- Public install and GitHub Action examples now target `@v0.7.0` for the launch
  release.
- Widened the public-surface guard (`scripts/assert-no-internal-public-artifacts`)
  to enumerate every tracked text file, so internal planning/session artifacts
  cannot re-accrete in any tracked path.
- Tightened claim and version precision across the docs, including
  `CITATION.cff` (now `0.7.0`), and reorganized internal release-truth docs under
  `docs/internal/`.

### Fixed

- Fixed the CGO/Docker build: the `Dockerfile` now builds with `CGO_ENABLED=1`
  and a C toolchain so the cgo-linked Postgres SQL classifier
  (`pganalyze/pg_query_go`) compiles, and the README documents the C-toolchain
  prerequisite.

## [0.6.1] - 2026-05-28

### Added

- `boundary version` text and JSON output for local release metadata, module
  path, Go runtime, and build-info fallback.
- `boundary demo action-boundary` fixture-only demo spanning MCP / Secure
  GitHub, Command Boundary, and Edit Boundary without credentials, network
  calls, or live mutation.
- Refitted `boundary doctor` local diagnostics for MCP, Command Boundary, and
  Edit Boundary routed-surface readiness and bypass caveats.
- `boundary evidence bundle` and `boundary evidence verify` for local evidence
  manifests, SHA-256 artifact hashes, fixture-safe utility outputs, and bundle
  integrity checks.

### Changed

- `make release-check` now covers the v0.6.x utility train: version, action
  boundary demo, doctor JSON, evidence bundle, and evidence verify.
- Public install and GitHub Action examples now target `@v0.6.1` for
  repeatable utility-train installs.

## [0.6.0] - 2026-05-28

### Added

- Filesystem/Edit Boundary preview with `boundary edit inspect`,
  `boundary edit apply`, edit decision records, and fixture-only edit redteam
  packs for selected file-mutation risk paths.

### Changed

- Public install and GitHub Action examples now target `@v0.6.0` for
  repeatable Edit Boundary preview installs.

## [0.5.0] - 2026-05-28

### Added

- Secure GitHub live conformance preview harness with GitHub App auth,
  sanitized read evidence, denied write-after-taint no-mutation proof, and
  `boundary secure github conformance` commands.

### Changed

- Public install and GitHub Action examples now target `@v0.5.0` for
  repeatable Secure GitHub live conformance preview installs.

## [0.4.0] - 2026-05-27

### Added

- Command Boundary preview with `boundary command classify`, `boundary command run`, project-local shims, `boundary shell`, command decision records, and fixture-only command redteam packs.

### Changed

- Public install and GitHub Action examples now target `@v0.4.0` for repeatable Command Boundary preview installs.

## [0.3.0] - 2026-05-27

### Added

- MCP Firewall `boundary init` and `boundary inventory` commands with read-only config discovery, capability classification, and JSON, Markdown, and SARIF reports.
- MCP Firewall `boundary graph` and `boundary policy generate` commands with inventory-derived risk paths and verifiable starter policy templates.
- MCP Firewall `boundary install`, `boundary uninstall`, `boundary lock`, and `boundary verify-lock` commands with dry-run support, byte-for-byte backup restore, install receipts, and descriptor drift checks.
- MCP Firewall `boundary redteam` command with fixture-only GitHub lethal-trifecta deny proof, decision-record output, and reserved attack-pack stubs.
- `boundary selftest` no-credential local release smoke test covering inventory, risk graph, starter policies, descriptor drift, GitHub lethal-trifecta redteam, Secure GitHub fail-closed live mode, and decision-record emission.
- `boundary demo github-lethal-trifecta` fixture command with text, JSON, Markdown, report output, and optional local dashboard artifacts for the Secure GitHub write-after-taint denial path.
- MCP Firewall `boundary inventory --format ndjson` record stream with a versioned JSON schema for tool-ingestible discovery output.
- MCP Firewall `boundary inventory ingest` command for Boundary, generic, and external MCP inventory NDJSON mapping.
- MCP Firewall GitHub Action for repo-local MCP config audits with Markdown summaries and optional SARIF output.
- MkDocs Material docs-site skeleton with GitHub Pages workflow and strict docs build target.
- CLI reference, stable example output files, and intentional help text for first-run, firewall, demo, Secure GitHub, inventory ingest, install/uninstall, dashboard, and release verification commands.
- Repository presentation guidance with description, topics, badge policy, social preview source, and static walkthrough asset plan.
- Final no-vendor repository presentation reconciliation, including README
  install copy alignment to `@main` until a post-rename release tag exists.
- Install and release workflow polish with `make selftest`, `make demo-github`, `make release-check`, and `docs/INSTALL.md`.
- Final public release truth report covering README first-run status, claims, feature status, adapter/profile maturity, install status, external inventory ingest, and forbidden release language.
- MCP Firewall `boundary dashboard` command with local-only text, JSON, HTML, and loopback server views over inventory, risk paths, policies, install receipts, descriptor locks, and local decision-record files.
- Secure GitHub MCP preview profile with fixture setup/serve commands, taint tracking, one-repo-per-session policy, W1/W2 write-after-taint denial before upstream, and decision-record output.
- Added fixture-safe demo/docs for the GitHub write-after-taint path.
- Firewall + Secure GitHub release truth reconciliation report tying claims, readiness, launch docs, and demo copy to current evidence.
- Secure MCP contract, server template, tool taxonomy, and profile docs for claim-safe governed MCP profile development.
- Fulcrum Boundary language system docs, lexicon, copy rules, product primitives, and a public-language lint gate for controlled overclaim phrases.
- Boundary claims ledger with a machine-readable release truth gate.
- Adapter readiness declarations and reusable lifecycle conformance tests.
- Release truth reconciliation report tying claims, adapter maturity, changelog, README, and launch truth freeze to current repo evidence.
- Adapter readiness matrix and release checklist linking claims and readiness gates.
- Production MCP JSON-RPC proxy path with governed forwarding, response metadata, tools/list filtering, batch handling, and lifecycle tests.
- Preview Managed Agents adapter with policy-driven tool confirmations, per-thread budget and trust tracking, and credential-bound bypass proofing docs.
- Preview A2A governed lifecycle adapter with protocol snapshot, denial shaping, governed forwarding, response inspection, governance metadata, and fail-closed handling for malformed or unsupported mandatory fields.
- Preview CLI governed command execution lifecycle with deny-before-execute behavior, `os/exec` forwarding, decision records, metadata, and direct-shell bypass limitation coverage.
- Preview CodeExec governed execution lifecycle with policy-gated forwarding, explicit execution-boundary metadata, sandbox-policy denials, output inspection, and direct-execution bypass limitation coverage.
- Preview gRPC unary lifecycle with governance response trailers, best-effort response inspection, fail-closed interceptor behavior, and explicit streaming limitations.
- Preview Webhook mode split between post-execution informational audit and pre-execution approval, with execution-mode deny-before-forwarding coverage.
- Policy schema v1 validation, richer PolicyEval request projection, and a Postgres AST guard with a 30+ case evasion corpus.
- Standalone trust integration with adaptive termination, trust transition audit events, and `boundary trust show`.
- Standalone and kernel integration contracts with runtime config validation and proof-correspondence documentation.

### Changed

- README now leads with a one-minute selftest/demo path, Mermaid diagrams,
  demo success signals, docs navigation, and release verification commands.
- Raised the root module and CI baseline to Go 1.25 to consume patched dependency releases.
- Reordered README first-run copy around install, five-minute demo, claim boundaries, MCP Firewall, and Secure GitHub preview language.
- Renamed repo and module path from `boundary` to `fulcrum-boundary` for naming consistency across the Fulcrum repo family.
- README transport adapter documentation now separates adapter maturity.

### Fixed

- Resolved active Dependabot alerts by upgrading `github.com/jackc/pgx/v5` to `v5.9.2` and removing the stale indirect `golang.org/x/crypto` dependency from the root module graph.

## [0.2.0] - 2026-05-26

### Added

- `boundary` CLI with `serve`, `demo`, `verify`, `doctor`, and `audit` subcommands.
- MCP Safety Gateway Docker demo with bypass-resistant network topology.
- YAML policy loading from configuration files.
- Structured decision records with `matched_rule`, `policy_file`, and `gateway_version` fields.
- Six transport adapters: MCP, CLI, CodeExec, gRPC, Webhook, and A2A (experimental).
- HTTP middleware for custom integrations.
- Release-surface documentation: `docs/DECISION_RECORDS.md`, `docs/LIMITATIONS.md`, `docs/BOUNDARY_CONDITIONS.md`, `docs/THREAT_MODEL.md`, and `docs/LAUNCH_TRUTH_FREEZE.md`.
- `DecisionMode` type with epistemic labels attached to every `GovernanceDecision` and `AuditEvent`.
- `ParseError` typed error for uniform adapter failure semantics across MCP, CLI, and code-execution transports.
- Dry-run test coverage for deny-to-allow rewrite and audit emission paths.
- Transport fail-mode matrix at `docs/security/FAIL_MODE_MATRIX.md`.

### Changed

- Renamed the OSS project and module path to Fulcrum Boundary at `github.com/fulcrum-governance/fulcrum-boundary`.
- Extracted `PolicyEvaluator` as a named interface with explicit error-path tests.
- Aligned public docs around the pre-execution action-boundary framing.
- Added README cross-links, `CITATION.cff`, and `CODE_OF_CONDUCT.md` for the Fulcrum public-surface standard.

### Fixed

- Security-critical transports now fail closed when policy evaluation errors.
- Post-v0.1.0 hardening fixes surfaced by external code review.

## [0.1.0] - 2026-04-15

Initial public release of the project now known as Fulcrum Boundary.

### Added

- Protocol-agnostic pre-execution enforcement for AI agent tool calls.
- Four-stage governance pipeline: trust, static policy, domain interceptor, and portable policy evaluator.
- Initial transport adapters for MCP, CLI, and code execution.
- Apache 2.0 license.

---

[Unreleased]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.1.0
