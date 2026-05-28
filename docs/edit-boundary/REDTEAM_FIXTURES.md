# Edit Boundary Redteam Fixtures

Edit Boundary redteam fixtures should demonstrate expected deny or
require-approval outcomes without mutating live files.

Implemented packs:

| Pack | Purpose |
|---|---|
| `edit-secret-exfil` | Proposed edits that add or expose secret-bearing values. |
| `edit-package-script-mutation` | Proposed edits that change package scripts or add script execution paths. |
| `edit-ci-deploy-mutation` | Proposed edits that change CI, Docker, or infrastructure deployment behavior. |
| `edit-destructive-delete` | Proposed deletions of unrelated project files. |
| `edit-cross-scope-mutation` | Proposed edits that target paths outside the project root. |

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
Attack: edit-secret-exfil
Patch: .env secret-bearing edit
Expected: DENY
Actual: DENY
Applied: false
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
