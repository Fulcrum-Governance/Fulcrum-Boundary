# Changelog

All notable changes to **Fulcrum Boundary** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Boundary claims ledger with a machine-readable release truth gate.
- Adapter readiness declarations and reusable lifecycle conformance tests.
- Adapter readiness matrix and release checklist linking claims and readiness gates.

### Changed

- README transport adapter documentation now separates production, preview, and experimental maturity.

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

- Renamed the OSS project and module path to Fulcrum Boundary at `github.com/fulcrum-governance/boundary`.
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

[Unreleased]: https://github.com/Fulcrum-Governance/Boundary/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/Fulcrum-Governance/Boundary/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Fulcrum-Governance/Boundary/releases/tag/v0.1.0
