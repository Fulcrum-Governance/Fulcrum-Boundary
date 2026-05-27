# Command Boundary Run Wrapper

`boundary command run` evaluates a project-local command before execution.

It governs only commands that are routed through this wrapper. Direct shell
execution, remote SSH, CI jobs, cron jobs, and processes that invoke commands
without Boundary remain outside the governed route.

## Usage

```bash
boundary command run -- git status
boundary command run -- rm -rf dist
boundary command run --record-out .boundary/command/decision-records.jsonl -- git status
```

Commands are parsed as argv. Boundary does not invoke `sh -c`, `bash -c`, or
`zsh -c` for this wrapper path.

## Preview Policy

The default preview policy is intentionally conservative:

| Class | Default action |
| --- | --- |
| C0 observe/read | allow |
| C1 local file write | warn |
| C2 network egress | require_approval |
| C3 repo mutation | require_approval |
| C4 destructive local mutation | deny |
| C5 infrastructure/runtime mutation | deny |
| C6 credential/secret access | deny |
| C7 package lifecycle execution | require_approval |

`allow` and `warn` execute the command. `deny` and `require_approval` do not
execute it in this preview wrapper.

## Decision Records

Every evaluated command writes a local JSONL command decision record. The
default path is:

```text
.boundary/command/decision-records.jsonl
```

The record includes the command name, redacted arguments, argv hash, cwd,
class, action, execution status, exit code, request ID, envelope ID, and matched
policy rule when available.

Boundary does not store raw secret-looking arguments in command decision
records. Arguments such as tokens, passwords, bearer values, `.env` paths, and
SSH key paths are redacted; the raw argv is represented by `args_hash`.

## What This Proves

- Wrapper-routed commands can be classified before execution.
- Denied or approval-required commands do not execute.
- Allowed or warned commands execute via `os/exec` without shell
  interpolation.
- A local command decision record is emitted for each evaluated command.

## What This Does Not Prove

- Global shell control.
- Protection for direct shell access.
- CI, SSH, or cron control unless those paths route through Boundary.
- Shell sandboxing.
- Universal prevention of overeager agent behavior.
