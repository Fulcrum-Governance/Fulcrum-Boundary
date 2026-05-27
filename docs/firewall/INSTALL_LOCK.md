# MCP Firewall Install And Descriptor Lock

Boundary can rewrite selected MCP client config entries through a Boundary-owned
route, preserve a byte-for-byte backup, and create descriptor lockfiles for
tool-surface drift checks.

This surface is intentionally conservative. Install changes local MCP config
files only when `boundary install` is invoked explicitly. Inventory, graph, and
policy generation remain read-only.

## Install

Preview an install without writing files:

```bash
boundary install --client claude --dry-run
boundary install --config ./mcp.json --server github --dry-run
```

Install into an explicit config:

```bash
boundary install --config ./mcp.json --server github --out .boundary/firewall
```

Install discovers existing configs through the same client selection used by
inventory:

```bash
boundary install --client claude
boundary install --client cursor
boundary install --client repo --root .
boundary install --all
```

Useful flags:

| Flag | Meaning |
|---|---|
| `--config` | Explicit MCP config path. May be repeated or comma-separated. |
| `--client` | Select existing default config paths for `claude`, `cursor`, `vscode`, `repo`, or `custom`. |
| `--all` | Select all discovered existing default config paths. |
| `--server` | Route only named MCP servers. Defaults to all servers in the selected config. |
| `--out` | Boundary-owned workspace for backups and receipts. Defaults to `.boundary/firewall`. |
| `--mode` | Policy mode recorded in the installed Boundary route. Defaults to `balanced`. |
| `--dry-run` | Report what would change without writing backups, receipts, or config files. |
| `--force` | Rewrite entries already routed through Boundary. |

Install writes:

- a backup under `.boundary/firewall/backups/`
- an install receipt under `.boundary/firewall/install-receipts/`
- a rewritten MCP server entry that invokes `boundary mcp proxy`

The backup preserves the original config bytes. The receipt records the config
hash before and after install, the backup path, the routed server names, the
original descriptor hash, and the Boundary route descriptor hash. Receipts
redact secret-like CLI args, URL credentials, and environment values; backups
may contain the original secrets because they are exact config copies and are
written with owner-only permissions.

The default `.boundary/` workspace is ignored by git because backups can contain
secret-bearing MCP client configuration. Operators should treat backups as local
secret material.

## Uninstall

Restore from the install receipt:

```bash
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json
```

Dry-run restore:

```bash
boundary uninstall --receipt .boundary/firewall/install-receipts/<receipt>.json --dry-run
```

Uninstall verifies that the backup bytes match the pre-install hash in the
receipt and that the current config still matches the post-install hash in the
receipt, then restores the config byte-for-byte from that backup.

If the config changed after install, uninstall refuses to clobber those edits.
Use `--force` only after preserving or intentionally discarding post-install
changes.

## Generic Proxy Entry

Installed generic routes invoke:

```bash
boundary mcp proxy --install-receipt <receipt> --server <name> --mode balanced
```

The generic proxy entrypoint is fail-closed in this preview. It exists so
installed configs route to Boundary rather than silently bypassing it, but live
forwarding requires a configured Secure MCP profile or another governed runtime
path. Do not describe a generic install as live runtime protection by itself.

## Descriptor Lock

Create a descriptor lockfile:

```bash
boundary lock --config ./mcp.json --out .boundary/firewall/locks/descriptor-lock.json
```

Verify descriptors:

```bash
boundary verify-lock --lock .boundary/firewall/locks/descriptor-lock.json
boundary verify-lock --lock .boundary/firewall/locks/descriptor-lock.json --on-change warn
```

Descriptor hashes include the server name, command, URL with credentials
redacted, redacted args, environment variable names, and declared tool
descriptors. Tool descriptors include tool name, description, input schema, and
output schema when those fields are available in the local MCP config or
descriptor fixture. They do not include environment values.

Allowed descriptor-change modes:

| Mode | Behavior |
|---|---|
| `warn` | Report drift and exit successfully so the operator can inspect it. |
| `require_approval` | Report drift and fail closed until the operator approves or updates the lock. |
| `deny` | Report drift and fail closed until the lock is regenerated through an explicit command. |

`boundary verify-lock` defaults to `deny`. Warning mode must be requested
explicitly with `--on-change warn`.

Descriptor lock detects changes to the tool surface that Boundary can observe
from MCP config and descriptor fixtures. It does not prove the upstream tool
implementation is safe, and it does not replace policy evaluation.

## Claim Boundary

This release proves reversible MCP config install/uninstall mechanics and
descriptor drift detection from local config data. Runtime protection still
requires calls to pass through a governed runtime profile with reviewed
policies. Direct access to an upstream MCP server or API remains a bypass path
unless deployment topology removes it.
