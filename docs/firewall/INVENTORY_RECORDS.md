# Boundary Inventory Records

Boundary inventory can emit a newline-delimited JSON record stream for tools
that want to ingest MCP Firewall discovery results incrementally:

```bash
boundary inventory --format ndjson --out boundary-inventory.ndjson
```

Each line is one JSON object and validates against
[`schemas/boundary-inventory-record.v1.json`](../../schemas/boundary-inventory-record.v1.json).
The stream is ordered by `sequence`, scoped by a stable `scan_id`, and begins
with `scan_start`. It ends with `scan_summary`.

## Completion Semantics

Consumers should treat a scan as complete only when the final
`scan_summary.status` is `complete` and `scan_summary.complete` is `true`.
If discovery encountered unreadable or invalid MCP config files, Boundary still
emits records for what it could inspect, but the summary status is `partial`.

`scan_summary.record_counts` records how many records of each type were emitted
for that scan. `scan_start` and `scan_summary` are always emitted.

## Record Types

| Type | Meaning |
|---|---|
| `scan_start` | Scan metadata, inventory schema version, and declared scope. |
| `agent_client` | MCP client and scope grouping, such as repo-local or user-level config. |
| `mcp_config` | MCP config file discovered during read-only inventory. |
| `mcp_server` | MCP server launcher, URL, descriptor names, highest risk, and governed-route signal. |
| `tool_descriptor` | Tool name declared by an MCP descriptor fixture or config. |
| `tool_capability` | Boundary's capability classification for a discovered tool. |
| `risk_path` | Inventory-derived path from source to privileged or external sink. |
| `policy_recommendation` | Starter-policy recommendation for operator review. |
| `descriptor_lock_status` | Descriptor-lock scope for the config; inventory reports `not_checked` until `boundary verify-lock` is run. |
| `install_status` | Whether the discovered MCP route appears governed by Boundary or bypassable. |
| `decision_record_ref` | Reserved for inventory streams that include references to existing decision records. The plain `boundary inventory` command does not create decision records. |
| `scan_summary` | Final counts and complete/partial status for the scan. |

## Secret Handling

Inventory records include environment variable names but not environment
variable values. URL credentials and secret-bearing query parameters are
redacted. CLI args that were redacted by inventory discovery remain redacted in
NDJSON output.

Do not treat the record stream as a secret store or full forensic capture. It
is intended to describe MCP discovery and policy-routing facts, not raw runtime
payloads.

## Scope Boundary

This schema covers MCP agent clients, MCP config files, MCP servers, tool
descriptors, capability classifications, risk paths, governed and ungoverned
routes, policy recommendations, descriptor-lock scope, and inventory summaries.

It is not a full SBOM, EDR feed, package vulnerability scanner, asset inventory,
or universal endpoint scanner. Package, extension, host, or vulnerability data
should only enter Boundary through a separate mapping layer when it clearly
relates to an MCP action path.
