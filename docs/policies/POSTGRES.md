# Postgres AST Guard

Boundary includes a Postgres SQL interceptor that classifies statements with
`pg_query_go`, the Go binding for the PostgreSQL parser. It is an AST guard,
not a general SQL firewall claim.

## Classes

| Class | Examples | Default behavior |
|---|---|---|
| `READ` | `SELECT`, `SHOW` | Continue through the pipeline. |
| `WRITE` | `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `COPY` | Continue through the pipeline with `sql_class=WRITE`. |
| `ADMIN` | `ALTER`, `CREATE`, `GRANT`, `VACUUM`, `EXPLAIN`, `BEGIN` | Escalate by default. |
| `DESTRUCTIVE` | `DROP`, `TRUNCATE` | Deny by default. |
| `UNKNOWN` | Empty, invalid, or unparsable SQL | Deny fail-closed. |

When SQL parses into multiple statements, Boundary uses the highest-severity
class. For example, `SELECT 1; DROP TABLE users` is `DESTRUCTIVE`.

## Request Annotation

The guard reads `arguments.sql` by default and writes:

- `arguments.sql_class`
- `arguments.sql_statement_types`

These fields are available to later PolicyEval projection as `risk.class` and
`argument.sql_class`.

## Evasion Corpus

The corpus at
[`interceptors/sql/evasion_corpus/postgres.yaml`](../../interceptors/sql/evasion_corpus/postgres.yaml)
covers comments, dollar strings, invalid tokens, mixed statements, destructive
DDL, writes, reads, and administrative statements. The test gate requires at
least 30 cases and verifies each expected class.

## Boundary

The guard is parser-based; it does not execute queries, normalize every
database dialect, inspect application-level authorization, or claim to prevent
all SQL injection. Production deployments still need database permissions,
network isolation, and application authorization around the governed route.
