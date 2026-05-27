# Fulcrum Boundary Firewall + Secure GitHub Goal Set

Date: 2026-05-27

Source inputs:

- `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Execution_Pack.md`
- `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Context_from_GPT.md`
- `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/active-builds/secure-github-mcp/SPEC.md`

## Active Parent Goal

Ship the Fulcrum Boundary Firewall + Secure GitHub MCP release train: codify the language system, build the MCP Firewall install wedge, define the Secure MCP contract, implement Secure GitHub MCP as the flagship preview profile, prepare the demo/YC launch surface, and complete final claims/readiness truth reconciliation under current merge-autonomy rules.

## Operating Policy

This plan intentionally removes stale "Tony reviews" and "do not auto-merge" language from the source execution pack. Current agent operating rules apply:

- Codex-owned PRs may be squash-merged and branches deleted once the task is scoped cleanly, required verification passes, GitHub reports the PR mergeable, and no real decision gate is triggered.
- Pause for explicit approval only when work changes public-release status, performs destructive data operations, touches secrets or billing, changes legal/compliance posture, violates controlled editing protocol, bypasses branch protection, or has ambiguous high-risk consequences.
- Keep all claim, readiness, README, changelog, and release-truth updates synchronized with repo evidence.
- Do not weaken validation gates to make claims pass.

## Guardrails

- Lead with the concrete action boundary, not broad platform language.
- Do not claim universal prompt-injection prevention, universal agent safety, SQL firewalling, secure sandboxing, or full GitHub production security.
- MCP remains the production adapter unless readiness evidence changes.
- CLI, CodeExec, gRPC, Managed Agents, Webhook, A2A, and Secure GitHub remain preview unless their production gates are explicitly satisfied.
- Secure GitHub MCP is Boundary-native for this release. Do not split it into a new repository or new binary unless implementation evidence shows the Boundary CLI cannot carry it cleanly.
- The `boundary` CLI name remains unchanged.
- BYO GitHub App is the intended live-auth model, but fixture redteam evidence lands first. Live GitHub App integration is required before any production GitHub claim.
- Avoid unverified competitive claims such as "no one else detects this."

## Source-Inspired Improvements

The Secure GitHub source spec sharpens the demo, but some language is too broad for the current release surface. This goal set preserves the useful product primitives and tightens the claims boundary:

- Keep "lethal trifecta" as the fixture demo: untrusted GitHub content enters agent context, then the agent attempts a private-repo mutation, and Boundary denies before the GitHub action executes.
- Implement the first Secure GitHub profile inside Fulcrum Boundary with `boundary secure github ...` commands.
- Start with the minimal high-value tool set: `get_issue`, `get_pull_request`, `get_file_contents`, `create_issue`, `create_pull_request`, `create_or_update_file`, `push_files`, and `merge_pull_request`.
- Document the larger GitHub tool taxonomy as a contract and roadmap, not as delivered runtime coverage.
- Use one-repo-per-session, repo allowlists, risk classes, taint sources, and write-after-taint denial as the release-proof path.
- Treat W1/W2 private-repo mutations after taint as the initial hard-deny claim. W0 external-publication behavior can be configured and tested before it becomes a delivered claim.
- Keep Managed Agents, semantic judge, full toxic-combination enforcement, and formal verification as integration paths unless the release branch implements and verifies them.
- Keep fixture redteam proof separate from live GitHub App conformance proof.

## Goal Queue

Codex has one active `/goal` slot, so this release train should run as a parent goal with sequential subgoals. Each subgoal gets its own scoped branch and closes cleanly before the next implementation-heavy branch starts.

### Subgoal 0 - Preflight And Planning Artifact

Objective: Establish the refined release-train goal set from the sprint execution pack, source Secure GitHub spec, current Boundary truth artifacts, and current merge-autonomy rules.

Branch:

- `codex/2026-05-27-boundary-firewall-securegithub-goals`

Deliverables:

- `docs/superpowers/specs/2026-05-27-boundary-firewall-securegithub-goals.md`
- `CODEX_SESSION_LOG.md` update

Verification:

- `git diff --check`

### Subgoal 1 - Language System Lock

Objective: Make the Fulcrum action-boundary lexicon canonical inside Boundary before new public feature work lands.

Suggested branch:

- `codex/2026-05-27-language-system-lock`

Deliverables:

- `docs/LANGUAGE_SYSTEM.md`
- `docs/LEXICON.md`
- `docs/COPY_RULES.md`
- `docs/BOUNDARY_PRODUCT_PRIMITIVES.md`
- `claims/language_lint_test.go`
- README, claims ledger, and changelog alignment as needed

Key acceptance:

- Canonical sentence is defined: "Before an AI agent touches a real system, Fulcrum decides whether that action is allowed."
- Forbidden public phrases are machine-checked outside approved historical/claim-control contexts.
- README hero uses action-boundary language without over-broad governance-platform framing.

Verification:

- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 2 - Secure MCP Contract

Objective: Define Secure MCP as a governed MCP-server pattern before Secure GitHub becomes the flagship implementation.

Suggested branch:

- `codex/2026-05-27-secure-mcp-contract`

Deliverables:

- `docs/SECURE_MCP_CONTRACT.md`
- `docs/SECURE_MCP_SERVER_TEMPLATE.md`
- `docs/SECURE_MCP_TOOL_TAXONOMY.md`
- `docs/secure-mcp/README.md`
- Claims updates only when supported by docs/tests

Key acceptance:

- Contract includes descriptor hashes, capability classification, source/sink classification, mutation classification, tenant/resource scope, taint hooks, policy projection, deny-before-execution behavior, decision records, and bypass model.
- Secure MCP docs do not imply every secure server exists or is production-ready.

Verification:

- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 3 - Firewall Discovery And Inventory

Objective: Make Boundary useful within minutes by discovering local MCP client configs and inventorying MCP server capabilities.

Suggested branch:

- `codex/2026-05-27-firewall-discovery-inventory`

Deliverables:

- `boundary init`
- `boundary inventory`
- MCP client/config discovery for Claude Desktop, Cursor, VS Code, repo-local `.mcp.json`, and repo-local `mcp.json`
- Capability and risk classification
- JSON, Markdown, and SARIF inventory reports
- Fixture configs and docs under `docs/firewall/`

Key acceptance:

- Discovery is read-only unless a future install command is explicitly invoked.
- Inventory can identify GitHub read/write capabilities and other high-risk categories from fixtures.
- Claims are added only for tested discovery/inventory surfaces.

Verification:

- `go test ./internal/firewall/... -count=1 -timeout 5m`
- `go test ./tests/firewall/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 4 - Firewall Risk Graph And Policy Generation

Objective: Turn inventory into visible risk paths and starter policies.

Suggested branch:

- `codex/2026-05-27-firewall-graph-policy`

Deliverables:

- `boundary graph`
- `boundary policy generate`
- Risk path detection for untrusted input to private data, external sinks, privileged mutations, descriptor changes, destructive DB actions, filesystem exfil, and repo write paths
- Policy templates for filesystem, GitHub, Postgres, Slack, shell, and descriptor integrity
- Docs under `docs/firewall/`

Key acceptance:

- Generated policies pass `boundary verify --policies <generated-policy-dir>`.
- Risk graph output supports JSON and Mermaid.
- Claims distinguish generated starter policies from production policy completeness.

Verification:

- `go test ./internal/firewall/... -count=1 -timeout 5m`
- `go test ./tests/firewall/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 5 - Firewall Install, Uninstall, And Descriptor Lock

Objective: Safely route MCP client configs through Boundary and detect descriptor changes without risking user config loss.

Suggested branch:

- `codex/2026-05-27-firewall-install-lock`

Deliverables:

- `boundary install`
- `boundary uninstall`
- `boundary lock`
- `boundary verify-lock`
- Byte-for-byte backup/restore
- Dry-run mode
- Install receipt
- Descriptor lockfile and hash verification docs

Key acceptance:

- Install rewrites fixture config only through explicit command invocation.
- Uninstall restores the fixture backup byte-for-byte.
- Dry-run changes nothing.
- Descriptor change behavior is documented as warn, require approval, or deny depending policy mode.

Verification:

- `go test ./internal/firewall/... -count=1 -timeout 5m`
- `go test ./tests/firewall/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 6 - Redteam Framework And GitHub Lethal-Trifecta Fixture

Objective: Add safe fixture attacks that prove Boundary blocks dangerous paths without mutating real systems.

Suggested branch:

- `codex/2026-05-27-redteam-lethal-trifecta`

Deliverables:

- `boundary redteam`
- Fixture pack framework
- `github-lethal-trifecta` pack
- Additional fixture pack stubs for secrets exfil, tool poisoning, rug pull, Postgres destruction, GitHub PR exfil, filesystem credential read, and Slack exfil
- Decision-record output for redteam results
- Docs under `docs/firewall/REDTEAM.md`

Key acceptance:

- Fixture mode is default.
- No real secrets, no live mutation, no destructive real-system target.
- GitHub lethal-trifecta fixture shows expected DENY, actual DENY, reason, and decision record.

Verification:

- `go test ./internal/redteam/... -count=1 -timeout 5m`
- `go test ./tests/redteam/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 7 - Secure GitHub MCP Preview Profile

Objective: Build Secure GitHub MCP as Boundary's flagship governed-tool profile for the write-after-taint demo.

Suggested branch:

- `codex/2026-05-27-secure-github-mcp`

Deliverables:

- `adapters/securegithub/`
- `boundary secure github setup`
- `boundary secure github serve`
- GitHub execution envelope
- Taint tracking and taint-source recording
- One-repo-per-session policy
- Fixture collaborator model
- Risk classes for the MVP tool set
- MCP-shaped denial before GitHub mutation
- Decision record emission
- `docs/secure-mcp/GITHUB.md`
- `docs/secure-mcp/GITHUB_REDTEAM.md`
- `docs/deployment/secure-github-bypass-proofing.md`
- Tests under `tests/securegithub/` and redteam integration tests

Key acceptance:

- Fixture redteam attack blocks before write.
- Denied write does not call GitHub.
- Taint source, target repo, and W1/W2 write class are recorded.
- Secure GitHub remains preview unless live GitHub App integration evidence exists.
- README/demo language says fixture proof unless live evidence is added.

Verification:

- `go test ./adapters/securegithub/... -count=1 -timeout 5m`
- `go test ./tests/securegithub/... -count=1 -timeout 5m`
- `go test ./tests/redteam/... -run GitHub -count=1 -timeout 5m`
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 8 - Dashboard, Local Demo Visibility, And Optional GitHub Action

Objective: Provide a local visual/demo surface and low-friction repo scanning path after core Firewall and Secure GitHub proof exist.

Suggested branch:

- `codex/2026-05-27-firewall-dashboard-action`

Deliverables:

- `boundary dashboard` or `boundary tui`
- Live decisions, risk paths, policies, recent decision records, install status, and lock status
- Optional `actions/mcp-audit/` GitHub Action if it can be kept small and claim-safe

Key acceptance:

- Dashboard is local-only for this release.
- GitHub Action emits Markdown/SARIF over repo-local MCP configs and Boundary lock/policy files if implemented.
- If the GitHub Action creates conflict or scope creep, split it into a follow-up goal.

Verification:

- Relevant package tests
- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 9 - Release Demo And YC Surface

Objective: Make the release easy to understand, demo, and submit as proof of real product work.

Suggested branch:

- `codex/2026-05-27-yc-demo-surface`

Deliverables:

- `docs/DEMO_SCRIPT.md`
- `docs/YC_DEMO_NARRATIVE.md`
- `docs/LAUNCH_README.md`
- `docs/SCREENSHOT_SCRIPT.md`
- README quickstart update

Key acceptance:

- Demo leads with a poisoned GitHub issue to private-repo mutation attempt.
- Copy says what the fixture proves and what it does not prove.
- Public claims remain inside the claims ledger and readiness matrix.

Verification:

- `go test ./claims/... -count=1`
- `go test ./... -short -count=1 -timeout 5m`

### Subgoal 10 - Final Firewall + Secure GitHub Truth Reconciliation

Objective: Reconcile claims, readiness, README, changelog, launch docs, and implemented evidence after the selected release-train PRs land.

Suggested branch:

- `codex/2026-05-27-firewall-securegithub-truth`

Deliverables:

- `docs/RELEASE_TRUTH_FIREWALL_SECUREGITHUB.md`
- README, changelog, claims, and readiness alignment as required

Key acceptance:

- Secure GitHub status is preview unless live GitHub App evidence exists.
- Firewall claims are partial/delivered only where tests support them.
- Adapter maturity remains honest.
- Forbidden release language remains absent.

Verification:

- `go test ./... -count=1 -timeout 5m`
- `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)`
- `go test ./tests/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1 -timeout 5m`
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`
- `go run ./cmd/boundary verify-record --help`
- `boundary inventory --help`
- `boundary redteam --help`

## Release Positioning

Approved external frame, once supported by repo evidence:

> Fulcrum Boundary is the action boundary for MCP-native agents. It discovers dangerous MCP tool paths, generates policies, red-teams risky flows, and blocks unsafe actions before privileged tools execute. The first flagship profile is Secure GitHub MCP: a governed GitHub tool path that blocks coding agents from turning untrusted GitHub content into private-repo mutations.

Short version:

> See what your AI tools can do. Block what they should not.

YC version:

> Coding agents are getting direct access to GitHub, filesystems, databases, and messaging tools through MCP. Fulcrum Boundary sits before those tools, discovers dangerous action paths, and blocks unsafe actions before they execute. The flagship demo shows a coding agent reading untrusted GitHub content and trying to write to a private repo; Boundary detects the tainted context and denies the GitHub mutation before it happens.

## Deferred Or Gated Work

- Full 51-tool GitHub taxonomy implementation.
- Live GitHub App integration evidence.
- Managed Agents transport integration for Secure GitHub.
- Semantic Judge integration for W0+ GitHub actions.
- Full cross-MCP toxic-combination runtime enforcement.
- Registry listings and external launch assets.
- Production Secure GitHub status.
