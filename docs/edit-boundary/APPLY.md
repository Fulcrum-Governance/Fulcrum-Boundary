# Edit Boundary Apply

`boundary edit apply` evaluates a proposed file mutation before writing it.
It is a preview surface for routed edit envelopes, not a filesystem sandbox and
not a global editor/IDE control plane.

```bash
boundary edit apply --patch proposed.diff
boundary edit apply --patch proposed.diff --dry-run
boundary edit apply --patch proposed.diff --require-approval
```

The apply path:

1. reads exactly one patch source,
2. classifies the patch with `boundary edit inspect`,
3. evaluates the shared governance pipeline,
4. refuses deny decisions and missing approval,
5. applies through Boundary's internal patch applier only when permitted,
6. emits a JSONL edit decision record.

No shell is invoked. `sh -c`, `bash -c`, and `zsh -c` are not used by the
preview applier.

## Default Preview Policy

| Class | Default action |
| --- | --- |
| E0 metadata/no-op | allow |
| E1 safe content edit | allow |
| E2 source/config mutation | require_approval |
| E3 deployment/infrastructure mutation | require_approval |
| E4 secret-bearing edit | deny |
| E5 destructive edit | deny |
| E6 execution behavior mutation | require_approval |
| E7 outside project scope | deny |

`--require-approval` is a preview local operator acknowledgement for
`require_approval` classes. It records `approval_mode=local_flag`. It is not a
production approval artifact, and it does not override hard-deny classes.

## Dry Run

`--dry-run` records the governance decision and never invokes the applier. This
is true even when the decision would otherwise allow the edit.

## Decision Records

Records are written to `.boundary/edit/decision-records.jsonl` by default:

```json
{
  "record_type": "edit_decision",
  "schema_version": "boundary.edit_decision.v1",
  "patch_sha256": "sha256:...",
  "files": ["package.json"],
  "class": "E2",
  "action": "require_approval",
  "approval_present": false,
  "dry_run": false,
  "applier_invoked": false,
  "applied": false,
  "index_changed": false
}
```

Secret-looking paths are redacted. Patch hashes are recorded so the decision is
bound to the exact bytes inspected by Boundary. The preview internal applier
changes the worktree only; it does not mutate the Git index.
