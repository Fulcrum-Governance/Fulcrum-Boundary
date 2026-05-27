# Inventory

Boundary inventory is read-only. It discovers local MCP client config paths,
classifies MCP servers and tools, and redacts secret-bearing values in rendered
output.

Useful outputs:

```bash
boundary inventory --format json
boundary inventory --format markdown
boundary inventory --format ndjson
```

The NDJSON stream is versioned for tool ingestion and schema-backed in the
repository.
