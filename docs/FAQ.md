# FAQ

A skeptic's FAQ for Fulcrum Boundary. Every answer is bound to a canonical
document in this repo; follow the link to read the authoritative version and its
limitations. If an answer here ever drifts from its cited doc, the cited doc
wins.

## What does "routed-only" actually mean, and how do I close the unrouted paths?

Boundary governs an action only when the route is forced through Boundary. It
does not control direct shell, editor, filesystem, CI, SSH, or API access to the
same tool unless your deployment topology removes that direct path. A direct path
to the same tool is a bypass, not a thing Boundary missed. Closing it is a
deployment-topology job: make the Boundary-fronted route the only path to the
tool (network policy, credential custody, removing the direct binary/socket),
then confirm it with the route-conformance checklist. A passing checklist
confirms the route is forced through Boundary in your deployment; it does not
prove that no other path to the same tool exists.

Canonical: [`LIMITATIONS.md`](../LIMITATIONS.md),
[`docs/ROUTE_CONFORMANCE_CHECKLIST.md`](ROUTE_CONFORMANCE_CHECKLIST.md).

## Why is the flagship demo "fixture-only"? Does it prove the product works in production?

The two proof lanes (`boundary demo github-lethal-trifecta` and
`boundary demo command-secret-exfil`) run against committed synthetic fixtures.
They use no credentials, make no network calls, and perform no live mutation —
that is the point: they demonstrate the deny-before-execution decision
deterministically and offline. A passing demo shows the decision and the
hash-verifiable record; it does not prove that every deployment route is
protected, and it is not a live-traffic benchmark. The Secure GitHub profile
denies the tested write-after-taint fixture before upstream mutation; that
fixture proof does not close deployment bypasses.

Canonical: [`LIMITATIONS.md`](../LIMITATIONS.md),
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

## What does "hash-verifiable" mean — and what does it *not* mean?

A decision record is receipt-grade when it carries the request, policy-bundle,
and decision hashes. Tampering after emission is detectable by recomputing those
hashes with `boundary verify-record`. The hashes are unkeyed SHA-256 over
canonical bytes: this is integrity, not authenticity. "Hash-verifiable" here does
not mean the record is proved, tamper-proof, immutable, or attested. The hash
is unkeyed, so anyone can edit a record and recompute a new internally consistent
hash — a passing check does not prove **who** produced the record. Signatures are
off by default. A passing check is not evidence that the action was executed
or prevented, and `upstream_called=false` / `executed=false` are adapter
self-reports, not independently observed network facts.

Canonical: [`docs/RECEIPTS.md`](RECEIPTS.md),
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

## Is the decision record "standards-conformant"?

Scoped to the decision record specifically, yes: the record's canonical bytes are
hashed in RFC 8785 / JCS form, and `decision_hash` is the SHA-256 of that
canonical form. That scoped, record-level statement is the only conformance claim
made — it is not a claim that Boundary as a whole is standards-conformant.
Because the canonical form is a published standard, the record's `decision_hash`
is reproducible by a stock RFC 8785 implementation.

Canonical: [`verifiers/python/README.md`](../verifiers/python/README.md).

## Where do decision records live, and how do I verify one offline?

A record-emitting command prints `decision record path: <path>` for the
single-record JSON object, and `decision record log: <path>` for the multi-record
JSONL stream. The two demos write their JSON under `--out` at a predictable
`*-artifacts/decision-record.json` location. You can verify a record two ways,
both shipping on `main`: the Go binary (`boundary verify-record <record.json>`),
or the stock RFC 8785 Python verifier (`pip install rfc8785;
python3 verifiers/python/boundary_verify.py record.json`). Both recompute the
same `decision_hash`. The printed paths are local-only file locations; they are
not network endpoints and do not prove the action was enforced.

Canonical: [`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md),
[`verifiers/python/README.md`](../verifiers/python/README.md).

## Why is MCP the only "production" route, and what does "preview" mean?

A route earns the `production` label only by passing the adapter-readiness gate:
non-stub lifecycle steps, a `bypass_proof` step that is implemented or formally
delegated, at least one fail-closed transport, and test evidence on disk. Today
exactly one adapter clears that bar: `adapters/mcp`. "Preview" means the surface
is delivered and labeled but has not earned the production label — its gaps are
listed in the ledger and readiness files. Command Boundary and Edit Boundary are
delivered previews for routed command paths and routed edit envelopes; A2A, CLI,
CodeExec, gRPC, Managed Agents, Secure GitHub, and Webhook are labeled previews.
Check `adapters/<name>/readiness.yaml` and the readiness matrix before describing
any adapter as production.

Canonical: [`docs/ADAPTER_READINESS_MATRIX.md`](ADAPTER_READINESS_MATRIX.md),
[`docs/HOW_WE_KEEP_OURSELVES_HONEST.md`](HOW_WE_KEEP_OURSELVES_HONEST.md).

## Why does the build need cgo / a C compiler? What does `CGO_ENABLED=0` do?

The bundled Postgres AST guard (`interceptors/sql`) links `pg_query_go`, a cgo
binding to libpg_query. The default build therefore needs a working C compiler,
and **`CGO_ENABLED=0` builds fail** — there is no pure-Go build path that keeps
the Postgres AST guard. With cgo disabled the classifier's parse entry point is
not generated, so the build stops with an `undefined: pg_query.Parse` error. This
is a separate concern from the classifier's runtime fail-safe: at runtime, empty,
invalid, or unparsable SQL is classified `UNKNOWN` and denied fail-closed. That
fail-closed `UNKNOWN` behavior is on `main`. The classifier is a Postgres AST
guard for routed requests; it is not a general SQL firewall and does not
prevent all SQL injection.

Canonical: [`docs/TROUBLESHOOTING.md`](TROUBLESHOOTING.md),
[`docs/policies/POSTGRES.md`](policies/POSTGRES.md).

## Is there a model / LLM in the verdict path?

No. The local verdict path is deterministic: trust check, static policies, domain
interceptors, then the portable PolicyEval engine. No model runs to decide
allow/deny in the standalone path. Semantic policy rules do not silently pass or
guess — they escalate (`escalate` / `require_approval`) so a human or upstream
system adjudicates. `decision_mode` on a record is `deterministic` or
`classified`; Boundary does not emit `proved`.

Canonical: [`ARCHITECTURE.md`](../ARCHITECTURE.md),
[`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

## What is the difference between standalone and kernel mode?

Standalone is the zero-dependency OSS path in this repo: YAML policies,
in-process trust (a Beta evaluator), optional in-process budget tracking, and
structured JSON audit via slog. Kernel mode is the commercial Fulcrum
integration: Redis / NATS / Fulcrum API supply policy, trust, budget, escalation,
audit, and envelope lifecycle. Kernel mode fails hard on incomplete
configuration by design — Boundary should not start with a half-declared kernel
connection, because that can turn an intended fail-closed deployment into a
silently local one. Do not expect hosted/control-plane behavior from the
standalone path.

Canonical: [`docs/STANDALONE_VS_KERNEL.md`](STANDALONE_VS_KERNEL.md).

## Does Boundary emit "proved" decisions backed by the Lean proofs?

No. Boundary uses proof correspondence as a design constraint, not as a runtime
certificate, and does not emit `proved` decisions. Correspondence type
`design` means the runtime behavior was designed to satisfy a proved invariant;
it does not mean the Go implementation was mechanically extracted from Lean.
Runtime proof-backed decisions belong to upstream Fulcrum components that
actually discharge or attach formal evidence. The proof lineage does not
prove that every deployment is safe.

Canonical: [`docs/PROOF_BOUNDARY.md`](PROOF_BOUNDARY.md).

## What does `boundary replay` prove, and what does it not prove?

`replay` re-evaluates a recorded request against a supplied static policy bundle
in a hermetic in-process run and compares the decision-defining fields (`action`,
`reason`, `decision_mode`, `matched_rule`, `policy_file`), not `action` alone. It
reproduces the **decision**, not enforcement. A reproduced `deny` is not
evidence the action was blocked; replay does not prove that no upstream bytes
moved; and a match does not prove the original verdict was correct — only
that the same inputs reproduce the same decision. Replay re-evaluates the static
policy bundle only, so a decision that originated from an interceptor (such as the
Postgres AST classifier) or from trust state does not reproduce, and replay
reports a mismatch rather than a false reproduction.

Canonical: [`docs/DECISION_RECORDS.md`](DECISION_RECORDS.md).

## How does the claims gate keep the docs honest? Can I run it myself?

Two build gates enforce honesty mechanically. The claims-ledger gate
(`claims/claims_test.go`) requires every `delivered` claim to cite at least one
test path and one doc path that exist on disk, every `partial` claim to list
structured gaps, and every `false` claim to be absent from `README.md`. The
language-lint gate (`claims/language_lint_test.go`) fails the build if a
controlled overclaim phrase appears on a non-negated, non-limitation-framed line
in a scanned public doc; the controlled-phrase list lives in the test itself, so
the doc you are reading is linted against it too. None of this
runs on trust — it is an exit code. Clone the repo and run it yourself:

```bash
go test ./claims/... -count=1
make release-check
```

Canonical: [`docs/HOW_WE_KEEP_OURSELVES_HONEST.md`](HOW_WE_KEEP_OURSELVES_HONEST.md),
[`docs/CLAIMS_LEDGER.md`](CLAIMS_LEDGER.md).

## What is `boundary doctor` / `boundary evidence` good for?

They are local-only diagnostics. `boundary doctor` checks the toolchain (Go
version, cgo / C-toolchain readiness, `PATH` resolution after `go install`).
`boundary evidence bundle` / `boundary evidence verify` package and re-check a
local evidence bundle. A passing doctor or a verified evidence bundle is useful
for confirming a clean local install and reproducible artifacts; it does not
prove that every deployment route is protected or that no bytes moved outside
Boundary.

Canonical: [`docs/TROUBLESHOOTING.md`](TROUBLESHOOTING.md),
[`LIMITATIONS.md`](../LIMITATIONS.md).

## Where is the single source of truth for what is shipped vs planned?

Release truth lives in the README "Current Release Truth" / "Adapter Readiness"
tables, the machine-readable [`claims/`](../claims/) ledger (rendered at
[`docs/CLAIMS_LEDGER.md`](CLAIMS_LEDGER.md)), [`LIMITATIONS.md`](../LIMITATIONS.md),
and each `adapters/<name>/readiness.yaml`. Phase-level shipped-on-main vs planned
status is tracked in the roadmap. Verify against those before trusting any
"covers / handles / prevents" statement — including the ones on this page.

Canonical: [`docs/BOUNDARY_ROADMAP.md`](BOUNDARY_ROADMAP.md),
[`docs/CLAIMS_LEDGER.md`](CLAIMS_LEDGER.md).
