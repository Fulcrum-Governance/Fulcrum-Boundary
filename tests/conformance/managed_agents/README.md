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
- sanitized transcript evidence;
- `mode` must be `"live"` (`TestLiveModeTranscriptOnly` rejects stub
  transcripts so an offline driver run can never pass as live evidence).

## Driver

`driver/` contains the session driver that produces these transcripts. It
runs one Managed Agents session through the Boundary adapter stack
(`SessionProxy` + `ToolResolver` + `governance.Pipeline`, embedded in-process
because `cmd/boundary` exposes no managed-agents listener) and writes a raw
event log, a sanitized transcript, and `provenance.json` under `--out-dir`,
which must be outside the git worktree.

```bash
go run ./tests/conformance/managed_agents/driver --mode stub \
  --out-dir /tmp/ma-run
```

- `--mode stub` (default) is fully offline: an in-process fake upstream emits
  scripted events covering all conformance scenarios. It never reads
  `BOUNDARY_MA_UPSTREAM_KEY` and writes `mode: "stub"` evidence.
- `--mode live` refuses to start unless `--i-understand-this-spends-money`
  is passed, the gate file
  `~/.fulcrum-evidence/ma-conformance/LIVE_GO` exists, and
  `BOUNDARY_MA_UPSTREAM_KEY` is set. It enforces driver-side spend, turn,
  output-token, and wall-clock ceilings, aborting at 80 percent of the spend
  ceiling and failing closed when usage data is missing.

Stub runs are validated in-process by the driver's own tests
(`driver/internal/madriver`), which run the same criterion checks this
harness uses (see `checks.go`). The env-gated live run is:

```bash
BOUNDARY_MA_CONFORMANCE=true \
BOUNDARY_MA_TRANSCRIPT=/path/to/transcript.sanitized.json \
go test ./tests/conformance/managed_agents/ -v -timeout 5m
```

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
  "mode": "live",
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
