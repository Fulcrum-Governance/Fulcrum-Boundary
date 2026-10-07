<h1 align="center">Fulcrum Boundary</h1>

<p align="center"><strong>Auto mode decides. Boundary keeps the receipts.</strong></p>

<p align="center">
  Boundary records the routed decision — a hash-verifiable receipt written
  before the tool ran. Verification is integrity, not authenticity: it shows
  the record was not altered after emission, not that the verdict was right.
  Boundary does not replace Claude Code's own permission system or any other
  safety mode; it is a separate, routed gate that runs alongside them.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/fulcrum-governance/fulcrum-boundary"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/fulcrum-governance/fulcrum-boundary.svg"></a>
  <a href="https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Fulcrum-Governance/Fulcrum-Boundary/actions/workflows/ci.yml/badge.svg?branch=main"></a>
  <a href="https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/Fulcrum-Governance/Fulcrum-Boundary"></a>
  <a href="https://goreportcard.com/report/github.com/fulcrum-governance/fulcrum-boundary"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/fulcrum-governance/fulcrum-boundary"></a>
  <a href="./LICENSE"><img alt="License: Apache 2.0" src="https://img.shields.io/badge/License-Apache_2.0-blue.svg"></a>
</p>

<p align="center">
  <a href="#60-second-quickstart">Claude Code hook</a> ·
  <a href="#first-run-in-one-minute">Install Boundary</a> ·
  <a href="#see-boundary-stop-the-action">Run the MCP demo</a> ·
  <a href="./docs/GOVERN_MCP_SERVER.md">Govern an MCP server</a> ·
  <a href="https://fulcrum-governance.github.io/Fulcrum-Boundary/">Docs</a> ·
  <a href="./SECURITY.md">Security</a>
</p>

Boundary governs only actions whose route is forced through Boundary. Direct
access to the same tool is a bypass unless deployment topology removes that
path. MCP is the first production route; Command Boundary and Edit Boundary —
including the Claude Code hook below, which routes through them — are
delivered previews.

![A staged Claude Code Bash tool call, "git status && rm -rf ~/", denied before it runs; boundary verify-record then recomputes the hash of the decision record Boundary wrote before the denial reached Claude Code.](./demo/boundary-drill.gif)

*Real binary output against a staged event — the destructive command exists
only as text inside a JSON payload, never a real shell call. If the GIF above
has not rendered, that is exactly what it shows: a deny before execution, then
a receipt verified after.*

## 60-second quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/Fulcrum-Governance/Fulcrum-Boundary/main/scripts/install-claude-code.sh | sh
```

Then, inside Claude Code:

```
/plugin marketplace add fulcrum-governance/boundary-plugins
/plugin install boundary@boundary-plugins
```

> **Availability.** The hook first shipped in `v0.13.0` and remains included in
> the current `v0.13.1` release: the
> one-line installer's default mode fetches the latest published release
> binary, which now includes `boundary hook`. The public
> [`boundary-plugins`](https://github.com/Fulcrum-Governance/boundary-plugins)
> repository publishes the exact two-file marketplace package retained in
> this repository. On 2026-09-02, Claude Code `2.1.258` added that public
> repository, installed `boundary@boundary-plugins` `v0.13.1`, resolved the
> plugin to release commit
> `8a5762888be8404f8a4a0e64a2ad6206667b71b6`, and validated the installed
> manifest. A checkout remains an alternative: `git clone` this repo, then
> `sh scripts/install-claude-code.sh --plugin-drop`. Either installation is
> personal — see
> [`/boundary:protect`](./skills/protect/SKILL.md) to wire a
> floor every contributor to a project inherits. Marketplace installation
> changes distribution, not coverage: Boundary still governs only tool calls
> routed through the configured hook, and direct or bypass routes remain
> outside it.

Restart Claude Code, then run `/boundary:drill`.

## What just happened

- **Decided before execution.** Claude Code's `PreToolUse` hook handed the
  tool call to `boundary hook pretooluse` before the tool ran; a `deny` means
  nothing reached a shell or a file.
- **Recorded before the decision returned.** The decision record was written
  to disk before the verdict reached stdout — the record exists whether or
  not you go looking for it.
- **Verify it yourself.** `boundary verify-record <file>` recomputes the
  record's hash independently, so an edit after the fact is detectable by
  recomputation — not by trusting this README or the transcript.
- **A separate layer from permission prompts.** `PreToolUse` hooks run
  before Claude Code's own permission handling, and Claude Code's hook
  contract documents a `deny` as blocking the call in any permission mode —
  that is the client's documented behavior, not something Boundary can
  enforce from outside it. Ask-class verdicts surface through the client's
  own prompting, which varies by mode; the decision record is written before
  the verdict returns either way. (Enterprise `disableAllHooks` is a
  host-level switch that turns hooks off entirely — see
  `boundary hook doctor`.)

## What it proves, and what it does not

| This proves | This does not prove |
| --- | --- |
| The routed tool call was decided — allow, warn, ask, or deny — before it ran. | That every tool call in the session was governed. Only `Bash`/`Shell` and `Edit`/`Write`/`MultiEdit`/`NotebookEdit` route through this hook; an MCP tool, a subprocess's own command, or shell used outside Claude Code is a bypass. |
| `boundary verify-record` recomputes the record's hash independently, so tampering after emission is detectable. | That the verdict was correct, or who produced the record. Hashes are integrity, not authenticity. |
| A denied call's `execution_claim` reports `upstream_called=false` / `executed=false`. | Anything about upstream side effects beyond this route — that field is the hook's own self-report, not independent corroboration. |
| The hook is a routed pre-execution gate that runs independently of Claude Code's permission prompts. | That Boundary is a sandbox, contains the agent, or detects prompt injection in tool inputs. It classifies and records; it does not isolate the process or inspect prompt content. |
| Command Boundary and Edit Boundary classified this call under their current preview posture. | Production-grade classification coverage. Both are delivered previews — validate against your own policy before relying on a verdict for anything load-bearing. |

Full detail: [`docs/integrations/CLAUDE_CODE_HOOK.md`](./docs/integrations/CLAUDE_CODE_HOOK.md)
(honest scope and limitations) and [`LIMITATIONS.md`](./LIMITATIONS.md)
(repo-wide). The Claude Code hook landed on `main` after the `v0.12.0` tag and
first shipped in `v0.13.0` and remains included in `v0.13.1` — see the Claude Code hook lane
section of the [Boundary Roadmap](./docs/BOUNDARY_ROADMAP.md).

## The wider Boundary project

The Claude Code hook above routes Bash and Edit/Write tool calls into
Boundary's existing Command Boundary and Edit Boundary preview classifiers —
it adds no new governed action surface. The rest of this README covers the
project those classifiers, and the production MCP route, belong to.

## First run in one minute

Install the macOS Homebrew cask, run the self-test, and reproduce a denial:

```bash
brew install fulcrum-governance/tap/boundary
boundary selftest
boundary demo github-lethal-trifecta --out demo.txt
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
```

The demo is fixture-only: no credentials, network calls, or live mutations. It
denies a routed GitHub write-after-taint action before upstream execution and
writes a decision record that `verify-record` recomputes independently.

On Linux or Windows, use a signed-checksum [release archive](./docs/INSTALL.md)
or the published `v0.13.1` container image:

```bash
docker run --rm ghcr.io/fulcrum-governance/boundary:v0.13.1 selftest
```

To inspect the MCP servers your agents can already reach without modifying
configuration:

```bash
boundary init --dry-run
```

The Homebrew cask, container image, and `_static-nocgo` archives are static
builds. Their Postgres AST classifier is unavailable, so routed SQL classifies
as `UNKNOWN` and the Postgres guard denies it fail-closed. Use a `_cgo` archive
or build from source for the full classifier. See the [install guide](./docs/INSTALL.md)
for platform commands, checksums, SBOMs, and provenance evidence.

## See Boundary stop the action

A coding agent reads untrusted GitHub issue content and proposes a write to a
private repository. On the routed MCP path, Boundary returns `DENY` before the
GitHub mutation call, reports `upstream_called=false`, and emits a decision
record.

<picture>
  <source media="(prefers-color-scheme: light)" srcset="./docs/assets/readme-hero-light.svg">
  <source media="(prefers-color-scheme: dark)" srcset="./docs/assets/readme-hero-dark.svg">
  <img alt="A proposed GitHub action reaches the Fulcrum wedge, is denied before execution, and leaves a verifiable decision record." src="./docs/assets/readme-hero-dark.svg">
</picture>

![Boundary denies a GitHub write-after-taint action before upstream execution, with upstream_called=false and a hash-verifiable decision record](./docs/assets/github-lethal-trifecta-demo.gif)

| Step | What happens |
|---|---|
| **Proposed action** | `github.create_or_update_file` targets a private repository. |
| **Boundary** | The MCP route evaluates the action before the underlying tool runs. |
| **Verdict** | `DENY`, with `reason=lethal_trifecta_detected`. |
| **Record** | `rec_...` plus a `sha256:` decision hash that `boundary verify-record` recomputes. |

The same shape exists on Command Boundary, a delivered preview: a selected
routed command is evaluated before execution and its verdict is recorded.

## What you get

| Discover | Decide | Record and test |
|---|---|---|
| Inventory MCP client configurations and render reachable risk paths. | Evaluate routed actions against trust, static policy, domain interceptors, and policy rules. | Emit decision records, recompute their hashes, replay decisions, and test policies against local fixtures. |
| Start with `boundary init --dry-run`. | Start with the production MCP route or a labeled preview adapter. | Start with `boundary verify-record`, `boundary replay`, and `boundary test`. |

For a real routed setup, follow [Govern Your MCP Server](./docs/GOVERN_MCP_SERVER.md).
It covers discovery, reversible installation, a live routed denial, `boundary
doctor`, and uninstall. The [host setup guide](./docs/firewall/HOST_SETUP.md)
includes Claude Desktop, Claude Code, Cursor, and VS Code.

## How Boundary works

```mermaid
flowchart LR
  A[Agent intent] --> B[Proposed action]
  B --> C{Boundary}
  C -->|allow| D[Execute routed action]
  C -->|deny / warn / escalate / require approval| E[No execution or approval path]
  C --> F[Decision record]
```

Every routed evaluation follows the same four-stage pipeline: trust check,
static policies, domain interceptors, and the portable policy evaluator. A deny
is terminal. Fault handling is fail-closed on configured protected transports.
See [Architecture](./ARCHITECTURE.md) for the contracts and extension points.

## The record it leaves

Every governed verdict produces a structured [decision record](./docs/DECISION_RECORDS.md).
A receipt-grade record carries request, policy-bundle, and decision hashes, so
an edit after emission is detectable by recomputation:

```bash
boundary verify-record github-lethal-trifecta-artifacts/decision-record.json
# record verification: ok
```

Independent verifiers ship in [Python](./verifiers/python/),
[TypeScript](./verifiers/typescript/), and [Rust](./verifiers/rust/), pinned to
the Go implementation by committed conformance vectors. Optional Ed25519
signing is off by default and adds authorship for operators who manage keys.

A recomputing hash detects changes to covered record fields. It does not prove
that the verdict was correct, that enforcement occurred, or that the signer
managed keys safely. `upstream_called=false` and `executed=false` are adapter
self-reports, not independently corroborated fields of the decision hash. See
[Receipts](./docs/RECEIPTS.md), [Signing](./docs/SIGNING.md), and
[Limitations](./LIMITATIONS.md).

## Two proof lanes

The release has two fixture-only demonstration lanes. Each denies a dangerous
action pattern before it runs and emits a hash-verifiable decision record.

![Two equal-weight proof lanes: MCP denies a write-after-taint GitHub action before upstream; Command Boundary denies a routed secret-exfiltration command before execution.](./docs/assets/two-lane-proof.svg)

| Lane | Status | Demo | Verified shape |
|---|---|---|---|
| **MCP** | Production for routes forced through Boundary | `boundary demo github-lethal-trifecta` | `actual=DENY`, `upstream_called=false`, `reason=lethal_trifecta_detected` |
| **Command Boundary** | Delivered preview, routed-only | `boundary demo command-secret-exfil` | `actual=DENY`, `executed=false`, `class=C6` |

Both lanes use fixtures only: no credentials, network calls, or live mutations.
They do not establish coverage for every prompt, direct tool path, deployment,
or policy. Secure GitHub remains preview.

## Hard limits

- Boundary governs a route only when deployment forces the action through it.
- Direct shell, editor, filesystem, CI, SSH, or API paths remain outside the
  boundary unless topology removes them.
- Secure GitHub, Command Boundary, Edit Boundary, and the remaining non-MCP
  adapters are labeled previews.
- The Claude Code `PreToolUse` hook governs only the tool calls it is wired to
  intercept; an un-wired tool, an MCP tool, a tool a subprocess runs on its
  own, or shell used outside Claude Code is a bypass.
- Generated policies are starter policies for operator review.
- Decision records provide checkable integrity for covered fields, not a proof
  of correct enforcement or production safety.
- Dashboard and evidence utilities are local-only unless an operator supplies
  separate infrastructure.

Read the full [Limitations](./LIMITATIONS.md), [Claims Ledger](./docs/CLAIMS_LEDGER.md),
and [Release Truth](./docs/RELEASE_TRUTH_PUBLIC.md) before relying on a route.
The repository's claims and controlled language are mechanically checked by
`go test ./claims/...` and `make release-check`.

## Current release truth

This table describes the published `v0.13.1` release. The Claude Code hook
above first shipped in `v0.13.0` and remains included; see the
[Boundary Roadmap](./docs/BOUNDARY_ROADMAP.md).

| Surface | Status | Limit |
|---|---|---|
| MCP adapter | Production | Governs MCP routes forced through Boundary. |
| Secure GitHub | Preview | Fixture proof and opt-in conformance do not close deployment bypasses. |
| Command Boundary | Delivered preview | Routed command paths only. |
| Edit Boundary | Delivered preview | Routed edit envelopes only. |
| Policy-as-code testing | Local-only | `boundary test` checks routed request fixtures against local policy bundles. |
| Policy generation | Starter policy utility | Requires operator review. |
| Dashboard | Local artifact visibility | Not hosted monitoring. |
| Witnessed-log verifier | Delivered, air-gapped | Checks bundle integrity and cosignatures against supplied keys; it does not establish independent witness operation. |

Current published release: **v0.13.1**. See the [release notes](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/releases/tag/v0.13.1),
[changelog](./CHANGELOG.md), and [public release-truth record](./docs/RELEASE_TRUTH_PUBLIC.md).

## Adapter readiness

Adapter maturity is declared in `adapters/<adapter>/readiness.yaml` and checked
against the [Adapter Readiness Matrix](./docs/ADAPTER_READINESS_MATRIX.md).

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

## Documentation

| Need | Start here |
|---|---|
| Wire the Claude Code hook | [Claude Code PreToolUse Hook](./docs/integrations/CLAUDE_CODE_HOOK.md) |
| Install and verify a release | [Install](./docs/INSTALL.md) |
| Govern an MCP server | [Govern Your MCP Server](./docs/GOVERN_MCP_SERVER.md) |
| Configure a supported host | [Host Setup](./docs/firewall/HOST_SETUP.md) |
| Understand the model | [Architecture](./ARCHITECTURE.md) and [Boundary Spec](./docs/BOUNDARY_SPEC.md) |
| Compare Boundary with adjacent tools | [Where Boundary Fits](./docs/COMPARISON.md) |
| Read current capability limits | [Release Truth](./docs/RELEASE_TRUTH_PUBLIC.md) and [Limitations](./LIMITATIONS.md) |
| Verify decision records | [Decision Records](./docs/DECISION_RECORDS.md), [Receipts](./docs/RECEIPTS.md), and [Signing](./docs/SIGNING.md) |
| Test policies | [Policy Testing](./docs/POLICY_TESTING.md) |
| Check route deployment | [Route Conformance](./docs/ROUTE_CONFORMANCE_CHECKLIST.md) |
| Review public claims | [Claims Ledger](./docs/CLAIMS_LEDGER.md) and [How We Keep Ourselves Honest](./docs/HOW_WE_KEEP_OURSELVES_HONEST.md) |
| Troubleshoot first run | [Troubleshooting](./docs/TROUBLESHOOTING.md) |
| Browse all guides | [Documentation site](https://fulcrum-governance.github.io/Fulcrum-Boundary/) |

## Development

Requires Go 1.25+. The default source build links the full Postgres classifier
through cgo and requires a C toolchain:

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.13.1
```

`v0.13.1` is the current public release. The root and nested-module tags both
resolve to the same immutable release commit; see the release-truth record.

Before submitting a change:

```bash
make release-check
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
make docs-build
```

See [Contributing](./CONTRIBUTING.md), [Testing](./docs/TESTING.md), and the
[Code of Conduct](./CODE_OF_CONDUCT.md). Questions belong in
[GitHub Discussions](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/discussions);
security reports follow [SECURITY.md](./SECURITY.md).

## Part of the Fulcrum architecture

Boundary is the open action boundary in the Fulcrum repo family. The hosted
operator surfaces remain private during build; [`fulcrum-trust`](https://github.com/Fulcrum-Governance/fulcrum-trust)
supplies public trust-modeling work, and [Fulcrum-Proofs](https://github.com/Fulcrum-Governance/Fulcrum-Proofs)
holds the Lean proof work used through documented correspondence.

Boundary's runtime behavior corresponds by design to a machine-checked equilibrium analysis
upstream in Fulcrum-Proofs. That analysis is a design
constraint, not a runtime certificate. Boundary itself does not emit `proved`
decisions. See [Proof Boundary](./docs/PROOF_BOUNDARY.md).

## Project

- [Apache 2.0 License](./LICENSE)
- [Security Policy](./SECURITY.md)
- [Support](./SUPPORT.md)
- [Maintainers](./MAINTAINERS.md)
- [Contributing](./CONTRIBUTING.md)
- [Code of Conduct](./CODE_OF_CONDUCT.md)
- [Changelog](./CHANGELOG.md)
- [Citation](./CITATION.cff)

---

Boundary is the open edge of Fulcrum. Join the waitlist for the hosted
operator surfaces at [fulcrumlayer.io](https://fulcrumlayer.io); the trust
modeling behind Boundary's Stage-1 trust checks is public in
[`fulcrum-trust`](https://github.com/Fulcrum-Governance/fulcrum-trust).
