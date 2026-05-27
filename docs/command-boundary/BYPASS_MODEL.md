# Command Boundary Bypass Model

Command Boundary governs commands only when the command routes through Boundary.
Everything else is outside the governed route.

Required public wording:

> Command Boundary governs commands only when the command routes through
> Boundary. Direct shell execution is a bypass. Global `PATH` outside Boundary
> is a bypass. CI jobs are bypasses unless explicitly routed through Boundary.

## Governed Routes

The planned governed routes are:

| Route | Example | Governed condition |
|---|---|---|
| Explicit wrapper | `boundary command run -- git push origin main` | The command is passed directly to Boundary. |
| Project shell | `boundary shell` | A subshell prepends `.boundary/bin` and the command has a shim. |
| Project shim | `.boundary/bin/git push origin main` | The shim calls `boundary command run -- git "$@"`. |

In all governed routes, Boundary must classify and evaluate the command before
execution. Denied commands must not execute.

## Bypass Routes

The following are bypasses unless the operator explicitly routes them through
Boundary:

- direct shell execution, such as `git push origin main`;
- a shell whose `PATH` does not place `.boundary/bin` ahead of system commands;
- commands without installed shims inside `boundary shell`;
- local scripts that invoke commands directly;
- cron jobs;
- launchd jobs;
- remote SSH sessions;
- CI jobs;
- editor tasks;
- package manager lifecycle scripts that spawn subprocesses outside a governed
  command route;
- arbitrary processes invoking commands directly.

These bypasses are limitations, not product failures. They are part of the
deployment model and must be visible in documentation.

## Deployment Evidence Required For Production Claims

Command Boundary must remain preview until deployment evidence shows that the
Boundary route is the relevant command path for the protected project or
workflow.

Production command governance would require evidence such as:

- operators intentionally route protected commands through Boundary;
- project shell or shim setup is reproducible and reversible;
- direct bypass paths are blocked by environment policy or documented as out of
  scope;
- decision records exist for routed command attempts;
- tests prove denied commands do not execute;
- tests prove allowed commands execute exactly once;
- bypass limitations are documented in release truth.

Without that evidence, public copy must say "preview" and "when routed through
Boundary."

## What Boundary Can Prove In Preview

The preview implementation can prove:

- wrapper-routed commands are classified before execution;
- denied wrapper-routed commands do not execute;
- project-local shims can route selected commands;
- decision records can capture command decisions without logging secret values;
- fixture redteam cases demonstrate expected deny outcomes without live mutation.

The preview implementation does not prove:

- global shell control;
- CI control;
- remote SSH control;
- coverage for every command path;
- universal coding-agent safety;
- sandboxing of allowed commands.

## Operator Responsibilities

Operators who use Command Boundary are responsible for choosing where Boundary
sits in the command path. For project-local preview use, the expected choices are:

1. Run sensitive commands through `boundary command run`.
2. Launch `boundary shell` for a governed project session.
3. Install project-local shims and opt into `.boundary/bin` in `PATH`.

Boundary must report these choices plainly and avoid implying that install alone
protects command paths.
