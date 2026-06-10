# Fulcrum Boundary — Claude Code PreToolUse hook

A [Claude Code](https://docs.anthropic.com/en/docs/claude-code) `PreToolUse`
hook that runs Fulcrum Boundary's preview classifiers **before** Claude Code
runs a tool, and blocks the tool call when Boundary returns a `deny` verdict.

- `Bash` / shell tool calls are routed through **Command Boundary**
  (`boundary command classify`).
- `Edit` / `Write` / `MultiEdit` / `NotebookEdit` tool calls are routed through
  **Edit Boundary** (`boundary edit inspect`).

The hook adds portable, testable policy and a clear, redaction-aware deny reason
to the raw hook surface. It governs **only the tool calls it is wired to
intercept** — that routed interception *is* the boundary. A tool call that does
not reach this hook is a bypass and is not governed. Command Boundary and Edit
Boundary are **delivered previews**, not production GA surfaces; treat their
verdicts as preview-grade.

Full documentation, scope, and honest limitations:
[`docs/integrations/CLAUDE_CODE_HOOK.md`](../../docs/integrations/CLAUDE_CODE_HOOK.md).

## Files

| File | Purpose |
| --- | --- |
| `pretooluse-boundary.sh` | The hook. POSIX `sh`; reads the PreToolUse JSON event on stdin. |
| `settings.snippet.json` | The `hooks.PreToolUse` wiring to merge into your Claude Code settings. |

## Quick install

1. Build (or install) the `boundary` binary so it is on your `PATH`:

   ```bash
   make build            # produces ./bin/boundary
   # then put bin/ on PATH, or set BOUNDARY_BIN to the absolute binary path
   ```

2. Make the hook executable (it ships executable, but after a fresh clone):

   ```bash
   chmod +x integrations/claude-code/pretooluse-boundary.sh
   ```

3. Merge `settings.snippet.json` into your Claude Code settings
   (`.claude/settings.json` in the project, or `~/.claude/settings.json` for all
   projects). The snippet uses `$CLAUDE_PROJECT_DIR` so the path resolves from
   the project root. If your `boundary` binary is not on `PATH`, also export
   `BOUNDARY_BIN` in your environment.

4. Restart Claude Code (or run `/hooks`) so it picks up the new hook.

## Dependencies

- **`boundary`** on `PATH`, or `BOUNDARY_BIN` set to its absolute path. Required.
- **`jq`** on `PATH`. Strongly recommended. Without `jq` the hook falls back to a
  reduced POSIX-only parse that handles the common single-line tool-input shapes;
  if it cannot parse the event it follows `BOUNDARY_HOOK_FAILMODE`.

## Environment knobs

| Variable | Default | Effect |
| --- | --- | --- |
| `BOUNDARY_BIN` | `boundary` | Path to the boundary binary. |
| `BOUNDARY_HOOK_FAILMODE` | `open` | On an internal hook fault (binary missing, event unparseable, classifier error): `open` allows the call so a flaky hook never bricks a session; `closed` blocks it. A Boundary `deny` always blocks regardless of this setting. |
| `BOUNDARY_HOOK_AGENT_ID` | _(unset)_ | Optional advisory agent-id label. |
| `BOUNDARY_HOOK_DEBUG` | _(unset)_ | When non-empty, prints diagnostic lines to stderr. |

## Deny contract

On a Boundary `deny` the hook prints Claude Code's JSON decision to stdout and
exits 0:

```json
{ "decision": "block", "reason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation. ..." }
```

Claude Code reads that JSON and stops the tool call before it runs, surfacing the
reason to the model. On allow (or any non-`deny` verdict) the hook is silent and
exits 0. See the canonical doc for the full contract and the alternative
exit-code-2 form.

## Scope (read this)

- This hook governs the tool calls wired in `settings.json`. That route is the
  boundary; an un-wired tool, an MCP tool, or a subprocess Claude spawns that
  runs another command is a **bypass** and is not governed.
- `boundary command classify` parses **argv only** and does not interpret shell
  operators. A compound Bash line (`safe && dangerous`) is classified by its
  **leading** command, so a dangerous command chained after a benign one is not
  decomposed.
- The edit route classifies by **path shape** (it reliably denies secret-bearing
  paths, class `E4`). It does not synthesize the content hunk, so content-only
  edit classes are not asserted by this hook.
- Boundary decision records are **hash-verifiable** (integrity), not proof of
  authenticity and not "tamper-proof". Nothing here proves a tool call is safe;
  it records and gates a routed pre-execution verdict.
