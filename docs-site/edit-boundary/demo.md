# Edit Boundary Demo

Classify without applying:

```bash
boundary edit inspect --patch fixtures/editboundary/docs.diff
```

Run an apply decision without writing files:

```bash
boundary edit apply --patch fixtures/editboundary/docs.diff --dry-run
```

Deny a fixture edit-risk path without mutation:

```bash
boundary redteam --pack edit-secret-exfil
```

## What It Proves

- Routed patch envelopes can be classified before application.
- Dry-run apply evaluates policy and emits a decision record without invoking
  the applier.
- Denied edit redteam fixtures report `applied=false`.

## What It Does Not Prove

- Direct editor-write protection.
- Direct filesystem-write protection.
- Shell redirection or direct `git apply` control.
- IDE API control.
- Arbitrary filesystem sandboxing.
- Universal prevention of unsafe file edits.

Canonical repository demo:
[docs/edit-boundary/DEMO.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/DEMO.md)
