# Command Boundary Preview

Command Boundary is the preview follow-on to Fulcrum Boundary v0.3.0 and is
packaged in v0.4.0.

Boundary can govern project-local command paths when commands route through
`boundary command run`, `boundary shell`, or project-local shims.

```bash
boundary command classify -- git push origin main
boundary command run -- git status
boundary shell
```

This is preview. Direct shell access is outside Boundary unless the environment
routes commands through the wrapper or shims.

## Current Preview Routes

| Route | Scope |
| --- | --- |
| `boundary command classify` | Classifies command risk without execution. |
| `boundary command run` | Evaluates wrapper-routed commands before execution. |
| `boundary command install --project` | Creates project-local shims under `.boundary/bin`. |
| `boundary shell` | Launches a scoped subshell with project shims on `PATH`. |
| `boundary redteam --pack command-*` | Runs fixture command-risk packs without live mutation. |

## Claim Boundary

Command Boundary governs commands only when the command routes through Boundary.
Direct shell execution, global `PATH` outside Boundary, SSH sessions, cron jobs,
and CI jobs are bypasses unless explicitly routed through Boundary.

Canonical repository docs:
[docs/command-boundary/README.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/README.md)
