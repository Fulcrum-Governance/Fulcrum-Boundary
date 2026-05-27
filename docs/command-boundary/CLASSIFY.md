# Command Classification

`boundary command classify` classifies a command without executing it.

This is the first implementation slice of Command Boundary preview. It does not
run commands, install shims, launch a project shell, or enforce policy. It
produces a command class, risk level, recommended action, and reason for later
governance steps.

## Usage

```bash
boundary command classify -- git status
boundary command classify -- git push origin main
boundary command classify -- rm -rf dist
boundary command classify -- cat .env
boundary command classify -- npm install
boundary command classify --json -- git push origin main
```

The `--` separates Boundary flags from the command being classified.

## Text Output

```text
Command: git push origin main
Class: C3 repo mutation
Risk: HIGH
Recommended action: require_approval
Reason: external repository mutation
```

## JSON Output

```json
{
  "schema_version": "boundary.command_classification.v1",
  "command": "git",
  "args_redacted": [
    "push",
    "origin",
    "main"
  ],
  "class": "C3",
  "risk": "HIGH",
  "recommended_action": "require_approval",
  "reason": "external repository mutation"
}
```

## Redaction

Classification output redacts secret-looking arguments before printing or
encoding output. Redaction triggers include:

- `--token`
- `--api-key`
- `--password`
- `Authorization`
- `bearer`
- `secret`
- `.env` paths and values
- SSH private key paths

Redaction preserves command shape while avoiding raw secret values.

## What It Proves

`boundary command classify` proves that Boundary can map routed command argv into
the Command Boundary taxonomy without executing the command.

It does not prove:

- command execution governance;
- denial before execution;
- project shell routing;
- shim routing;
- global shell control;
- CI, cron, SSH, or editor task control.

Those belong to later Command Boundary preview implementation slices.
