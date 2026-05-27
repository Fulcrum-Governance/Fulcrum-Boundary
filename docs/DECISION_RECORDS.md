# Decision Records

Fulcrum Boundary emits one structured JSON decision record for every governed
request. Receipt-grade fields are defined in
[`docs/RECEIPTS.md`](RECEIPTS.md); signatures are optional and schema-supported.

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
  "policy_bundle_hash": "sha256:policy-content-hash",
  "request_hash": "sha256:canonical-request-hash",
  "decision_hash": "sha256:canonical-record-hash",
  "request_id": "generated-request-id",
  "envelope_id": "generated-envelope-id"
}
```

Field notes:

- `action`: `allow`, `deny`, `warn`, `escalate`, or `require_approval`.
- `decision_mode`: `deterministic` for static policy outcomes in this release.
- `matched_rule`: the static YAML rule that influenced the verdict, when one matched.
- `policy_file`: the YAML file that supplied the matched rule.
- `policy_bundle_hash`: stable hash of canonical YAML policy content.
- `request_hash`: stable hash of the canonical governed request.
- `raw_shape_hash`: present on parse rejections where no governed request was built.
- `decision_hash`: stable hash of the decision record.
- `gateway_version`: the Boundary build that emitted the record.
- `trace_id`: included when the caller supplied one.

Use `boundary verify-record` to check a stored record against its request and
policy bundle.
