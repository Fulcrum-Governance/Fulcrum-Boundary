# External Inventory Ingest

Boundary can ingest external MCP inventory NDJSON and map recognizable
MCP-related records into Boundary inventory records.

This is a vendor-neutral ingest surface. Boundary does not depend on, shell out
to, import, endorse, or claim compatibility with any named third-party scanner.
External inventory may describe many endpoint facts. Boundary only promotes
records that relate to agent clients, MCP configs, MCP servers, MCP tools,
capability classes, risk paths, governed routes, or policy recommendations.

```bash
boundary inventory ingest --file inventory.ndjson --source boundary
boundary inventory ingest --file generic-mcp.ndjson --source generic --out ingest-report.json
boundary inventory ingest --file external-mcp-inventory.ndjson --source external-mcp --summary
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
| `external-mcp` | Vendor-neutral mapping for external MCP inventory records with recognizable MCP server, tool, config, package, extension, endpoint, or summary fields. |

The `external-mcp` source name selects a Boundary-owned mapping mode covered by
the fixture in
[`fixtures/external-inventory/external-mcp-inventory.ndjson`](../../fixtures/external-inventory/external-mcp-inventory.ndjson).

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
