# Decision Records

Fulcrum Boundary emits one structured JSON decision record for every governed
request. In the MCP Safety Gateway preview these records are logs, not
cryptographic receipts.

Example:

```json
{
  "msg": "governance_decision",
  "gateway_version": "0.2.0-dev",
  "agent_id": "demo-agent",
  "tenant_id": "demo",
  "tool_name": "query",
  "action": "deny",
  "reason": "Destructive SQL blocked by Boundary policy",
  "decision_mode": "deterministic",
  "matched_rule": "block-drop-table",
  "policy_file": "postgres.yaml",
  "request_id": "generated-request-id",
  "envelope_id": "generated-envelope-id"
}
```

Field notes:

- `action`: `allow`, `deny`, `warn`, `escalate`, or `require_approval`.
- `decision_mode`: `deterministic` for static policy outcomes in this release.
- `matched_rule`: the static YAML rule that influenced the verdict, when one matched.
- `policy_file`: the YAML file that supplied the matched rule.
- `gateway_version`: the Boundary build that emitted the record.
- `trace_id`: included when the caller supplied one.

Receipt-grade records are a later upgrade and require policy hashes, input
hashes, build identity, and a verification command.
