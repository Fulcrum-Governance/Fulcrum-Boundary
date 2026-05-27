# Command Boundary Design

## Purpose

Command Boundary defines a project-local command governance model for commands
that route through Boundary.

The design brings the existing Boundary shape to command paths:

1. Discover or receive a command route.
2. Classify the requested action.
3. Evaluate policy before execution.
4. Deny, require approval, or allow.
5. Execute only when policy allows.
6. Emit a command decision record.

This is a preview design. It is not part of the v0.3.0 release claim and does
not describe current runtime behavior unless a later implementation document
states otherwise.

## Product Sentence

Use this sentence:

> Boundary can govern project-local command paths when commands route through
> `boundary command run`, `boundary shell`, or project-local shims.

Do not use these sentences:

- Boundary controls your shell.
- Boundary protects all CLI activity.
- Boundary prevents every overeager agent action.
- Boundary provides production command governance.
- Boundary provides shell sandboxing.

## Non-Negotiable Boundaries

Command Boundary must not:

- mutate global shell startup files by default;
- edit `~/.zshrc`, `~/.bashrc`, `~/.profile`, or fish shell config by default;
- install global shims by default;
- claim protection for direct shell execution;
- claim control over CI, cron, SSH, or arbitrary external processes unless those
  processes route commands through Boundary;
- claim sandboxing unless a named, tested sandbox boundary exists.

## Mode 1: Explicit Wrapper

The explicit wrapper is the narrowest mode:

```bash
boundary command run -- git status
boundary command run -- git push origin main
```

Only the command passed through `boundary command run` is governed. Boundary
parses the command and arguments without invoking a shell, classifies the action,
evaluates policy, and executes only if the decision permits execution.

The preview execution rule is:

- `allow`: execute once through `os/exec`;
- `warn`: execute only if the policy mode allows warnings to proceed;
- `require_approval`: do not execute in non-interactive preview mode unless an
  approval mechanism is explicitly implemented;
- `deny`: do not execute.

## Mode 2: Project Shell

The project shell launches a subshell with project-local shims first in `PATH`:

```bash
boundary shell
```

Preview environment:

```bash
PATH="$PWD/.boundary/bin:$PATH"
BOUNDARY_COMMAND_MODE=project
BOUNDARY_PROJECT_ROOT="$PWD"
```

The shell banner must state that commands with shims route through Boundary and
commands without shims are outside Boundary.

The project shell must not modify global shell profile files. The `PATH` change
is scoped to the subshell process and its children.

## Mode 3: Project Shims

Project shim mode creates command wrappers under `.boundary/bin`:

```bash
boundary command install --project
```

Default shims:

```text
git
gh
rm
mv
cp
curl
wget
npm
pnpm
yarn
bun
node
python
python3
docker
kubectl
terraform
psql
```

Each shim calls Boundary with the original command name:

```sh
#!/usr/bin/env sh
exec boundary command run -- git "$@"
```

The operator chooses whether to prepend `.boundary/bin` to `PATH` or to launch
`boundary shell`. The install command should print the opt-in commands, not
modify the user's global shell automatically.

## Planned Command Flow

```text
argv
  -> parse without shell interpolation
  -> redact secret-looking values for logs and records
  -> classify command risk
  -> build governance request
  -> evaluate policy
  -> deny, require approval, warn, or allow
  -> execute only when allowed
  -> write command decision record
```

The implementation must not use `sh -c`, `bash -c`, or `zsh -c` by default. If a
shell invocation is explicitly requested, it must be classified as a high-risk
command form because shell interpolation expands the attack surface.

## Preview Policy Shape

The default preview policy should be conservative:

| Class | Default action |
|---|---|
| C0 observe/read | allow |
| C1 local file write | warn |
| C2 network egress | require approval |
| C3 repo mutation | require approval |
| C4 destructive local mutation | deny |
| C5 infrastructure/runtime mutation | deny |
| C6 credential/secret access | deny |
| C7 package lifecycle execution | require approval |

Implementation branches may tune this policy, but any tuning must preserve the
claim boundary: Command Boundary governs only routed commands.

## Decision Records

Command Boundary decision records should avoid raw sensitive values and include
only redacted arguments plus hashes where possible.

Planned record shape:

```json
{
  "record_type": "command_decision",
  "schema_version": "boundary.command_decision.v1",
  "command": "git",
  "args_hash": "sha256:...",
  "cwd": "/path/to/project",
  "class": "C3",
  "action": "deny",
  "executed": false,
  "reason": "repo mutation requires approval"
}
```

Raw tokens, passwords, bearer values, API keys, `.env` values, and private key
material must not be logged.

## Relationship To v0.3.0

v0.3.0 remains an MCP Firewall plus Secure GitHub preview release. Command
Boundary is a later preview train and must not be described as shipped in the
v0.3.0 release truth.
