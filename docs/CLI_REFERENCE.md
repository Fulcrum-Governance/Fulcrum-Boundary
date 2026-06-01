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

## 1. First-Run Commands

Run this exact sequence after install. It is the canonical first-run path and
matches the [README](../README.md) quickstart; the demos appear in the same
order there.

```bash
go install github.com/fulcrum-governance/fulcrum-boundary/cmd/boundary@v0.7.0
boundary selftest                                            # Local-only smoke test
boundary doctor --json                                       # Local-only diagnostics + bypass caveats
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
and bypass caveats for MCP, Command Boundary, and Edit Boundary. It does not call
the network. Doctor output is local diagnostics, not proof that every deployment
route is protected. See [docs/DOCTOR.md](./DOCTOR.md).

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
boundary demo postgres --gateway http://localhost:8080/mcp
boundary demo trust-degradation
```

The Command Boundary secret-exfil demo wraps the `command-secret-exfil`
fixture red-team pack: an untrusted task proposes posting a secret-looking
environment file with `curl`, Boundary classifies it as Class C6 and denies it
before execution (`executed=false`), and a decision record is emitted. It reads
no real `.env`, makes no network call, and executes nothing.

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
boundary evidence bundle --include-demo --out /tmp/boundary-evidence
boundary evidence verify /tmp/boundary-evidence
```

These checks keep public language, claims, docs, examples, and release gates in
sync before shipping a Boundary release branch.

## 9. Decision-Record Verification Commands

```bash
boundary verify-record record.json
boundary verify-record --binary-digest fixture-only record.json
```

`boundary verify-record` (Local-only) reads one `DecisionRecordV1` JSON object,
recomputes its stable hashes, and confirms `schema_version` and the
self-`decision_hash` match. With no flags it confirms only that the record is
internally hash-consistent and unmodified since emission.

The first-run demos and evidence steps can print a decision-record path. The
fixture-safe way to produce a committable record is:

```bash
boundary demo github-lethal-trifecta --json --out demo.json
# writes github-lethal-trifecta-artifacts/decision-records.jsonl (one record per line)
```

Split that JSONL into one object per file, then run bare `verify-record` on a
single record. The default demo without `--out` does not retain a workspace, so
the records are printed but not persisted.

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
