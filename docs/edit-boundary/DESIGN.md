# Edit Boundary Design

## Purpose

Edit Boundary defines a preview governance model for proposed file mutations
that route through Boundary.

The design brings the existing Boundary shape to file edits:

1. Receive an edit envelope.
2. Parse the proposed mutation without applying it.
3. Classify edit risk.
4. Evaluate policy before mutation.
5. Deny, require approval, warn, or allow.
6. Apply only when policy and approval rules permit.
7. Emit an edit decision record.

This design is for the v0.6 release train. It is not part of the v0.5 public
release claim, and it must not be described as delivered behavior until
implementation and reconciliation tests pass.

## Product Sentence

Use this sentence:

> Boundary can classify and gate proposed file mutations before they are
> applied.

Use this caveat whenever the sentence appears in public copy:

> Edit Boundary applies only to file mutations routed through a Boundary edit
> envelope.

Do not use these sentences:

- Boundary controls all file writes.
- Boundary protects direct editor writes.
- Boundary prevents every unsafe edit.
- Boundary provides filesystem sandboxing.
- Boundary provides universal coding-agent file safety.
- Boundary governs direct file edits outside routed edit envelopes.

## Non-Negotiable Boundaries

Edit Boundary must not:

- claim protection for direct editor saves;
- claim protection for direct shell redirection, direct `cp`, direct `mv`, or
  direct `git apply` outside Boundary;
- claim arbitrary filesystem interception;
- claim IDE control unless an IDE integration routes edits through Boundary;
- claim filesystem sandboxing unless a real sandbox boundary is named, tested,
  and documented;
- mutate files in inspect mode;
- invoke a shell to apply patches;
- weaken MCP, Secure GitHub, Command Boundary, receipt, or claims gates.

## Routed Edit Envelope

The initial envelope is a unified diff plus project metadata:

```json
{
  "schema_version": "boundary.edit_envelope.v1",
  "project_root": "/path/to/repo",
  "actor": "agent-or-user",
  "source": "boundary edit apply",
  "patch_sha256": "sha256:...",
  "diff_format": "unified",
  "patch": "..."
}
```

The patch bytes classified by Boundary must be exactly the patch bytes evaluated
and applied. The implementation must compute a `patch_sha256` before policy
evaluation and bind any approval artifact to that hash.

## Inspect Flow

`boundary edit inspect` is read-only:

```text
patch bytes
  -> parse unified diff
  -> canonicalize target paths
  -> reject unsafe or unsupported patch forms
  -> redact secret-looking content for output
  -> classify edit risk
  -> emit text or JSON classification
```

Inspect mode must never invoke an applier and must never modify the worktree,
index, or files outside the project.

## Apply Flow

`boundary edit apply` is the governed mutation wrapper:

```text
patch bytes
  -> parse and classify
  -> build governance request
  -> evaluate shared governance pipeline
  -> deny, require approval, warn, or allow
  -> verify exact patch bytes and target list
  -> apply only when permitted
  -> emit edit decision record
```

The apply wrapper must use the same governance pipeline shape as other Boundary
surfaces. It must not create a second decision pipeline.

## Dry-Run Rule

Dry-run is a hard no-mutation mode. It must never invoke the applier, regardless
of the returned governance decision.

This is stricter than generic governance dry-run semantics because edit dry-run
is a filesystem safety guarantee for the wrapper itself. Even if a policy layer
returns a caller-visible allow in dry-run, Edit Boundary must record
`dry_run=true`, `applier_invoked=false`, and `applied=false`.

## Path Safety

Every target path must be canonicalized relative to the configured project root
before policy evaluation and again before apply.

Reject these forms:

- absolute paths;
- `..` traversal;
- Windows drive paths;
- UNC paths;
- backslash traversal;
- NUL or control characters;
- `.git/` mutations;
- symlink escapes outside the project root;
- paths that normalize outside the project root;
- paths that rely on ambiguous nested repo or submodule behavior.

Unsafe path forms classify as E7 outside project scope and must deny without
mutation.

## Patch Parser Strictness

The first implementation should accept a conservative subset of unified diffs.
Unsupported forms must fail closed or classify as require-approval or deny.

Reject or explicitly classify:

- malformed hunks;
- duplicate file headers;
- copy or rename edge cases;
- mode changes;
- binary patches;
- submodule or gitlink changes;
- combined diffs;
- incorrect `/dev/null` use;
- multiple entries for the same path;
- huge patches beyond configured limits.

## Approval Rule

The v0.6 preview CLI exposes `--require-approval` as a local operator
acknowledgement for routed patch application. It is recorded as
`approval_mode=local_flag` and must not be described as a production approval
artifact.

A future production approval artifact must be bound to:

- patch hash;
- target file list;
- project root;
- highest edit class;
- actor;
- decision id;
- expiry time.

Approval must never override hard denies for E4, E5, or E7 classes. If the patch
changes after approval, the apply wrapper must deny and leave the worktree and
index unchanged.

## Git Apply Backend

If the implementation uses Git to apply patches, it must:

- run `git apply --check` immediately before applying;
- invoke Git with explicit argv, never through a shell;
- never pass `--unsafe-paths`;
- feed the exact classified patch bytes through stdin or a controlled temp file;
- pin `cwd` to the canonical project root;
- reject patch filenames that could be interpreted as command options;
- verify the touched files match the parser target list.

Whether the index is mutated must be explicit. If `git apply --index` is used,
the decision record and docs must state that both worktree and index may change.
If staging is not in scope, apply to the worktree only and record
`index_changed=false`.

## Preview Policy Shape

The default preview policy should be conservative:

| Class | Default action |
|---|---|
| E0 metadata/no-op | allow |
| E1 safe content edit | allow |
| E2 source/config mutation | require approval |
| E3 deployment/infrastructure mutation | require approval |
| E4 secret-bearing edit | deny |
| E5 destructive deletion or broad rewrite | deny |
| E6 execution behavior mutation | require approval |
| E7 outside project scope | deny |

Implementation branches may tune this policy, but tuning must preserve the
claim boundary: Edit Boundary governs only routed edit envelopes.

## Decision Records

Edit decision records must avoid raw patch content and secret values. Store
hashes, classes, redacted path summaries, and mutation evidence.

Planned record shape:

```json
{
  "record_type": "edit_decision",
  "schema_version": "boundary.edit_decision.v1",
  "patch_sha256": "sha256:...",
  "project_root": "/path/to/repo",
  "targets_redacted": ["README.md"],
  "highest_class": "E2",
  "action": "require_approval",
  "dry_run": false,
  "applier_invoked": false,
  "applied": false,
  "approval_id": "",
  "index_changed": false,
  "unexpected_files_touched": false,
  "reason": "source or config changes require approval"
}
```

Records for denied, approval-required, and dry-run edits must prove no mutation:
`applier_invoked=false`, `applied=false`, and no observed worktree or index
change.
