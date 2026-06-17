# Proof-Receipt Sidecar

The `proof-receipt-v0.1` sidecar is invariant evidence that can be attached to a
Boundary decision record. A checker validates the named invariants (budget,
static-privilege, trust-circuit) against the runtime values and records each
result as a checker-validated invariant line. The sidecar is bound to a specific decision
record by `decision_hash`; it is not embedded in the record and does not change
`decision_hash` or `decision_mode`.

The decision record itself is not validated by a formally verified checker; only
the sidecar's named invariants are.

## Schema (proof-receipt-v0.1)

```json
{
  "receipt_version":    "proof-receipt-v0.1",
  "decision_hash":      "<sha256-hex of the canonical decision record JSON>",
  "checker_id":         "<opaque identifier of the checker build>",
  "checker_build_hash": "<sha256-hex of the checker binary/module>",
  "invariants": [
    {
      "theorem_id":   "THM-BUDGET-LOCAL",
      "predicate":    "spent_before + requested <= limit",
      "inputs_hash":  "sha256:<hex of canonical JSON of the witness inputs>",
      "status":       "ok | fail",
      "values":       { "spent_before": 0, "requested": 10, "limit": 100 }
    }
  ],
  "recorded_at": "<RFC 3339 timestamp>"
}
```

Fields:

| Field | Type | Description |
|---|---|---|
| `receipt_version` | string | Always `"proof-receipt-v0.1"` for this version |
| `decision_hash` | string | SHA-256 hex of the canonical decision record; the binding anchor |
| `checker_id` | string | Identifies the checker build that validated the invariants |
| `checker_build_hash` | string | SHA-256 hex of the checker binary or module for traceability |
| `invariants` | array | One entry per named invariant checked (see below) |
| `recorded_at` | string | RFC 3339 timestamp when the sidecar was generated |

Each invariant entry:

| Field | Type | Description |
|---|---|---|
| `theorem_id` | string | One of `THM-BUDGET-LOCAL`, `THM-PRIVILEGE-STATIC`, `THM-TRUST-TERMINATION` |
| `predicate` | string | Human-readable statement of the invariant |
| `inputs_hash` | string | `"sha256:"` + hex SHA-256 of the RFC 8785 canonical JSON of the witness inputs |
| `status` | string | `"ok"` when the check passed; `"fail"` otherwise |
| `values` | object | Runtime values the check was evaluated against |

## Binding Rule

The sidecar is bound to a decision record by the `decision_hash` field. The
decision record is NOT re-encoded or modified when the sidecar is attached.
`AttachAll` writes the sidecar as a companion object; the record's own
`decision_hash` field and verbatim JSON are unchanged. A verifier can confirm
the binding by re-computing the record's canonical hash and comparing it to
`receipt.decision_hash`.

## Theorem IDs and Predicates

### THM-BUDGET-LOCAL

**Predicate:** `spent_before + requested <= limit`

The budget checker validates that the sum of previously-spent budget and the
requested cost does not exceed the agent's budget limit. This corresponds to the
`Fulcrum.thm_budget_local` Lean theorem (design correspondence). The checker
does not prove termination or accumulation across a session; it validates a
single-request budget assertion.

### THM-PRIVILEGE-STATIC

**Predicate:** `requested_caps ⊆ authorized_caps`

The static-privilege checker validates that every capability in the requested
set is a member of the authorized set. This corresponds to the
`Fulcrum.thm_privilege_static` Lean theorem (design correspondence). The
checker validates the subset relationship at the time of the check; it does not
prove that the authorized set itself is correctly provisioned.

### THM-TRUST-TERMINATION

**Predicate:** `circuit_open iff (alpha+1)*q < p*(alpha+beta+2)`

This is a **circuit-transition consistency check, not a per-decision termination
proof.** The trust-circuit checker validates that the circuit-open/closed state
is consistent with the Beta-distribution parameters `(alpha, beta)` and the
threshold ratio `p/q`. It does not prove absorbing-state convergence,
no-resurrection, or termination in the session-level sense. The correspondence
to `Fulcrum.trust_termination_invariant` is of type `design`.

## What This Does NOT Prove

- The sidecar does not make the attached decision a `` `proved` `` decision. The
  decision record's `decision_mode` field is not changed; Boundary does not emit
  `proved` decisions.
- The sidecar does not prove that the enforcement action is correct or
  deployment-safe — enforcement is the pipeline's responsibility, not the
  checker's.
- The sidecar does not prove resistance to deployment bypass. Bypass resistance
  is a deployment-topology property delegated to the operator.
- The sidecar does not prove the authenticity of the values supplied to the
  checker. The checker validates the predicate; the caller is responsible for
  supplying accurate runtime values.
- The proof receipt is not hash-chained to prior receipts.
- Boundary does not sign the receipt by default. The `checker_build_hash` field
  supports traceability but is not a cryptographic signature over the receipt.
- THM-TRUST-TERMINATION is a circuit-transition consistency check, not a proof
  that the trust circuit eventually terminates or that terminated states are
  absorbing in all deployments.

## See Also

- `docs/PROOF_BOUNDARY.md` — correspondence table and scope boundary
- `governance/proofreceipt/` — Go implementation of the sidecar, checkers, and
  AttachAll binding helper
- `tests/proof_receipt_warehouse_test.go` — end-to-end attach + verify test
