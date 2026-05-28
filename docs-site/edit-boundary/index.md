# Edit Boundary Preview

Filesystem/Edit Boundary is a preview follow-on to MCP Firewall, Secure GitHub,
and Command Boundary.

Boundary can inspect and gate proposed file mutations when edits route through
a Boundary edit envelope.

```bash
boundary edit inspect --patch proposed.diff
boundary edit apply --patch proposed.diff --dry-run
boundary redteam --pack edit-secret-exfil
```

This is preview. Direct editor writes, direct filesystem writes, shell
redirection, direct `git apply`, and unwrapped IDE APIs are outside Boundary.

## Current Preview Routes

| Route | Scope |
| --- | --- |
| `boundary edit inspect` | Classifies proposed patch bytes without applying them. |
| `boundary edit apply --dry-run` | Evaluates a proposed edit and records the verdict without writing files. |
| `boundary edit apply` | Applies only when the preview policy allows or local approval is supplied for approval-required classes. |
| `boundary redteam --pack edit-*` | Runs fixture edit-risk packs without live mutation. |

## Claim Boundary

Edit Boundary governs proposed file mutations only when the mutation routes
through a Boundary edit envelope. Direct editor writes, direct filesystem
writes, direct `git apply`, shell redirection, IDE saves, CI jobs, and arbitrary
processes are bypasses unless explicitly routed through Boundary.

Canonical repository docs:
[docs/edit-boundary/README.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/README.md)
