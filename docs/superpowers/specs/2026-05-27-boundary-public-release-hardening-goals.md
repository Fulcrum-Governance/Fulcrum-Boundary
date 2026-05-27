# Fulcrum Boundary Public Release Hardening Goal Set

Date: 2026-05-27

Source inputs:

- `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Last_Pass_Specs.md`
- Prior Boundary release-train truth artifacts in this repo
- Current Codex operating rules for branch, merge, and release verification

## Active Parent Goal

Ship the final public release hardening train for Fulcrum Boundary so a developer can install Boundary, understand the MCP Firewall wedge, run a no-credential selftest, run the GitHub lethal-trifecta fixture demo, inspect machine-readable inventory, optionally ingest external MCP inventory records, run a repo-local MCP audit GitHub Action, and see claims/readiness truth reconciled without overclaiming production coverage.

Release theme:

> Fulcrum Boundary: the action boundary for MCP-native agents.

Core sentence:

> Before an AI agent touches a real system, Fulcrum decides whether that action is allowed.

Public hook:

> See what your AI tools can do. Block what they should not.

## Operating Policy

This goal set intentionally removes stale "Tony reviews" and "do not auto-merge" language from the source spec. Current agent operating rules apply:

- Codex-owned PRs may be squash-merged and branches deleted once the task is scoped cleanly, required verification passes, GitHub reports the PR mergeable, and no real decision gate is triggered.
- Pause for explicit approval only when work changes public-release status, performs destructive data operations, touches secrets or billing, changes legal/compliance posture, violates controlled editing protocol, bypasses branch protection, or has ambiguous high-risk consequences.
- Each implementation subgoal starts from clean `main`, uses a dedicated `codex/2026-05-27-*` branch, updates `CODEX_SESSION_LOG.md`, verifies locally, pushes, opens a PR, merges when green, deletes the branch, and resyncs `main`.
- Keep README, claims ledger, adapter readiness, release truth, changelog, and public docs synchronized with tested repo evidence.
- Do not weaken validation gates to make claims pass.

## Global Guardrails

- Do not claim universal prompt-injection prevention, universal agent safety, universal SQL injection prevention, secure sandboxing, or full GitHub production security.
- Do not claim all adapters are production. MCP remains the only production adapter unless readiness evidence changes.
- Secure GitHub remains preview until live GitHub App conformance and deployment bypass evidence are recorded.
- Boundary protects only routed tools. Always distinguish governed routes from bypass paths.
- Generated policies are starter policies for operator review, not complete production policy guarantees.
- Dashboard output remains local-only unless a hosted service is actually implemented.
- External MCP inventory ingestion is fixture-proven mapping, not official named third-party scanner integration.
- Do not shell out to, import, depend on, endorse, or claim compatibility with any named third-party scanner.
- Do not perform real system mutation by default in demos, selftests, redteam packs, or release checks.
- The CLI binary remains `boundary`.
- Every user-facing command added in this train supports `--help`.
- Every machine-readable output added in this train has fixture tests.
- Every demo command states what it proves and what it does not prove.

## Success Criteria

- `go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest` is documented and verified when a tag or proxy state makes `@latest` reliable; branch or commit verification may use an explicit SHA.
- `boundary selftest` passes without credentials, network mutation, or external setup.
- `boundary demo github-lethal-trifecta` runs in a clean local environment and emits a decision record.
- README presents the five-minute path before deep architecture.
- Inventory output supports JSON, Markdown/SARIF where already present, and NDJSON records.
- External MCP inventory NDJSON fixtures ingest into Boundary inventory records with explicit scope limits.
- GitHub Action audits repo-local MCP configs and does not scan host home directories in CI.
- Claims validation passes and public claims match evidence.
- Final release truth report records feature status, adapter/profile maturity, remaining preview gates, and forbidden language.

## Goal Queue

Codex has one active `/goal` slot, so this release train runs as one parent goal with sequential subgoals. The heavy selftest/demo work is split into two implementation branches so each branch has a tighter verification surface.

### Subgoal 0 - Planning Artifact

Objective: Establish this refined release-train goal set from `Last_Pass_Specs.md`, current Boundary truth artifacts, and current merge-autonomy rules.

Branch:

- `codex/2026-05-27-boundary-release-next`

Deliverables:

- `docs/superpowers/specs/2026-05-27-boundary-public-release-hardening-goals.md`
- `CODEX_SESSION_LOG.md` update

Verification:

- `git diff --check`

### Subgoal 1 - Public README And Copy Polish

Objective: Make the public entry surface five-minute useful before architecture-heavy details.

Suggested branch:

- `codex/2026-05-27-public-release-polish`

Deliverables:

- README reordered around hero, install, five-minute demo, what it proves/does not prove, MCP Firewall, Secure GitHub preview, MCP Safety Gateway/Postgres demo, adapter maturity, library quickstart, architecture, and claims language.
- `docs/PUBLIC_RELEASE_COPY.md`
- Changelog and claims docs updates only if the public language surface changes claim status or proof references.

Key acceptance:

- README hero uses: "The action boundary for MCP-native agents."
- Install docs include `go install`, `boundary --help`, and source clone/run paths.
- No Homebrew or package-manager claim lands unless the channel exists.
- The five-minute demo path points to `boundary selftest` and `boundary demo github-lethal-trifecta`, with availability coordinated with Subgoals 2 and 3.
- "What this proves" and "What this does not prove" tables avoid universal safety claims.

Verification:

- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`
- `git diff --check`

### Subgoal 2 - Boundary Selftest

Objective: Add a no-credential selftest command that proves the local release surface boots and core fixtures still work.

Suggested branch:

- `codex/2026-05-27-selftest`

Deliverables:

- `boundary selftest`
- `boundary selftest --json`
- `boundary selftest --no-color`
- `internal/boundarycli/selftest.go`
- `internal/selftest/`
- `tests/selftest/selftest_cli_test.go`
- `docs/SELFTEST.md`

Selftest checks:

1. CLI boots.
2. Inventory fixture loads.
3. Risk graph fixture renders.
4. Policy generator emits valid starter policies.
5. Descriptor lock baseline has no drift.
6. Descriptor lock detects modified drift.
7. GitHub lethal-trifecta redteam fixture denies.
8. Secure GitHub live mode fails closed.
9. Decision record is emitted.
10. Output points to `go test ./claims/...` for claims validation without running the full claims suite by default.

Key acceptance:

- No credentials, no network mutation, no real system mutation.
- Text output is human-readable and stable enough for fixture tests.
- JSON output is machine-readable and fixture-tested.
- Failure output explains the failing check and the local command to rerun it.

Verification:

- `go test ./internal/selftest/... -count=1 -timeout 5m`
- `go test ./tests/selftest/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 3 - GitHub Lethal-Trifecta Demo Command

Objective: Add a one-command fixture demo that shows tainted GitHub context followed by denied private-repo mutation before upstream execution.

Suggested branch:

- `codex/2026-05-27-github-lethal-trifecta-demo`

Deliverables:

- `boundary demo github-lethal-trifecta`
- `--json`
- `--markdown`
- `--out demo-report.md`
- `--dashboard`
- `internal/boundarycli/demo_github.go`
- `internal/demo/github_lethal_trifecta.go`
- `tests/demo/github_lethal_trifecta_demo_test.go`
- `docs/DEMO_GITHUB_LETHAL_TRIFECTA.md`

Key acceptance:

- Demo creates an isolated temp workspace unless an explicit output path is provided.
- Demo runs inventory, risk graph, starter policy generation, policy verification, Secure GitHub fixture setup, redteam execution, decision-record emission, and optional local dashboard artifact generation.
- Demo reports expected action `DENY`, actual action `DENY`, reason `lethal_trifecta_detected`, and `upstream_called=false`.
- Demo states what it proves and what it does not prove.
- No credentials, no network mutation, no destructive real-system target.

Verification:

- `go test ./internal/demo/... -count=1 -timeout 5m`
- `go test ./tests/demo/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 4 - Inventory NDJSON Record Schema

Objective: Make MCP Firewall inventory ingestible by other tools through a stable NDJSON record stream.

Suggested branch:

- `codex/2026-05-27-inventory-ndjson-schema`

Deliverables:

- `boundary inventory --format ndjson`
- `boundary inventory --out <path>` support for NDJSON where needed
- `schemas/boundary-inventory-record.v1.json`
- `internal/firewall/ndjson.go`
- `internal/firewall/records.go`
- `internal/firewall/run_summary.go`
- `docs/firewall/INVENTORY_RECORDS.md`
- `tests/firewall/ndjson_output_test.go`

Record types:

- `scan_start`
- `agent_client`
- `mcp_config`
- `mcp_server`
- `tool_descriptor`
- `tool_capability`
- `risk_path`
- `policy_recommendation`
- `descriptor_lock_status`
- `install_status`
- `decision_record_ref`
- `scan_summary`

Key acceptance:

- Every line is valid JSON.
- `scan_start` and `scan_summary` are always emitted.
- A snapshot is complete only when `scan_summary.status=complete` is emitted.
- No secrets are serialized.
- Fixture output validates against the schema.
- Scope remains MCP agent clients, configs, servers, tools, descriptors, risk paths, governed/ungoverned routes, policy recommendations, and descriptor locks. It does not become a full SBOM, EDR, package vulnerability scanner, or universal endpoint inventory.

Verification:

- `go test ./internal/firewall/... -count=1 -timeout 5m`
- `go test ./tests/firewall/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 5 - External Inventory Ingest

Objective: Ingest Boundary, generic, and external MCP inventory NDJSON records into Boundary inventory records without claiming official external-product integration.

Suggested branch:

- `codex/2026-05-27-external-inventory-ingest`

Deliverables:

- `boundary inventory ingest --file inventory.ndjson`
- `--source generic`
- `--source external-mcp`
- `--out`
- `--format json`
- `--summary`
- `internal/firewall/external_ingest.go`
- `internal/firewall/external_mapping.go`
- `internal/firewall/external_summary.go`
- `internal/boundarycli/inventory_ingest.go`
- `docs/firewall/EXTERNAL_INVENTORY_INGEST.md`
- `fixtures/external-inventory/generic-mcp.ndjson`
- `fixtures/external-inventory/external-mcp-inventory.ndjson`
- `fixtures/external-inventory/mixed-endpoint.ndjson`
- `tests/firewall/external_ingest_test.go`

Key acceptance:

- Boundary inventory NDJSON round-trips through ingest.
- Generic NDJSON maps recognizable MCP fields such as `mcp`, `mcpServers`, `server_name`, `server`, `command`, `args`, `launcher`, `npx`, `uvx`, `docker`, `claude_desktop_config.json`, `mcp.json`, and `.mcp.json`.
- External MCP fixture records are supported only as fixture-proven mapping, not official compatibility.
- Package/extension findings become `external_inventory_component` or `external_exposure_finding` for reporting only unless they map to an MCP action path.
- If no complete summary exists, ingest warns, marks the snapshot partial, and does not use it for install recommendations unless `--allow-partial` is explicit.
- No shell-out to, import from, runtime dependency on, endorsement of, or compatibility claim with any named third-party scanner.

Verification:

- `go test ./internal/firewall/... -count=1 -timeout 5m`
- `go test ./tests/firewall/... -run Ingest -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 6 - GitHub Action MCP Audit

Objective: Provide a repo-local MCP audit GitHub Action that runs Boundary inventory and reporting in CI without scanning host home directories.

Suggested branch:

- `codex/2026-05-27-github-action-mcp-audit`

Deliverables:

- `actions/mcp-audit/action.yml`
- `actions/mcp-audit/README.md`
- `scripts/actions/mcp-audit.sh`
- `tests/actions/mcp_audit_fixture_test.go`
- `docs/firewall/GITHUB_ACTION.md`

Key acceptance:

- Action usage is documented as `Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0`.
- Inputs include `root`, `format`, `sarif`, `fail-on-critical`, and `include-defaults`.
- Outputs include `critical-count`, `high-count`, `report-path`, and `sarif-path`.
- Default CI behavior scans repo-local MCP configs only.
- Optional policy generation is dry-run.
- Step summary is Markdown.
- SARIF output is uploaded only when requested and generated.

Verification:

- `go test ./tests/actions/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 7 - Install And Release Workflow Polish

Objective: Make install, selftest, demo, and release verification one-command friendly from source.

Suggested branch:

- `codex/2026-05-27-install-release-polish`

Deliverables:

- `make selftest`
- `make demo-github`
- `make release-check`
- `scripts/selftest.sh`
- `scripts/demo-github.sh`
- `scripts/release-check.sh`
- `docs/INSTALL.md`

Key acceptance:

- Scripts use `set -euo pipefail`, print commands, and do not require live credentials or real mutation.
- `make release-check` runs full tests, gRPC adapter tests, tests suite, claims, policy verification, `verify-record --help`, selftest, and GitHub demo.
- No nonexistent distribution channel is claimed.
- Install docs cover Go install, source install, first useful commands, uninstall, and future-only Homebrew placeholder if mentioned at all.

Verification:

- `make selftest`
- `make demo-github`
- `make release-check`
- `git diff --check`

### Subgoal 8 - Final Public Release Truth Reconciliation

Objective: Reconcile README, claims, readiness, changelog, install docs, demo docs, external ingest docs, GitHub Action docs, and release truth after all selected hardening work lands.

Suggested branch:

- `codex/2026-05-27-final-public-truth`

Deliverables:

- `docs/RELEASE_TRUTH_PUBLIC.md`
- README, claims, readiness, changelog, and truth-freeze updates only where needed to remove drift.

Key acceptance:

- Full release check passes.
- README first-run path works.
- Secure GitHub remains preview.
- Generated policies remain starter/operator-reviewed.
- Dashboard remains local-only.
- GitHub Action is scoped to repo-local MCP audit.
- External ingest is framed as record mapping, not official external-product compatibility.
- False claims and forbidden language are absent outside approved claim-control or historical contexts.
- Remaining preview/partial work is explicit.

Verification:

- `make release-check`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`
- `go run ./cmd/boundary selftest`
- `go run ./cmd/boundary demo github-lethal-trifecta`
- `git diff --check`

## Final Public Product Line

After this train, the intended public line is:

- Boundary Firewall: discovery, risk graph, starter policy generation, install, descriptor lock, redteam fixture, local dashboard, inventory records, external ingest, and repo-local CI audit.
- Secure MCP: the governed MCP-server contract.
- Secure GitHub MCP: flagship preview fixture profile for write-after-taint denial before upstream execution.
- Fulcrum Platform: future hosted control plane for team policy, approvals, audits, evidence, trust, budgets, and formal correspondence.

Final release feel:

> Inventory shows what exists. Boundary decides what can act.
