# Claude Code PreToolUse Hook

Put Fulcrum Boundary in front of [Claude Code](https://docs.anthropic.com/en/docs/claude-code)
so an agent's tool calls are decided **before** they run. Claude Code's
`PreToolUse` hook fires after a tool is selected but before it executes; this
integration runs Boundary's preview classifiers at that point, **blocks the tool
call when Boundary returns a `deny` verdict**, and records the verdict.

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

- `pretooluse-boundary.sh` — the hook entrypoint (POSIX `sh`).
- `sessionend-boundary.sh` — the `SessionEnd` companion (POSIX `sh`). It gates
  nothing; it appends one summary line counting what Boundary decided during the
  session, and exits 0 silently when Boundary is absent or too old.
- `settings.snippet.json` — the `hooks.PreToolUse` and `hooks.SessionEnd` wiring.
- `README.md` — short, install-focused.

## How it works

1. Claude Code selects a tool and, before running it, invokes the wrapper with
   the `PreToolUse` event as JSON on stdin.
2. The wrapper parses **nothing**. It probes that a `boundary` binary is present
   and knows the `hook` lane, then `exec`s `boundary hook pretooluse`, which
   reads the same stdin.
3. The binary (`internal/hookboundary`) routes by tool type, classifies, persists
   a decision record, and writes the decision as JSON on stdout.

| Tool routed | Routed to | Classified by |
| --- | --- | --- |
| `Bash` / `Shell` | Command Boundary (preview) | The whole command **line**, decomposed into segments |
| `Edit` / `Write` / `MultiEdit` / `NotebookEdit` | Edit Boundary (preview) | The target path's **shape**, as a synthesized one-file diff |
| everything else | — | Not governed here: allowed silently, no record |

The hook classifies; it does **not** re-run the command or re-apply the edit.
Claude Code performs the actual tool action itself after the hook allows it, so
there is no double execution. Nothing on this path invokes a shell, executes any
part of a command line, or reads or writes the edit target.

## The decision contract

The JSON on stdout — **not** the exit code — is what tells Claude Code what to
do. The hook exits 0 on every decided path.

| Boundary verdict | `permissionDecision` | stdout |
| --- | --- | --- |
| `deny` | `deny` | The dual-shape object: the legacy `{"decision":"block","reason":…}` keys **and** `hookSpecificOutput`, so older and newer clients both block. |
| `require_approval` | `ask` | `hookSpecificOutput` only. |
| `warn` | `ask` | `hookSpecificOutput` only; the warning is the reason. |
| `allow` | — | Nothing at all. Silent happy path; the host's own permission flow runs as usual. |

**The hook never emits `permissionDecision: "allow"`.** In Claude Code's
vocabulary that value does not mean "Boundary has no objection" — it means *skip
the permission system*, a grant the host would not otherwise have made. A gate
that granted the classes it rates riskiest while staying silent on the safest
class would be strictly more permissive than no gate at all, so `warn` asks
rather than grants.

A deny looks like this:

```json
{
  "decision": "block",
  "reason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation (rm -rf dist). This is a routed pre-execution deny; the command was not run.",
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "deny",
    "permissionDecisionReason": "Fulcrum Boundary (Command Boundary preview) denied this command [C4]: destructive local mutation (rm -rf dist). This is a routed pre-execution deny; the command was not run."
  }
}
```

Claude Code reads whichever key its version supports and stops the tool call
before it runs, surfacing the reason to the model. The exit-code-2-plus-stderr
form is an alternative the `PreToolUse` contract also accepts; this hook
standardizes on the JSON form because it carries a structured reason.

## What gets denied (preview policy defaults)

Command Boundary classifies commands into classes `C0`–`C7`; Edit Boundary into
`E0`–`E7`. The classes whose recommended action is `deny` block the call:

- **Commands:** `C4` destructive local mutation, `C5` infrastructure/runtime
  mutation, `C6` credential/secret access. Network egress `C2`, repo mutation
  `C3`, and package lifecycle `C7` resolve to `require_approval`, which this hook
  now enforces as an `ask` rather than waving through.
- **Edits:** `E4` secret-bearing paths and `E7` outside-the-envelope paths —
  which covers writes outside the project root, `.git` mutations, and the
  governance control surfaces below.

The full taxonomies and default postures are in
[`docs/command-boundary/COMMAND_TAXONOMY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/COMMAND_TAXONOMY.md),
[`docs/command-boundary/CLASSIFY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/command-boundary/CLASSIFY.md),
[`docs/edit-boundary/EDIT_TAXONOMY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/EDIT_TAXONOMY.md),
and
[`docs/edit-boundary/APPLY.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/APPLY.md).

### Governance control surfaces

Both routes deny writes to the files that decide **how** this agent is governed,
or that hold the evidence of what was governed:

```text
.boundary/**
.claude/settings.json
.claude/settings.local.json
.claude/hooks/**
**/claude-code/pretooluse-boundary.sh
```

An agent that can rewrite its own `PreToolUse` wiring can turn the hook off
before its next tool call, so this is closed at the classifier rather than left
to policy — and it is closed on **both** routes, because a self-protection that
denies `.claude/settings.json` as an `Edit` while permitting
`cp evil .claude/settings.json` as a `Bash` call is not closed at all.

On the command route only segments whose class **writes** a file are checked
(`C1`, `C4`, and the write segment a shell redirection contributes), so reading
or listing a control surface — `cat .claude/settings.json`, `ls .boundary/hook` —
keeps the verdict its own class earns. Within a checked segment the argument
*position* is not analyzed, so a control surface named as the **source** of a
write is refused too: `cp .claude/settings.json backup.json` denies. Telling
source from destination would need a per-command table of argument grammars, and
a table that is wrong about one command is wrong in the permissive direction.

This is a list of path **shapes**, not an inventory of every way governance could
be disabled: a hook wired from a path not listed here, a settings file outside
these shapes, or a path an interpreter builds at runtime is not matched.

## Install

1. **Get the `boundary` binary on `PATH`.** Build with `make build` (produces
   `./bin/boundary`) and add `bin/` to `PATH`, or set `BOUNDARY_BIN` to the
   binary's absolute path. It must be new enough to carry `boundary hook
   pretooluse`; the wrapper checks and asks rather than falling through if it is
   not.

2. **Make the hooks executable** (after a fresh clone):

   ```bash
   chmod +x integrations/claude-code/pretooluse-boundary.sh
   chmod +x integrations/claude-code/sessionend-boundary.sh
   ```

3. **Wire the hooks** by merging this into your Claude Code settings —
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
       ],
       "SessionEnd": [
         {
           "hooks": [
             {
               "type": "command",
               "command": "$CLAUDE_PROJECT_DIR/integrations/claude-code/sessionend-boundary.sh"
             }
           ]
         }
       ]
     }
   }
   ```

   The `matcher` is a Claude Code tool-name pattern. `$CLAUDE_PROJECT_DIR`
   resolves the hook paths from the project root. If `boundary` is not on `PATH`,
   also export `BOUNDARY_BIN` in the environment Claude Code runs in.

   The `PreToolUse` block is what governs tool calls; the `SessionEnd` block only
   writes the summary log below. Omitting `SessionEnd` costs you that log and
   changes nothing about what is governed. The plugin manifest at
   `hooks/hooks.json` registers the same two hooks, so a plugin install needs no
   settings edit at all.

4. **Reload** — restart Claude Code or run `/hooks` so it loads the hook.

The same JSON ships at
[`integrations/claude-code/settings.snippet.json`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/integrations/claude-code/settings.snippet.json).

## Tools this hook governs

| Tool | Routed to | Governed |
| --- | --- | --- |
| `Bash` | Command Boundary | Yes |
| `Edit`, `Write`, `MultiEdit`, `NotebookEdit` | Edit Boundary | Yes |
| `Read`, `Grep`, `Glob`, web tools, MCP tools, every other tool | — | No (allowed silently, no record) |

MCP tool calls are **not** governed here. Govern those at the MCP route — see
[`docs/GOVERN_MCP_SERVER.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/GOVERN_MCP_SERVER.md),
Boundary's production route.

## The decision Boundary leaves

Every **decided** event — allow, warn, ask, and deny alike — produces a
`governance.DecisionRecordV1`, persisted **before** the decision reaches stdout,
so a decision Claude Code acts on is one Boundary already wrote down. A tool this
hook does not route leaves **no** record, because nothing was decided.

Two artifacts are written per decision, under `BOUNDARY_HOOK_DIR` (default
`.boundary/hook`, relative to the hook process's working directory), at mode
`0600` inside a `0700` directory:

```text
.boundary/hook/decision-records.jsonl        # append-only, one record per line
.boundary/hook/records/<ts>-<record_id>.json # one single-record object per decision
```

The per-record file is what `boundary verify-record` consumes directly (it
rejects a multi-record `.jsonl`). The records also work with `boundary explain`
and `boundary replay`; see
[`docs/CLI_REFERENCE.md`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/CLI_REFERENCE.md).

```bash
boundary verify-record .boundary/hook/records/<file>.json
```

Records are **hash-verifiable for integrity** — `verify-record` recomputes
`decision_hash`. They are not authenticity, not proof the verdict was correct,
and not proof the action was executed or prevented. `execution_claim` on these
records reports `upstream_called=false` / `executed=false`: this route decides
before execution and never runs the command or writes the file. That is a
self-report about the hook, not corroboration that nothing else ran.

**A record that cannot be written escalates the call.** The fail mode governs
whether Boundary could *classify*; the sink is whether Boundary could *record*,
and an unrecorded decision is escalated to the user rather than waved through. A
deny stays a deny — escalation may only strengthen a verdict, never soften one.
Both artifacts are written or neither is, so a failed write leaves nothing behind
that could read as a verdict the hook did not enforce.

## Configuration

| Variable | Default | Effect |
| --- | --- | --- |
| `BOUNDARY_BIN` | `boundary` | Path to the boundary binary. Read by the wrapper; everything below is read by the binary. |
| `BOUNDARY_HOOK_FAILMODE` | `ask` | Posture on an internal **fault** (event unparseable, nothing to classify, classifier error): `ask` prompts the user, `open` allows so a flaky hook never bricks a session, `closed` denies. A Boundary `deny` always blocks regardless. |
| `BOUNDARY_HOOK_AGENT_ID` | `claude-code` | Advisory agent-id label written to the decision record. Nothing authenticates it. |
| `BOUNDARY_HOOK_DIR` | `.boundary/hook` | Directory decision records are written to. |
| `BOUNDARY_HOOK_DEBUG` | _(unset)_ | When non-empty, prints diagnostic lines to stderr from both the wrapper and the binary. |

The `ask` default is deliberate. A **deny** is a decision and always blocks; a
fault means Boundary could not decide, so it declines to answer for the operator
in either direction. `open` restores the older permissive posture where a fault
allows silently; `closed` denies on a fault.

## Dependencies

- **`boundary`** on `PATH`, or `BOUNDARY_BIN` set to its absolute path.
- Nothing else. **No `jq`**, no Python, no other runtime — the wrapper parses no
  JSON, so there is no reduced no-`jq` path that could diverge from the real one.

## When Boundary cannot decide

The wrapper resolves two conditions on its own, both by printing a
`permissionDecision: "ask"` object and exiting 0:

- **No binary at all** — `boundary` is not on `PATH` and `BOUNDARY_BIN` points
  nowhere.
- **A binary that predates the `hook` lane** — an older `boundary` left over from
  before this hook shipped exits non-zero with empty stdout, which Claude Code
  reads as a hook error and lets the tool call through ungoverned and silently.
  The wrapper probes `boundary hook --help` before touching stdin, so this is
  caught rather than mistaken for a working install.

Both branches ask rather than exit non-zero, because a hook that exits non-zero
is treated as a hook **error** and the tool call proceeds ungoverned and
silently. Asking makes the ungoverned state visible. If you do not want to be
asked, remove the hook from `settings.json` rather than leaving a hook installed
that cannot decide.

## Honest scope and limitations

- **Routed-only.** The hook governs the tool calls wired in `settings.json`. That
  is the boundary. An un-wired tool, an MCP tool, a tool a subprocess runs on its
  own, or direct shell use outside Claude Code is a **bypass** and is not
  governed. This hook does not and cannot claim total coverage of what an agent
  can do.
- **Delivered previews.** Command Boundary and Edit Boundary are previews, not
  production GA. Their classification posture is conservative and may change.
- **Compound lines are decomposed, but not by a real shell.** The whole Bash line
  is split into simple commands on `&&`, `||`, `;`, `|`, `&`, and newline;
  command substitution, subshells, and `sh -c` payloads are decomposed
  recursively; an output redirection contributes the file it writes; and a
  command in argument position (`find -exec`, `xargs`) is decomposed too. The
  most restrictive segment sets the verdict, so the shell hook's leading-command
  gap is closed. What remains is the decomposer's own modelling limit — heredocs,
  process substitution, `eval`, unbalanced quotes, and nesting past
  `MaxLineDepth` come back undecomposable and escalate to `ask`; they are never
  allowed, but no class claim is made about them either. Argument-position shapes
  outside the modelled set (`watch`, `parallel`, `make`, an interpreter's inline
  program) leave their payload unclassified.
- **The edit route classifies by path shape.** The synthesized diff names the
  target path with no content hunk, so content-only edit classes are **not**
  asserted by this hook. Path shape is: an absolute target is resolved against
  the project root, so a write outside it stays absolute and denies as `E7`; a
  target inside it is classified by its position, so a leading `.git` or
  `.claude` component still counts. Both paths are canonicalized as far as the
  filesystem resolves them, so a symlink out of the project is judged where the
  write actually lands — a point-in-time answer, and a symlink swapped between
  the check and the write is not caught. Case is not folded, so on a
  case-insensitive filesystem a case-flipped root is not matched.
- **Redaction is pattern-based.** `commandboundary.RedactArgs` and Edit
  Boundary's secret-path redaction run before anything is persisted, and cover
  secret-looking flags and paths plus inline `scheme://user:password@host` URL
  credentials. Pattern matching is not a guarantee that no secret-looking value
  survives into a record or into the reason handed back to the model.
- **Hash-verifiable, not tamper-proof and not "proved".** Boundary records are
  hash-verifiable for **integrity** — not authenticity, not proof the verdict was
  right, and not proof of safety. This integration does not prevent all dangerous
  actions and makes no universal prompt-injection or agent-safety claim. Boundary
  does not emit `proved` decisions.

## Verify the hook yourself

```bash
# Syntax (POSIX):
sh -n integrations/claude-code/pretooluse-boundary.sh

# Settings JSON parses:
jq . < integrations/claude-code/settings.snippet.json

# Deny a destructive command (expect a {"decision":"block",...} JSON):
printf '{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Deny a destructive command smuggled behind a benign one:
printf '{"tool_name":"Bash","tool_input":{"command":"git status && rm -rf ~"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Deny a rewrite of the hook's own wiring, through the Bash route:
printf '{"tool_name":"Bash","tool_input":{"command":"cp /tmp/x .claude/settings.json"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Allow an observe command (expect no output, exit 0):
printf '{"tool_name":"Bash","tool_input":{"command":"git status"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Deny a write to a secret-bearing path (expect a block JSON):
printf '{"tool_name":"Write","tool_input":{"file_path":"config/.env","content":"x"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Deny a write outside the project root:
printf '{"tool_name":"Write","tool_input":{"file_path":"/etc/passwd","content":"x"}}' \
  | BOUNDARY_BIN=./bin/boundary sh integrations/claude-code/pretooluse-boundary.sh

# Check what was recorded:
boundary verify-record "$(ls -t .boundary/hook/records/*.json | head -1)"
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
