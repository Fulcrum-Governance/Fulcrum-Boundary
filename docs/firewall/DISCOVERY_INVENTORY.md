# MCP Firewall Discovery And Inventory

Boundary can inventory local MCP client configuration before any install or
rewrite step exists. Discovery is read-only for MCP client configs: it reads
known config files, classifies server capability risk, and reports what it
found. It does not mutate Claude Desktop, Cursor, VS Code, or repo-local MCP
configs.

## Commands

Initialize a Boundary-owned firewall workspace:

```bash
boundary init
```

The init command may create `.boundary/firewall/boundary-firewall.json`. It does
not change MCP client config files.

Inventory discovered MCP configs:

```bash
boundary inventory --format json
boundary inventory --format ndjson --out boundary-inventory.ndjson
boundary inventory --format markdown
boundary inventory --format sarif --out boundary-mcp.sarif.json
```

Useful flags:

| Flag | Meaning |
|---|---|
| `--root` | Project root for repo-local `.mcp.json`, `mcp.json`, `.cursor/mcp.json`, and `.vscode/mcp.json`. |
| `--home` | Home directory for user-level Claude Desktop, Cursor, and VS Code config discovery. |
| `--config` | Extra MCP config path. May be repeated or comma-separated. |
| `--include-defaults` | Include known default paths. Defaults to true. |
| `--format` | `json`, `ndjson`, `markdown`, or `sarif`. |
| `--out` | Write the report to a file instead of stdout. |

## Config Paths

Boundary looks for:

- Claude Desktop user config:
  - `~/Library/Application Support/Claude/claude_desktop_config.json`
  - `~/.config/Claude/claude_desktop_config.json`
  - `~/AppData/Roaming/Claude/claude_desktop_config.json`
- Cursor user and repo config:
  - `~/Library/Application Support/Cursor/User/mcp.json`
  - `~/.cursor/mcp.json`
  - `~/.config/Cursor/User/mcp.json`
  - `<root>/.cursor/mcp.json`
- VS Code user and repo config:
  - `~/Library/Application Support/Code/User/mcp.json`
  - `~/.config/Code/User/mcp.json`
  - `~/AppData/Roaming/Code/User/mcp.json`
  - `<root>/.vscode/mcp.json`
- Repo-local config:
  - `<root>/.mcp.json`
  - `<root>/mcp.json`

## Classification

Inventory classifies by MCP server name, command, URL, args, and optional tool
descriptors embedded in fixture configs.

Built-in categories:

| Category | Example risk |
|---|---|
| GitHub | Reads can taint context; private-repo writes are W1; merge/create/fork paths are W2. |
| Filesystem | Local reads are R0; writes and deletes are W1. |
| Database | Query tools are W1 because statement class determines read or mutation behavior. |
| Messaging | Message sends are W0 external publication paths. |
| Shell | Command execution is W2. |
| Unknown | Server could not be classified and should be reviewed before routing. |

GitHub inventory uses the Secure MCP preview taxonomy in
[`docs/SECURE_MCP_TOOL_TAXONOMY.md`](../SECURE_MCP_TOOL_TAXONOMY.md).

## Report Shapes

JSON is the canonical snapshot format. NDJSON is the record-stream format for
tool ingestion, one JSON object per line. Markdown is for local review. SARIF
is for code scanning surfaces and marks W1/W2 servers as high-risk MCP
capability findings.

NDJSON records validate against
[`schemas/boundary-inventory-record.v1.json`](../../schemas/boundary-inventory-record.v1.json)
and are documented in
[`docs/firewall/INVENTORY_RECORDS.md`](./INVENTORY_RECORDS.md). A consumer
should treat a stream as complete only when the final `scan_summary.status` is
`complete`; partial scans can still contain useful MCP config and server
records, but they are not complete inventory snapshots.

Inventory reports include environment variable names, but they do not include
environment variable values. CLI args that look like token, key, password, or
secret values are redacted, including opaque values that follow secret-bearing
flags such as `--token` or `--api-key`.

The inventory scope is MCP client configs, servers, tool descriptors,
capabilities, risk paths, governed or ungoverned routes, descriptor-lock scope,
and starter policy recommendations. It is not a full SBOM, EDR feed, package
vulnerability scanner, or universal endpoint inventory.

## Claim Boundary

This release proves local discovery and inventory classification from config
files and fixtures. Discovery still does not mutate client configs. Install and
descriptor lock behavior is documented separately in
[`docs/firewall/INSTALL_LOCK.md`](./INSTALL_LOCK.md). Runtime protection begins
only when tool calls route through a governed profile with reviewed policies.
