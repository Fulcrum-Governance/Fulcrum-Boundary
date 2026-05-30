# Boundary MCP Audit GitHub Action

The MCP audit action gives repositories a low-friction CI check for repo-local
MCP configs. It runs Boundary inventory and risk-graph reporting without
installing the Boundary gateway.

```yaml
name: MCP Audit
on:
  pull_request:
  push:

permissions:
  contents: read
  security-events: write

jobs:
  mcp-audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.7.0
        with:
          format: sarif
```

Use the release tag for repeatable CI behavior. SARIF upload requires
`security-events: write`.

## What It Runs

The action runs these Boundary surfaces:

- `boundary inventory --format json`
- `boundary inventory --format markdown`
- `boundary graph --format json`
- `boundary inventory --format sarif`, when SARIF is requested
- `boundary policy generate` into the action output directory as a dry-run
  starter-policy artifact

The Markdown summary is appended to the GitHub step summary. SARIF is uploaded
through GitHub's SARIF upload action only when requested and generated.

## Inputs

| Input | Default | Description |
|---|---:|---|
| `root` | `.` | Repository root to audit. |
| `format` | `markdown` | Primary report format: `markdown` or `sarif`. |
| `sarif` | `true` | Generate and upload SARIF when true. |
| `fail-on-critical` | `false` | Fail the workflow when critical MCP risk paths are found. |
| `include-defaults` | `false` | Include user-level default MCP client paths. Keep this false in CI. |

## Outputs

| Output | Description |
|---|---|
| `critical-count` | Count of critical risk paths in the repo-local MCP graph. |
| `high-count` | Count of high-risk MCP tools in the repo-local inventory. |
| `report-path` | Path to the generated primary report. |
| `sarif-path` | Path to the generated SARIF report, when requested. |

## Scope Boundaries

By default the action scans repo-local MCP config paths only. It does not scan
the runner's home directory or host-level MCP client paths unless
`include-defaults: true` is explicitly set.

The action is an audit and reporting surface. It does not install Boundary, does
not route MCP traffic, does not mutate real systems, and does not provide
runtime protection unless the relevant tool calls are deployed through Boundary.

Generated policies are starter policies written as action artifacts for review.
They are not production-complete and are not installed by the action.
