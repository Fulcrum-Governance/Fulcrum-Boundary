# Release Truth: Command Boundary Preview

Date: 2026-05-27

Audited base commit: `6deb560eaf4b6dfbc0a7c2d22d53e71db6651318`

Branch: `codex/2026-05-27-command-boundary-truth`

Release train: post-v0.3.0 Command Boundary preview

## Summary

This report reconciles the Command Boundary preview after the design, classifier,
run wrapper, project-local shell/shims, fixture redteam packs, and demo/docs
branches landed.

Final Command Boundary truth:

- Command Boundary is delivered as a preview surface.
- Command Boundary governs only commands routed through `boundary command run`,
  `boundary shell`, or project-local shims.
- Direct shell execution is outside Boundary.
- Global `PATH` outside Boundary is outside Boundary.
- CI jobs, SSH sessions, cron jobs, editor tasks, and arbitrary local processes
  are outside Boundary unless explicitly routed through Boundary.
- Project-local shims are reversible and do not modify global shell startup
  files.
- `boundary command classify` classifies without executing.
- `boundary command run` evaluates before execution and does not execute denied
  or approval-required commands.
- Command decision records redact secret-looking arguments and retain argv
  hashes.
- Command redteam packs are fixture-only and report `executed=false`.
- Command Boundary does not claim shell sandboxing.
- Command Boundary does not claim production command governance.
- MCP production status is unchanged.
- Secure GitHub preview status is unchanged.

## Test Commands

| Command | Result |
|---|---|
| `make release-check` | Pass |
| `go test ./internal/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Command -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |

## Command Boundary Status

| Surface | Status | Truth |
|---|---|---|
| `boundary command classify` | delivered preview | Classifies command argv without execution and redacts secret-looking arguments. |
| `boundary command run` | delivered preview | Builds a command governance request, evaluates the preview policy, executes allowed/warned commands, and blocks denied or approval-required commands. |
| Command decision records | delivered preview | Writes local JSONL command decision records with redacted args and argv hashes. |
| `boundary command install --project` | delivered preview | Creates project-local `.boundary/bin` shims for selected commands without global shell mutation. |
| `boundary command uninstall --project` | delivered preview | Removes Boundary-generated project-local shims. |
| `boundary shell` | delivered preview | Launches a scoped subshell with `.boundary/bin` first on `PATH`; commands without shims remain outside Boundary. |
| Command redteam packs | delivered preview | Run fixture-only cleanup, secret-exfiltration, and repo-mutation command-risk packs without live mutation. |

## MCP Status Unchanged

MCP remains the production adapter path. Command Boundary does not change the
MCP adapter maturity, MCP Firewall release truth, or `v0.3.0` release claims.

## Secure GitHub Status Unchanged

Secure GitHub remains a preview Secure MCP profile. Command Boundary does not
make Secure GitHub production, add live GitHub App conformance evidence, or add
deployment bypass proof.

## Direct Shell Bypass Statement

Command Boundary governs commands only when the command routes through Boundary.
Direct shell execution is a bypass. Global `PATH` outside Boundary is a bypass.
CI jobs are bypasses unless explicitly routed through Boundary.

## Approved Command Boundary Copy

Use this copy for public docs:

> Boundary provides preview project-local command governance for commands routed
> through `boundary command run`, `boundary shell`, or project-local shims.

Supporting copy:

- Boundary can classify command risk without executing commands.
- Boundary can deny or require approval before wrapper-routed command execution.
- Project-local shims can route selected commands through Boundary when the
  operator opts into `.boundary/bin`.
- Command redteam packs are fixture-only and do not perform live mutation.

## Forbidden Command Boundary Copy

Do not use these statements as public capability claims:

- Boundary controls all shell commands.
- Boundary protects direct shell access.
- Boundary prevents every overeager agent action.
- Boundary provides production command governance.
- Boundary provides shell sandboxing.
- Boundary controls CI jobs by default.
- Boundary controls remote SSH by default.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Claims Status

| Claim | Status | Truth |
|---|---|---|
| BND-CLAIM-CMD-001 | delivered | Preview project-local command governance exists only for commands routed through Boundary. |
| BND-CLAIM-CMD-002 | delivered | Fixture Command Boundary redteam packs deny or require approval for selected command-risk paths without live mutation. |

## Docs Checked

- `README.md`
- `docs/command-boundary/README.md`
- `docs/command-boundary/DESIGN.md`
- `docs/command-boundary/COMMAND_TAXONOMY.md`
- `docs/command-boundary/BYPASS_MODEL.md`
- `docs/command-boundary/CLASSIFY.md`
- `docs/command-boundary/RUN.md`
- `docs/command-boundary/INSTALL.md`
- `docs/command-boundary/SHELL.md`
- `docs/command-boundary/REDTEAM.md`
- `docs/command-boundary/DEMO.md`
- `docs/command-boundary/PREVIEW_CLAIMS.md`
- `docs/CLAIMS_LEDGER.md`
- `claims/boundary_claims.yaml`
- `docs-site/command-boundary/index.md`
- `docs-site/command-boundary/demo.md`
- `docs-site/reference/claims.md`
- `docs-site/reference/cli.md`

## Drift Found

- `BND-CLAIM-CMD-001` was still partial after the demo/docs branch landed.
- `docs/command-boundary/PREVIEW_CLAIMS.md` still described the primary command
  claim as partial pending release truth reconciliation.

## Drift Fixed

- Marked `BND-CLAIM-CMD-001` delivered as a preview claim with release truth
  evidence.
- Added this release truth report.
- Kept all Command Boundary public copy preview-scoped and routed-path-only.

## Remaining Limits

- Production command governance requires deployment evidence that Boundary is
  the relevant command path for the protected project or workflow.
- Direct shell access remains outside Boundary unless routed through the wrapper,
  shell, or project-local shims.
- CI, SSH, cron, editor tasks, and arbitrary processes remain outside Boundary
  unless explicitly routed through Boundary.
- Command Boundary does not provide a shell sandbox.
