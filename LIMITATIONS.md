# Limitations

The MCP Safety Gateway preview proves pre-execution control for one concrete
tool path: MCP-style Postgres calls routed through Fulcrum Boundary.

The included SQL policy is demo-grade destructive-action blocking. It is not a
general SQL firewall. The first release only demonstrates substring rules such
as `DROP TABLE`, `DELETE FROM`, `TRUNCATE`, and `ALTER TABLE`.

Known limits:

- SQL comments, whitespace tricks, dialect-specific syntax, and semantic SQL
  analysis are outside this preview unless explicitly covered by tests.
- Direct tool calls are governed only when routed through Boundary.
- Decision records are structured logs, not cryptographic receipts.
- The Docker bypass proof is topology-specific: the demo agent is isolated from
  the backend network, while the gateway can reach Postgres.
