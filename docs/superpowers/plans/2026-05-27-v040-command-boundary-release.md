# Fulcrum Boundary v0.4.0 Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Package the already-merged Command Boundary preview as Fulcrum Boundary `v0.4.0` with release docs, truth sweep, verification gates, tag, GitHub release, and install smoke tests.

**Architecture:** This is a release-mechanics lane, not a product-code lane. The branch updates public install/action copy, release notes, changelog, release truth, and the session log, then verifies the existing Command Boundary implementation before tagging from `main`.

**Tech Stack:** Go 1.25+, MkDocs, GitHub Releases, `gh`, Make release gates, Boundary CLI smoke tests.

---

## File Structure

- Modify `CHANGELOG.md`: add `0.4.0`, preserve prior `0.3.0` release history, and reset `Unreleased`.
- Modify `README.md`: update primary install and action examples to `@v0.4.0` while keeping Command Boundary preview-scoped.
- Modify `docs/INSTALL.md`, `docs/CLI_REFERENCE.md`, `docs/DEMO_SCRIPT.md`, and `docs/firewall/GITHUB_ACTION.md`: update active copy/paste examples to `@v0.4.0`.
- Create `docs/releases/v0.4.0.md`: public release notes for Command Boundary preview.
- Create `docs/RELEASE_TRUTH_V040.md`: v0.4 release truth and post-tag verification checklist.
- Modify `docs/RELEASE_TRUTH_COMMAND_BOUNDARY.md`: add v0.4 tag/release status after verification.
- Modify `CODEX_SESSION_LOG.md`: record the release lane scope, verification, and next train.

## Task 1: Prepare Release Branch

- [ ] **Step 1: Confirm clean release base**

Run:

```bash
git status --branch --short
git rev-parse HEAD
git rev-parse origin/main
gh release list --limit 10
```

Expected: clean branch from current `origin/main`, no existing `v0.4.0` release.

- [ ] **Step 2: Create release branch**

Run:

```bash
git checkout -b codex/2026-05-27-v040-command-boundary-release
```

Expected: branch created from clean `main`.

## Task 2: Update Release Copy

- [ ] **Step 1: Update active install/action examples**

Replace active public copy/paste examples that currently use `@v0.3.0` with `@v0.4.0` in:

```text
README.md
docs/INSTALL.md
docs/CLI_REFERENCE.md
docs/DEMO_SCRIPT.md
docs/firewall/GITHUB_ACTION.md
```

Do not rewrite historical v0.3 release truth documents.

- [ ] **Step 2: Add v0.4 release notes**

Create `docs/releases/v0.4.0.md` with:

```markdown
# Fulcrum Boundary v0.4.0

Fulcrum Boundary v0.4.0 packages the Command Boundary preview as the next
release after the v0.3.0 MCP Firewall and Secure GitHub release.

## Install

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
boundary selftest
boundary demo github-lethal-trifecta
```

## Command Boundary Preview

```bash
boundary command classify -- git push origin main
boundary command run -- git status
boundary shell
```

## What It Proves

- Boundary can classify command risk without executing commands.
- Boundary can evaluate wrapper-routed commands before execution.
- Denied or approval-required commands do not execute.
- Project-local shims can route selected commands through Boundary.
- Fixture command redteam packs report command-risk outcomes without live mutation.

## What It Does Not Prove

- Global shell control.
- Protection for direct shell access.
- CI, SSH, cron, editor, or arbitrary process control by default.
- Shell sandboxing.
- Production command governance.
- Governance of direct file edits outside routed command paths.
```

- [ ] **Step 3: Add v0.4 release truth**

Create `docs/RELEASE_TRUTH_V040.md` with date, commit placeholder, tests to run, Command Boundary status, MCP status unchanged, Secure GitHub status unchanged, approved copy, forbidden copy, and post-tag smoke commands.

## Task 3: Update Changelog

- [ ] **Step 1: Split current unreleased history**

Change the top of `CHANGELOG.md` so:

```markdown
## [Unreleased]

No changes yet.

## [0.4.0] - 2026-05-27

### Added

- Command Boundary preview with `boundary command classify`, `boundary command run`, project-local shims, `boundary shell`, decision records, and fixture command redteam packs.

### Changed

- Public install and GitHub Action examples now target `@v0.4.0`.

## [0.3.0] - 2026-05-27
```

Move the existing large unreleased release notes under `0.3.0`.

- [ ] **Step 2: Update changelog compare links**

Use:

```markdown
[Unreleased]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/Fulcrum-Governance/Fulcrum-Boundary/compare/v0.2.0...v0.3.0
```

## Task 4: Verify Before Commit

- [ ] **Step 1: Run release gates**

Run:

```bash
make docs-build
go test ./internal/commandboundary/... -count=1 -timeout 5m
go test ./tests/commandboundary/... -count=1 -timeout 5m
go test ./tests/redteam/... -run Command -count=1 -timeout 5m
go test ./claims/... -count=1
make release-check
```

Expected: all pass.

- [ ] **Step 2: Public-copy sweep**

Run:

```bash
rg -n "Boundary controls all shell commands|Boundary protects direct shell access|Boundary prevents every overeager agent action|Boundary provides production command governance|Boundary provides shell sandboxing" README.md docs claims
```

Expected: matches only in forbidden-language or limitation sections.

## Task 5: Commit And Push

- [ ] **Step 1: Commit release packaging**

Run:

```bash
git add -A
git commit -m "release: package Command Boundary preview as v0.4.0"
git push -u origin HEAD
```

Expected: branch pushed.

## Task 6: Merge, Tag, Publish, Smoke Test

- [ ] **Step 1: Merge clean branch**

After verification and mergeability:

```bash
gh pr create --fill
gh pr merge --squash --delete-branch
git checkout main
git pull --ff-only origin main
```

- [ ] **Step 2: Tag v0.4.0**

Run:

```bash
git tag -a v0.4.0 -m "Fulcrum Boundary v0.4.0"
git push origin v0.4.0
```

- [ ] **Step 3: Publish GitHub Release**

Run:

```bash
gh release create v0.4.0 --title "Fulcrum Boundary v0.4.0" --notes-file docs/releases/v0.4.0.md
```

- [ ] **Step 4: Smoke test tag install**

Run:

```bash
tmp=$(mktemp -d)
GOBIN="$tmp/bin" GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
"$tmp/bin/boundary" selftest
"$tmp/bin/boundary" demo github-lethal-trifecta
"$tmp/bin/boundary" command classify -- git push origin main
```

Expected: install succeeds; selftest and demo pass; command classify prints C3 repo mutation.

## Self-Review

- Spec coverage: plan covers release docs, changelog, public copy, verification gates, commit, merge, tag, release, and smoke tests.
- Placeholder scan: no `TBD`, `TODO`, or unspecified file edits remain.
- Scope check: product behavior is intentionally out of scope; only release packaging and verification are included.
