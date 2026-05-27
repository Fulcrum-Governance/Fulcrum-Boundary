# Managed Agents Conformance Harness

This package verifies live Managed Agents conformance evidence captured from a
Boundary-mediated run. It intentionally skips unless explicitly enabled so
ordinary CI and local test runs do not call upstream services or require
operator credentials.

## Default Run

```bash
go test ./tests/conformance/managed_agents/ -v -timeout 5m
```

Expected result: all conformance tests are skipped with exit code 0 because
`BOUNDARY_MA_CONFORMANCE` is not set.

## Live Evidence Run

Run the Managed Agents session through Boundary with operator-owned
credentials, sanitize the transcript, then point this harness at the sanitized
file:

```bash
BOUNDARY_MA_CONFORMANCE=true \
ANTHROPIC_API_KEY=... \
BOUNDARY_MA_TRANSCRIPT=/absolute/path/to/managed-agents.sanitized.json \
go test ./tests/conformance/managed_agents/ -v -timeout 5m
```

The harness verifies the sanitized transcript contains evidence for:

- session creation through the Boundary proxy;
- tool confirmation allow;
- tool confirmation deny;
- MCP tool use via `agent.mcp_tool_use`;
- thread creation and tracking;
- budget tracking against a ceiling;
- trust tracking in decision records;
- decision metadata: `agent_id`, `session_id`, `thread_id`, `tool`, `action`,
  `rule`, and `trust`;
- fail-closed behavior on pipeline error;
- sanitized transcript evidence.

## Transcript Safety

NEVER commit raw transcripts. Always sanitize first. Raw transcript writes
should go outside the repo unless `BOUNDARY_MA_WRITE_TRANSCRIPT=true` is set.

Before committing any transcript:

- redact API keys;
- redact bearer tokens;
- redact session secrets;
- redact email addresses;
- redact PII;
- run a secret scan over the transcript directory;
- commit only sanitized `.sanitized.json` files if needed;
- prefer storing transcript hashes in docs over storing full payloads.

Ignored raw transcript patterns are declared in the repository `.gitignore`.

## Transcript Shape

The sanitized evidence file is JSON:

```json
{
  "sanitized": true,
  "session_created_through_boundary": true,
  "session_id": "sess-redacted",
  "thread_id": "thread-redacted",
  "agent_id": "agent-redacted",
  "events": [
    {"type": "session.thread_created", "session_id": "sess-redacted", "thread_id": "thread-redacted"},
    {"type": "agent.mcp_tool_use", "session_id": "sess-redacted", "thread_id": "thread-redacted", "tool": "safe_tool"}
  ],
  "confirmations": [
    {"tool_use_id": "tool-redacted-1", "result": "allow", "tool": "safe_tool"},
    {"tool_use_id": "tool-redacted-2", "result": "deny", "tool": "blocked_tool"}
  ],
  "decisions": [
    {
      "agent_id": "agent-redacted",
      "session_id": "sess-redacted",
      "thread_id": "thread-redacted",
      "tool": "safe_tool",
      "action": "allow",
      "rule": "allow-safe-tool",
      "trust": 1.0
    }
  ],
  "budget": {"ceiling": 1.0, "used": 0.25},
  "trust": {"tracked": true, "score": 1.0},
  "fail_closed": {"observed": true, "action": "deny", "reason": "pipeline error"},
  "transcript_sha256": ""
}
```
