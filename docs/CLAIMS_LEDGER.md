# Claims Ledger

This ledger is the Boundary-specific extension to Fulcrum's broader claims-lock
discipline. It binds public language to repo evidence so release notes, README
copy, and demo language do not outrun what the Boundary code and docs prove.

The machine-readable source is [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml).
The release gate in [`claims/claims_test.go`](../claims/claims_test.go) parses
that file and validates the evidence rules.

Public copy also follows the Boundary language system:
[`docs/LANGUAGE_SYSTEM.md`](./LANGUAGE_SYSTEM.md),
[`docs/LEXICON.md`](./LEXICON.md),
[`docs/COPY_RULES.md`](./COPY_RULES.md), and
[`docs/BOUNDARY_PRODUCT_PRIMITIVES.md`](./BOUNDARY_PRODUCT_PRIMITIVES.md).
The language lint gate in
[`claims/language_lint_test.go`](../claims/language_lint_test.go) scans public
docs for controlled overclaim phrases while preserving explicit limitation and
claim-control language.

## Status Vocabulary

| Status | Meaning |
|---|---|
| `delivered` | May be used in release notes when the claim has at least one test path and one doc path. |
| `partial` | May be used only with maturity or gap caveats. Each partial claim must name linked build tasks. |
| `planned` | Roadmap only. Do not state as current behavior. |
| `false` | Do not use as a public claim. The validation gate checks that the claim text is absent from `README.md`. |

## Public-language lists

Each claim may carry `public_language.allowed` and `public_language.forbidden`
lists in [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml).

- **`allowed`** is human-facing approved copy — phrasings sanctioned for that claim.
- **`forbidden`** is **advisory**: each entry names a capability framing the claim
  must never assert. Forbidden phrases are governed in public copy by the hardcoded
  language-lint rules ([`claims/language_lint_test.go`](../claims/language_lint_test.go))
  plus human review. They are **not** literally substring-scanned against documents —
  concept words such as `signature` and `decision hashes` appear legitimately in
  honest, hedged copy, so literal enforcement would brick truthful text.

The forbidden lists are not inert:
[`claims/forbidden_test.go`](../claims/forbidden_test.go) fails the build if any
entry is empty, duplicated within a claim, or also present in that claim's `allowed`
list, and pins the forbidden phrases currently governed by language-lint terms
(matched with the lint's own substring semantics) so the ledger and the lint
cannot silently drift apart.

> **Honest boundary.** Sentence-form overclaims in the forbidden lists that are
> *not* also hardcoded lint terms (e.g. "Secure GitHub is production", "Boundary
> controls all file writes") are governed by review, not by a document scan. The
> hardcoded Gate-2 rules do not catch them today, so this ledger does not claim
> they are machine-enforced. Opt-in literal enforcement of that safe subset is a
> documented follow-up option, intentionally not yet adopted.

## Current Claims

| ID | Status | Claim | Evidence | Public boundary |
|---|---|---|---|---|
| BND-CLAIM-001 | delivered | Boundary governs MCP Safety Gateway requests before execution when the tool route passes through Boundary. | `internal/boundarycli/cli_test.go`, `docs/BOUNDARY_CONDITIONS.md`, `docs/internal/LAUNCH_TRUTH_FREEZE.md` | Scoped to routed deployments and the Docker demo topology. |
| BND-CLAIM-002 | delivered | Boundary emits structured decision records for every governed verdict. | `governance/slog_audit_test.go`, `docs/DECISION_RECORDS.md` | Structured record emission only. Advanced record integrity details are covered by BND-CLAIM-005. |
| BND-CLAIM-003 | partial | Boundary ships one production MCP adapter and seven preview adapter/profile packages with lifecycle maturity tracked per adapter. | Adapter package tests, CLI/CodeExec/gRPC/Webhook/A2A lifecycle and conformance tests, Managed Agents integration tests, Secure GitHub fixture tests, `docs/ADAPTER_READINESS_MATRIX.md` | Use the maturity matrix. MCP is production; CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub remain preview until their named gaps are closed. |
| BND-CLAIM-004 | false | Boundary is a SQL firewall. | `docs/internal/LAUNCH_TRUTH_FREEZE.md` | Boundary includes a Postgres AST guard for statement classification, but it is not a general SQL firewall. |
| BND-CLAIM-005 | delivered | Boundary produces receipt-grade decision records. | `tests/receipt_verification_test.go`, `internal/boundarycli/cli_test.go`, `docs/RECEIPTS.md` | Receipt-grade means hash-verifiable records; do not imply signed receipts by default. |
| BND-CLAIM-REC-001 | delivered | Boundary records additive route-context (`adapter_id`, `route_id`, `topology_profile`, `execution_claim`) in `schema_version "2"` decision records covered by `decision_hash`, while V1 records stay valid. | `governance/receipt_v2_test.go`, `tests/decision_record_v2_test.go`, `docs/DECISION_RECORDS.md`, `docs/examples/decision-record-v2.example.json` | Route-context recording extends tamper-detection to those fields; it does not add topology attestation, authenticity, or independent execution corroboration. |
| BND-CLAIM-EXPLAIN-001 | delivered | `boundary explain` renders a decision record (`schema_version "1"` or `"2"`), describing the decision-defining fields, route-context, and what each hash covers, without verifying the record. | `internal/explain/explain_test.go`, `tests/explain/explain_test.go`, `docs/CLI_REFERENCE.md`, `docs/DECISION_RECORDS.md` | Explain is read-only rendering. It does not verify hashes, prove enforcement, or prove the verdict was correct. |
| BND-CLAIM-REPLAY-001 | delivered | `boundary replay` re-evaluates a recorded request against the recorded policy bundle and reproduces the recorded decision-defining fields, failing closed on any mismatch. | `internal/replay/replay_test.go`, `tests/replay/replay_test.go`, `docs/CLI_REFERENCE.md`, `docs/DECISION_RECORDS.md` | Replay reproduces the decision, not enforcement. It does not prove the action was blocked, no upstream bytes moved, or the verdict was globally correct. |
| BND-CLAIM-006 | delivered | Boundary provides a production MCP JSON-RPC proxy adapter. | `tests/integration/mcp_gateway_lifecycle_test.go`, `docs/adapters/MCP.md`, `docs/ADAPTER_READINESS_MATRIX.md` | Production MCP protection still requires deployment isolation around the upstream tool server. |
| BND-CLAIM-007 | partial | Boundary provides a preview Managed Agents proxy adapter. | `tests/integration/managed_agents_lifecycle_test.go`, `tests/integration/managed_agents_multiagent_test.go`, `docs/adapters/MANAGED_AGENTS.md` | Preview only until a live upstream conformance run is recorded. |
| BND-CLAIM-008 | delivered | Boundary includes a Postgres AST guard for classifying SQL statements before PolicyEval. | `tests/interceptors/sql_postgres_test.go`, `tests/interceptors/sql_evasion_test.go`, `docs/policies/POSTGRES.md`, `docs/POLICY_SCHEMA.md` | Do not turn this into a general SQL firewall or SQL injection prevention claim. |
| BND-CLAIM-BUILD-001 | delivered | Boundary builds with `CGO_ENABLED=0`; in no-cgo builds the Postgres AST classifier is unavailable, so every routed SQL statement classifies as `UNKNOWN` and the Postgres guard denies it fail-closed. | `interceptors/sql/ast_classifier_nocgo_test.go`, `docs/INSTALL.md`, `docs/policies/POSTGRES.md` | The static build's reduction is classification capability, not deny posture. It never allows SQL the cgo build would deny; do not describe static builds as carrying the full classifier. |
| BND-CLAIM-DIST-001 | partial | Boundary publishes one-command install channels (Homebrew tap formula, prebuilt static and native-cgo archives with SHA256 checksum manifests, container image) from a tag-gated release pipeline. | `.goreleaser.yaml`, `.github/workflows/release.yml`, `docs/INSTALL.md` | Partial until the first tag-gated release with binary assets is published (gap BND-DIST-001). Channels apply to releases after `v0.9.0`; earlier releases are source-only. Windows native-cgo archives are not published. |
| BND-CLAIM-DIST-002 | partial | Boundary's tag-gated release pipeline generates an SPDX SBOM for each static release archive and records GitHub build-provenance attestations for its release artifacts. | `tests/supplychain/wiring_test.go`, `docs/SUPPLY_CHAIN.md`, `docs/INSTALL.md` | Static-archive SBOM is verified via `goreleaser release --snapshot`; build-provenance attestation is release-gated (live from the next tagged release) and the cgo-archive SBOM is not yet generated (gap BND-DIST-002). Distribution provenance only — distinct from decision-record signing (#134); `boundary version` does not prove provenance. |
| BND-CLAIM-009 | delivered | Boundary provides standalone trust integration and adaptive termination for protected adapters. | `tests/adaptive_termination_test.go`, `tests/integration/trust_production_test.go`, `tests/integration/trust_fail_closed_test.go`, `docs/TRUST_INTEGRATION.md`, `docs/ADAPTIVE_TERMINATION.md` | Boundary can isolate repeated violators before later protected calls execute; it does not replace deployment isolation or own Fulcrum's canonical trust model. |
| BND-CLAIM-010 | delivered | Boundary defines standalone and kernel integration contracts for the Fulcrum control plane. | `tests/integration/standalone_test.go`, `tests/integration/kernel_test.go`, `docs/INTEGRATION.md`, `docs/STANDALONE_VS_KERNEL.md`, `docs/PROOF_BOUNDARY.md` | Contracts name the seams; they do not mean Boundary emits `proved` decisions or connects to every Fulcrum service without operator config. |
| BND-CLAIM-011 | delivered | Boundary inventories local MCP client configs and classifies discovered MCP server capabilities without mutating those configs. | `internal/firewall/inventory_test.go`, `tests/firewall/inventory_cli_test.go`, `docs/firewall/DISCOVERY_INVENTORY.md` | Discovery is read-only for MCP client configs; protection begins only after future install/routing steps. |
| BND-CLAIM-012 | delivered | Boundary renders inventory-derived MCP risk graphs and generates verifiable starter firewall policies. | `internal/firewall/graph_policy_test.go`, `tests/firewall/inventory_cli_test.go`, `docs/firewall/RISK_GRAPH_POLICY_GENERATION.md` | Generated policies are starter policies requiring operator review; graph output identifies tested risk paths but does not protect servers by itself. |
| BND-CLAIM-013 | delivered | Boundary installs reversible MCP config routes and verifies descriptor lockfiles for local MCP server descriptors. | `internal/firewall/install_lock_test.go`, `tests/firewall/inventory_cli_test.go`, `docs/firewall/INSTALL_LOCK.md` | Install creates a reversible Boundary route and descriptor checks; live runtime protection still requires a governed profile and reviewed policies. |
| BND-CLAIM-014 | delivered | Boundary runs fixture redteam packs that demonstrate expected deny outcomes without real secrets or live system mutation. | `internal/redteam/redteam_test.go`, `tests/redteam/redteam_cli_test.go`, `docs/firewall/REDTEAM.md` | Fixture redteam proves tested deny paths only; it is not live exploit conformance or universal MCP attack prevention. |
| BND-CLAIM-015 | delivered | Boundary provides a preview Secure GitHub MCP profile for fixture write-after-taint denial before GitHub mutation. | `adapters/securegithub/adapter_test.go`, `tests/securegithub/secure_github_cli_test.go`, `tests/redteam/secure_github_integration_test.go`, `docs/secure-mcp/GITHUB.md`, `docs/secure-mcp/GITHUB_REDTEAM.md`, `docs/secure-mcp/GITHUB_LIVE_CONFORMANCE.md`, `docs/deployment/secure-github-bypass-proofing.md` | Preview fixture proof remains distinct from live conformance. Production still requires deployment bypass evidence. |
| BND-CLAIM-016 | delivered | Boundary provides a local-only MCP Firewall dashboard over inventory, risk paths, policies, install receipts, descriptor locks, and local decision-record files. | `internal/firewall/dashboard_test.go`, `tests/firewall/inventory_cli_test.go`, `docs/firewall/DASHBOARD.md` | Local visibility only. It is not hosted monitoring and does not protect MCP servers by itself. |
| BND-CLAIM-017 | delivered | Boundary provides a GitHub Action that audits repo-local MCP configs and emits Markdown and SARIF reports. | `tests/actions/mcp_audit_fixture_test.go`, `docs/firewall/GITHUB_ACTION.md`, `actions/mcp-audit/README.md` | CI audit/reporting only. Repo-local scans are the default; runtime protection requires governed routing through Boundary. |
| BND-CLAIM-018 | delivered | Boundary provides an opt-in Secure GitHub live conformance harness for GitHub App read evidence and denied write-after-taint no-mutation proof. | `adapters/securegithub/github_app_auth_test.go`, `adapters/securegithub/live_conformance_test.go`, `internal/boundarycli/secure_github_conformance_test.go`, `tests/conformance/secure_github/conformance_test.go`, `docs/secure-mcp/GITHUB_LIVE_CONFORMANCE.md`, `docs/secure-mcp/GITHUB_LIVE_EVIDENCE.md`, `docs/internal/RELEASE_TRUTH_V050.md` | Delivered preview harness only. It proves the configured live read and denied-write no-mutation path, not production deployment bypass resistance or full GitHub MCP catalog coverage. |
| BND-CLAIM-019 | partial | Operator-owned Secure GitHub live conformance has passed against a real GitHub repository. | `docs/secure-mcp/GITHUB_LIVE_EVIDENCE.md`, `docs/internal/RELEASE_TRUTH_V050.md` | Partial until an operator-owned live conformance run is executed and a sanitized transcript evidence hash is recorded. |
| BND-CLAIM-UTIL-001 | delivered | Boundary reports local version and build metadata in text and JSON formats. | `internal/versioninfo/version_test.go`, `tests/cli_output/version_test.go`, `docs/CLI_REFERENCE.md` | Local metadata output only. The version command does not prove cryptographic release provenance, binary integrity, or supply-chain integrity. |
| BND-CLAIM-UTIL-002 | delivered | Boundary runs a fixture-only action-boundary demo across MCP, Secure GitHub, Command Boundary, and Edit Boundary examples. | `tests/demo/action_boundary_demo_test.go`, `docs/DEMO_ACTION_BOUNDARY.md` | Fixture-only demo output. It does not prove all attacks are blocked, every bypass is closed, or production deployment safety. |
| BND-CLAIM-UTIL-003 | delivered | Boundary doctor reports local first-run diagnostics, routed-surface diagnostics, routed-path caveats, and redacted support reports without remote service checks. | `tests/doctor/doctor_test.go`, `docs/DOCTOR.md` | Local diagnostics only. Doctor output is not proof that every deployment route is protected, that remote runtime enforcement is active, or that deployment bypasses are closed. |
| BND-CLAIM-UTIL-004 | delivered | Boundary creates and verifies local evidence bundles with manifest hashing and fixture-safe utility outputs. | `tests/evidence/bundle_test.go`, `tests/evidence/verify_test.go`, `docs/EVIDENCE_BUNDLE.md`, `docs/EVIDENCE_VERIFY.md` | Local evidence packaging and verification only. Evidence bundles do not prove production deployment safety or close deployment bypasses by themselves. |
| BND-CLAIM-TEST-001 | delivered | Boundary runs operator-authored policy-as-code test cases against local policy bundles and reports the verdict for each case without live mutation. | `tests/test_runner/boundary_test_runner_test.go`, `docs/POLICY_TESTING.md` | Local fixture-only policy verdict assertions for routed requests only. Does not prove production route enforcement, bypass resistance, or verdict correctness beyond supplied fixtures. |
| BND-CLAIM-CMD-001 | delivered | Boundary provides preview project-local command governance for commands routed through `boundary command run`, `boundary shell`, or project-local shims. | `internal/commandboundary/classifier_test.go`, `internal/commandboundary/executor_test.go`, `internal/commandboundary/shim_test.go`, `tests/commandboundary/run_test.go`, `docs/command-boundary/PREVIEW_CLAIMS.md`, `docs/command-boundary/DEMO.md`, `docs/internal/RELEASE_TRUTH_COMMAND_BOUNDARY.md` | Delivered preview only. Command Boundary is routed-path-only and does not prove global shell control, direct-shell protection, CI/SSH control, shell sandboxing, or production command governance. |
| BND-CLAIM-CMD-002 | delivered | Boundary runs fixture Command Boundary redteam packs that deny selected command-risk paths without live mutation. | `internal/redteam/redteam_test.go`, `tests/redteam/command_redteam_test.go`, `docs/command-boundary/REDTEAM.md` | Fixture proof only. It does not prove global shell control, direct-shell protection, or universal coding-agent safety. |
| BND-CLAIM-EDIT-001 | delivered | Boundary provides preview Edit Boundary governance for proposed file mutations routed through Boundary edit envelopes. | `internal/editboundary/classifier_test.go`, `tests/editboundary/inspect_test.go`, `tests/editboundary/apply_test.go`, `tests/redteam/edit_redteam_test.go`, `docs/edit-boundary/INSPECT.md`, `docs/edit-boundary/APPLY.md`, `docs/edit-boundary/REDTEAM.md`, `docs/edit-boundary/DEMO.md`, `docs/internal/RELEASE_TRUTH_EDIT_BOUNDARY.md` | Delivered preview only. Edit Boundary applies only to routed edit envelopes and does not prove direct editor-write protection, filesystem sandboxing, arbitrary filesystem interception, direct `git apply` control, or universal coding-agent file safety. |
| BND-CLAIM-EDIT-002 | delivered | Boundary runs fixture Edit Boundary redteam packs that deny or require approval for selected file-mutation risk paths without live project mutation. | `tests/redteam/edit_redteam_test.go`, `docs/edit-boundary/REDTEAM.md` | Fixture proof only. It does not prove direct editor-write protection, arbitrary filesystem interception, IDE control, filesystem sandboxing, or universal file safety. |
| BND-CLAIM-REC-002 | delivered | Boundary's decision record is canonicalized with RFC 8785 (JSON Canonicalization Scheme), so its decision hash is reproducible by a stock RFC 8785 implementation independent of Boundary's code. | `governance/canonicaljson_test.go`, `tests/conformance/verifier_vectors_test.go`, `verifiers/python/README.md`, `CHANGELOG.md` | The RFC 8785 / JCS statement is scoped to the decision record. The decision hash is an unkeyed integrity digest: it detects edits, not authorship; Boundary as a whole is not standards-conformant. |
| BND-CLAIM-VERIFY-002 | delivered | A standalone non-Go verifier recomputes a decision record's hash and detects a one-field forgery, pinned to the Go implementation by a shared conformance-vector corpus. | `tests/conformance/verifier_vectors_test.go`, `verifiers/python/test_boundary_verify.py`, `verifiers/python/README.md` | A Python verifier ships today; the canonical format is reproducible by any RFC 8785 implementation, but Boundary does not provide a verifier in every language. Integrity, not authenticity. |

## Release Rule

Release notes can only make uncaveated behavior claims whose status is
`delivered`. Partial claims must carry the gap language from the YAML ledger.
False claims must not appear in `README.md`, release notes, or launch copy.
Public language must also avoid controlled overclaim phrases unless they appear
in claim-control, language-control, historical, or explicit limitation context.
