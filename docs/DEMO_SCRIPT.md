# Fulcrum Boundary Demo Script

This script is for the Firewall + Secure GitHub release surface. It shows a
safe fixture path, not a live GitHub App integration.

## Demo Promise

An agent reads a poisoned GitHub issue, carries that untrusted context forward,
then tries to mutate a private repository. Boundary sees the tainted context and
denies the private-repo write before any upstream GitHub call is made.

What the fixture proves:

- the GitHub lethal-trifecta scenario is modeled as local fixture data
- taint metadata reaches the governed request
- the Secure GitHub preview profile denies protected W1/W2 private-repo writes
- the denial happens before upstream execution
- a decision record and decision hash are emitted

What the fixture does not prove:

- live GitHub App conformance
- production bypass resistance
- full GitHub MCP tool coverage
- every prompt-injection or MCP exploit path
- hosted monitoring or remote telemetry

## Setup

Use a local build or installed `boundary` binary.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.0
boundary --help
```

For a repo-local run:

```bash
go run ./cmd/boundary --help
```

## 1. Open With The Concrete Risk

Say:

```text
Coding agents now read GitHub issues, inspect code, create branches, write
files, and merge pull requests through MCP tools. The dangerous moment is not
that the agent can read an issue or write a file. The dangerous moment is when
untrusted issue text enters the agent context and the next action is a
private-repo mutation.
```

Then name the boundary:

```text
Fulcrum Boundary sits before the privileged MCP tool. It makes an allow or deny
decision before the tool executes, and records why.
```

## 2. Show The Current Product Surface

```bash
boundary --help
```

Point out these commands:

- `inventory` for local MCP config discovery
- `graph` for inventory-derived risk paths
- `policy generate` for starter firewall policies
- `install` and `uninstall` for reversible local routes
- `redteam` for safe fixture attacks
- `secure github` for the preview Secure GitHub profile
- `dashboard` for local-only visibility

## 3. Discover A Local GitHub MCP Config

From a clean demo directory, use the checked-in fixture config:

```bash
tmp=$(mktemp -d)
cp docs/firewall/fixtures/claude_desktop_config.json "$tmp/mcp.json"
boundary inventory --config "$tmp/mcp.json" --format markdown
```

Expected points to show:

- GitHub is detected as an MCP server.
- High-risk GitHub capabilities are classified.
- Secret-like environment values are not printed.
- Inventory is read-only and does not rewrite the config.

## 4. Render Risk Paths And Starter Policies

```bash
boundary graph --config "$tmp/mcp.json" --format mermaid
boundary policy generate --out "$tmp/boundary-firewall-policies"
boundary verify --policies "$tmp/boundary-firewall-policies"
```

Expected points to show:

- The risk graph is derived from the inventory.
- Generated policies are starter policies for operator review.
- `boundary verify` loads the generated policy bundle cleanly.

## 5. Generate The Secure GitHub Preview Profile

```bash
boundary secure github setup --out "$tmp/secure-github"
boundary secure github serve --fixture --dry-run
```

Expected points to show:

- Status is `preview`.
- Fixture mode is enabled.
- Live GitHub mutation is `none`.
- Live GitHub App evidence is not claimed.

## 6. Run The Poisoned-Issue Fixture

```bash
boundary redteam --pack github-lethal-trifecta
```

Narrate the scenario:

```text
The fixture models an external GitHub issue entering the agent context. The
agent then attempts a protected private-repo file mutation. Boundary evaluates
the action before forwarding and denies the write-after-taint path.
```

Expected output includes:

```text
redteam mode: fixture
pack: github-lethal-trifecta
live mutation: none
real secrets: none
scenario: github-write-after-taint
expected: DENY
actual: DENY
result: pass
matched rule: deny-github-write-after-taint-fixture
decision record: rec_<hash-prefix>
decision hash: sha256:<hash>
```

Say:

```text
This is the proof point: the private-repo mutation never reaches upstream in
the fixture path. Boundary returns a denial with the matched rule and a
receipt-grade decision record.
```

## 7. Show Local Visibility

```bash
boundary dashboard \
  --format html \
  --out "$tmp/dashboard.html" \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

Open the generated HTML file locally, or serve it on loopback:

```bash
boundary dashboard \
  --serve \
  --listen 127.0.0.1:8942 \
  --config "$tmp/mcp.json" \
  --policies "$tmp/boundary-firewall-policies"
```

Expected points to show:

- inventory status
- risk path count
- policy status
- install receipt status
- descriptor lock status
- recent decision-record status if a local JSONL file is provided

State the limit:

```text
The dashboard reads local artifacts only. It is visibility, not hosted
monitoring and not runtime protection by itself.
```

## 8. Close

Use this close:

```text
Fulcrum Boundary now has a production MCP Safety Gateway and a claim-gated MCP
Firewall surface. The Secure GitHub profile is preview, but the core demo is
real: untrusted GitHub context plus a protected private-repo write becomes a
deny-before-execute decision with an auditable record.
```
