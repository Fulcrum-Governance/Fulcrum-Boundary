# CODEX Session Log

## 2026-05-27 - Install And Release Workflow Polish

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 7 - Install And Release Workflow Polish`.
- Branch: `codex/2026-05-27-install-release-polish`
- Scope: make install, selftest, demo, and release verification one-command friendly without claiming nonexistent distribution channels or requiring live credentials.

### What changed

- Added `make selftest`, `make demo-github`, and `make release-check`.
- Added `scripts/selftest.sh`, `scripts/demo-github.sh`, and `scripts/release-check.sh`; each prints commands, uses `set -euo pipefail`, and runs fixture/no-credential release paths.
- Added `docs/INSTALL.md` with Go install, source checkout, first useful commands, receipt-based uninstall, and future-only Homebrew language.
- Updated README install commands to run `boundary selftest` and link to install docs.
- Updated the changelog with the install/release workflow polish.

### Verification

- `make selftest`: pass
- `make demo-github`: pass
- `make release-check`: pass
- `git diff --check`: pass
- `rg -n "brew|Homebrew|tap|formula" README.md docs/INSTALL.md docs | head -100`: only the future-only Homebrew placeholder and planning/spec references

### Notes For Next Step

- After this branch lands, start `Subgoal 8 - Final Public Release Truth Reconciliation` from clean `main`.

## 2026-05-27 - GitHub Action MCP Audit

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 6 - GitHub Action MCP Audit`.
- Branch: `codex/2026-05-27-github-action-mcp-audit`
- Scope: add a repo-local MCP audit GitHub Action without installing the gateway, scanning runner home directories by default, or claiming runtime protection.

### What changed

- Added `actions/mcp-audit/action.yml` and `actions/mcp-audit/README.md`.
- Added `scripts/actions/mcp-audit.sh` to run Boundary inventory, risk graph, optional SARIF output, dry-run starter-policy generation into the action artifact directory, GitHub step summary output, and action outputs.
- Added `docs/firewall/GITHUB_ACTION.md`.
- Added `tests/actions/mcp_audit_fixture_test.go`.
- Added delivered claim `BND-CLAIM-017` for the repo-local MCP audit action.
- Split MCP discovery defaults so repo-local root configs are always inspected while `--include-defaults=false` suppresses user-level HOME/client defaults for CI.
- Updated the changelog with the MCP audit action.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/actions/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `golangci-lint run ./...`: pass, `0 issues`
- `bash -n scripts/actions/mcp-audit.sh && git diff --check`: pass
- `git ls-files '*.go' | xargs gofmt -l`: pass

### Notes For Next Step

- After this branch lands, start `Subgoal 7 - Install And Release Workflow Polish` from clean `main`.

## 2026-05-27 - External Inventory Ingest

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 5 - External Inventory Ingest`.
- Branch: `codex/2026-05-27-external-inventory-ingest`
- Scope: ingest Boundary, generic, and fixture-proven Bumblebee-style MCP inventory NDJSON without claiming official third-party compatibility or adding runtime third-party dependencies.

### What changed

- Added `boundary inventory ingest --file <inventory.ndjson>` with `--source boundary|generic|bumblebee`, `--format json`, `--out`, `--summary`, and `--allow-partial`.
- Added external inventory mapping for Boundary inventory records, generic MCP config/server fields, launcher hints (`npx`, `uvx`, `docker`), report-only package/extension components, and report-only exposure findings.
- Added partial-snapshot semantics: missing or incomplete summaries warn, mark the snapshot partial, and disable install recommendations unless `--allow-partial` is explicitly set.
- Added docs and fixtures for generic MCP, Bumblebee-style MCP, and mixed endpoint/package inventory streams.
- Hardened inventory allocation capacity paths after CI CodeQL flagged potential integer-overflow allocation patterns.
- Updated the changelog with the external inventory ingest command.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -run Ingest -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `golangci-lint run ./...`: pass
- `git diff --check`: pass
- `go run ./cmd/boundary inventory ingest --file fixtures/external-inventory/generic-mcp.ndjson --source generic --summary`: pass
- `go run ./cmd/boundary inventory ingest --file fixtures/external-inventory/mixed-endpoint.ndjson --source generic --summary`: pass

### Notes For Next Step

- After this branch lands, start `Subgoal 6 - GitHub Action MCP Audit` from clean `main`.

## 2026-05-27 - Inventory NDJSON Record Schema

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 4 - Inventory NDJSON Record Schema`.
- Branch: `codex/2026-05-27-inventory-ndjson-schema`
- Scope: add machine-ingestible MCP Firewall inventory records without expanding into SBOM, EDR, package vulnerability scanning, or universal endpoint inventory.

### What changed

- Added `boundary inventory --format ndjson` through the existing inventory renderer and `--out` path.
- Added versioned inventory record builders for scan start, agent clients, MCP configs, MCP servers, tool descriptors, capabilities, risk paths, starter-policy recommendations, descriptor-lock status, install status, and scan summary.
- Added `schemas/boundary-inventory-record.v1.json` and schema-backed fixture tests for every emitted NDJSON line.
- Added `docs/firewall/INVENTORY_RECORDS.md` and updated discovery inventory docs with completion semantics, secret handling, and scope boundaries.
- Updated the changelog with the NDJSON inventory record stream.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `jq empty schemas/boundary-inventory-record.v1.json`: pass
- `go run ./cmd/boundary inventory --format ndjson` against a temp MCP fixture: pass
- `git diff --check`: pass

### Notes For Next Step

- After this branch lands, start `Subgoal 5 - External Inventory Ingest` from clean `main`.
- The `decision_record_ref` record type is reserved for streams that include references to existing decision records; plain inventory does not create decision records.

## 2026-05-27 - GitHub Lethal-Trifecta Demo

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 3 - GitHub Lethal-Trifecta Demo Command`.
- Branch: `codex/2026-05-27-github-lethal-trifecta-demo`
- Scope: add a fixture-only one-command Secure GitHub denial demo; no credentials, network mutation, or real system mutation.

### What changed

- Added `boundary demo github-lethal-trifecta` with text, `--json`, `--markdown`, `--out`, and `--dashboard` output paths.
- Added `internal/demo` orchestration for fixture MCP inventory, risk graph generation, starter policy generation and verification, Secure GitHub fixture setup, redteam execution, Secure GitHub read-then-write denial proof, decision-record artifact output, and optional local dashboard rendering.
- Added `tests/demo/github_lethal_trifecta_demo_test.go` and `docs/DEMO_GITHUB_LETHAL_TRIFECTA.md`.
- Updated the changelog with the demo command.

### Verification

- `go test ./internal/demo/... -count=1 -timeout 5m`: pass
- `go test ./tests/demo/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go run ./cmd/boundary demo github-lethal-trifecta`: pass
- `golangci-lint run ./...`: pass
- `go run github.com/securego/gosec/v2/cmd/gosec@latest ./...`: pass, `Issues: 0`
- `git diff --check`: pass

### Notes For Next Step

- PR `#51` received a scoped `#nosec G304` annotation for the internally constructed demo fixture decision-record path after CI security scan flagged the artifact write.
- After this branch lands, start `Subgoal 4 - Inventory NDJSON Record Schema` from clean `main`.

## 2026-05-27 - Boundary Selftest

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 2 - Boundary Selftest`.
- Branch: `codex/2026-05-27-selftest`
- Scope: add a local no-credential selftest command for public release smoke checks; no live credentials, network mutation, or real system mutation.

### What changed

- Added `boundary selftest`, `boundary selftest --json`, and `boundary selftest --no-color`.
- Added `internal/selftest` with fixture-only checks for CLI boot, MCP inventory, risk graph rendering, starter policy validation, descriptor lock baseline/drift, GitHub lethal-trifecta redteam denial, Secure GitHub live-mode fail-closed behavior, decision-record emission, and claims-validation pointer output.
- Added `tests/selftest/selftest_cli_test.go` and `docs/SELFTEST.md`.
- Updated the changelog with the selftest command.

### Verification

- `go test ./internal/selftest/... -count=1 -timeout 5m`: pass
- `go test ./tests/selftest/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go run ./cmd/boundary selftest`: pass
- `git diff --check`: pass

### Notes For Next Step

- After this branch lands, start `Subgoal 3 - GitHub Lethal-Trifecta Demo Command` from clean `main`.

## 2026-05-27 - Public Release README And Copy Polish

### Context

- Parent goal: Final public Boundary release hardening.
- Subgoal: `Subgoal 1 - Public README And Copy Polish`.
- Branch: `codex/2026-05-27-public-release-polish`
- Scope: public README structure and reusable release copy only; no product behavior changes.

### What changed

- Reordered the README opening surface around install, a current five-minute local demo path, explicit claim limits, MCP Firewall local visibility, Secure GitHub preview, and the MCP Safety Gateway/Postgres demo.
- Added `docs/PUBLIC_RELEASE_COPY.md` with short copy, medium copy, Secure GitHub preview copy, MCP Firewall copy, claim-safe demo wording, and forbidden public copy examples.
- Updated the changelog with the README/copy polish.
- Kept `boundary selftest` and `boundary demo github-lethal-trifecta` out of README commands until those commands land in later subgoals.

### Verification

- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass

### Notes For Next Step

- After this branch lands, start `Subgoal 2 - Boundary Selftest` from clean `main`.

## 2026-05-27 - Public Release Hardening Goal Setup

### Context

- Parent goal: Final public Boundary release hardening from `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Last_Pass_Specs.md`.
- Branch: `codex/2026-05-27-boundary-release-next`
- Scope: translate the last-pass sprint spec into a durable repo-owned goal set before implementation.

### What changed

- Added `docs/superpowers/specs/2026-05-27-boundary-public-release-hardening-goals.md`.
- Removed stale review/no-auto-merge gates from the execution plan and aligned it with current Codex merge-autonomy rules.
- Split the heavy selftest/demo step into two subgoals so each implementation branch has a tighter verification surface.

### Verification

- `git diff --check`: pass

### Notes For Next Step

- After this planning artifact lands, execute the subgoals sequentially from clean `main`: public README/copy polish, selftest, GitHub lethal-trifecta demo, inventory NDJSON, external inventory ingest, GitHub Action MCP audit, install/release workflow polish, and final public truth reconciliation.

## 2026-05-27 - Dependabot Alert Remediation

### Context

- Branch: `codex/2026-05-27-dependabot-alerts`
- Scope: resolve active GitHub Dependabot alerts on the default branch with minimal dependency and CI updates.

### What changed

- Upgraded `github.com/jackc/pgx/v5` from `v5.7.6` to `v5.9.2` to pick up the patched release for the active `pgx` advisories.
- Removed the stale indirect `golang.org/x/crypto` dependency from the root module graph through `go mod tidy`, resolving the active `x/crypto` advisories by removal.
- Raised the root module `go` directive to `1.25.0` because `pgx v5.9.2` declares Go 1.25.
- Updated CI root test and lint jobs to use Go 1.25+.

### Verification

- `go test ./... -count=1 -timeout 5m`: pass
- `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)`: pass
- `go test ./claims/... -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `git ls-files '*.go' | xargs gofmt -l`: pass, empty
- `GOTOOLCHAIN=go1.25.0 go test ./... -short -count=1 -timeout 5m`: pass
- `GOTOOLCHAIN=go1.26.3 go test ./... -short -count=1 -timeout 5m`: pass
- `GOTOOLCHAIN=go1.26.3 go run golang.org/x/vuln/cmd/govulncheck@latest ./...`: pass, no vulnerabilities found
- `git diff --check`: pass

### Notes For Next Step

- Push PR, verify CI, merge, and confirm Dependabot alerts close on `main`.

## 2026-05-27 - Firewall + Secure GitHub Truth Reconciliation

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 10 - Final Firewall + Secure GitHub Truth Reconciliation`.
- Branch: `codex/2026-05-27-firewall-securegithub-truth`
- Scope: release truth report, launch truth freeze, README copy tightening, changelog, and verification.

### What changed

- Added `docs/RELEASE_TRUTH_FIREWALL_SECUREGITHUB.md` covering claims, readiness, launch docs, demo docs, drift found/fixed, and remaining preview gates.
- Added a dated Firewall + Secure GitHub release-train section to `docs/LAUNCH_TRUTH_FREEZE.md`.
- Tightened the README opening description from broad production-agent language to the concrete privileged-tool action-boundary surface.
- Updated the changelog with the final release truth report.

### Verification

- `go test ./... -count=1 -timeout 5m`: pass
- `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)`: pass
- `go test ./tests/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1 -timeout 5m`: pass
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass (`policy files: 1`, `rules: 5`, `warnings: 0`)
- `go run ./cmd/boundary verify-record --help`: pass
- `go run ./cmd/boundary inventory --help`: pass
- `go run ./cmd/boundary redteam --help`: pass
- `git diff --check`: pass

### Notes For Next Step

- After verification and merge, run branch cleanup and final goal closeout.

## 2026-05-27 - Release Demo And YC Surface

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 9 - Release Demo And YC Surface`.
- Branch: `codex/2026-05-27-yc-demo-surface`
- Scope: launch/demo documentation and README quickstart only; no product behavior changes.

### What changed

- Added `docs/DEMO_SCRIPT.md` for the poisoned GitHub issue to private-repo mutation fixture demo.
- Added `docs/YC_DEMO_NARRATIVE.md` with claim-safe YC copy and evidence links.
- Added `docs/LAUNCH_README.md` with the local launch flow and verification checklist.
- Added `docs/SCREENSHOT_SCRIPT.md` for real local screenshots only.
- Updated README with a Firewall + Secure GitHub demo quickstart.
- Updated the changelog with the launch documentation surface.

### Verification

- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass
- Public-language scan for forbidden overclaim phrases in the new docs: no new matches outside the claims authority files.
- Launch command smoke with a freshly built local `boundary` binary: inventory, graph, policy generate, policy verify, Secure GitHub setup, Secure GitHub dry-run, redteam fixture, and dashboard HTML generation all passed.

### Notes For Next Step

- After verification and merge, start Subgoal 10 final Firewall + Secure GitHub truth reconciliation from clean `main`.

## 2026-05-27 - Firewall Dashboard And Local Demo Visibility

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 8 - Dashboard, Local Demo Visibility, And Optional GitHub Action`.
- Branch: `codex/2026-05-27-firewall-dashboard-action`
- Scope: local-only dashboard command, rendered dashboard docs, tests, and claim entry.

### What changed

- Added a local dashboard model that reads Boundary's real MCP Firewall artifacts: inventory, risk graph, policy directory, install receipts, descriptor lock status, and operator-provided decision-record JSONL files.
- Added `boundary dashboard` with text, JSON, HTML, and loopback-only local server modes.
- Added loopback enforcement for dashboard serving so the release surface stays local-only.
- Added `docs/firewall/DASHBOARD.md`, README quick commands, changelog entry, and delivered claim `BND-CLAIM-016`.
- Split the optional GitHub Action into a follow-up package because action output, SARIF behavior, and public claims need their own small release lane.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -count=1 -timeout 5m`: pass
- `go test ./internal/boundarycli/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `git ls-files '*.go' | xargs gofmt -l`: pass, empty
- `golangci-lint run --timeout=5m`: pass, `0 issues`
- `gosec ./internal/firewall/... ./internal/boundarycli/... ./tests/firewall/...`: pass, `Issues: 0`
- `git diff --check`: pass
- `go run ./cmd/boundary dashboard --root "$tmp/root" --home "$tmp/home" --policies "$tmp/policies" --lock "$tmp/root/.boundary/firewall/locks/descriptor-lock.json" --format text`: pass

### Notes For Next Step

- After verification and merge, start Subgoal 9 release demo and YC surface from clean `main`.

## 2026-05-27 - Secure GitHub MCP Preview Profile

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 7 - Secure GitHub MCP Preview Profile`.
- Branch: `codex/2026-05-27-secure-github-mcp`
- Scope: fixture-first Secure GitHub MCP profile, CLI setup/serve commands, docs, tests, readiness, and claim entry.

### What changed

- Added `adapters/securegithub` as a preview Secure MCP profile for GitHub fixture governance.
- Added GitHub tool classification for the MVP tool set: `R0`, `W0`, `W1`, and `W2`.
- Added session taint tracking, one-repo-per-session binding, fixture collaborator handling, and MCP-shaped denial responses.
- Added default policy rules for one-repo scope violations and W1/W2 private-repo writes after external GitHub taint.
- Added `boundary secure github setup` and `boundary secure github serve` with fixture-only behavior and live-mode fail-closed handling.
- Routed the existing GitHub lethal-trifecta redteam fixture through the shared Secure GitHub policy rules.
- Added Secure GitHub docs, redteam docs, bypass-proofing docs, readiness declaration, and delivered claim `BND-CLAIM-015`.
- Updated adapter/profile maturity language so Secure GitHub is preview and not described as production or live GitHub App conformance evidence.

### Verification

- `go test ./adapters/securegithub/... -count=1 -timeout 5m`: pass
- `go test ./tests/securegithub/... -count=1 -timeout 5m`: pass
- `go test ./tests/redteam/... -run GitHub -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./internal/boundarycli/... -count=1 -timeout 5m`: pass
- `go test ./tests/adapter_conformance/... -count=1 -timeout 5m`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `git ls-files '*.go' | xargs gofmt -l`: pass, empty
- `golangci-lint run --timeout=5m`: pass, `0 issues`
- `gosec ./adapters/securegithub/... ./internal/boundarycli/... ./internal/redteam/... ./tests/securegithub/... ./tests/redteam/...`: pass, `Issues: 0`
- `git diff --check`: pass
- `go run ./cmd/boundary secure github setup --out "$tmp/secure-github"`: pass
- `go run ./cmd/boundary secure github serve --dry-run`: pass
- `go run ./cmd/boundary redteam --format json --pack github-lethal-trifecta`: pass

### Notes For Next Step

- If merged, start demo and YC launch surface from clean `main`.
- Secure GitHub remains preview until live GitHub App conformance and deployment bypass evidence are recorded.

## 2026-05-27 - Redteam Framework And GitHub Lethal-Trifecta Fixture

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 6 - Redteam Framework And GitHub Lethal-Trifecta Fixture`.
- Branch: `codex/2026-05-27-redteam-lethal-trifecta`
- Scope: fixture-only redteam runner, `boundary redteam` CLI, GitHub write-after-taint denial fixture, redteam docs, tests, and claim entry.

### What changed

- Added `internal/redteam` with a fixture runner that uses the existing governance pipeline and builds decision records from captured audit events.
- Added the implemented `github-lethal-trifecta` pack for external GitHub content taint followed by protected private-repo file mutation.
- Added reserved redteam pack stubs for secrets exfiltration, tool poisoning, rug pull, Postgres destruction, GitHub PR exfiltration, filesystem credential reads, and Slack exfiltration.
- Added `boundary redteam` with default fixture mode, JSON/text output, pack listing, and fail-closed rejection of non-fixture modes.
- Added `docs/firewall/REDTEAM.md`.
- Added delivered claim `BND-CLAIM-014` for fixture redteam packs with expected deny outcomes and no live mutation.

### Verification

- `go test ./internal/redteam/... -count=1 -timeout 5m`: pass
- `go test ./tests/redteam/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./internal/boundarycli/... -count=1 -timeout 5m`: pass
- `go run ./cmd/boundary redteam`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `golangci-lint run --timeout=5m`: pass, `0 issues`
- `git diff --check`: pass

### Notes For Next Step

- If merged, start Secure GitHub MCP preview profile from clean `main`.

## 2026-05-27 - Firewall Install, Uninstall, And Descriptor Lock

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 5 - Firewall Install, Uninstall, And Descriptor Lock`.
- Branch: `codex/2026-05-27-firewall-install-lock`
- Scope: reversible MCP config route install/uninstall, descriptor lock hashing and verification, docs, tests, and claim entry.

### What changed

- Added `boundary install` with explicit config/client selection, server filtering, dry-run mode, byte-for-byte backups, install receipts, and Boundary route rewrites.
- Added `boundary uninstall` to restore the exact backup bytes recorded by an install receipt.
- Added `boundary lock` and `boundary verify-lock` for local MCP descriptor hash creation and drift checks.
- Added fail-closed `boundary mcp proxy` preview entrypoint for generic installed routes so no generic route silently passes through as protected.
- Tightened uninstall safety so post-install user edits are not clobbered without explicit `--force`.
- Tightened descriptor hashes to include available tool descriptions, input schemas, and output schemas.
- Added `.boundary/` to `.gitignore` because install backups may contain exact secret-bearing MCP config bytes.
- Added `docs/firewall/INSTALL_LOCK.md`.
- Added delivered claim `BND-CLAIM-013` for reversible install/uninstall and descriptor lock verification.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./internal/boundarycli/... -count=1 -timeout 5m`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `go vet ./...`: pass
- `git diff --check`: pass

### Notes For Next Step

- If merged, start the redteam fixture framework and GitHub lethal-trifecta fixture from clean `main`.

## 2026-05-27 - Firewall Risk Graph And Policy Generation

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 4 - Firewall Risk Graph And Policy Generation`.
- Branch: `codex/2026-05-27-firewall-graph-policy`
- Scope: inventory-derived graph rendering, starter policy generation, docs, tests, and claim entry.

### What changed

- Added `boundary graph` with JSON and Mermaid output.
- Added inventory-derived risk-path detection for untrusted GitHub reads, private repo mutations, external publication, privileged mutations, descriptor-change review, destructive DB actions, filesystem exfiltration, and unclassified-review paths.
- Added `boundary policy generate --mode balanced` with verifiable schema v1 starter policies for filesystem, GitHub, Postgres/database, Slack/messaging, shell, and descriptor integrity.
- Added `docs/firewall/RISK_GRAPH_POLICY_GENERATION.md`.
- Added delivered claim `BND-CLAIM-012` for risk graphs and verifiable starter policy generation.
- Updated the human claims ledger with BND-CLAIM-011 and BND-CLAIM-012.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go run ./cmd/boundary policy generate --out "$tmp/policies"`: pass
- `go run ./cmd/boundary verify --policies "$tmp/policies"`: pass, `policy files: 6`, `rules: 12`, `warnings: 0`
- `go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass

### Notes For Next Step

- If merged, start Firewall install, uninstall, and descriptor lock from clean `main`.
- Descriptor risk is currently graph/policy-generation evidence only. Real descriptor lockfile hashing and verification belongs to the next subgoal.

## 2026-05-27 - Firewall Discovery And Inventory

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 3 - Firewall Discovery And Inventory`.
- Branch: `codex/2026-05-27-firewall-discovery-inventory`
- Scope: MCP config discovery, inventory reports, docs, tests, and claim entry.

### What changed

- Added `internal/firewall` discovery, MCP config parsing, capability classification, and report rendering.
- Added `boundary init` and `boundary inventory`.
- Added JSON, Markdown, and SARIF inventory report support.
- Added `docs/firewall/DISCOVERY_INVENTORY.md` and fixture MCP configs.
- Added delivered claim `BND-CLAIM-011` for read-only MCP config inventory.
- Added focused package and CLI tests.

### Verification

- `go test ./internal/firewall/... -count=1 -timeout 5m`: pass
- `go test ./tests/firewall/... -count=1 -timeout 5m`: pass
- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass

### Notes For Next Step

- If merged, start Firewall risk graph and policy generation from clean `main`.

## 2026-05-27 - Secure MCP Contract

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 2 - Secure MCP Contract`.
- Branch: `codex/2026-05-27-secure-mcp-contract`
- Scope: docs and changelog only; no runtime or claim-status changes.

### What changed

- Added `docs/SECURE_MCP_CONTRACT.md`.
- Added `docs/SECURE_MCP_SERVER_TEMPLATE.md`.
- Added `docs/SECURE_MCP_TOOL_TAXONOMY.md`.
- Added `docs/secure-mcp/README.md`.
- Updated `CHANGELOG.md` with the Secure MCP contract entry.

### Verification

- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass
- `git diff --check`: pass

### Notes For Next Step

- If merged, start Firewall discovery and inventory from clean `main`.

## 2026-05-27 - Language System Lock

### Context

- Parent goal: Firewall + Secure GitHub MCP release train.
- Subgoal: `Subgoal 1 - Language System Lock`.
- Branch: `codex/2026-05-27-language-system-lock`
- Scope: docs, README, claims-ledger language discipline, changelog, and claims test gate only.

### What changed

- Added Boundary language authority docs:
  - `docs/LANGUAGE_SYSTEM.md`
  - `docs/LEXICON.md`
  - `docs/COPY_RULES.md`
  - `docs/BOUNDARY_PRODUCT_PRIMITIVES.md`
- Added `claims/language_lint_test.go` to scan public docs for controlled overclaim phrases.
- Updated README hero copy to the action-boundary frame.
- Linked the language system from the claims ledger and README.
- Updated `CHANGELOG.md` with the language-lock entry.

### Verification

- `go test ./claims/... -count=1`: pass
- `go test ./... -short -count=1 -timeout 5m`: pass

### Notes For Next Step

- If merged, start Subgoal 2 or Subgoal 3 from clean `main` depending on whether docs or implementation should lead the next lane.

## 2026-05-27 — Firewall + Secure GitHub Goal Set

### Context

- Goal: ship the Fulcrum Boundary Firewall + Secure GitHub MCP release train.
- Source planning docs:
  - `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Execution_Pack.md`
  - `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/Context_from_GPT.md`
  - `/Users/td/ConceptDev/Projects/Fulcrum/.claude/sprint/active-builds/secure-github-mcp/SPEC.md`
- Branch: `codex/2026-05-27-boundary-firewall-securegithub-goals`
- Scope: planning and goal decomposition only; no product behavior changes.

### What changed

- Added `docs/superpowers/specs/2026-05-27-boundary-firewall-securegithub-goals.md`.
- Converted the sprint execution pack into a parent goal plus sequential subgoals.
- Removed stale "Tony reviews" and "do not auto-merge" gates from the working plan.
- Preserved current merge-autonomy rules: Codex-owned PRs can be merged when scoped, verified, mergeable, and not blocked by a real decision gate.
- Folded in Secure GitHub MCP source inspiration while tightening overclaim risk:
  - Secure GitHub stays Boundary-native for this release.
  - Fixture evidence lands before live GitHub App production claims.
  - No "no one else detects this" or universal prompt-injection prevention claims.
  - Full 51-tool coverage remains deferred unless implemented and tested.

### Verification

- `git diff --check`: pass

### Notes For Next Step

- Start Subgoal 1 from clean `main`: Language System Lock.
- Use claim/readiness gates as the release-truth authority for every follow-on branch.

## 2026-05-26 — Boundary Phase 1 Claims And Adapter Readiness

### Context

- Spec: `/Users/td/ConceptDev/Projects/Fulcrum-codex/.claude/sprint/BOUNDARY_SPEC_SERIES.md`
- Phase: `Phase 1 — SPEC 1 Claims Ledger + Release Truth System` and `SPEC 2 Adapter Completion Standard`
- Branch: `codex/2026-05-26-boundary-phase1-foundation`
- Scope boundary: foundation artifacts only; no adapter runtime behavior changes and no adapter expansion.

### Preflight

- Confirmed work in Boundary repo: `/Users/td/ConceptDev/Projects/Fulcrum-Boundary`
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

- Phase: `Phase 2 — Fulcrum-Boundary proof/citation cleanup`
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
- Project identity: Fulcrum Boundary, module `github.com/fulcrum-governance/fulcrum-boundary`.
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

- GitHub license detection via `gh repo view --json licenseInfo` returned `null`, but `gh api repos/Fulcrum-Governance/Fulcrum-Boundary/license` detects Apache-2.0 correctly and the repository has a visible Apache 2.0 `LICENSE` file.
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

## 2026-05-26 — Spec 4 Managed Agents Adapter

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing after Spec 3 was committed as `c19e53b`.
- Scope: Spec 4 only. Add a Managed Agents proxy adapter and keep its public status at preview until live upstream Anthropic conformance is recorded.
- Current API check: the Managed Agents session-events contract supports `user.tool_confirmation` with `result: "allow"` or `result: "deny"` plus optional `deny_message`, so Boundary can deny directly rather than relying on timeout behavior.

### Built

- Added `adapters/managedagents/` with protocol types, event parsing, `TransportAdapter` implementation, session proxy, policy-driven tool resolver, confirmation forwarder, metadata attachment, response inspection, and per-thread budget/trust tracking.
- Added `governance.TransportManagedAgents` and included it in the default fail-closed transport set.
- Added integration coverage for end-to-end session proxying, per-tool policy resolution, allow/deny confirmations, decision-record emission, thread budget denial, trust isolation denial, and bypass-config verification.
- Added `docs/adapters/MANAGED_AGENTS.md`, `docs/deployment/managed-agents-bypass-proofing.md`, and `examples/managed-agents-governed-session/`.
- Updated README, adapter readiness, fail-mode matrix, adapter contract, claims ledger, and changelog to include Managed Agents as a preview adapter.

### Verification

- `env -u GOROOT go test ./adapters/managedagents ./tests/integration ./claims ./tests/adapter_conformance`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.

### Notes

- Bypass proof is credential/topology based: Boundary must be the only component with the upstream Managed Agents API key, and customer apps must not be able to send confirmations directly.
- Standalone budget and trust tracking are in-process; kernel-connected deployments should sync to fulcrum-io budget enforcement and fulcrum-trust state.

## 2026-05-26 — Spec 5 Policy v1 + SQL AST Guard

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing after Spec 4 was committed as `134deeb`.
- Scope: Spec 5 only. Add policy schema v1 validation, richer PolicyEval request projection, and a Postgres AST guard while preserving v0.2.0 legacy YAML policy loading.

### Built

- Added `schemas/policy.v1.yaml`, `policyeval/schema.go`, and validation tests for schema-versioned policy documents.
- Kept legacy top-level YAML policies backward compatible while validating v1 documents strictly through the same loader used by `boundary verify` and `boundary serve`.
- Expanded static policy matching with typed conditions, scopes, transport restrictions, regex/equality/not conditions, and decision-mode propagation.
- Added `governance.ProjectPolicyEvalRequest` so PolicyEval receives tenant, agent, transport, tool, action, arguments, trust state, risk class, resource IDs, request hash, policy version, and provenance.
- Added `interceptors/sql/` with a Postgres parser-backed AST classifier and fail-closed guard for unknown or destructive SQL.
- Added a 30+ case SQL evasion corpus plus interceptor tests for comments, dollar strings, mixed statements, invalid tokens, destructive DDL, writes, reads, and administrative statements.
- Registered the Postgres AST guard in `boundary serve` for the demo `query` tool.
- Added `docs/POLICY_SCHEMA.md` and `docs/policies/POSTGRES.md`; updated README, changelog, launch truth, fail-mode docs, and claims ledger.

### Verification

- `env -u GOROOT go test ./policyeval ./governance ./interceptors/sql ./internal/boundarycli ./tests ./tests/interceptors`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go run ./cmd/boundary verify --policies schemas`: pass, `warnings: 0`.
- `env -u GOROOT go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass, preserving v0.2.0 YAML policy compatibility.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.

### Notes

- The Postgres AST guard is a statement classifier and fail-closed interceptor, not a general SQL firewall or universal SQL injection prevention claim.
- Static policies run before interceptors; AST class conditions are intended for PolicyEval or for adapters that pre-populate `sql_class` before the static-policy stage.

## 2026-05-26 — Spec 6 Receipt-Grade Decision Records

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing after Spec 5 was committed as `a1f2453`.
- Scope: Spec 6 only. Add verifiable decision records, canonical request and policy hashes, parse-rejection events, optional signature schema support, and CLI verification.

### Built

- Added v1 decision-record primitives in `governance/receipt*.go`, including canonical decision hashing, canonical JSON request hashing, canonical YAML policy-bundle hashing, optional Ed25519 signing support, and record verification.
- Extended audit events and slog output with `schema_version`, `record_id`, `policy_bundle_hash`, `request_hash`, `raw_shape_hash`, `decision_hash`, `trust_state`, `boundary_build_digest`, and optional signature fields.
- Added `boundary verify-record` to validate stored records against request JSON, policy directories, build digests, and record hashes.
- Added MCP parse-rejection auditing so malformed or invalid JSON-RPC requests emit `parse_rejected` records with `raw_shape_hash` even when no pipeline evaluation occurs.
- Added `schemas/decision-record.v1.json`, `docs/RECEIPTS.md`, and refreshed decision-record, launch-truth, and claims-ledger language to allow receipt-grade records while forbidding signed-by-default claims.
- Added tests for canonical request hashing, metadata-independent policy hashing, tamper detection, CLI `verify-record`, and parse-rejection audit emission.

### Verification

- `env -u GOROOT go test ./internal/boundarycli ./tests -run 'TestRun_VerifyRecord|TestReceipt|TestPolicyBundle|TestParseRejection' -count=1`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.
- `env -u GOROOT go run ./cmd/boundary verify --policies schemas`: pass, `warnings: 0`.
- `env -u GOROOT go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass, `warnings: 0`.

### Notes

- Receipt-grade means hash-verifiable decision records. Signature fields exist in the v1 schema, but Boundary does not claim signed receipts by default.
- Parse rejections use `raw_shape_hash`, not `request_hash`, because no canonical governed request exists for malformed input.

## 2026-05-26 — Spec 7 Trust Integration + Adaptive Termination

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing after Spec 6 was committed as `030884f`.
- Scope: Spec 7 only. Add standalone trust state, kernel Redis trust-state integration, adaptive termination, trust transition records, and CLI inspection while keeping fulcrum-trust as the canonical Beta-model owner.

### Built

- Added trust backend interfaces and implementations in `governance/trust_*.go`: in-memory standalone Beta trust, kernel Redis IPC reads/writes, production trust config, and trust outcome mapping.
- Added circuit-breaker thresholds and adaptive action handling so repeated protected-tool failures move agents from `TRUSTED` to `EVALUATING` to `ISOLATED`.
- Extended the governance pipeline with required-agent-ID enforcement, pre-policy trust denials for isolated or terminated agents, fail-closed trust backend errors on protected transports, post-decision trust updates, and `trust_transition` audit events.
- Added `boundary trust show`, standalone `boundary trust reset`, `boundary serve --trust-mode`, `--trust-redis-url`, `--require-agent-id`, and `boundary demo trust-degradation`.
- Added trust docs at `docs/TRUST_INTEGRATION.md` and `docs/ADAPTIVE_TERMINATION.md`, plus `examples/trust-degradation-demo/`.
- Updated launch truth, threat model, fail-mode matrix, claims ledger, and changelog with bounded delivered trust claims.

### Verification

- `env -u GOROOT go test ./governance ./tests ./tests/integration -run 'TestStandaloneTrust|TestTrustStateFromScore|TestAdaptiveTermination|TestTrustShow|TestTrustDegradation|TestKernelTrust' -count=1`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.
- `env -u GOROOT go run ./cmd/boundary demo trust-degradation`: pass; demo reaches `ISOLATED` and blocks the later protected query pre-execution.
- `env -u GOROOT go run ./cmd/boundary trust show demo-agent`: pass; prints a standalone unknown-agent trusted snapshot.

### Notes

- Standalone mode is process-local and is intended for demos and disconnected deployments.
- Kernel mode reads fulcrum-trust IPC state through Redis and fails closed on protected transports when trust state cannot be checked.

## 2026-05-26 — Spec 8 Cross-Repo Integration Contract

### Context

- Branch: `codex/2026-05-26-boundary-phase1-foundation`, continuing after Spec 7 was committed as `9b069e3`.
- Scope: Spec 8 only. Define Boundary's standalone/kernel seams against the Fulcrum control plane, add runtime config validation, and document proof correspondence without claiming runtime proof extraction.

### Built

- Added `governance/providers.go` with integration interfaces for policy, cost, budget, escalation, envelope lifecycle, and proof correspondence.
- Added `governance/standalone/` implementations for local policy loading, in-process trust, local budget tracking, approval escalation, local envelopes, and static proof correspondence.
- Added `governance/kernel/` bridge implementations for Redis policies, Redis trust, HTTP budget enforcement, NATS-style escalation, NATS-style audit, and NATS-style envelope events using injectable transports.
- Added `config/schema.v1.yaml`, runtime config loading/validation, and `boundary serve --config`; unsafe kernel config fails before startup.
- Added `docs/INTEGRATION.md`, `docs/STANDALONE_VS_KERNEL.md`, and `docs/PROOF_BOUNDARY.md`.
- Updated README, claims ledger, and changelog with bounded integration-contract language.
- Reviewed `docs/PROOF_BOUNDARY.md` against actual theorem names in `/Users/td/ConceptDev/Projects/Fulcrum-Proofs/proofs/lean/`.

### Verification

- `env -u GOROOT go test ./claims ./tests/integration ./internal/boundarycli ./cmd/boundary -run 'TestClaims|TestStandaloneBundle|TestKernelBundle|TestRuntimeConfig|TestRun_Serve' -count=1`: pass.
- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check`: pass.
- `env -u GOROOT go run ./cmd/boundary verify --policies schemas`: pass, `warnings: 0`.
- `env -u GOROOT go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass, `warnings: 0`.

### Notes

- Kernel package tests use fake Redis, HTTP, and publisher transports; live fulcrum-io service conformance remains an operator-environment acceptance step.
- Boundary proof correspondence remains `design` scope only. Boundary itself still must not emit `proved` decisions.

## 2026-05-26 — Boundary Spec Series Closeout

### Completed Phases

- Phase 1 foundation gates: claims ledger and adapter readiness standard.
- Spec 3: production MCP JSON-RPC proxy adapter.
- Spec 4: preview Managed Agents proxy adapter.
- Spec 5: policy schema v1 and Postgres AST guard.
- Spec 6: receipt-grade decision records.
- Spec 7: trust integration and adaptive termination.
- Spec 8: cross-repo standalone/kernel integration contract.

### Final Verification

- `env -u GOROOT go test ./... -short`: pass.
- `env -u GOROOT go vet ./...`: pass.
- `git ls-files '*.go' | xargs gofmt -l`: pass.
- `git diff --check HEAD`: pass.
- `env -u GOROOT go run ./cmd/boundary verify --policies schemas`: pass, `warnings: 0`.
- `env -u GOROOT go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`: pass, `warnings: 0`.

### Notes

- Branch: `codex/2026-05-26-boundary-phase1-foundation`.
- All phases in `.claude/sprint/BOUNDARY_SPEC_SERIES.md` have been implemented, validated, and committed in sequence.
