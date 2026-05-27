# Boundary CLI Reference

Boundary CLI commands are intentionally local-first. Commands that use fixtures
say so, commands that mutate MCP configs support dry-run review, and preview
surfaces stay labeled preview.

## 1. First-Run Commands

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@latest
boundary --help
boundary selftest
```

`boundary selftest` runs no-credential release checks. It uses local fixtures,
does not call the network, and does not perform live mutation.

Example output: [examples/cli/selftest.txt](../examples/cli/selftest.txt)

## 2. Firewall Commands

```bash
boundary inventory --format markdown
boundary graph --format mermaid
boundary policy generate --out boundary-firewall-policies
boundary verify --policies boundary-firewall-policies
```

Inventory discovers MCP config files and tools that Boundary can route. Risk
graphs make potential routes visible for review. Starter policies are a review
baseline and should be inspected before production use.

Examples:

- [examples/cli/inventory-markdown.md](../examples/cli/inventory-markdown.md)
- [examples/cli/risk-graph.mmd](../examples/cli/risk-graph.mmd)

## 3. Demo Commands

```bash
boundary demo github-lethal-trifecta
boundary demo github-lethal-trifecta --markdown --out demo.md
boundary demo postgres --gateway http://localhost:8080/mcp
boundary demo trust-degradation
```

The Secure GitHub demo is fixture-only: no credentials, no network, and no live
GitHub mutation. The Postgres demo requires a running Boundary gateway and
checks direct database bypass separately.

Example output:
[examples/cli/demo-github-lethal-trifecta.txt](../examples/cli/demo-github-lethal-trifecta.txt)

## 4. Secure GitHub Commands

```bash
boundary secure github --help
boundary secure github setup --out .boundary/secure-github
boundary secure github serve --fixture --dry-run
```

Secure GitHub is a preview profile for routed GitHub tools. Fixture mode writes
local profile and starter policy artifacts only. Live GitHub App conformance is
not claimed until live evidence exists.

## 5. Inventory Ingest Commands

```bash
boundary inventory ingest \
  --file fixtures/external-inventory/external-mcp-inventory.ndjson \
  --source external-mcp \
  --summary
```

External MCP inventory NDJSON is input data. Boundary promotes only records that
describe MCP clients, MCP configs, MCP servers, MCP tools, risk paths, governed
routes, or policy recommendations.

Example output:
[examples/cli/external-ingest-summary.txt](../examples/cli/external-ingest-summary.txt)

## 6. Install/Uninstall Commands

```bash
boundary install --config path/to/mcp.json --server shell --dry-run
boundary install --client repo --out .boundary/firewall
boundary uninstall --receipt .boundary/firewall/install-receipts/example.json --dry-run
```

Install rewrites selected MCP entries so routed tools execute through Boundary.
Use dry-run first. Direct upstream access remains a deployment bypass unless
operators remove that path.

## 7. Dashboard Commands

```bash
boundary dashboard --format html --out .boundary/firewall/dashboard.html
boundary dashboard --serve --listen 127.0.0.1:8942
```

The dashboard is local-only and intended for operator review. It can summarize
inventory, policies, receipts, descriptor locks, and decision records; it is not
a policy enforcement path.

## 8. Release Verification Commands

```bash
./scripts/assert-no-public-vendor-refs.sh
make docs-build
make release-check
go test ./claims/... -count=1
go test ./... -short -count=1 -timeout 5m
```

These checks keep public language, claims, docs, examples, and release gates in
sync before shipping a Boundary release branch.
