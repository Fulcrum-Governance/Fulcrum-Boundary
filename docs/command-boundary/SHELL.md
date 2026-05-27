# Command Boundary Shell

`boundary shell` launches a project-local subshell for the Command Boundary
preview.

It does not edit global shell startup files. It does not mutate the user's
global `PATH`. It only prepends the current project's `.boundary/bin` directory
inside the launched subshell.

```bash
boundary shell
```

The subshell environment sets:

```text
PATH="$PWD/.boundary/bin:$PATH"
BOUNDARY_COMMAND_MODE=project
BOUNDARY_PROJECT_ROOT="$PWD"
```

When a command has a project-local shim, the shell routes it through:

```bash
boundary command run -- <command> "$@"
```

Commands without shims are outside Boundary. Direct shells outside
`boundary shell` are outside Boundary unless the operator explicitly routes
commands through `boundary command run` or project-local shims.

## Banner

The shell prints the Command Boundary scope before launching:

```text
Boundary Command Shell
Project: /path/to/repo
Shims: .boundary/bin
Commands with shims route through Boundary.
Direct commands without shims are outside Boundary.
Exit with Ctrl-D.
```

## Flags

```bash
boundary shell --project-root /path/to/repo
boundary shell --no-install
boundary shell --print-env
```

`--no-install` skips project shim creation. Commands only route through Boundary
if `.boundary/bin` already contains shims or if commands are invoked explicitly
through `boundary command run`.

`--print-env` prints the project-scoped environment instead of launching a
subshell. It is useful for inspection and tests.

## Bypass Statement

Command Boundary governs commands only when they route through Boundary.
Direct shell execution is a bypass. Global `PATH` outside Boundary is a bypass.
CI jobs, cron jobs, remote SSH sessions, and arbitrary processes are bypasses
unless explicitly routed through Boundary.
