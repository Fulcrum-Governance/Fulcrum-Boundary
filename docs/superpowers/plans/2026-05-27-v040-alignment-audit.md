# v0.4.0 Alignment Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Audit and reconcile the public Fulcrum Boundary v0.4.0 release surface so install docs, claims, release truth, docs-site copy, GitHub Action docs, and Command Boundary caveats all agree with the shipped `v0.4.0` state.

**Architecture:** This is a release-alignment lane only. It may update docs, claims ledgers, release truth, and session-log artifacts, but it must not change product behavior, tests, adapters, governance code, policy code, or Command Boundary implementation.

**Tech Stack:** Go 1.25+, MkDocs, Make release gates, GitHub CLI/API, Boundary claims tests.

---

## File Structure

- Create or update `docs/RELEASE_TRUTH_V040_ALIGNMENT.md`: final audit report with exact findings, commands, and public-copy boundaries.
- Modify public docs only if the audit finds drift:
  - `README.md`
  - `docs/INSTALL.md`
  - `docs/CLI_REFERENCE.md`
  - `docs/CLAIMS_LEDGER.md`
  - `docs/RELEASE_TRUTH_PUBLIC.md`
  - `docs/RELEASE_TRUTH_REPO_POLISH.md`
  - `docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md`
  - `docs/LAUNCH_TRUTH_FREEZE.md`
  - `docs/PUBLIC_RELEASE_COPY.md`
  - `docs/command-boundary/*`
  - `docs-site/*`
  - `CHANGELOG.md`
  - `actions/mcp-audit/action.yml`
  - `actions/mcp-audit/README.md`
  - `docs/firewall/GITHUB_ACTION.md`
- Modify `claims/boundary_claims.yaml` only if v0.4 claim maturity or forbidden-language entries are out of sync with shipped evidence.
- Modify `CODEX_SESSION_LOG.md` to record the audit lane, findings, verification, and next step.

## Task 1: Establish Audit Baseline

- [ ] **Step 1: Confirm branch and clean state**

Run:

```bash
git status --branch --short
git rev-parse HEAD
git rev-parse origin/main
gh release view v0.4.0 --json tagName,targetCommitish,url,publishedAt
gh pr list --state open --limit 20 --json number,title,headRefName,url
```

Expected:

- Branch is `release/v040-alignment-audit`.
- Working tree is clean before edits.
- `v0.4.0` release exists.
- No open PRs exist unless explicitly documented.

- [ ] **Step 2: Inventory required files**

Run:

```bash
for path in \
  README.md \
  docs/INSTALL.md \
  docs/CLI_REFERENCE.md \
  docs/CLAIMS_LEDGER.md \
  claims/boundary_claims.yaml \
  docs/ADAPTER_READINESS_MATRIX.md \
  docs/RELEASE_TRUTH_PUBLIC.md \
  docs/RELEASE_TRUTH_REPO_POLISH.md \
  docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md \
  docs/LAUNCH_TRUTH_FREEZE.md \
  docs/PUBLIC_RELEASE_COPY.md \
  docs/command-boundary \
  docs-site \
  CHANGELOG.md \
  actions/mcp-audit/action.yml \
  docs/firewall/GITHUB_ACTION.md; do
  test -e "$path" && echo "present $path" || echo "missing $path"
done
```

Expected: missing files are called out in the alignment report, not silently ignored.

## Task 2: Run Drift Searches

- [ ] **Step 1: Search stale version references**

Run:

```bash
git grep -n '@v0.3.0' || true
git grep -n 'v0.3.0' -- ':!docs/releases/*' ':!CHANGELOG.md' || true
git grep -n '@main' -- README.md docs/INSTALL.md docs-site || true
```

Expected:

- Primary public install and GitHub Action docs do not use `@v0.3.0`.
- Primary public install docs do not use `@main`.
- Historical release docs and changelog history may mention `v0.3.0`.

- [ ] **Step 2: Search Command Boundary overclaims**

Run:

```bash
git grep -n -i 'controls all shell' || true
git grep -n -i 'protects direct shell' || true
git grep -n -i 'prevents every overeager' || true
git grep -n -i 'production command governance' || true
git grep -n -i 'governs direct file edits' || true
```

Expected: matches appear only in forbidden-copy, limitation, claim-control, or explicit roadmap-gap contexts.

- [ ] **Step 3: Inspect required copy surfaces**

Run focused inspections:

```bash
git grep -n 'go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0' README.md docs/INSTALL.md docs-site docs/CLI_REFERENCE.md docs/DEMO_SCRIPT.md || true
git grep -n 'Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.4.0' README.md docs/firewall/GITHUB_ACTION.md actions/mcp-audit || true
git grep -n 'boundary command run\\|boundary shell\\|project-local shim' README.md docs/command-boundary docs-site claims docs/CLAIMS_LEDGER.md || true
```

Expected: active public docs use `@v0.4.0`; Command Boundary docs state routed-path scope.

## Task 3: Fix Any Alignment Drift

- [ ] **Step 1: Update stale public install or action examples, if found**

If an active install example uses `@v0.3.0` or `@main`, replace it with:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
```

If an active GitHub Action example uses `@v0.3.0` or `@main`, replace it with:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.4.0
```

- [ ] **Step 2: Update claim-boundary copy, if found**

If public copy implies global shell control, production Command Boundary, direct shell protection, universal coding-agent safety, or direct file/edit governance, rewrite it to:

```text
Fulcrum Boundary v0.4.0 adds Command Boundary preview: project-local command classification and wrapper-routed command governance through `boundary command run`, `boundary shell`, and project-local shims.
```

Include this required caveat near any Command Boundary capability language:

```text
Command Boundary is preview. Direct shell access, CI jobs, SSH sessions, and direct file edits remain outside Boundary unless they are routed through Boundary.
```

- [ ] **Step 3: Keep claims synchronized**

If `claims/boundary_claims.yaml` or `docs/CLAIMS_LEDGER.md` differ on Command Boundary maturity, align them to the shipped evidence:

- Command Boundary is preview.
- Classification and wrapper-routed governance are delivered only where tests support them.
- Production Command Boundary remains gated on deployment evidence that Boundary is the relevant command path.

## Task 4: Produce Alignment Report

- [ ] **Step 1: Create `docs/RELEASE_TRUTH_V040_ALIGNMENT.md`**

The report must include:

```markdown
# Release Truth: v0.4.0 Alignment Audit

Date: 2026-05-27
Audited commit: <git rev-parse HEAD after fixes>
Release tag: v0.4.0
Release URL: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.4.0

## Summary

## Docs Checked

## Stale References Found And Fixed

## Command Boundary Claim Status

## Verification Commands

## Remaining v0.4 Gaps

## Approved Public Copy

## Forbidden Public Copy
```

Use concrete findings from Tasks 1-3. Do not use placeholders.

- [ ] **Step 2: Update `CODEX_SESSION_LOG.md`**

Add a top entry with:

- Branch name.
- Scope: v0.4.0 alignment audit only.
- Drift found/fixed.
- Verification commands.
- Next step: v0.5 planning spike after alignment is merged.

## Task 5: Run Verification Gates

- [ ] **Step 1: Run required commands**

Run:

```bash
./scripts/assert-no-public-vendor-refs.sh
make docs-build
make release-check
go test ./internal/commandboundary/... -count=1 -timeout 5m
go test ./tests/commandboundary/... -count=1 -timeout 5m
go test ./tests/redteam/... -run Command -count=1 -timeout 5m
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
```

Expected: all pass. If a required command fails, fix the cause or document the blocker before proceeding.

- [ ] **Step 2: Run final diff checks**

Run:

```bash
git diff --check
git diff --name-only main...HEAD
```

Expected: no whitespace errors; changed files remain scoped to docs, claims, changelog, and session log unless a spec requirement forced otherwise.

## Task 6: Commit, Push, PR, Merge

- [ ] **Step 1: Commit**

Run:

```bash
git add -A
git commit -m "release: reconcile v0.4.0 Command Boundary alignment"
```

- [ ] **Step 2: Push and create PR**

Run:

```bash
git push -u origin HEAD
gh pr create --title "release: reconcile v0.4.0 Command Boundary alignment" --body "<summary and verification>"
```

- [ ] **Step 3: Wait for checks and merge**

Run:

```bash
gh pr checks <number> --watch --interval 10
```

When all required checks pass, merge with squash through the most reliable available path. Then:

```bash
git checkout main
git pull --ff-only origin main
git fetch --prune origin
git status --short
gh pr list --state open --limit 20 --json number,title,headRefName,url
```

Expected: clean `main`, no unexpected open PRs, alignment branch pruned after merge.

## Self-Review

- Spec coverage: tasks cover version checks, overclaim checks, README/docs/action/claims/release-truth/docs-site checks, required verification commands, report creation, commit, PR, and merge.
- Placeholder scan: no `TBD`, `TODO`, or unresolved placeholders remain. The report template explicitly instructs replacing commit values with command output.
- Scope check: this is one docs/release alignment lane; v0.5 planning is deliberately excluded until the alignment audit is complete.
