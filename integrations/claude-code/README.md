# Fulcrum Boundary — Claude Code PreToolUse hook

A [Claude Code](https://docs.anthropic.com/en/docs/claude-code) `PreToolUse`
hook that runs Fulcrum Boundary's preview classifiers **before** Claude Code
runs a tool, and blocks the tool call when Boundary returns a `deny` verdict.

- `Bash` / shell tool calls are routed through **Command Boundary**.
- `Edit` / `Write` / `MultiEdit` / `NotebookEdit` tool calls are routed through
  **Edit Boundary**.

`pretooluse-boundary.sh` is a thin wrapper: it probes for the `boundary` binary
and then `exec`s `boundary hook pretooluse`, which reads the same stdin. All
routing, classification, and decision recording happen in the binary. The wrapper
parses no JSON, so **`jq` is not used and not required**.

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
| `pretooluse-boundary.sh` | The hook entrypoint. POSIX `sh`; probes for the binary, then `exec`s `boundary hook pretooluse`. |
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

- **`boundary`** on `PATH`, or `BOUNDARY_BIN` set to its absolute path.
- Nothing else. No `jq`, no Python, no other runtime.

If the binary is **not** found — or is found but predates the `hook` lane, which
the wrapper checks with a `boundary hook --help` probe before touching stdin —
the wrapper prints a fixed `permissionDecision: "ask"` object naming the install
or upgrade commands and exits 0. That is deliberate: a hook that exits non-zero
is treated as a hook error and the tool call proceeds ungoverned and silently, so
a missing or too-old Boundary would be invisible. Asking makes it visible. Remove
the hook from `settings.json` if you do not want to be asked.

## Environment knobs

| Variable | Default | Effect |
| --- | --- | --- |
| `BOUNDARY_BIN` | `boundary` | Path to the boundary binary. Read by the wrapper; everything else below is read by the binary. |
| `BOUNDARY_HOOK_FAILMODE` | `ask` | Posture on an internal **fault** (event unparseable, nothing to classify, classifier error): `ask` prompts the user, `open` allows so a flaky hook never bricks a session, `closed` blocks. A Boundary `deny` always blocks regardless of this setting. |
| `BOUNDARY_HOOK_AGENT_ID` | `claude-code` | Advisory agent-id label written to the decision record. Nothing authenticates it. |
| `BOUNDARY_HOOK_DIR` | `.boundary/hook` | Directory decision records are written to. |
| `BOUNDARY_HOOK_DEBUG` | _(unset)_ | When non-empty, prints diagnostic lines to stderr from both the wrapper and the binary. |

## Decision contract

The JSON on stdout — not the exit code — is what tells Claude Code what to do.
The hook exits 0 on every decided path.

| Boundary verdict | `permissionDecision` | stdout |
| --- | --- | --- |
| `deny` | `deny` | The dual-shape object: the legacy `{"decision":"block","reason":…}` keys **and** `hookSpecificOutput`, so older and newer clients both block. |
| `require_approval` | `ask` | `hookSpecificOutput` only. |
| `warn` | `ask` | `hookSpecificOutput` only; the warning is the reason. |
| `allow` | — | Nothing at all. Silent happy path; the host's own permission flow runs as usual. |

The hook **never** emits `permissionDecision: "allow"`. In Claude Code's
vocabulary that value does not mean "Boundary has no objection" — it means *skip
the permission system*, a grant the host would not otherwise have made. A gate
that granted the classes it rates riskiest while staying silent on the safest
class would be strictly more permissive than no gate at all, so `warn` asks.

Every decided event — allow, warn, ask, and deny alike — is written to a
hash-verifiable decision record under `BOUNDARY_HOOK_DIR` **before** the decision
reaches stdout. Check one with `boundary verify-record`. A tool this hook does
not route (`Read`, `Grep`, an MCP tool) is allowed silently and leaves **no**
record, because nothing was decided.

## Scope (read this)

- This hook governs the tool calls wired in `settings.json`. That route is the
  boundary; an un-wired tool, an MCP tool, or a subprocess Claude spawns that
  runs another command is a **bypass** and is not governed.
- A compound Bash line is decomposed into segments and governed by its most
  restrictive one, so `safe && dangerous` denies. An output redirection
  contributes the file it writes, so `cat x > important.db` is a write, not a
  read; and a command in argument position (`find -exec`, `xargs`) is decomposed
  too, so `find . -exec rm -rf {} +` denies. The decomposer is not a shell:
  heredocs, process substitution, `eval`, unbalanced quotes, and deep nesting are
  not modelled. Those lines come back undecomposable and escalate to `ask` — they
  are never allowed, but no class claim is made about them either. Nor is the
  argument-position set exhaustive: `watch`, `parallel`, `make`, and an
  interpreter's inline program leave their payload unclassified.
- The edit route classifies by **path shape**. It denies secret-bearing paths
  (class `E4`) and outside-the-envelope paths (class `E7`). It does not
  synthesize the content hunk, so content-only edit classes are not asserted by
  this hook.
- Claude Code's edit tools pass **absolute** paths, so the edit route resolves
  the target against the project root (the hook process's own working directory,
  never the event's `cwd`): a target inside the project is classified by its
  position in it, and a target outside — `/etc/passwd`, `~/.zshrc`, the
  `boundary` binary itself — stays absolute and denies as `E7`. Both paths are
  canonicalized as far as the filesystem resolves them, so a symlink out of the
  project is judged where the write actually lands; that is point-in-time, and a
  symlink swapped after the check is not caught.
- Writes to the governance control surfaces — `.claude/settings.json`,
  `.claude/settings.local.json`, `.claude/hooks/**`,
  `**/claude-code/pretooluse-boundary.sh`, and `.boundary/**` — are denied on
  **both** routes: as `E7` on the edit route, and by the same path shapes on the
  command route, so `cp evil .claude/settings.json` and
  `cat evil > .claude/settings.json` are refused too. An agent cannot edit the
  gate its own next tool call passes through with either tool. Only command
  segments that **write** are checked, so `cat .claude/settings.json` and
  `ls .boundary/hook` keep the verdicts their own classes earn; within a checked
  segment the argument position is not analyzed, so a control surface named as
  the source of a write (`cp .claude/settings.json backup.json`) is refused too.
  That list is a set of path shapes, not an inventory of every way governance
  could be disabled.
- Boundary decision records are **hash-verifiable** (integrity), not proof of
  authenticity and not "tamper-proof". Nothing here proves a tool call is safe;
  it records and gates a routed pre-execution verdict.
