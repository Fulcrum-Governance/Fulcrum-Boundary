# Command Boundary Demo

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

Run the Command Boundary secret-exfil denial demo (the user-facing Lane 2 demo):

```bash
boundary demo command-secret-exfil
```

A routed `curl -d @.env …` secret exfiltration is denied before execution
(`executed=false`, `class=C6`) with a decision record. The underlying
fixture/evidence path is the red-team pack:

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
