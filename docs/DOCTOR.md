# Boundary Doctor

`boundary doctor` reports local routed-surface diagnostics for Boundary without
credentials, network calls, or live mutation.

It is a readiness and caveat command, not a deployment proof. A passing doctor
run means the local Boundary command surface can describe the governed routes
and their bypass boundaries. It does not prove production deployment protection.

## Commands

Run all local surface diagnostics:

```bash
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

> Boundary Doctor reports local routed-surface readiness and bypass caveats for
> MCP, Command Boundary, and Edit Boundary.

Do not say doctor proves all routes are protected, proves live deployment
enforcement, or validates production bypass resistance.
