# Screenshot Script

Use these shots for the Firewall + Secure GitHub demo. Capture real local
output only. Do not use mock dashboards or edited terminal output.

## Prep

From the repo root:

```bash
tmp=$(mktemp -d)
cp docs/firewall/fixtures/claude_desktop_config.json "$tmp/mcp.json"
boundary policy generate --out "$tmp/boundary-firewall-policies"
boundary secure github setup --out "$tmp/secure-github"
boundary dashboard \
  --format html \
  --out "$tmp/dashboard.html" \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

## Shot 1 - Boundary CLI Surface

Command:

```bash
boundary --help
```

Capture:

- `inventory`
- `graph`
- `policy generate`
- `redteam`
- `secure`
- `dashboard`

Purpose:

```text
Boundary is now an operator CLI for MCP gateway, firewall discovery, Secure MCP
profiles, redteam fixtures, and local visibility.
```

## Shot 2 - MCP Inventory

Command:

```bash
boundary inventory --config "$tmp/mcp.json" --format markdown
```

Capture:

- GitHub server row
- high-risk capability classification
- absence of raw secret values

Purpose:

```text
Boundary can inspect local MCP config and classify GitHub tool risk without
rewriting the config.
```

## Shot 3 - Risk Graph

Command:

```bash
boundary graph --config "$tmp/mcp.json" --format mermaid
```

Capture:

- `flowchart LR`
- GitHub risk path labels
- high-risk path markers

Purpose:

```text
Inventory turns into visible MCP risk paths.
```

## Shot 4 - Policy Generation Verification

Command:

```bash
boundary verify --policies "$tmp/boundary-firewall-policies"
```

Capture:

- policy file count
- rule count
- `warnings: 0`

Purpose:

```text
Generated starter policies are verifiable, but still require operator review.
```

## Shot 5 - Secure GitHub Fixture Setup

Command:

```bash
boundary secure github serve --fixture --dry-run
```

Capture:

- `status: preview`
- fixture mode
- live GitHub mutation status

Purpose:

```text
Secure GitHub is a preview fixture profile in this release.
```

## Shot 6 - Lethal-Trifecta Denial

Command:

```bash
boundary redteam --pack github-lethal-trifecta
```

Capture:

- `scenario: github-write-after-taint`
- `expected: DENY`
- `actual: DENY`
- `result: pass`
- `matched rule: deny-github-write-after-taint-fixture`
- decision record and hash lines

Purpose:

```text
The fixture shows a poisoned GitHub issue leading to a private-repo mutation
attempt that Boundary denies before upstream execution.
```

## Shot 7 - Local Dashboard

Open:

```bash
open "$tmp/dashboard.html"
```

Or serve:

```bash
boundary dashboard \
  --serve \
  --listen 127.0.0.1:8942 \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

Capture:

- inventory summary
- risk paths
- policy status
- install receipt status
- descriptor lock status
- recent decision-record status if a local JSONL file is provided

Purpose:

```text
The dashboard is local visibility over Boundary artifacts, not hosted
monitoring and not protection by itself.
```

## Shot 8 - Claims Authority

Open:

```text
docs/CLAIMS_LEDGER.md
docs/ADAPTER_READINESS_MATRIX.md
claims/boundary_claims.yaml
```

Capture:

- MCP production row
- Secure GitHub preview language
- Firewall claims with evidence paths

Purpose:

```text
Public language is tied to repo evidence and maturity gates.
```

