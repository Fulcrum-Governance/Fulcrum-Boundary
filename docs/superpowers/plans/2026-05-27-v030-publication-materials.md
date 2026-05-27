# v0.3.0 Publication Materials Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the final v0.3.0 publication docs without changing product behavior or expanding release claims.

**Architecture:** This is a docs-only release packaging task. It creates durable publication artifacts under `docs/releases/` and a GitHub Pages setup note under `docs/`, reusing existing release truth from `docs/RELEASE_TRUTH_PUBLIC.md` and `docs/RELEASE_TRUTH_REPO_POLISH.md`.

**Tech Stack:** Markdown, MkDocs, existing Boundary release scripts, Go claims tests.

---

## Scope

In scope:

- Create `docs/releases/v0.3.0.md`.
- Create `docs/releases/v0.3.0-terminal-capture.md`.
- Create `docs/releases/v0.3.0-checklist.md`.
- Create `docs/GITHUB_PAGES_SETUP.md`.
- Update `CODEX_SESSION_LOG.md`.
- Run docs and release verification.

Out of scope:

- Command Boundary docs, claims, or code.
- v0.3.0 release claim changes except factual corrections.
- Runtime behavior changes.
- New tests or generated code.

## Files

- Create `docs/releases/v0.3.0.md`: public release notes for the already tagged v0.3.0 release.
- Create `docs/releases/v0.3.0-terminal-capture.md`: terminal GIF/screenshot capture script and evidence constraints.
- Create `docs/releases/v0.3.0-checklist.md`: operator checklist for publication tasks.
- Create `docs/GITHUB_PAGES_SETUP.md`: Pages setup note for repository settings and workflow expectations.
- Modify `CODEX_SESSION_LOG.md`: add a session log entry with scope and verification.

## Task 1: Add v0.3.0 Release Notes

**Files:**

- Create: `docs/releases/v0.3.0.md`

- [ ] **Step 1: Create the release notes file**

Add this exact structure, adjusting only if current release truth contradicts it:

````markdown
# Fulcrum Boundary v0.3.0

Date: 2026-05-27

Fulcrum Boundary v0.3.0 is the first post-rename public release of the Boundary
repo and module path:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0
boundary selftest
boundary demo github-lethal-trifecta
```

Requires Go 1.25+.

## What Shipped

- MCP Firewall inventory, risk graph, starter policy generation, install
  receipts, descriptor locks, and fixture redteam checks.
- `boundary selftest`, a no-credential local smoke test for the release
  surface.
- `boundary demo github-lethal-trifecta`, a fixture-only Secure GitHub preview
  demo.
- Secure GitHub preview profile for write-after-taint denial before upstream
  GitHub mutation.
- External MCP inventory NDJSON ingest into Boundary inventory records.
- Repo-local MCP audit GitHub Action with Markdown and optional SARIF output.
- Local dashboard artifact visibility.
- GitHub Pages docs skeleton.

## Try It In One Minute

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0
boundary selftest
boundary demo github-lethal-trifecta
```

The commands require no credentials, make no live GitHub calls, and do not
mutate real repositories.

## Demo Signal

The GitHub lethal-trifecta demo should include:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

## What The Demo Proves

- Boundary can inventory fixture MCP tool paths.
- Boundary can render a risk path from untrusted GitHub context to private-repo
  mutation.
- Boundary can generate starter policies that parse through its verifier.
- Secure GitHub preview can deny the tested write-after-taint fixture before
  upstream GitHub mutation.
- Boundary emits a decision record for the governed route.

## What The Demo Does Not Prove

- It does not prove universal prompt-injection prevention.
- It does not prove production GitHub security.
- It does not call live GitHub or mutate a real repository.
- It does not protect tools that bypass Boundary.
- It does not make Secure GitHub production-ready.

## Adapter And Profile Maturity

| Adapter/Profile | Status | Release truth |
| --- | --- | --- |
| MCP | production | Production JSON-RPC MCP proxy path with lifecycle tests. |
| CLI | preview | Wrapper-routed execution only; direct shell access is outside Boundary. |
| CodeExec | preview | Policy-gated execution; secure sandboxing is not claimed. |
| gRPC | preview | Unary lifecycle works; streaming workloads remain preview. |
| Managed Agents | preview | Live upstream conformance is still required for production. |
| Webhook | preview | Informational and execution modes are split. |
| A2A | preview | Governed lifecycle exists against a documented snapshot. |
| Secure GitHub | preview | Fixture-backed Secure MCP profile; live GitHub App conformance is still required for production. |

## Claims Boundary

Boundary governs routed tools. Tools that bypass Boundary are outside the
governed route.

MCP is the only production adapter in this release. Secure GitHub and the other
non-MCP adapter/profile surfaces remain preview until their documented
production gates are satisfied.

Generated policies are starter policies for operator review, not complete
production policy guarantees.
````

- [ ] **Step 2: Check fenced code block balance**

Run:

```bash
python3 - <<'PY'
from pathlib import Path
p = Path("docs/releases/v0.3.0.md")
text = p.read_text()
assert text.count("```") % 2 == 0, "unbalanced code fences"
print("ok")
PY
```

Expected: `ok`

- [ ] **Step 3: Commit Task 1**

```bash
git add docs/releases/v0.3.0.md
git commit -m "docs(release): add v030 release notes"
```

## Task 2: Add Terminal Capture Plan

**Files:**

- Create: `docs/releases/v0.3.0-terminal-capture.md`

- [ ] **Step 1: Create the terminal capture plan**

Add:

````markdown
# v0.3.0 Terminal Capture Plan

Date: 2026-05-27

Purpose: capture a short terminal GIF or screenshot sequence for the v0.3.0
release without credentials, live GitHub calls, or real mutation.

## Script

Use the tagged install path:

```bash
tmp=$(mktemp -d)
GOBIN="$tmp/bin" GOPROXY=https://proxy.golang.org,direct \
  go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0

"$tmp/bin/boundary" selftest
"$tmp/bin/boundary" demo github-lethal-trifecta
```

## Required On-Screen Evidence

The capture must show:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

## Constraints

- Do not use credentials.
- Do not call live GitHub.
- Do not mutate a real repository.
- Do not edit the captured output to imply broader protection.
- Do not crop away the fixture-only context if it is visible in output.

## What The Capture May Claim

The capture may say that Boundary denies the tested fixture write-after-taint
route before upstream GitHub mutation.

## What The Capture Must Not Claim

- Universal prompt-injection prevention.
- Production GitHub security.
- Protection for tools that bypass Boundary.
- Command Boundary coverage.
````

- [ ] **Step 2: Validate the capture commands are copy/paste-safe**

Run:

```bash
rg -n "v0.3.0|github-lethal-trifecta|upstream_called=false|Command Boundary" docs/releases/v0.3.0-terminal-capture.md
```

Expected: matches for `v0.3.0`, `github-lethal-trifecta`, `upstream_called=false`, and only forbidden-context `Command Boundary` text.

- [ ] **Step 3: Commit Task 2**

```bash
git add docs/releases/v0.3.0-terminal-capture.md
git commit -m "docs(release): add v030 terminal capture plan"
```

## Task 3: Add GitHub Pages Setup Note

**Files:**

- Create: `docs/GITHUB_PAGES_SETUP.md`

- [ ] **Step 1: Create the GitHub Pages setup note**

Add:

````markdown
# GitHub Pages Setup

Date: 2026-05-27

Fulcrum Boundary's docs site is built by the `Docs` workflow.

## Repository Setting

Use GitHub repository settings:

- Settings -> Pages
- Source: GitHub Actions

## Workflow

The Pages deployment workflow is:

```text
.github/workflows/docs.yml
```

The site builds from:

```text
mkdocs.yml
docs-site/
```

Local verification:

```bash
./scripts/docs-build.sh
```

## Publication Rule

Do not publish or advertise a docs URL until the `Docs` workflow deploys
successfully from `main`.

## Claim Boundary

The docs site is a static publication surface. It is not hosted monitoring,
runtime protection, telemetry, or a managed Boundary service.
````

- [ ] **Step 2: Verify workflow and MkDocs references exist**

Run:

```bash
test -f .github/workflows/docs.yml
test -f mkdocs.yml
test -d docs-site
```

Expected: all commands exit 0.

- [ ] **Step 3: Commit Task 3**

```bash
git add docs/GITHUB_PAGES_SETUP.md
git commit -m "docs(site): document GitHub Pages setup"
```

## Task 4: Add v0.3.0 Publication Checklist

**Files:**

- Create: `docs/releases/v0.3.0-checklist.md`

- [ ] **Step 1: Create the checklist**

Add:

````markdown
# v0.3.0 Publication Checklist

Date: 2026-05-27

## Release Verification

- [ ] `make release-check` passes.
- [ ] `./scripts/docs-build.sh` passes.
- [ ] `./scripts/assert-no-public-vendor-refs.sh` passes.
- [ ] `go test ./claims/... -count=1` passes.
- [ ] `go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.3.0` is verified.
- [ ] `@latest` resolves to `v0.3.0`.

## Public Copy

- [ ] README install example uses `@v0.3.0`.
- [ ] GitHub Action example uses `@v0.3.0`.
- [ ] Go 1.25+ requirement is visible.
- [ ] MCP is the only production adapter.
- [ ] Secure GitHub remains preview.
- [ ] Claims say Boundary governs routed tools only.

## Publication Assets

- [ ] GitHub Pages source is set to GitHub Actions.
- [ ] `Docs` workflow deploys successfully from `main`.
- [ ] Social preview is uploaded or explicitly deferred.
- [ ] Terminal GIF or screenshot is captured or explicitly deferred.

## Deferred Work

- Command Boundary remains post-v0.3.0 work.
- Secure GitHub production remains gated on live GitHub App conformance.
- Managed Agents production remains gated on live upstream conformance.
````

- [ ] **Step 2: Confirm checklist does not imply completion**

Run:

```bash
rg -n "completed|done|shipped Command Boundary|production Secure GitHub" docs/releases/v0.3.0-checklist.md || true
```

Expected: no matches.

- [ ] **Step 3: Commit Task 4**

```bash
git add docs/releases/v0.3.0-checklist.md
git commit -m "docs(release): add v030 publication checklist"
```

## Task 5: Update Session Log And Verify

**Files:**

- Modify: `CODEX_SESSION_LOG.md`

- [ ] **Step 1: Add a session log entry**

Insert at the top of `CODEX_SESSION_LOG.md` below the title:

````markdown
## 2026-05-27 - v0.3 Publication Materials

### Context

- Parent goal: execute the v0.3 publication plus v0.4 Command Boundary sequence.
- Subgoal: v0.3.0 publication materials.
- Branch: `codex/2026-05-27-v030-publication-materials`
- Scope: docs-only publication artifacts. No product behavior, release claims,
  or Command Boundary implementation changed.

### What changed

- Added `docs/releases/v0.3.0.md`.
- Added `docs/releases/v0.3.0-terminal-capture.md`.
- Added `docs/releases/v0.3.0-checklist.md`.
- Added `docs/GITHUB_PAGES_SETUP.md`.

### Verification

- `./scripts/assert-no-public-vendor-refs.sh`: pending.
- `./scripts/docs-build.sh`: pending.
- `make release-check`: pending.
- `go test ./claims/... -count=1`: pending.

### Notes For Next Step

- After this branch lands, start the Command Boundary design-doc subgoal from a
  clean `main`.
- Do not add Command Boundary to v0.3.0 release truth.
````

- [ ] **Step 2: Run required verification**

Run:

```bash
./scripts/assert-no-public-vendor-refs.sh
./scripts/docs-build.sh
make release-check
go test ./claims/... -count=1
```

Expected: all pass.

- [ ] **Step 3: Update the session log verification lines**

Replace each pending verification line with `pass` after the commands pass:

````markdown
- `./scripts/assert-no-public-vendor-refs.sh`: pass.
- `./scripts/docs-build.sh`: pass.
- `make release-check`: pass.
- `go test ./claims/... -count=1`: pass.
````

- [ ] **Step 4: Run final clean checks**

Run:

```bash
git diff --check
git status --short
```

Expected:

- `git diff --check` exits 0.
- `git status --short` shows only the intended docs/session-log files before commit.

- [ ] **Step 5: Commit Task 5**

```bash
git add CODEX_SESSION_LOG.md
git commit -m "docs(release): record v030 publication verification"
```

## Task 6: Push, PR, Merge, And Resync

**Files:**

- No file edits.

- [ ] **Step 1: Inspect branch history**

Run:

```bash
git log --oneline origin/main..HEAD
git status --short
```

Expected: publication docs commits only, clean worktree.

- [ ] **Step 2: Push branch**

Run:

```bash
git push -u origin HEAD
```

- [ ] **Step 3: Open PR**

Run:

```bash
gh pr create \
  --title "docs(release): add v0.3.0 publication materials" \
  --body "## Summary
- Add v0.3.0 release notes, terminal capture plan, publication checklist, and GitHub Pages setup note.
- Keep Command Boundary out of v0.3.0 release truth.
- Record verification in CODEX_SESSION_LOG.md.

## Verification
- ./scripts/assert-no-public-vendor-refs.sh
- ./scripts/docs-build.sh
- make release-check
- go test ./claims/... -count=1"
```

- [ ] **Step 4: Wait for checks**

Run:

```bash
gh pr view --json number,mergeable,statusCheckRollup,url
```

Expected: mergeable and all required checks pass.

- [ ] **Step 5: Merge with squash after checks pass**

Use either the local GitHub CLI or GitHub connector:

```bash
gh pr merge --squash --delete-branch
```

If the local CLI is blocked by desktop approval policy, use the GitHub connector
merge path with squash method and the expected head SHA.

- [ ] **Step 6: Resync local main and clean branch**

Run:

```bash
git fetch origin --prune
git checkout main
git pull --ff-only origin main
git status --short
```

Expected: local `main` is clean and includes the merged publication materials.
