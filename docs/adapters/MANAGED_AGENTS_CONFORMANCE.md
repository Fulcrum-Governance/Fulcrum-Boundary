# Managed Agents Conformance

Managed Agents remains preview until a live upstream conformance run is
recorded with operator-owned credentials. This document defines the evidence
Boundary needs before upgrading the adapter, claims, README, or readiness
matrix to production.

## Upstream Protocol Snapshot

Date checked: 2026-05-27

Boundary tracks the Anthropic Managed Agents beta surface that uses:

- beta header: `managed-agents-2026-04-01`
- tool-use events: `agent.tool_use`
- MCP tool-use events: `agent.mcp_tool_use`
- tool confirmations: `user.tool_confirmation`
- confirmation results: `allow` and `deny`

The conformance harness verifies sanitized evidence from a Boundary-mediated
run. It does not commit raw upstream transcripts, credentials, bearer tokens,
session secrets, email addresses, or PII.

## Commands

Without credentials:

```bash
go test ./tests/conformance/managed_agents/ -v -timeout 5m
```

Expected result: all conformance tests are skipped and the command exits 0
because `BOUNDARY_MA_CONFORMANCE` is not set.

With live evidence:

```bash
BOUNDARY_MA_CONFORMANCE=true \
ANTHROPIC_API_KEY=... \
BOUNDARY_MA_TRANSCRIPT=/absolute/path/to/managed-agents.sanitized.json \
go test ./tests/conformance/managed_agents/ -v -timeout 5m
```

Raw transcript writes should go outside the repo. If an operator explicitly
sets `BOUNDARY_MA_WRITE_TRANSCRIPT=true`, raw output still must be sanitized
before any repo commit.

## Conformance Criteria

| # | Criterion | Evidence Required |
|---|---|---|
| 1 | Session creation through Boundary proxy | Sanitized transcript marks `session_created_through_boundary=true` and includes `session_id`. |
| 2 | Tool confirmation allow | At least one `user.tool_confirmation` result `allow`. |
| 3 | Tool confirmation deny | At least one `user.tool_confirmation` result `deny`. |
| 4 | MCP tool use | At least one `agent.mcp_tool_use` event. |
| 5 | Thread creation and tracking | `thread_id` or `session.thread_created` evidence. |
| 6 | Budget tracking | Budget ceiling and used amount, with used amount within ceiling. |
| 7 | Trust tracking | Decision evidence includes trust score. |
| 8 | Metadata verification | Decision records include agent, session, thread, tool, action, rule, and trust. |
| 9 | Fail-closed behavior | Pipeline-error case records deny action. |
| 10 | Sanitized transcript evidence | Transcript declares `sanitized=true` and passes secret-pattern checks. |

## Transcript Security Gate

Before committing any transcript:

- redact API keys;
- redact bearer tokens;
- redact session secrets;
- redact email addresses;
- redact PII;
- run a secret scan over the transcript directory;
- commit only sanitized `.sanitized.json` files if needed;
- prefer storing transcript hashes in docs over storing full payloads.

The repository `.gitignore` blocks common raw transcript suffixes under
`tests/conformance/managed_agents/transcript/`.

## Post-Conformance Checklist

When all 10 live criteria pass:

- `docs/ADAPTER_READINESS_MATRIX.md`: Managed Agents -> production
- `claims/boundary_claims.yaml`: Managed Agents claim -> delivered
- `README.md`: Managed Agents moves to Production
- add conformance date
- add sanitized transcript evidence hash
- add operator-owned credential note

Do not perform the production upgrade until live evidence is actually present.
