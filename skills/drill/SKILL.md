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

Step 6's cleanup is the one exception, and it is deliberate: `rm -rf
.boundary-drill` IS a real Bash tool call, issued against this drill's own
scratch directory and nothing else. It is not part of the proof -- the proof
already happened in steps 3 to 5 -- so it is allowed to be governed like any
other call. Under a live-wired hook it will itself be asked or denied, which is
the correct outcome; step 6 says to leave the inert fixture behind rather than
work around a Boundary verdict.

One more thing worth knowing going in: Command Boundary's preview classifier
does not yet special-case the `boundary` binary itself, so `boundary
verify-record`, `boundary explain`, and `boundary hook pretooluse` each
classify as class C7 ("unclassified command requires review") when run as a
Bash tool call. Under a live-wired hook this means every `boundary` command in
this drill -- not just the staged one -- may itself prompt for approval once.
That is Boundary treating its own CLI like any other unrecognized command, not
a malfunction; approve each prompt and continue.

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
     - `"ok"` -- this project's settings wire the hook; the `detail` names the
       scope and which tools.
     - `"unknown"` -- no settings file wires it, but a Claude Code **plugin
       manifest** the doctor found does. A plugin registers its hooks only
       when Claude Code has that plugin enabled, which cannot be read from
       the file, so this is genuinely undetermined. Do not report it as "not
       wired": the `detail` names the manifest, and this drill is exactly the
       thing that settles it in practice.
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
printf '{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status && rm -rf .boundary-drill/vault"}}' | boundary hook pretooluse
```

Expect a JSON object containing `"decision":"block"` and
`"permissionDecision":"deny"`, with a reason naming class C4 and noting it
picked the most restrictive of two compound segments. Point out to the user:
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
was even printed:

```
ls -t .boundary/hook/records/*.json | head -1
```

## Step 5 -- Verify and explain it, and show both outputs to the user

Using the file path from step 4:

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
rm -rf .boundary-drill
```

This delete is itself class C4. If this project's hook is genuinely wired
live, Boundary may ask or deny this too -- exactly as it should, since the
same rule that just protected the user also applies to this drill's own
cleanup. If it does not complete immediately, do not retry, force, or work
around it: leave the tiny fixture directory behind (it is inert, two small
files under `.boundary-drill/`) and say so in the closing summary instead.

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
  - `"ok"` -- this project's settings wire the hook. Name the scope.
  - `"unknown"` -- a plugin manifest declares it and no settings file does, so
    whether it is live depends on that plugin being enabled. Say that, name
    the manifest, and offer `/boundary:protect` as the way to make it a
    project-level floor rather than a personal one.
  - `"broken"` -- nothing it could read wires the hook. Say so and point to
    `/boundary:protect`.
  In every case: this drill proved the decision engine works. It did not prove
  this project's real tool calls are currently intercepted.
- Mention that `/boundary:report` renders every decision from this session (or
  the current window, if the session cannot be identified) into one shareable
  markdown receipt.
