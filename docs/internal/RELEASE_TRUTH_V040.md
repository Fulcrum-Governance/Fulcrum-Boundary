# Release Truth: Fulcrum Boundary v0.4.0

Date: 2026-05-27

Audited runtime code commit SHA: `3d9b82f82af09793fd818ff63a9dc0aab650b3d8`

Release packaging branch: `codex/2026-05-27-v040-command-boundary-release`

Release tag: `v0.4.0`

## Summary

Fulcrum Boundary v0.4.0 packages the already-merged Command Boundary preview.
The release does not change the v0.3.0 MCP Firewall or Secure GitHub truth.

Final v0.4.0 truth:

- MCP remains the production adapter path.
- Secure GitHub remains preview and fixture-backed until live GitHub App
  conformance and deployment bypass evidence exist.
- Command Boundary is delivered as a preview surface.
- Command Boundary governs only commands routed through `boundary command run`,
  `boundary shell`, or project-local shims.
- Direct shell access is outside Boundary unless explicitly routed through the
  wrapper or project-local shims.
- CI, SSH, cron, editor tasks, and arbitrary local processes are outside
  Boundary unless explicitly routed through Boundary.
- Command Boundary does not claim shell sandboxing.
- Command Boundary does not claim production command governance.
- Direct file edits outside routed command paths remain a v0.5 design gap.

## Verification Commands

| Command | Result |
| --- | --- |
| `make docs-build` | Pass |
| `go test ./internal/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/commandboundary/... -count=1 -timeout 5m` | Pass |
| `go test ./tests/redteam/... -run Command -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1` | Pass |
| `make release-check` | Pass |

`make release-check` included:

- `./scripts/assert-no-public-vendor-refs.sh`
- `go vet ./...`
- `go vet ./...` in `adapters/grpc`
- `go test ./... -count=1 -timeout 5m`
- `go test ./... -count=1 -timeout 5m` in `adapters/grpc`
- `go test ./tests/... -count=1 -timeout 5m`
- `go test ./claims/... -count=1 -timeout 5m`
- `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies`
- `go run ./cmd/boundary verify-record --help`
- `go run ./cmd/boundary selftest`
- `go run ./cmd/boundary demo github-lethal-trifecta`

The GitHub lethal-trifecta demo reported:

```text
actual action: DENY
reason: lethal_trifecta_detected
upstream_called=false
```

## Command Boundary Status

| Surface | Status | Truth |
| --- | --- | --- |
| `boundary command classify` | delivered preview | Classifies command argv without execution and redacts secret-looking arguments. |
| `boundary command run` | delivered preview | Evaluates wrapper-routed commands before execution and blocks denied or approval-required commands. |
| Command decision records | delivered preview | Writes local JSONL records with class, action, execution status, reason, argv hash, cwd, and redacted args. |
| `boundary command install --project` | delivered preview | Creates reversible project-local `.boundary/bin` shims without global shell mutation. |
| `boundary command uninstall --project` | delivered preview | Removes Boundary-generated project-local shims. |
| `boundary shell` | delivered preview | Launches a scoped subshell with `.boundary/bin` first on `PATH`; commands without shims remain outside Boundary. |
| Command redteam packs | delivered preview | Run fixture-only cleanup, secret-exfiltration, and repo-mutation packs without live mutation. |

## Public Install Status

Active install examples use:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
```

Active GitHub Action examples use:

```yaml
- uses: Fulcrum-Governance/Fulcrum-Boundary/actions/mcp-audit@v0.4.0
```

Post-tag smoke verification must run:

```bash
tmp=$(mktemp -d)
GOBIN="$tmp/bin" GOPROXY=direct \
  go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0
"$tmp/bin/boundary" selftest
"$tmp/bin/boundary" demo github-lethal-trifecta
"$tmp/bin/boundary" command classify -- git push origin main
```

Expected command classification signal:

```text
Class: C3 repo mutation
Recommended action: require_approval
```

## Post-Tag Smoke Verification

Date: 2026-05-27

| Command | Result |
| --- | --- |
| `GOPROXY=direct go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.4.0` | Pass |
| `boundary selftest` | Pass |
| `boundary demo github-lethal-trifecta` | Pass |
| `boundary command classify -- git push origin main` | Pass |
| `GOPROXY=https://proxy.golang.org,direct go list -m github.com/fulcrum-governance/fulcrum-boundary@latest` | Pass; resolved `v0.4.0` |

Smoke signals:

- `boundary selftest` reported `status: pass`.
- `boundary demo github-lethal-trifecta` reported `actual action: DENY`,
  `reason: lethal_trifecta_detected`, and `upstream_called=false`.
- `boundary command classify -- git push origin main` reported
  `Class: C3 repo mutation` and `Recommended action: require_approval`.

## Approved Command Boundary Copy

Boundary provides preview project-local command governance for commands routed
through `boundary command run`, `boundary shell`, or project-local shims.

Supporting copy:

- Boundary can classify command risk without executing commands.
- Boundary can deny or require approval before wrapper-routed command execution.
- Project-local shims can route selected commands through Boundary when the
  operator opts into `.boundary/bin`.
- Command redteam packs are fixture-only and do not perform live mutation.

## Forbidden Command Boundary Copy

Do not use these as public capability claims:

- Boundary controls all shell commands.
- Boundary protects direct shell access.
- Boundary prevents every overeager agent action.
- Boundary provides production command governance.
- Boundary provides shell sandboxing.
- Boundary controls CI jobs by default.
- Boundary controls remote SSH by default.
- Boundary governs direct file edits outside routed command paths.

These phrases may appear only in claim-control, language-control, historical,
or explicit limitation context.

## Remaining Work

- Post-v0.4 planning update: v0.5 moved to Secure GitHub live conformance
  preview. Filesystem/Edit Boundary moved to v0.6.
- Keep Command Boundary preview-scoped until deployment evidence shows Boundary
  is the relevant command path for a protected project or workflow.
