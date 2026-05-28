# Edit Boundary Preview

Edit Boundary is the v0.6 preview train for Fulcrum Boundary. It extends the
routed action-boundary model from MCP tools and project-local commands to
proposed file mutations when those mutations are submitted through a Boundary
edit envelope.

The design premise is intentionally narrow:

> Boundary can classify and gate proposed file mutations before they are
> applied.

Edit Boundary does not claim control over direct editor writes, direct
filesystem writes, unwrapped shell edits, IDE saves without integration, CI
jobs, or arbitrary processes that mutate files without routing through
Boundary.

## Why This Exists

Coding agents can change repository state through ordinary file edits, not only
through MCP tool calls or shell commands. Command Boundary governs routed command
paths. Edit Boundary is the next preview surface for governing file-change
proposals before they touch the worktree.

## Preview Surface

Edit Boundary defines three routed surfaces:

| Surface | Example | Scope |
|---|---|---|
| Inspect | `boundary edit inspect --patch change.diff` | Classify a proposed edit without applying it. |
| Apply wrapper | `boundary edit apply --patch change.diff` | Evaluate policy before applying an edit envelope. |
| Fixture redteam | `boundary redteam --pack edit-secret-exfil` | Exercise edit-risk fixtures without live mutation. |

The preview is route-scoped. Protection starts only when the proposed mutation
is represented as a Boundary edit envelope.

## Current Status

The inspect, apply-wrapper, and fixture redteam surfaces are implemented as a
preview. This is not a filesystem sandbox, editor control plane, or global file
write interceptor. Production maturity still depends on deployment evidence
that edits route through Boundary-controlled envelopes.

## Documents

- [Design](./DESIGN.md)
- [Edit Taxonomy](./EDIT_TAXONOMY.md)
- [Bypass Model](./BYPASS_MODEL.md)
- [Inspect](./INSPECT.md)
- [Apply](./APPLY.md)
- [Redteam](./REDTEAM.md)
- [Demo](./DEMO.md)
- [Preview Claims](./PREVIEW_CLAIMS.md)
- [Redteam Fixtures](./REDTEAM_FIXTURES.md)
- [Relation to Command Boundary](./RELATION_TO_COMMAND_BOUNDARY.md)
- [Relation to Secure GitHub](./RELATION_TO_SECURE_GITHUB.md)
