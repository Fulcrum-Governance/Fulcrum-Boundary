# Changelog

All notable changes to **Fulcrum Boundary** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Add the `proof-receipt-v0.1` sidecar: checker-validated budget and
  static-privilege invariant lines bound to a decision record by `decision_hash`.
  The sidecar is invariant evidence attached to a decision, not a `proved`
  decision mode; attaching it does not change `decision_hash`, and Boundary does
  not emit proved decisions.

- Per-host install tutorials (#138): `docs/firewall/HOST_SETUP.md` adds short
  walkthroughs for routing Claude Desktop, Claude Code, Cursor, and VS Code
  through Boundary — where each MCP config lives (per OS), the `boundary install`
  command, confirming the live route with `boundary doctor`, reversibility via
  `boundary uninstall`, and the routed-only caveat per host. Claude Code is
  documented as the repo-local `.mcp.json` client (Boundary has no dedicated
  selector). Linked from `docs/INSTALL.md` and the Route Conformance Checklist;
  the documented per-host path rows are pinned, and their components coupled to
  `internal/firewall/discover.go`, by drift tests in `tests/docs/`.
- Windows static-only stance, made explicit and pinned (#139): `docs/INSTALL.md`
  now states that Windows ships the static (`CGO_ENABLED=0`) build **only** — a
  permanent stance, not a pending gap — because the cgo SQL classifier needs a
  C/MSYS2 toolchain the Windows release path does not carry. Routed SQL on Windows
  classifies `UNKNOWN` and is denied fail-closed (it never allows SQL a cgo build
  would deny). `tests/releasebuild/` pins it: Windows stays in the static build
  and the native-cgo release matrix gains no Windows lane.
- Release supply-chain metadata (`BND-CLAIM-DIST-002`, `partial`): the
  tag-gated release pipeline now generates an SPDX SBOM (syft) for each static
  (`.goreleaser.yaml`) and native-cgo (`cgo-binaries` job) release archive and
  records GitHub build-provenance attestations for release artifacts — static
  archives, their SBOMs, `SHA256SUMS`, and each native-cgo archive and its
  SBOM — via SHA-pinned `actions/attest-build-provenance`
  (`.github/workflows/release.yml`, with `id-token`/`attestations` OIDC
  permissions). Verify with `gh attestation verify`. This is distribution
  provenance, distinct from runtime decision-record signing (Boundary does not
  sign decision records by default; see `docs/PROOF_BOUNDARY.md`, #134). Static
  SBOM generation is verified via `goreleaser release --snapshot` and the
  cgo-archive SBOM command via local syft; provenance and the cgo SBOM are
  release-gated (`BND-DIST-002`) until the first tagged release runs the updated
  pipeline. Docs: `docs/SUPPLY_CHAIN.md`; wiring pinned by `tests/supplychain/`.

- Kernel escalation await mode: a new `governance/kernel.AwaitingEscalationHandler`
  publishes the existing frozen escalate envelope (`{"request": …, "reason": …}`
  on `fulcrum.foundry.escalate`), then blocks for a bounded window (default
  120s via `BundleConfig.EscalateAwaitTimeout`; negative values are a
  `NewBundle` error) awaiting a resolution message on
  `fulcrum.foundry.escalate.resolved`, with the waiter registered under
  `request_id` before the publish. Resolutions map `approved` → allow and
  `denied` → deny (both `human_approved`); a resolver-side `expired`
  resolution, a local timeout, and every fault (publish/subscribe error,
  duplicate in-flight `request_id`, cancelled context) deny fail-closed as
  `deterministic`, faults with the reason prefix
  `escalation fault (fail-closed):`. The handler asserts no trust
  (`TrustScore: 0`, empty `TrustState`; trust fields stay pipeline-owned).
  Await mode is selected in `kernel.NewBundle` when the new bare
  `BundleConfig.Subscriber` seam is set (Boundary ships no NATS
  implementation in-repo); without it the routing-mode
  `NATSEscalationHandler` is built exactly as before. Also new:
  `BundleConfig.EscalateResolvedSubject` and an additive `Bundle.Close()`
  that releases the resolution subscription. The resolved-message wire
  contract and deployment notes are documented in `docs/INTEGRATION.md`.
  This is not a wired human-in-the-loop capability until the fulcrum-io resolver half lands and a resolver is deployed consuming the escalate subject — absent one, awaited escalations deny at the window; routed paths only.
- Pipeline escalation seam: optional `PipelineConfig.Escalation` is invoked
  for Stage-4 `ActionEscalate` decisions and its resolved verdict
  (`Action`/`Reason`/`DecisionMode`) is adopted; nil — the default, and
  always nil on the standalone path — preserves the relabel-and-return
  behavior byte-for-byte. A handler error, a nil decision, or a returned
  action outside `allow`/`deny`/`warn`/`escalate`/`require_approval` denies
  fail-closed with the `escalation fault (fail-closed):` reason prefix.
  Dry-run does not block on the await: the decision keeps the relabel path
  and the reason notes that the await was skipped.

## [0.11.0] - 2026-06-11

### Added

- `boundary completion bash|zsh|fish`: static shell completion scripts for the
  top-level and compound commands, generated from the binary's own command
  table. Static by design — regenerate after upgrades.
- Native Go fuzz targets for the three byte-level parse/hash surfaces:
  `FuzzDecisionRecordRoundTrip` (decision-hash canonicalization stability,
  seeded with the conformance vectors), `FuzzPolicyParse` (static-policy YAML
  load), and `FuzzSQLClassifier` (Postgres AST classifier fail-safe). Seeds
  run on every `go test ./...`; a separate CI `fuzz` job runs the mutation
  engine for 60s per target.
- Hermetic serve-boot integration test (`tests/serve_boot/`) asserting the
  governed deny (JSON-RPC `-32001`) before upstream on a real binary boot,
  and RESP codec unit tests for the kernel trust path's hand-rolled Redis
  protocol (`governance/trust_redis_codec_test.go`), covering encode/decode
  edges, partial-read boundaries, and malformed-input error paths.
- `docs/ADAPTER_PRODUCTION_BAR.md`: contributor-facing guide explaining the
  mechanical bar (lifecycle steps, bypass-proof delegation, fail-closed
  transports, on-disk test evidence) and the process for advancing an adapter
  from `preview` to `production`, with a worked `mcp`-vs-`webhook` example.
- Standalone TypeScript (`verifiers/typescript/`) and Rust (`verifiers/rust/`)
  decision-record verifiers, each recomputing `decision_hash` via a stock
  RFC 8785 / JCS implementation (`canonicalize` npm package; `serde_jcs`
  crate) and pinned to the Go implementation by the shared conformance-vector
  corpus, enforced in CI. Records now verify in Go, Python, TypeScript, or
  Rust; the verifiers check integrity only, not authenticity.
- Optional Ed25519 signing of decision records (off by default) via
  `PipelineConfig.ReceiptSigner` and `boundary serve --receipt-seed FILE`
  (the 64-hex seed in FILE), with `boundary verify-record --verify-signature
  --public-key <hex|file>` to check the signature over the recomputed
  `decision_hash`, failing closed on mismatch or missing signature. `serve`
  startup also fails closed (exit 1, error on stderr, before the listener
  opens) on a missing, short, or non-hex seed, so Boundary never serves
  unsigned when signing was requested. Signing never changes `decision_hash`;
  a signature attests who signed the record, not the verdict or execution, and
  key custody is the operator's; the Python/TypeScript/Rust verifiers remain
  integrity-only and do not check signatures. `mcp proxy` / install-time
  enablement is not yet wired (#143). See `docs/SIGNING.md` for setup and the
  key-custody caveat.

## [0.10.1] - 2026-06-11

### Fixed

- The v0.10.0 release-pipeline run failed before publishing any assets:
  goreleaser refused the runner's dirty git state because the evidence bundle
  is staged in an untracked `.release-extras/` directory. That directory is
  now gitignored, and the pipeline fails fast with a clear error when the
  Homebrew tap token secret is empty instead of failing after the full build.
  `v0.10.0` remains a valid source-install tag (it has no release assets);
  `v0.10.1` is content-identical plus this fix and is the first release
  published with prebuilt binaries.

## [0.10.0] - 2026-06-11

### Added

- Prebuilt distribution: a tag-gated release pipeline builds static
  (`CGO_ENABLED=0`) archives for macOS and Linux with checksums, publishes a
  container image, and pushes a Homebrew formula to
  `fulcrum-governance/homebrew-tap`. v0.10.0 is the first release published
  through it; releases up to and including v0.9.0 shipped source-only.
- Static `CGO_ENABLED=0` builds are now supported. Without cgo the Postgres AST
  classifier is unavailable: every routed SQL statement classifies as `UNKNOWN`
  and is denied fail-closed, so the static build never allows SQL the cgo build
  would deny. The `_cgo` release archives carry the full classifier.
- A standalone Python decision-record verifier under `verifiers/python/`
  recomputes `decision_hash` with the stock `rfc8785` package. Committed
  conformance vectors under `tests/conformance/` pin the Go and Python
  implementations to the same canonical bytes, enforced by a CI job.
- `boundary demo tamper-evidence`: a fixture-only forge-the-receipt demo —
  mutate a recorded verdict and watch `boundary verify-record` refuse it.
  Demos gained TTY color and a guided firewall narrative.
- A Claude Code `PreToolUse` hook integration routes hook-delivered tool calls
  through `boundary command classify` before execution. Routed-only: the hook
  governs only the calls Claude Code sends through it.
- CLI: `--version`/`-v` aliases; `boundary help <topic...>` routes to the leaf
  command's help (compound topics included); rich structured help on `init`,
  `lock`, `verify-lock`, `redteam`, `serve`, `verify`, `verify-record`,
  `audit`, and `trust`; `--json` on `verify` (`boundary.verify.v1`) and
  `verify-record` (`boundary.verify_record.v1`) with unchanged exit codes.
- `boundary doctor` now includes first-run environment diagnostics for the Go
  toolchain, cgo / C-toolchain readiness, and `go install` PATH resolution.
- `boundary doctor --report` emits a redacted JSON report for support threads.
  It removes the local `project_root` while preserving diagnostic statuses,
  routed-surface caveats, and the local-only/no-network/no-mutation flags.
- Docs: a skeptic's FAQ; a "Where Boundary Fits" category comparison; a
  standards and incident mapping; a "Govern Your MCP Server" guide; a "How we
  keep ourselves honest" page; and a CLI reference command map covering all 28
  top-level commands with new `mcp proxy`, `serve`, `audit`, and `trust`
  sections.

### Changed

- Decision-record canonicalization now follows RFC 8785 (JSON Canonicalization
  Scheme) instead of Go's default `encoding/json` output. The canonical
  preimage behind every stable hash — `decision_hash`, `request_hash`, the raw
  request-hash counterpart, and `policy_bundle_hash` — is now produced with
  lexicographic (UTF-16 code unit) key ordering, literal `<`, `>`, and `&` (no
  HTML escaping), and ECMAScript shortest-round-trip number formatting. This is
  a **pre-1.0 format change**: it recomputes decision hashes, so a record
  emitted by an older build no longer verifies under this build, and the
  committed `docs/examples/decision-record*.example.json` fixtures were
  regenerated to the new hashes. The on-the-wire record shape (field set and
  types) is unchanged. The benefit is reproducibility: an independent, stock
  RFC 8785 / JCS implementation can now recompute a record's `decision_hash`
  from the record bytes, and a standalone Python verifier ships under
  `verifiers/python/`.
- The README first-run and install paths now lead with prebuilt binaries and
  the record-trust loop; `docs/INSTALL.md` documents the static-vs-cgo choice.
- The gateway now bounds inbound HTTP request bodies and sets server
  read/write timeouts.
- Policy-evaluator faults are surfaced instead of silently swallowed, with
  opt-in fail-closed routing per transport; malformed static-policy match
  blocks now fail closed.
- The Go toolchain floor is `go1.26.4` for the patched standard library.
- The public-language lint now scans `docs-site/` recursively (Go's
  `filepath.Glob` has no `**`) plus the Command Boundary, Edit Boundary, and
  release-notes doc trees.

### Fixed

- `boundary help` with a compound topic (for example `help policy generate`)
  reaches the leaf command's help instead of stopping at the parent
  dispatcher.
- The CLI reference no longer claims `trust reset` accepts Redis backend
  flags; reset operates on the in-process standalone backend only.

## [0.9.0] - 2026-06-02

First release to include the Phase 1 policy-as-code testing lane. `v0.9.0`
preserves the Phase 0A record-trust loop from `v0.8.0` and adds `boundary test`
as the developer-facing CI lever: local policy fixtures, expected verdicts, and
non-zero exits on drift. It adds no new governed action surface, no new
transport adapter, and no production upgrade for preview surfaces.

### Added

- `boundary test`: a local, fixture-only policy-as-code runner over operator
  YAML policy bundles. Cases assert `allow`, `deny`, `warn`,
  `require_approval`, `escalate`, or expected `parse_rejection`; text and JSON
  output both report credentials/network/live-mutation as false.
  (Claim `BND-CLAIM-TEST-001`.)
- `docs/POLICY_TESTING.md` and the docs-site policy-testing stub: canonical
  operator guidance for committed policy-test corpora and CI usage.
- Release-check coverage for the committed `tests/fixtures/policy-test/cases`
  corpus, keeping the feature inside the standard release gate.
- `docs/releases/v0.9.0.md`: public release notes for the record-trust plus
  policy-testing release.

### Changed

- Public install and GitHub Action examples now target `@v0.9.0`.
- `CITATION.cff` set to `0.9.0`.
- Roadmap and CLI availability language now mark `boundary test` as included in
  `v0.9.0` while preserving the caveat that it proves local policy verdicts for
  routed fixtures only, not production route enforcement or bypass resistance.
- Decision-record documentation now distinguishes the single-record
  `decision record path:` artifact from the multi-record `decision record log:`
  JSONL stream.

## [0.8.0] - 2026-06-01

First release to include the Phase 0A "Trust the Record" lane: route-context
decision records (`DecisionRecordV2`), `boundary explain`, `boundary replay`, and
uniform record-location output. These add no new governed action surface and
upgrade no preview surface to production. MCP remains the only production route;
Command Boundary, Edit Boundary, and Secure GitHub remain preview.

### Added

- `DecisionRecordV2` route-context: additive `schema_version "2"` records carrying
  `adapter_id`, `route_id`, `topology_profile`, and `execution_claim`.
  `schema_version "1"` records stay valid and byte-compatible; the route-context
  fields are content covered by `decision_hash` (so tampering is detected).
  `topology_profile` is asserted, not attested; `execution_claim` is an adapter
  self-report and is not independently corroborated by the hashed record.
  (`governance/receipt_schema.go`; claim `BND-CLAIM-REC-001`.)
- `boundary explain <record>`: read-only rendering of a decision record
  (`schema_version "1"` or `"2"`) — verdict, reason, decision mode, matched rule,
  route context, and what each of the three hashes covers — with a stable
  `--format json` (`boundary.explain.v1`). It renders only; it does not re-verify
  hashes or prove the verdict was correct or enforced.
  (Claim `BND-CLAIM-EXPLAIN-001`.)
- `boundary replay <record> --request <file> --policies <dir>`: local,
  fixture-safe re-evaluation that recomputes `request_hash` and
  `policy_bundle_hash`, re-runs the recorded request through the same pipeline,
  and compares the decision-defining fields (`action`, `reason`, `decision_mode`,
  `matched_rule`, `policy_file`), failing closed on any mismatch. It reproduces
  the decision, not enforcement and not the absence of upstream side effects; it
  is routed-only. (Claim `BND-CLAIM-REPLAY-001`.)
- `boundary verify-record` now accepts `schema_version` of `"1"` or `"2"` and
  recomputes `decision_hash` per the record's own version (it previously rejected
  anything other than `"1"`).
- `docs/TROUBLESHOOTING.md`: a first-run troubleshooting guide covering the Go
  1.25+ and C-toolchain (cgo) requirements, `PATH` issues after `go install`,
  the failure modes of each first-run command, and how to read
  `boundary doctor --json`. It documents that a clean checkout reports `doctor`
  surfaces as `warn` and `evidence verify` as `parsed_records: 0` — both are
  the expected first-run states, not errors.
- `docs/ROUTE_CONFORMANCE_CHECKLIST.md`: a documented (not code) per-route
  checklist for the ten governance lifecycle steps and the
  experimental/preview/production maturity criteria, derived from the readiness
  matrix, with a caveat table distinguishing a governed route from a globally
  controlled system. It does not prove a deployment removed every bypass path.
- `docs/examples/`: committed, fixture-safe example artifacts — a
  `DecisionRecordV1` object and an evidence-bundle manifest excerpt — with a
  walkthrough showing bare `boundary verify-record` self-verification and why
  the optional cross-check flags do not match the fixture records.
- `docs-site` reference stubs for Route Conformance and Troubleshooting,
  registered in the published docs navigation and pointing at the canonical
  in-repo files.

### Changed

- README, `docs/CLI_REFERENCE.md`, and `docs/TROUBLESHOOTING.md` now share one
  canonical first-run command sequence (install, `selftest`, `doctor --json`,
  the two proof-lane demos, `evidence bundle`/`verify`, `verify-record`).
- Tightened the decision-record, receipt, evidence-bundle, and evidence-verify
  docs to state plainly that `upstream_called=false` / `executed=false` are
  adapter self-reports of their own control flow, are not fields of the hashed
  record, and are not independently corroborated by it; Boundary does not emit
  `proved` decisions.
- Uniform record-location output across the record-emitting commands (`demo`,
  `redteam`, `evidence`): a single stable record-path/`record_id` line and uniform
  `--out` semantics, so the find → verify → explain → replay loop is copy-paste.
  Wired into the `docs/examples/` walkthrough.
- Public install and GitHub Action examples now target `@v0.8.0`.
- `CITATION.cff` set to `0.8.0`.

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

[Unreleased]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.11.0...HEAD
[0.11.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.10.1...v0.11.0
[0.10.1]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.1.0
