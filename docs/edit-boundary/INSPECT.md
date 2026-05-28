# Edit Inspect

`boundary edit inspect` classifies proposed file mutations without applying
them.

```bash
boundary edit inspect --patch proposed.diff
boundary edit inspect --patch proposed.diff --json
boundary edit inspect --from-git-diff
boundary edit inspect --stdin
```

Supported input forms:

- unified diff;
- `git diff` output;
- patch file path;
- stdin patch bytes.

Inspect mode never applies edits, never invokes `git apply`, and never shells
out to a patch applier. `--from-git-diff` reads the current Git diff so the
operator can classify local changes before applying or committing anything
elsewhere.

## Text Output

```text
Edit Boundary Inspection
Files touched: 3
Highest class: E6 execution behavior mutation
Risk: HIGH
Recommended action: require_approval
Findings:
- package.json modify: E6 execution behavior mutation (execution behavior mutation)
- src/app.ts modify: E2 source/config mutation (source or config mutation)
- [redacted-secret-path] add: E4 secret-bearing edit (secret-bearing path denied)
```

Secret-looking paths and content are redacted. The output should not contain raw
secret values or raw secret-bearing path names.

## JSON Output

```json
{
  "schema_version": "boundary.edit_inspection.v1",
  "files_touched": 1,
  "patch_sha256": "sha256:...",
  "highest_class": "E6",
  "risk": "HIGH",
  "recommended_action": "require_approval",
  "findings": [
    {
      "path": "package.json",
      "operation": "modify",
      "class": "E6",
      "risk": "HIGH",
      "recommended_action": "require_approval",
      "reason": "execution behavior mutation"
    }
  ]
}
```

## Claim Boundary

Inspect proves classification only. It does not prove that direct editor writes,
direct shell edits, direct `git apply`, CI jobs, or IDE saves are protected.
