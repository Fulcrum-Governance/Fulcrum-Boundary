# Proof Boundary

Boundary uses proof correspondence as a design constraint, not as a runtime
certificate. Boundary does not emit `proved` decisions. Boundary has no runtime proof-backed decisions. The Lean sources cited below are a reference copy in `docs/proofs/`, taken from Fulcrum-Proofs at `30e40ff`, where they were machine-checked.

Correspondence type `design` means the runtime behavior was designed to satisfy
the proved invariant. It does not mean the Go implementation was mechanically
extracted from Lean.

| Boundary behavior | Lean 4 theorem | Repo path | Correspondence |
|---|---|---|---|
| Budget check denies when remaining budget is below requested cost | `Fulcrum.budget_safety_guarantee` | `docs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Local budget invariant wrapper | `Fulcrum.thm_budget_local` | `docs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Trust below theta triggers isolation or termination behavior | `Fulcrum.trust_termination_invariant` | `docs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Repeated failures eventually cross the trust threshold | `Fulcrum.trust_guaranteed_termination` | `docs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Failure accumulation monotonically degrades trust | `Fulcrum.trust_failure_degrades` | `docs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Terminated circuit state is absorbing | `Fulcrum.terminated_is_absorbing` | `docs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Child or requested privileges remain a subset of available privileges | `Fulcrum.thm_privilege_static` | `docs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Constrained budget game has exact PoA 1 under theorem assumptions | `Fulcrum.GameTheory.constrained_poa_exact` | `docs/proofs/lean/Proofs/GameTheory/CoordinationEfficiencyExact.lean` | design |
| Fulcrum coordination game has a pure Nash equilibrium under theorem assumptions | `Fulcrum.GameTheory.fulcrum_pure_nash_exists` | `docs/proofs/lean/Proofs/GameTheory/NashExistence.lean` | design |
| Finite normal-form game has a mixed Nash equilibrium | `Fulcrum.GameTheory.mixed_nash_exists` | `docs/proofs/lean/Proofs/GameTheory/MixedNashExistence.lean` | design |

The copy is for reading and citation. It is not built or checked in this repo's CI.

## Proof-receipt sidecar (proof-receipt-v0.1)

Boundary can attach a checker-validated proof-receipt sidecar for the budget and
static-privilege invariants, bound to the decision record by `decision_hash`.
The sidecar is invariant evidence attached to a decision, not a `` `proved` ``
`decision_mode`, and Boundary does not emit `proved` decisions.

The decision record itself is not validated by a formally verified checker; only
the sidecar's named invariants are. The three theorem IDs and their predicates:

| Theorem ID | Predicate | Note |
|---|---|---|
| `THM-BUDGET-LOCAL` | `spent_before + requested <= limit` | Single-request budget assertion |
| `THM-PRIVILEGE-STATIC` | `requested_caps ⊆ authorized_caps` | Subset check at time of evaluation |
| `THM-TRUST-TERMINATION` | `circuit_open iff (alpha+1)*q < p*(alpha+beta+2)` | Circuit-transition consistency check, not a per-decision termination proof |

The proof receipt is not hash-chained, and Boundary does not sign the receipt by
default. See `docs/PROOF_RECEIPT.md` for the full schema, binding rule, and "What
This Does NOT Prove" subsection.

## Scope Boundary

The proof lineage constrains the shape of Boundary's runtime behavior:

- trust thresholds use the same 0.30 isolation and 0.60 degraded bands as the Fulcrum router;
- repeated failures degrade trust instead of improving it;
- isolated or terminated agents are denied before protected tool execution;
- budget and privilege claims stay scoped to the invariant being enforced.

The proof lineage does not prove that every deployment is safe. Deployment
isolation, credential custody, policy quality, live service availability, and
operator configuration are still operational responsibilities.
