# External Ingest

Boundary can ingest external MCP inventory NDJSON and map recognizable
MCP-related records into Boundary inventory records.

```bash
boundary inventory ingest --file external-mcp-inventory.ndjson --source external-mcp --summary
```

This is a vendor-neutral ingest surface. Boundary does not depend on, shell out
to, import, endorse, or claim compatibility with any named third-party scanner.

Boundary only promotes records that relate to agent clients, MCP configs, MCP
servers, MCP tools, capability classes, risk paths, governed routes, or policy
recommendations. Other endpoint facts remain report-only.
