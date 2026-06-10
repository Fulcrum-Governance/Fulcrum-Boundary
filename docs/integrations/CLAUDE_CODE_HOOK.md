# Claude Code PreToolUse Hook

Put Fulcrum Boundary in front of [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
so an agent's tool calls are decided **before** they run. Claude Code's
`PreToolUse` hook fires after a tool is selected but before it executes; this
integration runs Boundary's preview classifiers at that point and **blocks the
tool call when Boundary returns a `deny` verdict**.

Boundary governs Claude Code **only for the tool calls this hook is wired to
intercept**. That routed interception *is* the boundary. A tool call that does
not reach the hook — a tool not listed in the matcher, a tool a subprocess runs
on its own, direct shell use outside Claude Code — is a **bypass** and is not
governed. Closing those paths is a deployment responsibility, not a hook flag.

Command Boundary and Edit Boundary are **delivered previews**, not production GA
surfaces. Treat their verdicts as preview-grade and validate against your own
policy before relying on them.

## Files

The integration lives at
[`integrations/claude-code/`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/integrations/claude-code):

- `pretooluse-boundary.sh` — the hook (POSIX `sh`).
- `settings.snippet.json` — the `hooks.PreToolUse` wiring.
- `README.md` — short, install-focused.

## How it works

1. Claude Code selects a tool and, before running it, invokes the hook with the
   `PreToolUse` event as JSON on stdin. The hook reads `tool_name` and
   `tool_input`.
2. The hook routes by tool type:
   - **`Bash`** (shell) → `boundary command classify --json -- <argv>`
     (Command Boundary).
   - **`Edit` / `Write` / `MultiEdit` / `NotebookEdit`** → a minimal one-file
     diff naming the target path is piped to `boundary edit inspect --json
     --stdin` (Edit Boundary).
3. The hook reads `recommended_action` from Boundary's classification. If it is
   `deny`, the hook emits Claude Code's JSON block decision and the tool call is
   stopped before it runs. Any other verdict (`allow`, `warn`,
   `require_approval`, or an untracked tool) is allowed silently.

The hook classifies; it does **not** re-run the command or re-apply the edit.
Claude Code performs the actual tool action itself after the hook allows it, so
there is no double execution.

### Boundary subcommands used

| Tool routed | Subcommand | Reads |
| --- | --- | --- |
| `Bash` / shell | `boundary command classify --json -- <argv>` | `.recommended_action`, `.class`, `.reason` |
| `Edit` / `Write` / `MultiEdit` / `NotebookEdit` | `boundary edit inspect --json --stdin` | `.recommended_action`, `.highest_class`, `.findings[0].reason` |

Both are read-only classifiers — they never execute a command, never apply a
patch, and never invoke a shell. They redact secret-looking arguments and paths
before producing output. See
[`docs/command-boundary/CLASSIFY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/CLASSIFY.md)
and
[`docs/edit-boundary/INSPECT.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/INSPECT.md).

### What gets denied (preview policy defaults)

Command Boundary classifies commands into classes `C0`–`C7`; Edit Boundary into
`E0`–`E7`. The hook blocks **only the classes whose recommended action is
`deny`**:

- Commands: `C4` destructive local mutation, `C5` infrastructure/runtime
  mutation, `C6` credential/secret access. (Network egress `C2`, repo mutation
  `C3`, and package lifecycle `C7` resolve to `require_approval`, which this hook
  **allows** — the hook is a deny gate, not an approval workflow.)
- Edits: `E4` secret-bearing edit and the hard-deny edit classes. The hook
  reliably denies writes to **secret-bearing paths** (`E4`).

The full taxonomies and default postures are in
[`docs/command-boundary/COMMAND_TAXONOMY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/COMMAND_TAXONOMY.md),
[`docs/command-boundary/RUN.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/RUN.md),
[`docs/edit-boundary/EDIT_TAXONOMY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/EDIT_TAXONOMY.md),
and
[`docs/edit-boundary/APPLY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/APPLY.md).

## Install

1. **Get the `boundary` binary on `PATH`.** Build with `make build` (produces
   `./bin/boundary`) and add `bin/` to `PATH`, or set `BOUNDARY_BIN` to the
   binary's absolute path.

2. **Make the hook executable** (after a fresh clone):

   ```bash
   chmod +x integrations/claude-code/pretooluse-boundary.sh
   ```

3. **Wire the hook** by merging this into your Claude Code settings —
   `.claude/settings.json` in the project (committed or local), or
   `~/.claude/settings.json` for all projects:

   ```json
   {
     "hooks": {
       "PreToolUse": [
         {
           "matcher": "Bash|Edit|Write|MultiEdit|NotebookEdit",
           "hooks": [
             {
               "type": "command",
               "command": "$CLAUDE_PROJECT_DIR/integrations/claude-code/pretooluse-boundary.sh"
             }
           ]
         }
       ]
     }
   }
   ```

   The `matcher` is a Claude Code tool-name pattern. `$CLAUDE_PROJECT_DIR`
   resolves the hook path from the project root. If `boundary` is not on `PATH`,
   also export `BOUNDARY_BIN` in the environment Claude Code runs in.

4. **Reload** — restart Claude Code or run `/hooks` so it loads the hook.

The same JSON ships at
[`integrations/claude-code/settings.snippet.json`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/integrations/claude-code/settings.snippet.json).

## Tools this hook governs

| Tool | Routed to | Governed |
| --- | --- | --- |
| `Bash` | Command Boundary (`boundary command classify`) | Yes |
| `Edit`, `Write`, `MultiEdit`, `NotebookEdit` | Edit Boundary (`boundary edit inspect`) | Yes |
| `Read`, `Grep`, `Glob`, web tools, MCP tools, every other tool | — | No (allowed silently) |

MCP tool calls are **not** governed here. Govern those at the MCP route — see
[`docs/GOVERN_MCP_SERVER.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/GOVERN_MCP_SERVER.md),
Boundary's production route.

## The deny contract

This hook uses Claude Code's **JSON PreToolUse decision** form. On a Boundary
`deny` it writes a single JSON object carrying **both** the legacy and current
deny shapes to stdout and exits 0, so older and newer Claude Code clients both
block the call:

```json
{
  "decision": "block",
  "reason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation. This is a routed pre-execution deny; the command was not run.",
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation. This is a routed pre-execution deny; the command was not run."
  }
}
```

Claude Code reads whichever key its version supports — the legacy
`decision: "block"` or the current `hookSpecificOutput.permissionDecision:
"deny"` — and stops the tool call before it runs, surfacing the reason to the
model. Both keys carry the same deny verdict, so emitting both is safe. The
exit-code-2-plus-stderr form is an alternative the `PreToolUse` contract also
accepts; this hook standardizes on the JSON form because it carries a structured
reason. On any non-`deny` verdict the hook is silent and exits 0.

## The decision Boundary leaves

The classifiers this hook calls (`command classify`, `edit inspect`) are
read-only and produce a classification, not a persisted decision record. The
classification carries the class, risk, recommended action, redacted argv/path,
and a stable `patch_sha256` (for edits). It is **hash-verifiable** in the sense
that the edit inspection binds to the exact patch bytes; it is integrity, not
authenticity, and not proof the verdict was correct or enforced.

To persist a **hash-bound JSONL decision record** for a routed command or edit —
the receipt-grade lane — use the execution wrappers directly, outside the hot
path of this hook:

- `boundary command run -- <cmd>` writes
  `.boundary/command/decision-records.jsonl` (and does not execute `deny` /
  `require_approval` commands). See
  [`docs/command-boundary/RUN.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/RUN.md).
- `boundary edit apply --patch <p> --dry-run` writes
  `.boundary/edit/decision-records.jsonl` and never applies the patch. See
  [`docs/edit-boundary/APPLY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/APPLY.md).

These records are verifiable with the decision-record tooling
([`boundary verify-record`, `explain`, `replay`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLI_REFERENCE.md)).
The hook itself stays a fast, side-effect-free gate.

## Configuration

| Variable | Default | Effect |
| --- | --- | --- |
| `BOUNDARY_BIN` | `boundary` | Path to the boundary binary. |
| `BOUNDARY_HOOK_FAILMODE` | `open` | On an internal hook **fault** (binary missing, event unparseable, classifier error): `open` allows the call so a flaky hook never bricks an interactive session; `closed` blocks it. A Boundary `deny` always blocks regardless. |
| `BOUNDARY_HOOK_AGENT_ID` | _(unset)_ | Optional advisory agent-id label. |
| `BOUNDARY_HOOK_DEBUG` | _(unset)_ | When non-empty, prints diagnostic lines to stderr. |

The `open` default mirrors Boundary's own posture for interactive transports: a
policy that **denies** is a decision and always blocks; a crashed or missing
classifier is a **fault** and, for an interactive coding session, defaults to
permissive so a broken hook does not lock you out. Set
`BOUNDARY_HOOK_FAILMODE=closed` where a missing classifier should stop the tool.

## Dependencies

- **`boundary`** on `PATH`, or `BOUNDARY_BIN` absolute. Required.
- **`jq`** on `PATH`. Strongly recommended. Without `jq`, the hook uses a reduced
  POSIX-only parse (`grep`/`sed`) that handles the common single-line tool-input
  shapes; tool inputs with embedded quotes are not parsed on that path, and the
  edit deny reason degrades to a generic label. If the reduced parse cannot read
  the event, the hook follows `BOUNDARY_HOOK_FAILMODE`.

## Honest scope and limitations

- **Routed-only.** The hook governs the tool calls wired in `settings.json`. That
  is the boundary. An un-wired tool, an MCP tool, a tool a subprocess runs on its
  own, or direct shell use outside Claude Code is a **bypass** and is not
  governed. This hook does not and cannot claim total coverage of what an agent
  can do.
- **Delivered previews.** Command Boundary and Edit Boundary are previews, not
  production GA. Their classification posture is conservative and may change.
- **Leading-command only for compound lines.** `boundary command classify` parses
  **argv** and does not interpret shell operators (`&&`, `||`, `|`, `;`,
  subshells, command substitution). A compound Bash line is classified by its
  **leading** command, so a dangerous command chained or substituted after a
  benign one is **not** decomposed and may be allowed. This mirrors Boundary's
  own "parsed as argv, no `sh -c`" model.
- **Edit route is path-shape based.** The hook synthesizes a one-file diff naming
  the target path and classifies that. It reliably denies **secret-bearing
  paths** (`E4`). It does not reconstruct the content hunk, so content-only edit
  classes (and reliable outside-project-scope `E7` detection, which depends on
  path markers in the real patch) are **not** asserted by this hook.
- **`require_approval` is allowed, not enforced.** This hook is a deny gate. It
  does not implement an approval workflow, so classes that resolve to
  `require_approval` (e.g. network egress, repo mutation, source edits) are
  allowed through. Use `boundary command run` / `boundary edit apply` for the
  approval-aware wrappers.
- **Hash-verifiable, not tamper-proof and not "proved".** Boundary records are
  hash-verifiable for **integrity** — not authenticity, not a proof the verdict
  was right, and not a proof of safety. This integration does not prevent all
  dangerous actions and makes no universal prompt-injection or agent-safety
  claim. Boundary does not emit `proved` decisions.
- **Fail-open by default.** An internal fault allows the call unless
  `BOUNDARY_HOOK_FAILMODE=closed`.

## Verify the hook yourself

```bash
# Syntax (POSIX):
sh -n integrations/claude-code/pretooluse-boundary.sh

# Settings JSON parses:
jq . < integrations/claude-code/settings.snippet.json

# Deny a destructive command (expect a {"decision":"block",...} JSON):
printf '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Allow an observe command (expect no output, exit 0):
printf '{"tool_name":"Bash","tool_input":{"command":"git status"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Deny a write to a secret-bearing path (expect a block JSON):
printf '{"tool_name":"Write","tool_input":{"file_path":"config/.env","content":"x"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh
```

## Related

- [`docs/command-boundary/`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/README.md)
  — Command Boundary preview (routed command paths).
- [`docs/edit-boundary/`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/README.md)
  — Edit Boundary preview (routed file mutations).
- [`docs/GOVERN_MCP_SERVER.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/GOVERN_MCP_SERVER.md)
  — the production MCP route, for governing MCP tools.
- [`LIMITATIONS.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/LIMITATIONS.md)
  — the routed-only constraint in full.
