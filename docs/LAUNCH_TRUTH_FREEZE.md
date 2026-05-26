# Launch Truth Freeze — Fulcrum Boundary

**Gate:** 0 — Product Identity and Truth
**Plan:** Fulcrum Boundary v3.3 (`fulcrum-io:.claude/research/2026-05-25-gil-launch-plan-v3.md`)
**Frozen (UTC):** 2026-05-26
**Branch:** `gate0/truth-freeze-and-migration` (cut from `main`; not yet merged, not yet tagged)
**Migration commit:** `42ef711`

This is the verified ground-truth snapshot produced by Gate 0. Every claim is backed by a
command run during execution and its captured output. Treat it as read-only history —
supersede with a dated update rather than editing in place.

---

## 1. Module Path Migration — VERIFIED

| Item | Old | New | Verified |
|---|---|---|---|
| Go module path | `github.com/fulcrum-governance/gil` | `github.com/fulcrum-governance/boundary` | ✅ |
| Local directory | `…/Projects/governance-interception-layer` | `…/Projects/Boundary` | ✅ |
| GitHub repo | `Fulcrum-Governance/governance-interception-layer` | `Fulcrum-Governance/Boundary` | ✅ (old name redirects) |

**Rewrite scope: 45 tracked files**
- 34 `.go` files (36 references) — import paths
- 8 `go.mod` files (root + `adapters/grpc` + 6 `examples/*`) — `module`, `replace`, and `require` lines
- 3 docs — `README.md` (Go Reference + Go Report Card badges, import sample), `docs/ADAPTER_CONTRACT.md`, `policyeval/README.md`

**Checks**
- `git grep -F 'github.com/fulcrum-governance/gil'` → **0 occurrences** after rewrite.
- All 7 nested-module `replace … => ../../` right-hand paths left intact.
- Root module declares its own path with **no self-`replace`** → resolves externally clean (Hard Rule 1).

## 2. Remote URL — CONFIRMED

```
$ git remote -v
origin  https://github.com/Fulcrum-Governance/Boundary.git (fetch)
origin  https://github.com/Fulcrum-Governance/Boundary.git (push)
```

Repo is live, **PUBLIC**, default branch `main`. Description:
"Out-of-process enforcement boundary for the Fulcrum governance kernel: transport adapters
and pre-execution policy control." Old name redirects. **No tags / no releases yet**
(`v0.2.0` is a Gate 2 artifact).

## 3. Test Inventory — ALL PASS

| Field | Value |
|---|---|
| Recorded command | `env -u GOROOT go test ./... -short -count=1 -timeout 5m` |
| Count method | re-run with `-json` (format-only flag; same tests execute) parsed for machine counts |
| Date (UTC) | 2026-05-26T16:10:59Z |
| Toolchain | `go1.24.1 darwin/arm64` (go.mod directive `go 1.23` — no toolchain switch) |

| Module | Result |
|---|---|
| Root (`./...`) | **EXIT 0** — 8 packages pass, **243 top-level tests + 575 subtests**, 0 failures, 0 skips |
| `adapters/grpc` (separate module) | EXIT 0 — tests pass |
| `examples/*` (6 separate modules) | EXIT 0 — build-verified; no test files (demo programs) |

`go test ./...` from the root module does not recurse into nested modules; the nested
modules were tidied, built, and tested separately. All green.

**Count reconciliation:** prior docs cited "390" and "253 func Test + 575 subtests". The
fresh `-json` count is **243 top-level + 575 subtests** for the root module (subtest count
matches exactly). Per the plan, the fresh count is the only count.

## 4. External Install Verification — VERIFIED

Hard Rule 1: external `go get` must work without `replace`. Clean temp-module smoke:

```
$ tmp=$(mktemp -d) && cd "$tmp" && go mod init smoke
$ go get github.com/fulcrum-governance/boundary@42ef711
go: downloading github.com/fulcrum-governance/boundary v0.0.0-20260526161151-42ef71150db3
go: added github.com/fulcrum-governance/boundary v0.0.0-20260526161151-42ef71150db3
# EXIT 0
```

Resolved via the public proxy (`proxy.golang.org`) to pseudo-version
`v0.0.0-20260526161151-42ef71150db3`. The module's `go.mod` declaration matches the
requested path — no "module declares its path as …" mismatch.

**Scope of this check:** verified against the **branch commit** (Gate 0 lands as a PR; not
yet on `main`). Once merged, `@latest` resolves to the same; `@v0.2.0` resolves once tagged
(Gate 2). The **CLI install path** (`go install github.com/fulcrum-governance/boundary/cmd/boundary@latest`)
is **not** verifiable yet — `cmd/boundary/` is built in Gate 1.

## 5. Repo Hygiene Checklist

| File / surface | Status |
|---|---|
| `README.md` | ✅ present (demo-first rewrite is Gate 2) |
| `SECURITY.md` | ✅ present |
| `CONTRIBUTING.md` | ✅ present |
| `CHANGELOG.md` | ✅ present (Unreleased promotion is Gate 2) |
| `LICENSE` | ✅ present — **Apache License 2.0** |
| `CODE_OF_CONDUCT.md` | ✅ present |
| CI workflows | ✅ `.github/workflows/ci.yml`, `.github/workflows/codeql.yml` |
| `cmd/` (CLI binary) | ⛔ absent — Gate 1 build (`cmd/boundary/`) |
| Version tag / GitHub Release | ⛔ none — Gate 2 |

## 6. Transport Adapters (`adapters/`) — 6 present

| Adapter | Path | Notes |
|---|---|---|
| MCP | `adapters/mcp/` | first launch surface (MCP Safety Gateway) |
| CLI | `adapters/cli/` | command dispatch |
| CodeExec | `adapters/codeexec/` | code-execution interception |
| gRPC | `adapters/grpc/` | separate Go module |
| Webhook | `adapters/webhook/` | |
| A2A | `adapters/a2a/` | experimental |

Plus HTTP middleware (`interceptors/`, `examples/http-middleware/`). The plan's transport count of 6 is accurate. *Adapters change. The boundary does not.*

## 7. Examples (runnable demos) — 6 present

`simple`, `custom-interceptor`, `mcp-proxy`, `http-middleware`, `redis-trust`, `rate-limit` —
all build as standalone modules. The Gate 1 launch demo (`examples/mcp-postgres-gateway/`
with the Docker frontend/backend network topology) **does not exist yet**.

## 8. Out of Scope for This Gate / Still Pending

This freeze covers the **migration + verification** half of Gate 0. The following Gate 0
items are **NOT** complete and must not be treated as done:

- **Competitor baselines must be regenerated before any external use.** The AGT / Galileo /
  Anthropic Managed Agents figures in the launch plan are planning-context snapshots only.
  Gate 0's `.claude/research/gil-competitor-truth-freeze.md` (source links, retrieval
  commands, timestamps, `EXTERNAL_SAFE`/`NEEDS_REFRESH`/`DO_NOT_USE_EXTERNALLY` usage labels)
  is **not yet produced** in the `fulcrum-io` repo.
- **Claims freeze** against `docs/validation/claims-lock.md` (three-check rule) — not yet run.
- **Decision-record terminology lock** — for v0.2.0 the term is "decision records," not
  "receipts" (receipts require policy-hash + request-hash + `verify record`, which is Gate 4).
  Documented in the plan; not yet enforced across repo copy.
- **Decision-mode terminology lock** — runtime modes are `deterministic` / `escalated` /
  `human_approved` / `allowed` / `blocked`. `proved` is never a runtime mode. Documented in
  the plan; not yet enforced.
- **Launch claim checklist** — every planned public sentence tagged VERIFIED / UNVERIFIED —
  not yet produced.

**Do not start Gate 1 build work** (Docker demo topology, `cmd/boundary/`, YAML policy
loading, decision-record emission) on this branch.

---

*Generated during Gate 0 execution, 2026-05-26, commit `42ef711` on
`gate0/truth-freeze-and-migration`. This document records what was verified; it does not
authorize external claims beyond the checks above.*
