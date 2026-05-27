# Command Boundary Preview

Command Boundary is the planned next release train for Fulcrum Boundary after
v0.3.0. It extends Boundary's action-boundary model from MCP-routed tools to
project-local command paths.

The design premise is intentionally narrow:

> Boundary can govern project-local command paths when commands route through
> `boundary command run`, `boundary shell`, or project-local shims.

Command Boundary does not claim control over direct shell access, global shell
profiles, remote SSH, cron jobs, CI jobs, or arbitrary processes that invoke
commands without routing through Boundary.

## Why This Exists

Boundary v0.3.0 is an MCP-native release. It governs routed MCP tool calls,
records decisions, and demonstrates fixture write-after-taint denial through the
GitHub lethal-trifecta demo.

Coding agents also act through ordinary command paths: `git`, `gh`, package
managers, cloud CLIs, Docker, filesystem tools, and database clients. Command
Boundary is the preview path for bringing the same pre-execution decision model
to those command routes without taking over the user's shell globally.

## Planned Modes

Command Boundary has three planned modes:

| Mode | Example | Scope |
|---|---|---|
| Explicit wrapper | `boundary command run -- git status` | Only the wrapped command is governed. |
| Project shell | `boundary shell` | A subshell prepends project-local shims to `PATH`. |
| Project shims | `boundary command install --project` | Selected commands in `.boundary/bin` route through Boundary when the operator opts into that path. |

All three modes are project-local and reversible. They do not modify
`~/.zshrc`, `~/.bashrc`, `~/.profile`, or global `PATH` by default.

## Current Status

This directory defines the design only. Runtime implementation, CLI wiring,
project shims, command decision records, and command redteam packs land in later
branches.

Command Boundary is preview-only until implementation tests, bypass evidence,
and release truth reconciliation exist.

## Documents

- [Design](./DESIGN.md)
- [Command Taxonomy](./COMMAND_TAXONOMY.md)
- [Bypass Model](./BYPASS_MODEL.md)
- [Preview Claims](./PREVIEW_CLAIMS.md)
- [Redteam Fixtures](./REDTEAM_FIXTURES.md)
