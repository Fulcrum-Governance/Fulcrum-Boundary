---
name: verify
description: Verify a Fulcrum Boundary decision record or session receipt someone shared with you, and explain plainly what that verification does and does not prove. Use when given a Boundary record .json file, a .boundary-receipts/*.md receipt, or a decision_hash / record_id, and asked to "verify this record", "check this receipt", "is this boundary decision real", or "can I trust this deny".
---

# Boundary verify

Checks a decision record's integrity and states, without overclaiming, what
that check does and does not establish. This skill is for the recipient of a
record or receipt someone else produced -- if you produced it yourself this
session, `/boundary:drill` and `/boundary:report` already cover that.

## Step 1 -- Work out what you actually have

- **A single-record `.json` file** (for example something under
  `.boundary/hook/records/`). This is what `boundary verify-record` and
  `boundary explain` consume directly. Go to step 2.
- **A `.boundary-receipts/*.md` receipt** (from `/boundary:report`). The
  receipt names record files and prints their `decision_hash` and `record_id`,
  but it does not embed the raw record bytes. Ask for, or locate, the actual
  `.json` file(s) under `.boundary/hook/records/` in the project the receipt
  came from -- you cannot recompute a hash from a rendering of it. If you only
  have the markdown and cannot get the underlying record file, say so plainly:
  you can read the receipt's claims, but you cannot independently verify them
  without the record file.
- **A bare `decision_hash` or `record_id` with no file.** There is nothing to
  recompute a hash over. Ask for the record file.
- **A multi-record `.jsonl` log** (for example
  `.boundary/hook/decision-records.jsonl` itself). `boundary verify-record`
  rejects this -- it takes one single-record `.json` object, not a log.
  Extract the one line you care about and treat it as its own record, or point
  at the matching file under `.boundary/hook/records/` instead.

## Step 2 -- Run the two checks and show both outputs

```
boundary verify-record <record.json>
boundary explain <record.json>
```

Show the exact output of both, not a paraphrase. `verify-record` prints
`record verification: ok` and the `record_id` on success, or a specific failure
message and a non-zero exit on a mismatch. `explain` renders the record's
fields, describes its hashes, and prints its own "What this does not prove"
section -- include that section in what you show the user; it says the same
thing this skill says, in the tool's own words.

If the person only wants a machine-readable result, `boundary verify-record
--json <record.json>` emits a versioned `boundary.verify_record.v1` object with
`ok` and (on failure) `error` fields.

If someone also hands you the original request JSON or the policy directory
that produced the decision, add `--request <request.json>` and/or `--policies
<dir>` to also recompute `request_hash` and `policy_bundle_hash` -- without
them, verification still runs, it just checks less.

Note: this runs the `boundary` binary as a real Bash command. `boundary
verify-record` and `boundary explain` classify as first-party reads (class
C0) and are allowed silently under a live-wired hook; an unrecognized
`boundary` verb, or a recognized one with unexpected flags, still classifies
C7 and prompts like any unknown command.

## Step 3 -- State plainly what a passing verification proves

- **Integrity of this exact file, from the moment Boundary wrote it.**
  `verify-record` recomputes `decision_hash` over the record's decision-defining
  fields (and, when supplied, `request_hash` and `policy_bundle_hash`) and
  compares it to the value stored in the file. If they match, nothing in that
  covered data has changed since Boundary produced it.
- **Nothing else.** In particular it does not prove:
  - **Authenticity.** A hash match does not say who produced the file, or that
    it really came from the project it claims to. `--verify-signature
    --public-key <key>` additionally checks an optional Ed25519 signature over
    `decision_hash`, which proves the record was signed by whoever holds that
    key -- it still does not prove the verdict was correct, that the action
    was executed or prevented, or solve where that key came from and whether
    it should be trusted. Unsigned records (the common case) skip this
    entirely; do not read "verification: ok" on an unsigned record as an
    authenticity claim it never made.
  - **That the verdict was correct.** The record states what Boundary decided
    and why; it is not evidence the classification itself was right.
  - **That the action was actually blocked or run.** `execution_claim`
    (`upstream_called`, `executed`) on a hook record is a self-report by the
    hook about itself, not corroborated by anything outside the record. A deny
    record is not proof the command never ran somewhere else -- direct access
    to the same tool, outside the routed hook, is a bypass a record cannot see.
  - **Total coverage.** Boundary governs only routed tool calls. A record's
    existence says a decision was made on that route; it says nothing about
    activity that never reached it.

## Step 4 -- If verification fails

State the exact error `boundary verify-record` returned. The two ordinary
causes are: the file's covered fields were altered after Boundary wrote it (a
real integrity failure -- treat the record as untrustworthy), or the wrong kind
of file was given (a multi-record `.jsonl` line pasted without its object
braces, a truncated copy-paste, a receipt's rendered text mistaken for the
record itself). Do not guess which one it is; say what the error says and let
the user decide how to get the original file.
