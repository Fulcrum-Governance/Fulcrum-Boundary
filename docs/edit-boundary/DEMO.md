# Edit Boundary Demo

Edit Boundary is a preview for proposed file mutations routed through a
Boundary edit envelope. It can inspect patch bytes, evaluate the shared
governance policy, deny or require local approval before applying, and run
fixture-only edit redteam packs without live mutation.

## Try It

Classify a proposed patch without applying it:

```bash
boundary edit inspect --patch fixtures/editboundary/docs.diff
```

Inspect the same patch as JSON:

```bash
boundary edit inspect --patch fixtures/editboundary/docs.diff --json
```

Run an apply decision without writing files:

```bash
boundary edit apply --patch fixtures/editboundary/docs.diff --dry-run
```

Run a fixture redteam pack that must not mutate the worktree:

```bash
boundary redteam --pack edit-secret-exfil
```

Expected redteam signal:

```text
attack: edit-secret-exfil
actual: DENY
applied: false
```

## What It Proves

- Routed patch envelopes can be classified before application.
- Dry-run apply evaluates policy and emits a decision record without invoking
  the applier.
- Denied edit redteam fixtures report `applied=false`.
- Patch decisions are bound to exact patch bytes through a patch hash.

## What It Does Not Prove

- Direct editor-write protection.
- Direct filesystem-write protection.
- Shell redirection control.
- Direct `git apply` control.
- IDE API control.
- Arbitrary filesystem sandboxing.
- Universal prevention of unsafe file edits.

Edit Boundary only governs proposed file mutations when they route through the
Boundary edit envelope.
