# Boundary Firewall Dashboard

`boundary dashboard` renders a local-only view of the MCP Firewall artifacts that
already exist on disk:

- MCP config inventory
- inventory-derived risk paths
- generated or reviewed firewall policies
- install receipts
- descriptor lock status
- recent decision records from local JSONL files

It does not call remote services, upload telemetry, or claim runtime protection
by itself. Protection still depends on routing MCP calls through a governed
Boundary path with reviewed policies and deployment bypass controls.

## Static Output

Render a text summary:

```bash
boundary dashboard \
  --root . \
  --policies boundary-firewall-policies \
  --lock .boundary/firewall/locks/descriptor-lock.json \
  --receipts .boundary/firewall/install-receipts \
  --records .boundary/decision-records.jsonl
```

Render a standalone HTML file:

```bash
boundary dashboard \
  --format html \
  --out .boundary/firewall/dashboard.html \
  --root . \
  --policies boundary-firewall-policies \
  --lock .boundary/firewall/locks/descriptor-lock.json \
  --receipts .boundary/firewall/install-receipts \
  --records .boundary/decision-records.jsonl
```

Machine-readable JSON is available with `--format json`.

## Local Server

Serve the dashboard locally:

```bash
boundary dashboard --serve --listen 127.0.0.1:8942
```

The server accepts loopback listen addresses only. It rebuilds the dashboard on
each request so changes to local inventory, policy, receipt, lock, and
decision-record files are visible after refresh.

Append `?format=json` to the local URL for JSON output.

## Inputs

| Flag | Meaning |
|---|---|
| `--root` | Project root used for repo-local MCP config discovery. Defaults to `.`. |
| `--home` | Home directory used for user-level MCP config discovery. Defaults to the current user home. |
| `--config` | Additional MCP config path. May be repeated or comma-separated. |
| `--include-defaults` | Include known Claude Desktop, Cursor, VS Code, and repo-local config paths. Defaults to true. |
| `--policies` | Policy directory to parse and summarize. Defaults to `boundary-firewall-policies`. |
| `--lock` | Descriptor lockfile to verify in warning mode for visibility. |
| `--receipts` | Directory containing Boundary install receipts. |
| `--records` | Local decision-record JSONL file. May be repeated or comma-separated. |
| `--format` | `text`, `html`, or `json`. |
| `--out` | Write static output to a file instead of stdout. |
| `--serve` | Start the loopback-only local dashboard server. |

Missing optional artifacts appear as `missing` or `not_configured` rather than
being smoothed over. Parse errors are reported in the rendered status so the
operator can repair the local artifact.

## Decision Records

`--records` expects one JSON decision record per line using Boundary's
`DecisionRecordV1` shape. The dashboard displays recent record ID, timestamp,
adapter, tool, action, matched rule, decision mode, request hash, and decision
hash fields when available.

The dashboard reads only the files the operator provides. It does not tail logs
from remote infrastructure and does not make production monitoring claims.

## GitHub Action Split

The optional repo-scanning GitHub Action is intentionally left out of this
local-only dashboard subgoal. It should ship as a separate package after the
action output format, SARIF behavior, and public claims are reviewed against the
claims ledger.

## Claim Boundary

Boundary provides a local-only MCP Firewall dashboard over inventory, risk
paths, policy status, install receipts, descriptor lock status, and local
decision-record files. The dashboard is visibility, not a hosted monitoring
service and not runtime protection by itself.
