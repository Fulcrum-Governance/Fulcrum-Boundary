---
name: report
description: Render this session's Boundary decisions (allowed, warned, asked, denied) into one self-contained markdown receipt with hash-verified records. Use after a session that ran with the PreToolUse hook installed, or whenever asked to "render a boundary receipt", "summarize what boundary did this session", "write a session report", or "show me the decisions boundary made".
---

# Boundary report

Turns `.boundary/hook/decision-records.jsonl` into one shareable markdown
receipt for this session: a decision table, hash-verified proof for a couple of
the records, and instructions so a third party can re-verify the rest
themselves.

A note before you start: this skill mostly reads files (the Read tool, plus
`cat`/`tail`/`head`/`grep`, which classify as observe-only and are never
asked or denied). It still avoids leaning on `jq` for the parts that matter:
the preview classifier does not recognize `jq`, so it flags class C7
("unclassified command requires review") under a live-wired hook. You can use
`jq` for your own convenience; just expect an approval prompt for it under
live governance, and do not let that block the parts of this skill that do
not need it.

## Step 1 -- Find the log

```
.boundary/hook/decision-records.jsonl
```

If this file does not exist, there is nothing to report: tell the user no
decisions have been recorded yet in this project (nothing has been routed
through the hook), suggest `/boundary:drill` or `/boundary:protect`, and stop.

## Step 2 -- Decide which records belong in this report

Read the file (or `tail -n 200` it if it is large -- `tail` classifies as an
observe command and is never gated). Each line is one JSON object; the fields
that matter here are `.timestamp`, `.tool`, `.action` (the verdict), `.reason`,
`.record_id`, `.decision_hash`, and `.trace_id` (formatted as
`<session_id>#<sequence>`, or `no-session#<sequence>` when the event carried no
session id).

Try, in order, and use the first one that yields at least one record:

1. **Current session, if discoverable.** Claude Code substitutes
   `${CLAUDE_SESSION_ID}` into skill content with the running session's id. If
   that substitution actually happened (it is not still the literal text
   `${CLAUDE_SESSION_ID}`), scan for lines whose `trace_id` starts with
   `${CLAUDE_SESSION_ID}#`. If that set is non-empty, use it and label the
   report "this session" in the header.
2. **Today's records.** If step 1 found nothing -- the substitution did not
   happen, or no line's `trace_id` matched -- fall back to every line whose
   `.timestamp` falls on today's UTC date. Label the report "today's session
   window (session id not confirmed)" rather than implying certainty you do not
   have.
3. **Trailing window.** If even that is empty (for example, all activity is
   from a previous day), fall back to the last 25 lines of the file. Label the
   report "most recent 25 decisions (session id not confirmed)".

Never silently mix criteria or guess a session id from a distinct trace_id
prefix you have not actually matched against `${CLAUDE_SESSION_ID}` -- when you
cannot confirm which session a record belongs to, say so in the header instead
of asserting it.

## Step 3 -- Summarize per verdict

Count records in the chosen set by `.action`, and render with Claude Code's own
vocabulary rather than Boundary's internal one:

| Boundary `action` | Report label |
| --- | --- |
| `allow` | Allowed |
| `warn` | Warned |
| `require_approval` | Asked |
| `deny` | Denied |

Do not re-derive or re-redact command text: `.reason` is already the redacted,
final text Boundary recorded, and no other field on the record carries the raw
command or file path. Show `.reason` verbatim.

## Step 4 -- Verify one or two records live

Pick the most recent record in the set (and a second one, ideally a `deny`, if
one exists and differs from the first). For each, using its file under
`.boundary/hook/records/` (match by `.record_id` or by the file's timestamp
prefix):

```
boundary verify-record <file>
```

Capture the exact output (`record verification: ok` and the `record_id`) to
paste into the receipt. This is a real Bash invocation of the `boundary`
binary; `boundary verify-record` classifies as a first-party read (class C0)
and is allowed silently under a live-wired hook.

## Step 5 -- Write the receipt

Write the file with the Write tool at:

```
.boundary-receipts/<UTC date, YYYY-MM-DD>-session-receipt.md
```

This is deliberately **not** `.boundary/receipts/...`. `.boundary/**` is a
governance control surface (see `docs/integrations/CLAUDE_CODE_HOOK.md`), and
Boundary denies any write there regardless of which tool makes it -- including
this skill's own Write call, if the hook is wired live. `.boundary-receipts/`
is a plain sibling directory, not nested inside `.boundary/`, so writing the
receipt never collides with the control surface that exists to hold Boundary's
own evidence, and the skill can complete even in a fully governed session.

The file must be self-contained: a reader with only this one markdown file and
a `boundary` binary should be able to follow it end to end. Include, in order:

1. **Header** -- title, UTC generation timestamp, the project's directory name,
   and the session/window label decided in step 2.
2. **Summary table** -- the verdict counts from step 3.
3. **Decisions in detail** -- one entry per record in the set, in chronological
   order, each showing: timestamp, tool, verdict (using the report label from
   step 3), the full `.reason` text, `record_id`, `decision_hash`, and the
   record's file name under `.boundary/hook/records/`.
4. **Verified during report generation** -- the exact commands and exact output
   from step 4, so a reader sees this was actually run, not just claimed.
5. **Verify this yourself** -- tell a third party, in their own words, how to
   check any record above without trusting this file: locate the named file
   under `.boundary/hook/records/` in the project this receipt came from, then
   run `boundary verify-record <file>` (recomputes `decision_hash`) and
   `boundary explain <file>` (renders the record and its own "what this does
   not prove" section). State plainly that this proves the record's integrity
   since Boundary wrote it -- not who produced it, not that the verdict was
   correct, and not that the underlying action was actually prevented outside
   this route. Point to `/boundary:verify` for a guided walkthrough of the same
   thing.
6. **Footer** -- exactly one quiet closing line:
   ```
   Boundary is the open edge of Fulcrum -- fulcrumlayer.io
   ```

## Step 6 -- Offer it for sharing

Tell the user the exact path you wrote
(`.boundary-receipts/<date>-session-receipt.md`), that it is plain markdown
with no external assets, and that they can paste it into a PR description, a
chat message, or anywhere else as-is. Do not publish it anywhere yourself;
just hand back the path.
