# Release Truth

## 2026-05-28 - Edit Boundary Preview Reconciliation

Commit audited: `bdc887ba5708e115f145efb6897ac107108637a4`

This reconciliation locks the public truth after the Edit Boundary inspect,
apply-wrapper, fixture redteam, and demo/docs surfaces landed.

## Status

| Surface | Status | Truth |
| --- | --- | --- |
| MCP Firewall | Production | Unchanged. Production MCP protection still requires routed deployment isolation around the upstream tool server. |
| Secure GitHub | Preview | Unchanged. Fixture and opt-in live conformance harnesses remain preview; production still requires deployment bypass proof. |
| Command Boundary | Preview | Unchanged. It governs command paths only when routed through `boundary command run`, `boundary shell`, or project-local shims. |
| Edit Boundary | Preview | Delivered preview for proposed file mutations routed through Boundary edit envelopes. |

## Test Commands

| Command | Result |
| --- | --- |
| `make release-check` | Pass |
| `go test ./internal/editboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/editboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Edit -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `go test ./... -count=1 -timeout 5m` | Pass |

## What Edit Boundary Proves

- `boundary edit inspect` classifies proposed patch bytes without applying
  them.
- `boundary edit apply` evaluates routed edit envelopes through the shared
  governance pipeline before writing files.
- Denied edit envelopes do not apply.
- Approval-required edit envelopes do not apply without local preview approval.
- Dry-run edit apply records the decision and does not invoke the applier.
- Edit decision records include patch hash, class, action, file list, and
  applied/applier state.
- Fixture Edit Boundary redteam packs exercise selected secret-bearing,
  destructive, package-script, CI/deploy, and cross-scope mutation paths without
  live project mutation.

## What Edit Boundary Does Not Prove

- Direct editor-write protection.
- Direct filesystem-write protection.
- Shell redirection control.
- Direct `git apply` control.
- IDE API control.
- Arbitrary filesystem sandboxing.
- Universal prevention of unsafe file edits.
- Production edit governance.

## Bypass Statements

Edit Boundary governs proposed file mutations only when the mutation routes
through a Boundary edit envelope. Direct editor writes, direct filesystem
writes, direct `git apply`, shell redirection, IDE saves, CI jobs, and arbitrary
processes are bypasses unless explicitly routed through Boundary.

Command Boundary remains separate. Direct shell execution and commands that do
not route through Boundary command wrappers or project-local shims remain
bypasses.

## Approved Copy

Boundary provides preview Edit Boundary governance for proposed file mutations
routed through Boundary edit envelopes.

Boundary can classify and gate proposed file mutations before they are applied
when the edit routes through Boundary.

## Forbidden Copy

- Boundary controls all file writes.
- Boundary protects direct editor writes.
- Boundary prevents every unsafe edit.
- Boundary provides filesystem sandboxing.
- Boundary provides universal coding-agent file safety.
- Boundary governs direct file edits outside routed edit envelopes.
- Boundary provides production edit governance.
