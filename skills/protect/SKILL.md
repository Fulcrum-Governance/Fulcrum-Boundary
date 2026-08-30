---
name: protect
description: Wire Fulcrum Boundary's PreToolUse hook into this project's committed Claude Code settings so every contributor inherits the same deny floor. Use when asked to "install boundary", "protect this project", "wire up the boundary hook", "turn on boundary", or "set up pre-execution governance", or right after cloning a project that ships this hook but has not enabled it yet.
---

# Boundary protect

Wires Fulcrum Boundary's `PreToolUse` hook into this project's Claude Code
settings and explains what that hook does and does not govern. It does not edit
`.claude/settings.json` itself -- read on for why, because that is the point,
not an omission.

## What this hook governs

| Claude Code tool | Routed to | Preview classifier |
| --- | --- | --- |
| `Bash` / `Shell` | Command Boundary | Classes C0-C7; C4 (destructive), C5 (infra mutation), and C6 (credential access) deny; C2/C3/C7 ask; C1 warns; C0 allows silently. |
| `Edit`, `Write`, `MultiEdit`, `NotebookEdit` | Edit Boundary | Classes E0-E7 by path shape; E4 (secret path) and E7 (outside project, or a governance control surface) deny. |
| Everything else (`Read`, `Grep`, `Glob`, web tools, MCP tools) | Not routed here | Allowed silently, no record. Govern MCP tools at the MCP route instead -- see `docs/GOVERN_MCP_SERVER.md`. |

Both routes deny writes to a fixed set of governance control surfaces, on
purpose: `.boundary/**`, `.claude/settings.json`, `.claude/settings.local.json`,
`.claude/hooks/**`, and the wrapper script itself
(`**/claude-code/pretooluse-boundary.sh`). An agent that could rewrite its own
hook wiring could turn the hook off before its next tool call, so this is
closed at the classifier, not left to policy.

This is a **delivered preview**, not a production sandbox: it governs only the
tool calls Claude Code actually routes through it. An MCP tool, a subprocess
that runs its own command, or direct shell use outside Claude Code is a bypass
and is not governed by this hook. See `README_AI.md` and
`docs/integrations/CLAUDE_CODE_HOOK.md` for the full honest scope.

## Step 1 -- Confirm the binary is ready

```
"${BOUNDARY_BIN:-boundary}" version
```

That spelling is the effective-binary contract this whole integration shares:
`BOUNDARY_BIN` when set, else `boundary` on `PATH` -- the same resolution the
hook wrapper execs and the installer preflight validates. Use it for every
Boundary command in this skill, and do not shorten it to bare `boundary`,
which re-resolves `PATH` and can reach a different (older) binary than the one
the hook runs. If it fails because no binary was found (and `BOUNDARY_BIN` is
not set), stop and point the user to `README_AI.md` to build or install
`boundary` first. If it fails because `BOUNDARY_BIN` is set but does not run,
report that exact failure instead -- a broken explicit override is the user's
to fix; do not fall back to `PATH` on their behalf.

## Step 2 -- Read what is already there

Read (do not write) both files if they exist:

```
.claude/settings.json
.claude/settings.local.json
```

Reading is fine -- it is not a governed write, and it is how you find out
whether either file already has other `hooks` entries (a `SessionEnd` hook, a
different `PreToolUse` matcher, unrelated settings) that a naive overwrite
would destroy. Also read the canonical wiring this repo ships:

```
integrations/claude-code/settings.snippet.json
```

## Step 3 -- Show the user exactly what to paste, and why you are not pasting it yourself

This skill will not call Write or Edit on `.claude/settings.json` or
`.claude/settings.local.json`, whether or not the hook is wired yet. Two
reasons, and be upfront about both with the user:

1. Once wired, Boundary itself refuses that write for any agent, this one
   included -- try it and you get back exactly this:
   > Fulcrum Boundary (Edit Boundary preview) denied this edit to
   > ".claude/settings.json" [E7]: agent permission settings path is outside
   > the edit envelope scope. This is a routed pre-execution deny; the file
   > was not written.
2. Before it is wired, nothing technical stops an agent from writing it -- but
   the first placement of your own guard rail should come from you, not from
   the agent those guard rails are meant to govern. This skill tells you what
   to type; it does not type it for you.

The snippet carries **two** blocks, and they do different jobs. Say which is
which rather than presenting them as one thing:

- `hooks.PreToolUse` -> `pretooluse-boundary.sh`. This is the governing hook:
  it decides `Bash` and the edit tools before they run. Everything in the table
  at the top of this skill comes from this block.
- `hooks.SessionEnd` -> `sessionend-boundary.sh`. This one gates **nothing**. It
  appends one line per ended session to `.boundary/hook/session-summaries.jsonl`
  counting what Boundary decided (denied / asked / warned / allowed), so
  `/boundary:report` and any later reader have a per-session index. Omitting it
  costs that log and changes nothing about what is governed.

Render the exact block to merge, based on what step 2 found:

- **Nothing there yet, or an empty object.** Tell the user to create
  `.claude/settings.json` with exactly the contents of
  `integrations/claude-code/settings.snippet.json` -- both blocks.
- **A file with unrelated content already.** Show the `hooks.PreToolUse` and
  `hooks.SessionEnd` array entries from the snippet, and tell the user precisely
  where to merge them: alongside any existing `hooks` entries (if a `SessionEnd`
  block is already there, add Boundary's entry to its `hooks` array rather than
  replacing the block, and do not drop another tool's `PreToolUse` matcher),
  preserving every other top-level key untouched.
- **A `PreToolUse` hook already present but not pointing at
  `pretooluse-boundary.sh`.** Say so plainly and let the user decide whether to
  add Boundary alongside it or replace it -- do not choose for them.

Also remind the user, if this is a fresh clone:

```
chmod +x integrations/claude-code/pretooluse-boundary.sh
chmod +x integrations/claude-code/sessionend-boundary.sh
```

And that `$CLAUDE_PROJECT_DIR` in the snippet resolves from the project root
automatically; if `boundary` is not on `PATH` in the environment Claude Code
runs in, they also need to export `BOUNDARY_BIN` there.

If this repository ships a Claude Code plugin manifest
(`.claude-plugin/plugin.json` at the repo root) and the user already installs
plugins through Claude Code's own plugin flow, mention that as a possible
alternative to hand-editing settings -- the plugin's `hooks/hooks.json`
registers the same two hooks, so a plugin install needs no settings edit at
all. Do not state exact plugin-install command syntax you have not verified
against the user's Claude Code version. The settings-snippet path above is
verified and always works, and it is the one that gives a **project** a floor:
a plugin install is personal to whoever installed it.

## Step 4 -- Confirm it landed, and that it works

After the user says they have pasted and saved the change, re-Read
`.claude/settings.json` (and `.claude/settings.local.json` if that is where
they put it) to confirm the `PreToolUse` block is present -- this is a Read,
not a governed write. Then either:

- Run `"${BOUNDARY_BIN:-boundary}" hook doctor` (`--json` for the structured
  form -- **this subcommand may not exist yet** on an older build). Look at
  the `.checks[]` entry named `"hook registration"`:
  - `"ok"` -- this project's settings now wire the hook; the `detail` names
    the scope and tools covered. This is what you are aiming for here.
  - `"unknown"` -- no settings file wires it, but a plugin manifest the doctor
    found declares it. That means the paste did **not** land where doctor
    reads, and what you are seeing is a pre-existing plugin install. Say so
    rather than calling it success: a plugin install is personal to whoever
    installed it, which is the opposite of the team-wide floor step 5 is for.
  - `"broken"` -- nothing it could read wires the hook: the paste did not
    take, or was saved somewhere doctor cannot read.

  It also separately reports `peer_hooks` -- other `PreToolUse` hooks it found
  -- worth a glance if Boundary's verdict ever seems to interact oddly with
  another tool's hook, and `plugin_manifests`, the plugin hook manifests it
  located, or
- Run `/boundary:drill` for the fuller, hands-on proof (denies a staged
  command, shows a verified record).

Either way, tell the user to reload before either check: restart Claude Code,
or run `/hooks`, so the new wiring actually takes effect for this session.

## Step 5 -- Commit it so contributors inherit the same floor

`.claude/settings.json` is the project-level file -- typically committed, so
everyone who clones the project gets this wiring automatically.
`.claude/settings.local.json` is typically personal and gitignored; wiring the
hook there only protects the one person who edited it. Recommend the
project-level file for a team-wide floor, and tell the user the commit to run
(do not run it yourself):

```
git add .claude/settings.json \
  integrations/claude-code/pretooluse-boundary.sh \
  integrations/claude-code/sessionend-boundary.sh
git commit -m "Wire Boundary's PreToolUse and SessionEnd hooks"
```

Drop the `sessionend-boundary.sh` path from that command if step 3 wired only
the `PreToolUse` block.

Only include the `chmod +x` bit in that commit if step 3 actually needed it (a
fresh clone where the script had lost its executable bit) -- most clones will
already have it.
