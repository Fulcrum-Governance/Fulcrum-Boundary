# Edit Apply

`boundary edit apply` evaluates a proposed file mutation before writing it.

```bash
boundary edit apply --patch proposed.diff
boundary edit apply --patch proposed.diff --dry-run
boundary edit apply --patch proposed.diff --require-approval
```

The preview apply path reads one patch source, classifies the exact bytes,
evaluates the shared governance pipeline, refuses deny decisions and missing
approval, and emits a JSONL edit decision record. It does not invoke `sh -c`,
`bash -c`, `zsh -c`, or `git apply`.

Canonical repository docs:
[docs/edit-boundary/APPLY.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/APPLY.md)
