# External Inventory Ingest

Boundary can ingest newline-delimited inventory snapshots from Boundary itself
or from generic MCP inventory exports:

```bash
boundary inventory ingest --file inventory.ndjson --source boundary
boundary inventory ingest --file generic-mcp.ndjson --source generic --out ingest-report.json
boundary inventory ingest --file bumblebee-style-mcp.ndjson --source bumblebee --summary
```

The command maps recognizable MCP config, server, launcher, tool, and endpoint
fields into Boundary's MCP inventory model, then emits a JSON ingest report
containing:

- the normalized Boundary `inventory`;
- Boundary inventory `records`;
- `external_inventory_component` entries for package or extension facts that
  do not prove an MCP action path;
- `external_exposure_finding` entries for endpoint or exposure facts that stay
  report-only unless they explicitly map to an MCP action path;
- warnings and snapshot-completion state.

## Sources

| Source | Meaning |
|---|---|
| `boundary` | Boundary inventory NDJSON from `boundary inventory --format ndjson`. |
| `generic` | Generic NDJSON with recognizable MCP fields such as `mcp`, `mcpServers`, `server_name`, `server`, `command`, `args`, `launcher`, `npx`, `uvx`, `docker`, `claude_desktop_config.json`, `mcp.json`, or `.mcp.json`. |
| `bumblebee` | Fixture-proven mapping for Bumblebee-style MCP inventory records. This is not an official Bumblebee integration or compatibility claim. |

Boundary does not shell out to Bumblebee, import Bumblebee packages, or depend
on Bumblebee at runtime. The `bumblebee` source name only selects a mapping mode
covered by the fixture in
[`fixtures/external-inventory/bumblebee-style-mcp.ndjson`](../../fixtures/external-inventory/bumblebee-style-mcp.ndjson).

## Completion Semantics

An external snapshot is complete only when the input includes a complete summary
record, such as Boundary's `scan_summary` record or a generic summary with
`status: "complete"` or `complete: true`.

If no complete summary exists, ingest:

- emits a warning;
- marks the snapshot `partial`;
- sets `install_recommendations_enabled` to `false`;
- suppresses policy-recommendation records and marks install-status
  recommendations disabled.

Use `--allow-partial` only after operator review:

```bash
boundary inventory ingest --file mixed-endpoint.ndjson --source generic --allow-partial
```

`--allow-partial` does not make the snapshot complete. It only allows Boundary
to include install recommendations in the report for explicitly reviewed
partial data.

## Secret Handling

Ingest reuses Boundary inventory redaction for secret-bearing command args,
URLs, and launcher fields. External inventory reports should still be treated as
operator artifacts, not as secret stores or full forensic captures.

## Scope Boundary

External ingest is an MCP inventory mapping layer. It is not a full SBOM, EDR
feed, vulnerability scanner, package-manager integration, or official
third-party product compatibility surface.
