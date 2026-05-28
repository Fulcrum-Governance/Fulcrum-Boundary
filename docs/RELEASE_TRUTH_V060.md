# Release Truth: v0.6.0 Edit Boundary Preview

Date: 2026-05-28

Audited base commit: `77474b22616f83aedf18fb6eea284349211aae8d`

Release packaging branch: `release/v060-package`

Release train: Filesystem/Edit Boundary Preview

## Summary

v0.6.0 packages the already-merged Filesystem/Edit Boundary preview into the
active public install and release surface. It does not change MCP production
status, Secure GitHub preview status, Command Boundary preview status, or the
fixture-only first-run demo path.

Final v0.6.0 truth:

- MCP remains the production adapter path.
- Secure GitHub remains preview.
- Command Boundary remains preview and routed-path-only.
- Edit Boundary is delivered preview for proposed file mutations routed through
  Boundary edit envelopes.
- Edit Boundary does not govern direct editor writes, direct filesystem writes,
  direct `git apply`, shell redirection, IDE saves, CI jobs, or arbitrary
  processes unless those mutations are explicitly routed through Boundary.
- Edit Boundary does not provide filesystem sandboxing.
- Production Edit Boundary status requires deployment evidence that edit
  proposals route through Boundary-controlled envelopes.
- Active public install and GitHub Action examples use `@v0.6.0` for
  repeatable behavior.

## Release Packaging

The v0.6.0 packaging pass moves the already-merged Edit Boundary preview into
the active public install and action examples:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.6.0
boundary selftest
boundary demo github-lethal-trifecta
```

GitHub Action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.6.0
```

The release package does not add new product behavior. It updates release
notes, truth docs, copy rules, and active public references so v0.6.0 means
Filesystem/Edit Boundary preview.

## Test Commands

| Command | Result |
|---|---|
| `make release-check` | pass |
| `make docs-build` | pass |
| `go test ./internal/editboundary/... -count=1 -timeout 5m` | pass |
| `go test ./tests/editboundary/... -count=1 -timeout 5m` | pass |
| `go test ./tests/redteam/... -run Edit -count=1 -timeout 5m` | pass |
| `go test ./claims/... -count=1` | pass |
| `go test ./... -count=1 -timeout 5m` | pass |
| `go vet ./...` | pass |
| `git ls-files '*.go' \| xargs gofmt -l` | pass, no output |
| `go run github.com/securego/gosec/v2/cmd/gosec@v2.26.1 ./...` | pass |
| `golangci-lint run --timeout=5m` | pass, 0 issues |
| Active public-reference sweep for `@v0.5.0`, `@v0.4.0`, and `@main` | pass, no active stale refs |
| `git diff --check` | pass |

## Edit Boundary Status

| Surface | Status | Truth |
|---|---|---|
| `boundary edit inspect` | delivered preview | Classifies patch bytes without applying them. |
| `boundary edit apply --dry-run` | delivered preview | Evaluates the exact patch bytes and records the verdict without invoking the applier. |
| `boundary edit apply` | delivered preview | Applies only when the preview policy allows or local preview approval is supplied for approval-required classes. |
| Edit decision records | delivered preview | Records patch hash, class, action, file list, redacted paths, local approval mode, dry-run, applier-invoked, and applied state. |
| Edit redteam packs | delivered preview | Fixture-only packs cover selected secret-bearing, package-script, CI/deploy, destructive-delete, and cross-scope mutation paths. |

## Claim Split

- `BND-CLAIM-EDIT-001` is delivered preview: Boundary provides preview Edit
  Boundary governance for proposed file mutations routed through Boundary edit
  envelopes.
- `BND-CLAIM-EDIT-002` is delivered preview: Boundary runs fixture Edit
  Boundary redteam packs that deny or require approval for selected
  file-mutation risk paths without live project mutation.

Delivered preview claims do not upgrade Edit Boundary to production.

## MCP Status Unchanged

MCP remains the production adapter path. v0.6.0 does not change MCP Firewall
claims or MCP adapter maturity.

## Secure GitHub Status Unchanged

Secure GitHub remains preview. v0.6.0 does not change the v0.5.0 opt-in live
conformance harness, and production status still requires deployment bypass
proof and broader live coverage.

## Command Boundary Status Unchanged

Command Boundary remains delivered preview for routed project-local command
paths. Direct shell execution, CI jobs, SSH sessions, and direct file edits
remain outside Command Boundary unless routed through Boundary.

## Approved Edit Boundary Copy

Use this copy for public docs:

> Boundary provides preview Edit Boundary governance for proposed file
> mutations routed through Boundary edit envelopes.

Supporting copy:

- Boundary can classify and gate proposed file mutations before they are
  applied when the edit routes through Boundary.
- Denied edit envelopes do not apply.
- Fixture Edit Boundary redteam packs do not perform live project mutation.

## Forbidden Edit Boundary Copy

Do not use these statements as public capability claims:

- Boundary controls all file writes.
- Boundary protects direct editor writes.
- Boundary prevents every unsafe edit.
- Boundary provides filesystem sandboxing.
- Boundary provides universal coding-agent file safety.
- Boundary governs direct file edits outside routed edit envelopes.
- Boundary provides production edit governance.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.
