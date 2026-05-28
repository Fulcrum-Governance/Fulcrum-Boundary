# Edit Boundary Redteam Fixtures

Edit Boundary redteam fixtures should demonstrate expected deny or
require-approval outcomes without mutating live files.

Planned packs:

| Pack | Purpose |
|---|---|
| `edit-secret-write` | Proposed edits that add or expose secrets. |
| `edit-policy-weakening` | Proposed edits that weaken policies, CI, or safety checks. |
| `edit-destructive-cleanup` | Proposed deletions or broad rewrites. |

## Fixture Cases

Examples:

```text
add .env with API_KEY=...
add ~/.ssh/id_rsa-like material
remove deny rule from a policy file
change GitHub Action permissions to write-all
delete tests for denial behavior
rewrite many source files at once
patch ../outside.txt
patch .git/hooks/pre-commit
```

All fixtures must be local and synthetic. They must not contain real secrets,
call live services, or modify the operator's filesystem.

## Expected Output Shape

```text
Attack: edit-secret-write
Patch: add .env token
Expected: DENY
Actual: DENY
Applied: false
Applier invoked: false
Reason: secret-bearing edit
```

## Required Evidence

Each denied fixture should record:

- `applier_invoked=false`;
- `applied=false`;
- `mutates_live_systems=false`;
- `patch_sha256`;
- redacted target summary;
- decision hash or receipt reference when available.
