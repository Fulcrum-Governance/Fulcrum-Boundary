# Boundary Doctor

`boundary doctor` reports local routed-surface diagnostics for Boundary without
credentials, network calls, or live mutation.

> **Local-only caveat (read this first).** Doctor output is local diagnostics,
> not proof that every deployment route is protected. It is the third step of the
> first-run path ([README](../README.md),
> [docs/CLI_REFERENCE.md](./CLI_REFERENCE.md) section 1), and it reports
> readiness and bypass caveats only.

It is a readiness and caveat command, not a deployment proof. A passing doctor
run means the local Boundary command surface can describe the governed routes
and their bypass boundaries.

On a clean checkout each surface reports `warn`, not `fail`, because the optional
`.boundary` firewall, command, and edit workspaces are absent until you create
them. That is the expected first-run state.

## Commands

Run all local surface diagnostics (the first-run path uses `--json`):

```bash
boundary doctor --json
boundary doctor
```

Inspect one surface:

```bash
boundary doctor --surface mcp
boundary doctor --surface command
boundary doctor --surface edit
```

Emit JSON:

```bash
boundary doctor --json
```

## Surfaces

| Surface | What Doctor Checks | Bypass Caveat |
| --- | --- | --- |
| MCP | Local policy verification and optional firewall workspace presence | Direct upstream MCP server access is outside Boundary unless operators remove or block that path. |
| Command Boundary | Command classifier and optional project shims | Direct shell, scripts, cron, SSH, and CI jobs are bypasses unless routed through Boundary. |
| Edit Boundary | Edit classifier and optional edit evidence workspace | Direct editor writes, direct filesystem mutation, and direct `git apply` are bypasses. |

## Output Contract

JSON output uses:

```text
boundary.doctor.v1
```

Every output states:

- `credentials: none`
- `network: none`
- `live mutation: none`

## Claim Boundary

Use this wording:

> Boundary doctor reports local surface diagnostics and routed-path caveats.

> Doctor output is local diagnostics, not proof that every deployment route is
> protected.

Do not say that doctor proves all routes protected, that doctor proves
production deployment safety, that doctor verifies remote runtime enforcement, or
that doctor closes direct bypasses.
