---
name: drill
description: Prove Fulcrum Boundary's Claude Code PreToolUse hook actually denies a destructive command before it runs, and produces a hash-verifiable record of that denial. Use right after wiring the hook (see /boundary:protect) as a first-run check, or whenever asked to "test the boundary hook", "run the boundary drill", "prove boundary is working", or "show me a denied command with a receipt". Completes in under two minutes and never executes a real destructive command.
---

# Boundary drill

Proves, in one short run, that Fulcrum Boundary's `PreToolUse` hook denies a
destructive command before it runs and leaves a hash-verifiable record of that
denial. This is the first-run activation moment: the user should walk away
having seen a real deny and a real record, not a description of one.

## Safety design -- read this before changing anything below

The **staged destructive command** -- the one this drill exists to get denied
(step 3) -- is never issued through a real Bash tool call. It only ever exists
as text inside a JSON payload piped to `boundary hook pretooluse`. Two
independent reasons:

1. `boundary hook pretooluse` is not a simulation of the hook -- it is the exact
   binary verb Claude Code's own hook wrapper execs into
   (`integrations/claude-code/pretooluse-boundary.sh`). Piping a synthetic
   PreToolUse event into it runs the identical classify-and-record code path a
   live tool call would. Submitting the command this way is a real exercise of
   the decision engine, not a stand-in for one.
2. Command Boundary's preview posture denies every `rm` / `rm -rf` call by
   default, on any path (class C4), and separately denies any write under
   `.boundary/**` regardless of command, because that tree is a governance
   control surface. If this project's hook is genuinely wired live, a real
   `rm -rf` issued by this drill -- even to clean up its own scratch directory --
   would itself be asked or denied. That is correct behavior, not a bug, but it
   means a drill whose success depends on a real destructive Bash call landing
   is not reliable. Submitting the event directly keeps the drill deterministic
   whether or not live wiring happens to be present.

Do not change step 3 to run the command through the real Bash tool instead of
piping it to `boundary hook pretooluse`.

Step 6's cleanup is a real Bash tool call, and it is deliberately NOT
`rm -rf .boundary-drill`: a raw recursive delete is class C4 and Boundary
correctly denies it -- the same rule that just protected the user applies to
this drill's own litter. Cleanup instead runs `boundary drill cleanup`, a
scoped first-party verb that removes exactly the fixture this drill wrote,
refuses symlinks and recursion, and lists and leaves anything it does not
recognize. Never "fix" a denied `rm -rf` by deleting outside the session --
that is the exact bypass pattern the receipt's limitations section warns
about.

One more thing worth knowing going in: Command Boundary's preview classifier
recognizes the exact first-party commands this drill uses -- `boundary
version`, `boundary hook doctor --json`, `boundary hook pretooluse
[--print-record]`, `boundary verify-record`, and `boundary explain` classify
as read-only (class C0) and are allowed silently, so the guided path
generates no Boundary approval prompts of its own. The recognition is
exact-form, not trust in the name: any other `boundary` verb, or a recognized
verb with an unexpected flag, still classifies C7 ("unclassified command
requires review") like any unknown command. The one deliberate exception is
step 6's `boundary drill cleanup`, which deletes the drill fixture and
therefore classifies C1 and surfaces one visible warn-grade confirmation -- a
deleting command stays visible, even Boundary's own.

## Step 1 -- Confirm the decision path is real

Do not skip this. If Boundary cannot actually decide anything, do not stage
anything -- say so and stop.

1. Run `boundary version`. If this fails (binary not found), stop: tell the
   user Boundary is not installed and point to `README_AI.md`.
2. Run `boundary hook doctor --json`. **This subcommand may not exist yet** on
   an older installed build -- treat that as informational, not a failure of
   Boundary itself, and fall back to step 1.3.
   - If it runs and prints a `schema_version: "boundary.hook.doctor.v1"`
     object, the decision path itself is confirmed functional -- you do not
     also need step 1.3. Find the entry in `.checks[]` whose `name` is
     `"hook registration"` and read its `state`, which is one of three:
     - `"ok"` -- either this project's settings wire the hook (the `detail`
       names the scope and tools), or a plugin manifest declares it AND the
       decision log holds fresh records carrying a Claude Code session id --
       routed calls are being decided here, which is the fact a manifest
       alone cannot establish; the `detail` names both sources. In this drill
       the second form is the expected state: step 1.1's `boundary version`
       was itself decided and recorded moments before doctor ran.
     - `"unknown"` -- no settings file wires it, a Claude Code **plugin
       manifest** the doctor found declares it, and no fresh session-bearing
       records yet show routed calls being decided. A plugin registers its
       hooks only when Claude Code has that plugin enabled, which cannot be
       read from the file, so this is genuinely undetermined. Do not report
       it as "not wired": the `detail` names the manifest, and this drill is
       exactly the thing that settles it in practice.
     - `"broken"` -- nothing the doctor could read registers the hook. That is
       still a statement about what it read, not proof nothing routes here;
       the `detail` says which files it searched.
     Keep the state and its `detail` verbatim for the closing message in step
     7. It is a direct answer about the wiring this command can read, which is
     more than a guess and less than a claim that Boundary is the only route.
   - Do not treat any hook-registration state, or an overall `status` of
     `"warn"` or `"unknown"` (other checks, like an empty decision log, are
     routine and do not block this drill), as a reason to stop. The only thing
     that stops this drill is the decision path itself not working -- see
     step 1.5.
   - If stderr contains `unknown hook subcommand`, this build predates `hook
     doctor` -- continue to step 1.3.
3. Fallback probe (only needed when step 1.2's subcommand was unavailable):
   - `boundary hook --help` should exit 0 and list `pretooluse` among its
     commands.
   - Then run the functional probe:
     ```
     printf '{"tool_name":"Bash","tool_input":{"command":"git status"}}' | boundary hook pretooluse
     ```
     A working hook exits 0 and prints **nothing** (a silent allow, since
     `git status` classifies as an observe-only command). Any error, or output
     complaining about an unknown command or subcommand, means the decision
     path does not work here.
4. Only needed when step 1.2's `hook doctor` was unavailable (its
   `"hook registration"` check already answers this precisely when present).
   Best-effort and informational only -- this does not gate the drill:
   ```
   grep -l "pretooluse-boundary.sh\|hook pretooluse" .claude/settings.json .claude/settings.local.json 2>/dev/null
   ```
   Remember whether this found a match. You will state the result plainly in
   step 7 rather than implying live enforcement you have not confirmed.
5. If neither step 1.2 nor step 1.3 confirmed a working decision path, stop
   here. Tell the user: "Boundary's hook verb is not functional on this
   machine yet, so this drill will not fabricate a result. Install it per
   README_AI.md, then re-run /boundary:drill." Do not proceed to step 2.

## Step 2 -- Create a disposable scratch fixture

Use the Write tool (not a shell `mkdir`) so this setup step never needs its own
approval: write `.boundary-drill/vault/fixture.txt` with the content
`drill fixture -- safe to delete, never targeted for real`. The Write tool
creates the intervening `.boundary-drill/vault/` directories itself.

`.boundary-drill/` is a sibling of the real `.boundary/` evidence tree, not
nested inside it. `.boundary/**` is itself a governance control surface, and
nesting a scratch directory inside it would make it impossible to clean up
later once the hook is live (see the safety note above). This fixture file is
never read or executed by anything; it exists only so there is a real, visible
thing to point at.

## Step 3 -- Stage the destructive command against the real decision path

Build the exact event Claude Code would send for a Bash tool call, chaining a
benign command in front of the destructive one to exercise compound-line
decomposition, and submit it to `boundary hook pretooluse` directly (never
through the real Bash tool -- see the safety note above):

```
printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf .boundary-drill/vault"}}' | boundary hook pretooluse --print-record
```

Expect TWO JSON lines. First the decision: an object containing
`"decision":"block"` and `"permissionDecision":"deny"`, with a reason naming
class C4 and noting it picked the most restrictive of two compound segments.
Then the record pointer: a `boundary.hook.record-pointer.v1` object whose
`record_id` and `record_path` name the exact record this submission wrote --
keep both for steps 4 and 5. Point out to the user:
`git status` alone is harmless (class C0); chaining `rm -rf` after it does not
launder the verdict, because the whole line is decomposed and the most
restrictive segment wins. Then confirm the fixture is untouched:

```
cat .boundary-drill/vault/fixture.txt
```

It should still print the fixture line. Nothing was ever actually deleted --
the "destructive command" only ever existed as text inside the JSON payload
above.

## Step 4 -- Locate the record this decision just wrote

Every decided event, including this deny, is persisted before the JSON above
was even printed, and step 3's record pointer already names it: use its
`record_path` exactly as returned. Do not select by newest file -- your own
next command mints a newer record the moment it is decided, so "newest" stops
meaning "the deny" the instant you look for it.

## Step 5 -- Verify and explain it, and show both outputs to the user

Using the `record_path` from step 3's pointer:

```
boundary verify-record <the file from step 4>
boundary explain <the file from step 4>
```

`verify-record` recomputes `decision_hash` and reports `record verification:
ok` plus the `record_id`. `explain` renders the record's fields, its own
hashes, and an explicit "What this does not prove" section -- show that
section too. It is the same honest-scope language this whole integration uses,
not boilerplate to skip past.

## Step 6 -- Clean up

```
boundary drill cleanup
```

This scoped first-party verb removes exactly the fixture this drill wrote --
`.boundary-drill/vault/fixture.txt` and the two directories that held it --
and nothing else: it refuses symlinks, never deletes recursively, and lists
and leaves anything it does not recognize. It classifies C1 (a deleting
command stays visible, even Boundary's own), so under a live-wired hook it
surfaces one warn-grade confirmation; approve it and the drill ends with no
residue. Do NOT use `rm -rf .boundary-drill` instead: that is class C4 and
Boundary denies it -- correctly. If cleanup reports content it does not own,
leave that content in place and say so in the closing summary.

## Step 7 -- Tell the user exactly what just happened

State this plainly, without hedging into vagueness:

- The command was denied before it ran. Boundary's decision path -- the same
  binary verb the live hook uses -- refused it, so nothing ever reached a
  shell.
- That denial produced a real record. `boundary verify-record` recomputed its
  hash and `boundary explain` rendered it -- this was a receipt, not a
  description of one.
- State plainly what step 1.2's `"hook registration"` check (or step 1.4's
  fallback grep, if doctor was unavailable) established, using its own words:
  - `"ok"` via settings -- this project's settings wire the hook. Name the
    scope.
  - `"ok"` via manifest plus routed evidence -- a plugin manifest declares
    the hook and fresh session-bearing decision records show routed calls
    being decided here. Name both sources, and keep the check's own caveat:
    records are integrity evidence, not authenticity.
  - `"unknown"` -- a plugin manifest declares it, no settings file does, and
    no fresh session-bearing records yet show routing. Say that, name the
    manifest, and offer `/boundary:protect` as the way to make it a
    project-level floor rather than a personal one.
  - `"broken"` -- nothing it could read wires the hook. Say so and point to
    `/boundary:protect`.
  In every case: this drill proved the decision engine works. It did not prove
  this project's real tool calls are currently intercepted.
- Mention that `/boundary:report` renders every decision from this session (or
  the current window, if the session cannot be identified) into one shareable
  markdown receipt.
