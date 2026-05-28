# Boundary CLI Reference

Boundary CLI commands are intentionally local-first. Commands that use fixtures
say so, commands that mutate MCP configs support dry-run review, and preview
surfaces stay labeled preview.

## 1. First-Run Commands

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.1
boundary --help
boundary version
boundary selftest
boundary doctor
boundary evidence bundle --include-demo
boundary evidence verify boundary-evidence
```

`boundary version` prints build metadata in text or JSON form. Missing release
metadata is reported as `unknown` instead of failing the command.

`boundary selftest` runs no-credential release checks. It uses local fixtures,
does not call the network, and does not perform live mutation.

`boundary doctor` reports local routed-surface diagnostics and bypass caveats
for MCP, Command Boundary, and Edit Boundary. It does not call the network or
prove production deployment protection.

`boundary evidence bundle` collects local release evidence with a manifest and
SHA-256 hashes. `boundary evidence verify` checks manifest schema, artifact
existence, artifact hashes, declared JSON schemas, parseable decision records
when present, and summary references. Evidence verification is local integrity
checking; it does not prove production route enforcement.

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
boundary demo action-boundary
boundary demo action-boundary --markdown --out demo.md
boundary demo github-lethal-trifecta
boundary demo github-lethal-trifecta --markdown --out demo.md
boundary demo postgres --gateway http://localhost:8080/mcp
boundary demo trust-degradation
```

The Action Boundary demo composes fixture-only MCP / Secure GitHub, Command
Boundary, and Edit Boundary paths. It uses no credentials, no network, and no
live mutation. The Secure GitHub demo is fixture-only as well. The Postgres demo
requires a running Boundary gateway and checks direct database bypass
separately.

Example outputs:

- [examples/cli/demo-action-boundary.txt](../examples/cli/demo-action-boundary.txt)
- [examples/cli/demo-github-lethal-trifecta.txt](../examples/cli/demo-github-lethal-trifecta.txt)

## 4. Secure GitHub Commands

```bash
boundary secure github --help
boundary secure github setup --out .boundary/secure-github
boundary secure github serve --fixture --dry-run
boundary secure github conformance --help
```

Secure GitHub is a preview profile for routed GitHub tools. Fixture mode writes
local profile and starter policy artifacts only. Live conformance is opt-in and
skips unless `BOUNDARY_GITHUB_CONFORMANCE=true` is set.

Live conformance commands:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance read
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance all --out /tmp/boundary-secure-github
```

The denied-write path must report `actual action: DENY`,
`reason: lethal_trifecta_detected`, `upstream_called=false`, and
`github_mutation_called=false`. Secure GitHub remains preview until deployment
bypass proof exists.

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
boundary evidence bundle --include-demo --out /tmp/boundary-evidence
boundary evidence verify /tmp/boundary-evidence
```

These checks keep public language, claims, docs, examples, and release gates in
sync before shipping a Boundary release branch.
