# Edit Inspect

`boundary edit inspect` classifies proposed file mutations without applying
them.

```bash
boundary edit inspect --patch proposed.diff
boundary edit inspect --patch proposed.diff --json
boundary edit inspect --from-git-diff
boundary edit inspect --stdin
```

Inspect mode never applies edits, invokes a patch applier, or shells out.
Secret-looking paths and content are redacted from output.

Canonical repository docs:
[docs/edit-boundary/INSPECT.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/INSPECT.md)
