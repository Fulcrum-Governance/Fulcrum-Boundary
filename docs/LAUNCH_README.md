# Firewall + Secure GitHub Launch README

This launch surface is for the Fulcrum Boundary Firewall + Secure GitHub MCP
release train. It is designed to be run locally from a clean checkout.

## Release Truth

Production:

- MCP Safety Gateway

Delivered Firewall surfaces:

- `boundary inventory`
- `boundary graph`
- `boundary policy generate`
- `boundary install`
- `boundary uninstall`
- `boundary lock`
- `boundary verify-lock`
- `boundary redteam`
- `boundary dashboard`

Preview:

- Secure GitHub MCP fixture profile
- Command Boundary routed command preview
- Edit Boundary routed edit-envelope preview
- CLI, CodeExec, gRPC, Managed Agents, Webhook, and A2A adapters

Do not describe Secure GitHub as production until deployment bypass evidence
and broader live coverage are recorded.
Do not describe Command Boundary or Edit Boundary as production until routed
deployment evidence exists.

## Install Or Run Locally

Installed binary:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.8.0
boundary --help
```

Repo-local binary:

```bash
go run ./cmd/boundary --help
```

## Safety Gateway Demo

```bash
make demo
```

This is the production MCP Safety Gateway path. It demonstrates:

- safe Postgres `SELECT` allowed through Boundary
- destructive `DROP TABLE` denied through Boundary
- direct bypass blocked by network topology

## Firewall Demo

Create a local fixture workspace:

```bash
tmp=$(mktemp -d)
cp docs/firewall/fixtures/claude_desktop_config.json "$tmp/mcp.json"
```

Inventory local MCP config:

```bash
boundary inventory --config "$tmp/mcp.json" --format markdown
```

Render risk paths:

```bash
boundary graph --config "$tmp/mcp.json" --format mermaid
```

Generate and verify starter policies:

```bash
boundary policy generate --out "$tmp/boundary-firewall-policies"
boundary verify --policies "$tmp/boundary-firewall-policies"
```

Optional install preview:

```bash
boundary install --config "$tmp/mcp.json" --server github --dry-run
```

The dry run must report no config mutation.

## Secure GitHub Fixture Demo

Generate fixture profile files:

```bash
boundary secure github setup --out "$tmp/secure-github"
```

Inspect the serve configuration:

```bash
boundary secure github serve --fixture --dry-run
```

Run the fixture redteam:

```bash
boundary redteam --pack github-lethal-trifecta
```

Expected result:

```text
expected: DENY
actual: DENY
result: pass
matched rule: deny-github-write-after-taint-fixture
```

This fixture uses no real secrets and performs no live GitHub mutation.

## Local Dashboard

Render a standalone local HTML dashboard:

```bash
boundary dashboard \
  --format html \
  --out "$tmp/dashboard.html" \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

Or serve on loopback:

```bash
boundary dashboard \
  --serve \
  --listen 127.0.0.1:8942 \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

The dashboard is local-only. It reads local files and does not upload telemetry.

## Launch Checklist

- README quickstart matches the commands above.
- `docs/CLAIMS_LEDGER.md` and `claims/boundary_claims.yaml` remain in sync.
- `docs/ADAPTER_READINESS_MATRIX.md` keeps MCP as production and Secure GitHub
  as preview.
- `boundary redteam --pack github-lethal-trifecta` reports fixture DENY/pass.
- `boundary secure github serve --fixture --dry-run` reports preview fixture
  status and no live GitHub mutation.
- `boundary dashboard --format html` writes a local HTML file.
- Launch copy keeps live GitHub App conformance scoped to the opt-in preview
  harness.
- No launch copy claims hosted monitoring.
- No launch copy claims universal MCP attack prevention.

## Verification Commands

```bash
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
```
