# Claims Audit Report

Date: 2026-05-27

Audited commit: `5cda5d4b17202c00216081812a3baf2ea1981b94`

Branch: `audit/claims-evidence`

## Summary

The evidence audit found no missing cited evidence paths and no unsupported
`delivered` claims. No claim was downgraded.

The audit did find a language split issue: BND-CLAIM-002, the structured
decision-record claim, included hash and receipt-adjacent language that belongs
to BND-CLAIM-005. This PR moves that language boundary back into the claim
ledger and softens public README wording around preview adapters.

## Test Results

| Command | Result |
|---|---|
| `go test ./... -count=1 -timeout 5m` | Pass |
| `(cd adapters/grpc && go test ./... -count=1 -timeout 5m)` | Pass |
| `go test ./tests/... -count=1 -timeout 5m` | Pass |
| `go test ./claims/... -count=1 -timeout 5m` | Pass |
| `go run ./cmd/boundary verify --policies examples/mcp-postgres-gateway/policies` | Pass: `policy files: 1`, `rules: 5`, `warnings: 0` |
| `go run ./cmd/boundary verify-record --help` | Pass |

No required test command reported skipped tests. Some Go packages reported
`[no test files]`, which is normal package inventory, not a skipped test.

## Per-Claim Audit

| claim_id | status_in_file | evidence_exists | tests_pass | language_clean | final_status |
|---|---|---|---|---|---|
| BND-CLAIM-001 | delivered | Yes | Yes | Yes | delivered |
| BND-CLAIM-002 | delivered | Yes | Yes | Fixed in this PR | delivered |
| BND-CLAIM-003 | partial | Yes | Yes | Fixed README maturity wording | partial |
| BND-CLAIM-004 | false | Yes | Yes | Yes | false |
| BND-CLAIM-005 | delivered | Yes | Yes | Yes | delivered |
| BND-CLAIM-006 | delivered | Yes | Yes | Yes | delivered |
| BND-CLAIM-007 | partial | Yes | Yes | Fixed README maturity wording | partial |
| BND-CLAIM-008 | delivered | Yes | Yes | Yes | delivered |
| BND-CLAIM-009 | delivered | Yes | Yes | Yes | delivered |
| BND-CLAIM-010 | delivered | Yes | Yes | Yes | delivered |

## Evidence Checks

Every `evidence.tests[].path` and `evidence.docs[].path` in
`claims/boundary_claims.yaml` exists. Every `delivered` claim has at least one
test path and one doc path. Every `partial` claim has at least one gap with a
linked task ID.

## Downgrades

None.

No claim marked `delivered` depended on missing, skipped, failing, or live-only
evidence.

## Claims Passing All Checks

- BND-CLAIM-001
- BND-CLAIM-002 after the language split fix
- BND-CLAIM-005
- BND-CLAIM-006
- BND-CLAIM-008
- BND-CLAIM-009
- BND-CLAIM-010

## Partial Claims and Live Evidence

At the audited commit, BND-CLAIM-003 remained partial because A2A was still
experimental and Managed Agents production status required live upstream
conformance evidence. Later adapter PRs may update the live readiness matrix;
this report records the Step 2 audit state.

BND-CLAIM-007 remains partial because Managed Agents is preview until a live
upstream Anthropic Managed Agents conformance run is recorded with
operator-owned credentials.

## Language Split Fix

BND-CLAIM-002 is now limited to structured decision record emission:

- Every governed verdict produces a structured decision record.
- Decision records include action, reason, decision mode, matched rule, request
  ID, envelope ID, adapter, tenant, and agent context when available.

BND-CLAIM-005 remains the owner of receipt-grade wording:

- Boundary produces receipt-grade decision records with request, policy bundle,
  and decision hashes.

The YAML and claims ledger were updated so BND-CLAIM-002 no longer carries hash,
verification, receipt-grade, tamper, signature, or cryptographic language.

## Public Language Cleanup

README language was updated to:

- describe the first release as demonstrating the demo network bypass boundary,
  rather than broadly proving bypass resistance;
- describe non-MCP adapters as packages with maturity tracked per adapter;
- keep Managed Agents in preview and name live upstream conformance as the
  production gate.

## Scope Notes

Step 2 allowed edits to `claims/boundary_claims.yaml`,
`docs/CLAIMS_AUDIT_REPORT.md`, `docs/CLAIMS_LEDGER.md`, and `README.md` when
public-language cleanup was required. The following read-only findings were
not edited in this PR because they fall outside that Step 2 allowed-path list
and are better handled by adapter-specific or final reconciliation steps:

- `docs/LAUNCH_TRUTH_FREEZE.md` could more explicitly attribute structured
  decision records to BND-CLAIM-002 and receipt-grade hashes to BND-CLAIM-005.
- `docs/adapters/MANAGED_AGENTS.md` should move the preview/live-conformance
  caveat into the opening paragraph during the Managed Agents conformance work.
- `docs/RECEIPTS.md` uses "prove" when describing parse-rejected records; a
  later receipt-doc cleanup can soften that to "show" or "document".

## Claim Validation File Changes

`claims/claims_test.go` was not changed.
