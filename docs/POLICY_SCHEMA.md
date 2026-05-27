# Policy Schema v1

Boundary accepts two policy file shapes:

- Legacy v0.2.0 static YAML:
  `name`, `version`, and top-level `rules`.
- Schema v1 YAML:
  `schema_version: "1"` with a nested `policy` envelope.

The loader remains backward compatible with v0.2.0 files. Any file with a
`schema_version` or `policy` envelope is validated as v1 and fails startup or
`boundary verify` on schema errors.

## v1 Shape

```yaml
schema_version: "1"
policy:
  name: postgres-production
  version: "1.0.0"
  transport: mcp
  rules:
    - name: deny-destructive-postgres
      tool: query
      action: deny
      reason: Destructive SQL is blocked before upstream execution.
      conditions:
        - type: ast_class
          value: DESTRUCTIVE
```

Required fields:

| Field | Requirement |
|---|---|
| `schema_version` | Must be the string `"1"`. |
| `policy.name` | Required non-empty policy name. |
| `policy.version` | Required non-empty policy version. |
| `policy.rules` | One or more rules. |
| `rules[].name` | Required non-empty rule name. |
| `rules[].tool` | Required tool name or glob. |
| `rules[].action` | One of `allow`, `deny`, `warn`, `audit`, `escalate`, or `require_approval`. |

Optional rule fields:

| Field | Meaning |
|---|---|
| `transport` | Restricts the rule to one Boundary transport. Inherits `policy.transport` when omitted. |
| `tenant_scope` | Restricts the rule to listed tenant IDs. |
| `agent_scope` | Restricts the rule to listed agent IDs. |
| `decision_mode` | Labels the rule outcome, for example `deterministic` or `classified`. |
| `match` | Single condition. |
| `conditions` | All listed conditions must match. |
| `metadata` | Operator-owned key/value metadata. |

## Condition Types

| Type | Required input | Meaning |
|---|---|---|
| `contains` | `field` plus `contains` or `value` | Field contains text. |
| `not_contains` | `field` plus `contains` or `value` | Field does not contain text. |
| `equals` | `field` plus `value` | Field equals value. |
| `not_equals` | `field` plus `value` | Field does not equal value. |
| `regex` | `field` plus `regex`, `value`, or `contains` | Field matches a compiled regular expression. |
| `transport_is` | `value` | Request transport equals value. |
| `agent_in` | `value` or `values` | Agent ID is listed. |
| `agent_not_in` | `value` or `values` | Agent ID is not listed. |
| `ast_class` | `value` or `values` | Matches an interceptor-projected class such as `READ`, `WRITE`, `ADMIN`, `DESTRUCTIVE`, or `UNKNOWN`. |

`case_insensitive: true` applies to string matching where relevant.

## Request Context

Boundary now projects the full governance request into PolicyEval:

- tenant ID, agent ID, transport, tool, action, and envelope ID
- structured tool arguments as `argument.<key>` attributes
- trust score and trust state when available
- interceptor risk class as `risk.class`
- derived resource IDs
- canonical `request.hash`
- provenance fields including adapter and trace ID

For SQL policies, the Postgres AST guard annotates `arguments.sql_class` and
PolicyEval receives the same value as `risk.class`. Static policies run before
interceptors, so AST class matching is most useful in PolicyEval or in adapters
that pre-populate `sql_class`.

## Verification

Validate a directory:

```bash
boundary verify --policies ./policies
```

The same loader is used by `boundary serve --policies ./policies`; invalid v1
policies fail startup instead of being downgraded to warnings.
