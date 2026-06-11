# Boundary CLI Reference

This is the canonical repository reference for the Boundary CLI. Boundary CLI
commands are intentionally local-first. Commands that use fixtures say so,
commands that mutate MCP configs support dry-run review, and preview surfaces
stay labeled preview.

## Command Status Legend

Every command below carries one of these maturity labels. They match
[docs/RELEASE_TRUTH_PUBLIC.md](./RELEASE_TRUTH_PUBLIC.md) and the per-adapter
[docs/ADAPTER_READINESS_MATRIX.md](./ADAPTER_READINESS_MATRIX.md).

| Label | Meaning |
| --- | --- |
| **Delivered** | A production or delivered route is exercised. MCP is the first production route. |
| **Preview** | A labeled preview surface (Command Boundary, Edit Boundary, Secure GitHub, and the remaining adapters). Preview does not mean production. |
| **Local-only** | A local diagnostic, inventory, evidence, or visibility command. Local-only output does not prove that any deployment route is protected. |

## Command Map

All 28 top-level commands. Sub-commands and compound entries (`demo <name>`,
`policy generate`, `mcp proxy`) are noted in the Purpose column. Status follows
[README Current Release Truth](../README.md#current-release-truth) and
[docs/ADAPTER_READINESS_MATRIX.md](./ADAPTER_READINESS_MATRIX.md).

| Command | Purpose | Status | Owning doc |
| --- | --- | --- | --- |
| `version` | Print build metadata (version, commit, date). | Local-only | This file §1 |
| `init` | Initialize a `.boundary/firewall` workspace; discovers MCP configs. | Local-only | This file §6 |
| `inventory` | Discover MCP configs (`inventory`) or ingest NDJSON records (`inventory ingest`). | Local-only | This file §2, §5 |
| `graph` | Render inventory-derived MCP risk paths (text / Mermaid). | Local-only | This file §2 |
| `dashboard` | Render a local-only firewall dashboard (HTML file or HTTP serve). | Local-only | This file §7 |
| `install` | Rewrite selected MCP config entries to route through Boundary. | Preview / production-route config | This file §6 |
| `uninstall` | Restore an MCP config from a Boundary install receipt. | Preview / production-route config | This file §6 |
| `lock` | Create a descriptor lockfile for MCP server descriptors. | Local-only | [docs/firewall/INSTALL_LOCK.md](./firewall/INSTALL_LOCK.md) |
| `verify-lock` | Verify MCP server descriptors against a lockfile; drift triggers configurable action. | Local-only | [docs/firewall/INSTALL_LOCK.md](./firewall/INSTALL_LOCK.md) |
| `redteam` | Run safe fixture-only attacks and report expected deny records. | Local-only | [docs/firewall/REDTEAM.md](./firewall/REDTEAM.md) |
| `selftest` | Run local no-credential release smoke checks. | Local-only | This file §1 |
| `secure` | Manage Secure MCP preview profiles (`secure github …`). | Preview | This file §4 |
| `command` | Classify and govern project-local command paths (`command classify`, `command run`, `command install`, `command uninstall`). | Delivered preview | [docs/command-boundary/README.md](./command-boundary/README.md) |
| `edit` | Classify proposed file mutations routed through Boundary edit envelopes (`edit inspect`, `edit apply`). | Delivered preview | [docs/edit-boundary/README.md](./edit-boundary/README.md) |
| `shell` | Launch a project-local Command Boundary subshell with `.boundary/bin` prepended to PATH. | Delivered preview | [docs/command-boundary/SHELL.md](./command-boundary/SHELL.md) |
| `policy` | Generate starter YAML firewall policies (`policy generate`). | Local-only | This file §2 |
| `mcp` | Fail-closed generic MCP proxy entrypoint for installed routes (`mcp proxy`). | Production-route | This file §12 |
| `serve` | Start the Boundary HTTP gateway — the production MCP route. | Production-route | This file §13, [docs/adapters/MCP.md](./adapters/MCP.md) |
| `demo` | Run fixture-only or live demos (`demo action-boundary`, `demo github-lethal-trifecta`, `demo command-secret-exfil`, `demo trust-degradation`, `demo postgres`). | Mixed (see §3) | This file §3 |
| `verify` | Validate YAML policy files. | Local-only | This file §8 |
| `verify-record` | Verify a decision record's internal hash consistency. | Local-only | This file §9 |
| `explain` | Describe a decision record (read-only render, no hash recomputation). | Local-only | This file §10 |
| `replay` | Re-evaluate a recorded request and compare reproduced decision fields. | Local-only | This file §11 |
| `test` | Run local policy-as-code test cases against policy bundles. | Local-only | This file §8A |
| `doctor` | Check local routed-surface diagnostics and bypass caveats. | Local-only | This file §1, [docs/DOCTOR.md](./DOCTOR.md) |
| `evidence` | Bundle and verify local Boundary evidence artifacts (`evidence bundle`, `evidence verify`). | Local-only | This file §1, [docs/EVIDENCE_BUNDLE.md](./EVIDENCE_BUNDLE.md) |
| `audit` | Pretty-print structured decision records from a log file or stdin. | Local-only | This file §14 |
| `trust` | Inspect or reset trust state for an agent (`trust show`, `trust reset`). | Local-only | This file §15 |

## 1. First-Run Commands

Run this exact sequence after install. It is the canonical first-run path and
matches the [README](../README.md) quickstart; the demos appear in the same
order there.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.9.0
boundary selftest                                            # Local-only smoke test
boundary doctor --json                                       # Local-only diagnostics + bypass caveats
boundary doctor --report                                     # Source builds after v0.9.0: redacted local report
boundary demo github-lethal-trifecta      # Lane 1: MCP, the first production route (Delivered)
boundary demo command-secret-exfil        # Lane 2: Command Boundary, a delivered preview (Preview)
boundary evidence bundle --include-demo --out boundary-evidence   # Local-only evidence bundle
boundary evidence verify boundary-evidence                        # Local-only bundle integrity check
# when a demo or evidence step prints a decision-record path:
boundary verify-record <record.json>                              # Local-only record self-verification
```

`boundary --help` lists every command, and `boundary version` (text or `--json`)
prints build metadata. Missing release metadata is reported as `unknown` instead
of failing the command. `boundary version` is local build metadata only and is
not cryptographic release provenance.

`boundary selftest` (Local-only) runs no-credential release checks. It uses
local fixtures, does not call the network, and does not perform live mutation.

`boundary doctor --json` (Local-only) reports local routed-surface diagnostics
and bypass caveats for MCP, Command Boundary, and Edit Boundary, plus first-run
environment diagnostics for the Go toolchain, cgo / C-toolchain readiness, and
`go install` PATH resolution. It does not call the network. Doctor output is
local diagnostics, not proof that every deployment route is protected. Source
builds after `v0.9.0` also include `boundary doctor --report`, which emits
redacted JSON for support threads; the pinned `@v0.9.0` install does not include
that flag until the next release tag. See [docs/DOCTOR.md](./DOCTOR.md).

`boundary demo github-lethal-trifecta` (Delivered, Lane 1) is the fixture-only
MCP proof lane: untrusted GitHub issue context flows into a private-repo mutation
attempt, Boundary denies the routed action before upstream
(`upstream_called=false`), and a decision record is emitted. `boundary demo
command-secret-exfil` (Preview, Lane 2) is the Command Boundary delivered-preview
lane described in section 3. Both lanes are fixture-only: no credentials, no
network, no real mutation.

`boundary evidence bundle` (Local-only) collects local release evidence with a
manifest and SHA-256 hashes; `--include-demo` adds the action-boundary demo
artifacts. `boundary evidence verify` (Local-only) checks manifest schema,
artifact existence, artifact hashes, declared JSON schemas, parseable decision
records when present, and summary references. Evidence verification is local
integrity checking; it does not prove production route enforcement. See
[docs/EVIDENCE_BUNDLE.md](./EVIDENCE_BUNDLE.md) and
[docs/EVIDENCE_VERIFY.md](./EVIDENCE_VERIFY.md).

`boundary verify-record <record.json>` (Local-only) recomputes a decision
record's stable hashes and confirms the record is internally consistent and
unmodified since emission. Run it on a single `DecisionRecordV1` JSON object. See
section 9, [docs/DECISION_RECORDS.md](./DECISION_RECORDS.md), and
[docs/RECEIPTS.md](./RECEIPTS.md).

Example output: [examples/cli/selftest.txt](../examples/cli/selftest.txt)

## 2. Firewall Commands

```bash
boundary inventory --format markdown
boundary graph --format mermaid
boundary policy generate --out boundary-firewall-policies
boundary verify --policies boundary-firewall-policies
```

Inventory discovers MCP config files and tools that Boundary can route. Risk
graphs make potential routes visible for review. Starter policies are a review
baseline and should be inspected before production use.

Examples:

- [examples/cli/inventory-markdown.md](../examples/cli/inventory-markdown.md)
- [examples/cli/risk-graph.mmd](../examples/cli/risk-graph.mmd)

## 3. Demo Commands

```bash
boundary demo action-boundary
boundary demo action-boundary --markdown --out demo.md
boundary demo github-lethal-trifecta
boundary demo github-lethal-trifecta --markdown --out demo.md
boundary demo command-secret-exfil
boundary demo command-secret-exfil --json
boundary demo command-secret-exfil --out demo.txt
boundary demo postgres --gateway http://localhost:8080/mcp
boundary demo trust-degradation
```

The Command Boundary secret-exfil demo wraps the `command-secret-exfil`
fixture red-team pack: an untrusted task proposes posting a secret-looking
environment file with `curl`, Boundary classifies it as Class C6 and denies it
before execution (`executed=false`), and a decision record is emitted. It reads
no real `.env`, makes no network call, and executes nothing.

Both proof-lane demos print a uniform record-location pair —
`decision record id: rec_...` and, when `--out` writes a file,
`decision record path: <path>` — and land that file at a predictable
`*-artifacts/decision-record.json` location for `boundary verify-record`.
They also write a multi-record `decision-records.jsonl` audit log and print it
as `decision record log: <path>`.
`boundary demo command-secret-exfil --out demo.txt` writes
`command-secret-exfil-artifacts/decision-record.json`; without `--out` the
record is printed to stdout only and no path or log line appears.

The Action Boundary demo composes fixture-only MCP / Secure GitHub, Command
Boundary, and Edit Boundary paths. It uses no credentials, no network, and no
live mutation. The Secure GitHub demo is fixture-only as well. The Postgres demo
requires a running Boundary gateway and checks direct database bypass
separately.

Example outputs:

- [examples/cli/demo-action-boundary.txt](../examples/cli/demo-action-boundary.txt)
- [examples/cli/demo-github-lethal-trifecta.txt](../examples/cli/demo-github-lethal-trifecta.txt)

## 4. Secure GitHub Commands

```bash
boundary secure github --help
boundary secure github setup --out .boundary/secure-github
boundary secure github serve --fixture --dry-run
boundary secure github conformance --help
```

Secure GitHub is a preview profile for routed GitHub tools. Fixture mode writes
local profile and starter policy artifacts only. Live conformance is opt-in and
skips unless `BOUNDARY_GITHUB_CONFORMANCE=true` is set.

Live conformance commands:

```bash
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance read
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance denied-write
BOUNDARY_GITHUB_CONFORMANCE=true boundary secure github conformance all --out /tmp/boundary-secure-github
```

The denied-write path must report `actual action: DENY`,
`reason: lethal_trifecta_detected`, `upstream_called=false`, and
`github_mutation_called=false`. Secure GitHub remains preview until deployment
bypass proof exists.

## 5. Inventory Ingest Commands

```bash
boundary inventory ingest \
  --file fixtures/external-inventory/external-mcp-inventory.ndjson \
  --source external-mcp \
  --summary
```

External MCP inventory NDJSON is input data. Boundary promotes only records that
describe MCP clients, MCP configs, MCP servers, MCP tools, risk paths, governed
routes, or policy recommendations.

Example output:
[examples/cli/external-ingest-summary.txt](../examples/cli/external-ingest-summary.txt)

## 6. Install/Uninstall Commands

```bash
boundary install --config path/to/mcp.json --server shell --dry-run
boundary install --client repo --out .boundary/firewall
boundary uninstall --receipt .boundary/firewall/install-receipts/example.json --dry-run
```

Install rewrites selected MCP entries so routed tools execute through Boundary.
Use dry-run first. Direct upstream access remains a deployment bypass unless
operators remove that path.

## 7. Dashboard Commands

```bash
boundary dashboard --format html --out .boundary/firewall/dashboard.html
boundary dashboard --serve --listen 127.0.0.1:8942
```

The dashboard is local-only and intended for operator review. It can summarize
inventory, policies, receipts, descriptor locks, and decision records; it is not
a policy enforcement path.

## 8. Release Verification Commands

```bash
./scripts/assert-no-public-vendor-refs.sh
make docs-build
make release-check
go test ./claims/... -count=1
go test ./... -count=1 -timeout 5m
boundary test --path tests/fixtures/policy-test/cases
boundary verify --policies ./policies/ --json
boundary evidence bundle --include-demo --out /tmp/boundary-evidence
boundary evidence verify /tmp/boundary-evidence
```

`boundary verify` (Local-only) checks that a YAML policy bundle parses and
reports file/rule counts and warnings; it does not prove the policies are
correct or that any route enforces them. `--json` emits a versioned
`boundary.verify.v1` object (`ok`, `error`, `policy_files`, `rules`,
`warnings`); exit codes are unchanged.

These checks keep public language, claims, docs, examples, and release gates in
sync before shipping a Boundary release branch.

## 8A. Policy-as-Code Test Commands

> **Availability:** `boundary test` is included in the `v0.9.0` release. The
> `@v0.9.0` install includes it; the historical `@v0.8.0` install does not.

```bash
boundary test --path tests/fixtures/policy-test/cases
boundary test --path tests/fixtures/policy-test/cases --format json
boundary test --path .boundary/tests
```

`boundary test` (Local-only) runs YAML policy-as-code cases through the existing
Boundary governance pipeline. Each case names a local policy bundle, a
`GovernanceRequest` fixture, and an expected verdict. The runner exits non-zero
on any verdict mismatch, malformed case, unexpected policy-load error, or
expected `parse_rejection` that does not reject. It supports `allow`, `deny`,
`warn`, `require_approval`, `escalate`, and `parse_rejection`.

The command is fixture-only: no credentials, no network calls, and no live
mutation. JSON output emits a stable `boundary.test.v1` envelope with those
local-safety flags, a summary, one result per case, and a fixed
`does_not_prove` footer. See [docs/POLICY_TESTING.md](./POLICY_TESTING.md).

`boundary test` reports policy verdicts for routed request fixtures only. It
does **not** prove production route enforcement, does **not** prove a deployment
removed every direct or unrouted path to the same tool, and does **not** prove
the verdict was globally correct beyond the supplied fixture and local policy
bundle.

## 9. Decision-Record Verification Commands

> **Availability:** `boundary verify-record` and `schema_version "1"` records are
> baseline. The `schema_version "2"` route-context path described below, along
> with `boundary explain` / `boundary replay` in sections 10–11, shipped in
> `v0.8.0` and is included in `v0.9.0`; `go install …@v0.9.0` includes them.

```bash
boundary verify-record record.json
boundary verify-record --binary-digest fixture-only record.json
boundary verify-record --json record.json
```

`boundary verify-record` (Local-only) reads one decision-record JSON object,
recomputes its stable hashes, and confirms `schema_version` and the
self-`decision_hash` match. It accepts both `schema_version "1"` (no
route-context) and `schema_version "2"` (additive route-context: `adapter_id`,
`route_id`, `topology_profile`, `execution_claim`); the route-context fields are
covered by `decision_hash`, so altering one fails verification, but they are not
attestation — see [`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md). With no
flags it confirms only that the record is internally hash-consistent and
unmodified since emission. `--json` emits a versioned
`boundary.verify_record.v1` object (`ok`, `error`, `record_id`); exit codes are
unchanged.

The first-run demos and evidence steps print a uniform `decision record path:`
line whenever they write a record file. Either proof lane produces a committable
record under `--out`:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
# decision record path: github-lethal-trifecta-artifacts/decision-record.json
# decision record log:  github-lethal-trifecta-artifacts/decision-records.jsonl
boundary demo command-secret-exfil --out demo.txt
# decision record path: command-secret-exfil-artifacts/decision-record.json
# decision record log:  command-secret-exfil-artifacts/decision-records.jsonl
```

Run bare `verify-record` on the single-record JSON path. A demo without `--out`
prints its records to stdout but persists no file, so no `decision record path:`
or `decision record log:` line appears.

Optional cross-check flags bind a record to external inputs:

- `--binary-digest sha256:...` compares the record `boundary_build_digest`. The
  shipped fixture demo record carries the literal `fixture-only`, so use
  `--binary-digest fixture-only` against that record; do not pass a real build
  digest, because `boundary version` does not emit one.
- `--policies dir` recomputes a sorted canonical-YAML hash and compares the
  record `policy_bundle_hash`. The shipped fixture demo record carries a
  placeholder `policy_bundle_hash`, so `--policies` does not match it. To use
  `--policies`, supply a record whose `policy_bundle_hash` was computed from a
  committed policy directory, and commit that exact policy set alongside it.
- `--request request.json` recomputes a canonical request hash and compares the
  record `request_hash`. The fixture demo path does not export a raw request and
  its request hash derives from per-run identifiers, so `--request` does not
  match the shipped demo records.

`verify-record` checks stable hashes only; it does not verify signatures, and a
no-flag pass does not bind the record to the request, policy bundle, or build
that actually ran. See [docs/DECISION_RECORDS.md](./DECISION_RECORDS.md) and
[docs/RECEIPTS.md](./RECEIPTS.md).

## 10. Decision-Record Explanation Commands

> **Availability:** `boundary explain` (this section) and `boundary replay`
> (section 11) are in the `v0.9.0` release, so `go install …@v0.9.0` includes
> them. They join the rest of the `v0.9.0` first-run path — `selftest`, `doctor`,
> the two proof demos, `evidence bundle`/`verify`, and `verify-record` on a
> `schema_version "1"` or `"2"` record.

```bash
boundary explain record.json
boundary explain --json record.json
boundary explain docs/examples/decision-record.example.json
boundary explain --json docs/examples/decision-record-v2.example.json
```

`boundary explain` (Local-only) reads one decision-record JSON object and prints
a human-readable account of it: the decision-defining fields (`action`,
`reason`, `decision_mode`, `matched_rule`, `policy_file`), the route-context
fields for a `schema_version "2"` record (`adapter_id`, `route_id`,
`topology_profile`, `execution_claim`), each stable hash and exactly what it
covers, and a fixed "what this does not prove" footer. It accepts both
`schema_version "1"` and `"2"` records. `--json` emits a stable
`boundary.explain.v1` object, mirroring `boundary doctor --json` and
`boundary selftest --json`; the envelope carries `requires_credentials`,
`requires_network`, and `mutates_live_systems`, all `false`.

`explain` is read-only and renders only — it does **not** evaluate policy, call
the network, mutate anything, or recompute any hash. Because it does not verify,
it renders even a record whose `decision_hash` has been altered; use
`boundary verify-record` (section 9) to recompute the hashes. `explain` does not
prove the verdict was correct and does not prove enforcement: a `deny` record is
not evidence the action was blocked, and direct access to the same tool is a
bypass a record cannot see. `topology_profile` is asserted, not attested, and
`execution_claim` is an adapter self-report, not corroborated. See
[docs/DECISION_RECORDS.md](./DECISION_RECORDS.md) and
[docs/RECEIPTS.md](./RECEIPTS.md).

## 11. Decision-Record Replay Commands

> **Availability:** like `boundary explain` (section 10), `boundary replay` is
> in the `v0.9.0` release; `go install …@v0.9.0` includes it.

```bash
boundary replay record.json --request request.json --policies ./policies/
boundary replay --json record.json --request request.json --policies ./policies/
boundary replay docs/examples/decision-record-replay.example.json \
  --request docs/examples/replay-request.example.json \
  --policies docs/examples/replay-policies/
```

`boundary replay` (Local-only) re-evaluates a recorded request and compares the
reproduced decision against the record. A record carries `request_hash` but not
the request body, so replay takes the record plus `--request` (the canonical
`GovernanceRequest` JSON that was recorded) and `--policies` (the operator's
policy directory). It (1) recomputes `request_hash` from the supplied request and
confirms it matches the record, so it is replaying *the recorded request*; (2)
when the record carries a `policy_bundle_hash`, recomputes it from `--policies`
and confirms it matches, so it is replaying against *the recorded policy bundle*,
not a stale or different one; (3) rebuilds the request and runs it through the
same pipeline in a hermetic, in-process configuration with no audit side effects;
and (4) compares the decision-defining fields — `action`, `reason`,
`decision_mode`, `matched_rule`, and `policy_file` where present — **not `action`
alone**, because a stale or different bundle can reach the same `action` through a
different rule, reason, or decision mode.

`replay` exits non-zero on any decision-field mismatch, on a `request_hash`
mismatch, or on a `policy_bundle_hash` mismatch. `--json` emits a stable
`boundary.replay.v1` object, mirroring `boundary doctor --json` and
`boundary selftest --json`; the envelope carries `requires_credentials`,
`requires_network`, and `mutates_live_systems`, all `false`.

`replay` reproduces the *decision*, not enforcement. A reproduced `deny` is
**not** evidence the action was blocked; replay does **not** prove that no
upstream bytes moved; it reproduces the decision only for routed requests, and
direct access to the same tool is a bypass a record cannot see; and a match does
**not** prove the original verdict was correct — only that the same inputs
reproduce the same decision. No upstream tool is called and nothing is mutated.
See [docs/DECISION_RECORDS.md](./DECISION_RECORDS.md) and
[docs/RECEIPTS.md](./RECEIPTS.md).

## 12. MCP Proxy Command

```bash
boundary mcp proxy \
  --install-receipt .boundary/firewall/install-receipts/example.json \
  --server my-server \
  --mode balanced
```

`boundary mcp proxy` (Production-route) is the fail-closed stdio MCP proxy
entrypoint used by installed routes. It is spawned by the MCP client as a
subprocess — not invoked interactively — and its flags are written into the MCP
config entry by `boundary install`. Operators do not typically invoke it
directly.

Flags (transcribed from `boundary mcp proxy --help`):

| Flag | Default | Description |
| --- | --- | --- |
| `--install-receipt` | _(required)_ | Path to the Boundary install receipt written by `boundary install`. |
| `--server` | _(required)_ | MCP server name from the install receipt. |
| `--mode` | `balanced` | Policy mode recorded during install; passed through for audit context. |

`mcp proxy` governs only MCP tool calls that arrive through the installed route.
Direct client access to the upstream MCP server is a bypass unless the
deployment removes that path. See [docs/adapters/MCP.md](./adapters/MCP.md) for
the adapter lifecycle and bypass model.

## 13. Serve Command

```bash
boundary serve \
  --listen :8080 \
  --policies ./policies/ \
  --upstream http://127.0.0.1:9000/mcp
```

`boundary serve` (Production-route) starts the Boundary HTTP MCP gateway — the
primary path for MCP routes forced through Boundary. It accepts HTTP JSON-RPC
MCP requests, evaluates each action through the governance pipeline, and either
returns a JSON-RPC error `-32001` before forwarding (deny) or proxies the
request to the configured upstream MCP server. The legacy Postgres demo path is
still available when `--upstream` is a Postgres DSN; production deployments use
an HTTP(S) upstream URL.

Flags (transcribed from `boundary serve --help`):

| Flag | Default | Description |
| --- | --- | --- |
| `--listen` | `:8080` | HTTP listen address for the gateway. |
| `--policies` | `./policies/` | Directory containing YAML policy files. |
| `--upstream` | _(Postgres demo DSN)_ | Upstream MCP HTTP URL or Postgres demo DSN. |
| `--config` | _(none)_ | Boundary runtime config file. |
| `--trust-mode` | `disabled` | Trust mode: `disabled`, `standalone`, or `kernel`. |
| `--trust-redis-url` | `redis://localhost:6379` | Redis URL for kernel trust mode. |
| `--require-agent-id` | `false` | Deny protected adapter requests without agent identity. |

`boundary serve` governs only requests that arrive through the served route.
Direct access to the upstream MCP server is a deployment bypass unless network
policy, service mesh rules, or private networking blocks that path. Boundary
enforces policy only for the routed path; it does not govern what arrives at the
upstream directly. See [docs/adapters/MCP.md](./adapters/MCP.md) for the full
adapter lifecycle, bypass model, and policy shape.

## 14. Audit Command

```bash
boundary audit --file decision-records.jsonl
boundary audit --file decision-records.jsonl --filter-action deny
boundary audit --file decision-records.jsonl --filter-agent my-agent
boundary audit --file decision-records.jsonl --filter-tool create_or_update_file
# stdin: omit --file to read from stdin
boundary audit < decision-records.jsonl
```

`boundary audit` (Local-only) pretty-prints structured decision records. It
reads newline-delimited JSON records (JSONL) — one compact JSON object per line
— from a file (`--file`) or from stdin when `--file` is omitted. Each record
is rendered as a color-coded one-line summary: action (DENY in red, ALLOW in
default), tool name, agent ID, matched rule, reason, and request ID.

Behavioral observations (from running against fixture and demo logs):

- Accepts both `schema_version "1"` and `schema_version "2"` records in the
  same file.
- The demo `decision-records.jsonl` files produced by `boundary demo
  github-lethal-trifecta --out <dir>` and `boundary demo
  command-secret-exfil --out <dir>` are valid input.
- `docs/examples/decision-record.example.json` and
  `docs/examples/decision-record-v2.example.json` are single-line compact JSON
  and are valid input directly.
- Pretty-printed multi-line JSON (as produced by some tools) is not valid JSONL
  input; compact each record to one line first.
- Empty input produces no output and exits zero.

Flags (transcribed from `boundary audit --help`):

| Flag | Default | Description |
| --- | --- | --- |
| `--file` | _(stdin)_ | Decision record log file; stdin is used when empty. |
| `--filter-action` | _(all)_ | Only show records for this action (e.g. `deny`, `allow`). |
| `--filter-agent` | _(all)_ | Only show records for this `agent_id`. |
| `--filter-tool` | _(all)_ | Only show records for this `tool_name`. |

`audit` is a local read-only display utility. It does not verify hashes (use
`boundary verify-record` for that), does not recompute decisions, and does not
call the network. A `DENY` line in audit output is a record of a decision Boundary
made; it is not proof that the action was blocked — direct access to the same
tool is a bypass a record cannot see. See
[docs/DECISION_RECORDS.md](./DECISION_RECORDS.md).

## 15. Trust Command

```bash
boundary trust show <agent-id>
boundary trust show --redis-url redis://localhost:6379 <agent-id>
boundary trust show --ipc-prefix myprefix <agent-id>
boundary trust reset <agent-id>
```

`boundary trust` (Local-only) inspects or resets trust state for a named agent.
`trust show` queries the configured trust backend and prints the agent's current
trust record; it accepts `--redis-url` to target a kernel-mode Redis backend and
`--ipc-prefix` for IPC prefix override. `trust reset` clears accumulated trust
state for the agent and operates on the in-process standalone backend only — it
accepts no backend flags. These commands are diagnostic utilities; they do not
alter governance policy or affect running pipeline evaluations directly.
