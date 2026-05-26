# Claims Ledger

This ledger is the Boundary-specific extension to Fulcrum's broader claims-lock
discipline. It binds public language to repo evidence so release notes, README
copy, and demo language do not outrun what the Boundary code and docs prove.

The machine-readable source is [`claims/boundary_claims.yaml`](../claims/boundary_claims.yaml).
The release gate in [`claims/claims_test.go`](../claims/claims_test.go) parses
that file and validates the evidence rules.

## Status Vocabulary

| Status | Meaning |
|---|---|
| `delivered` | May be used in release notes when the claim has at least one test path and one doc path. |
| `partial` | May be used only with maturity or gap caveats. Each partial claim must name linked build tasks. |
| `planned` | Roadmap only. Do not state as current behavior. |
| `false` | Do not use as a public claim. The validation gate checks that the claim text is absent from `README.md`. |

## Current Claims

| ID | Status | Claim | Evidence | Public boundary |
|---|---|---|---|---|
| BND-CLAIM-001 | delivered | Boundary governs MCP Safety Gateway requests before execution when the tool route passes through Boundary. | `internal/boundarycli/cli_test.go`, `docs/BOUNDARY_CONDITIONS.md`, `docs/LAUNCH_TRUTH_FREEZE.md` | Scoped to routed deployments and the Docker demo topology. |
| BND-CLAIM-002 | delivered | Boundary emits structured decision records for every governed verdict. | `governance/slog_audit_test.go`, `docs/DECISION_RECORDS.md` | Decision records are logs, not receipt-grade cryptographic artifacts. |
| BND-CLAIM-003 | partial | Boundary ships six transport adapter packages with mixed maturity. | Adapter package tests, `docs/ADAPTER_READINESS_MATRIX.md` | Use the maturity matrix. Do not call the six adapters production-grade. |
| BND-CLAIM-004 | false | Boundary is a SQL firewall. | `docs/LAUNCH_TRUTH_FREEZE.md` | The launch policy is demo-grade destructive-action blocking via string matching. |
| BND-CLAIM-005 | false | Boundary produces receipt-grade decision records. | `docs/LAUNCH_TRUTH_FREEZE.md` | Receipt-grade records are a future spec, not v0.2.0 behavior. |

## Release Rule

Release notes can only make uncaveated behavior claims whose status is
`delivered`. Partial claims must carry the gap language from the YAML ledger.
False claims must not appear in `README.md`, release notes, or launch copy.
