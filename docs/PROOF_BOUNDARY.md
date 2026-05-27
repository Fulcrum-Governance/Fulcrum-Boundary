# Proof Boundary

Boundary uses proof correspondence as a design constraint, not as a runtime
certificate. Boundary does not emit `proved` decisions. Runtime proof-backed
decisions belong to upstream Fulcrum components that actually discharge or
attach formal evidence.

Correspondence type `design` means the runtime behavior was designed to satisfy
the proved invariant. It does not mean the Go implementation was mechanically
extracted from Lean.

| Boundary behavior | Lean 4 theorem | Repo path | Correspondence |
|---|---|---|---|
| Budget check denies when remaining budget is below requested cost | `Fulcrum.budget_safety_guarantee` | `Fulcrum-Proofs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Local budget invariant wrapper | `Fulcrum.thm_budget_local` | `Fulcrum-Proofs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Trust below theta triggers isolation or termination behavior | `Fulcrum.trust_termination_invariant` | `Fulcrum-Proofs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Repeated failures eventually cross the trust threshold | `Fulcrum.trust_guaranteed_termination` | `Fulcrum-Proofs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Failure accumulation monotonically degrades trust | `Fulcrum.trust_failure_degrades` | `Fulcrum-Proofs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Terminated circuit state is absorbing | `Fulcrum.terminated_is_absorbing` | `Fulcrum-Proofs/proofs/lean/Proofs/TrustTermination.lean` | design |
| Child or requested privileges remain a subset of available privileges | `Fulcrum.thm_privilege_static` | `Fulcrum-Proofs/proofs/lean/Proofs/BasicInvariants.lean` | design |
| Constrained budget game has exact PoA 1 under theorem assumptions | `Fulcrum.GameTheory.constrained_poa_exact` | `Fulcrum-Proofs/proofs/lean/Proofs/GameTheory/CoordinationEfficiency.lean` | design |
| Fulcrum coordination game has a pure Nash equilibrium under theorem assumptions | `Fulcrum.GameTheory.fulcrum_pure_nash_exists` | `Fulcrum-Proofs/proofs/lean/Proofs/GameTheory/NashExistence.lean` | design |
| Finite normal-form game has a mixed Nash equilibrium | `Fulcrum.GameTheory.mixed_nash_exists` | `Fulcrum-Proofs/proofs/lean/Proofs/GameTheory/MixedNashExistence.lean` | design |

## Scope Boundary

The proof lineage constrains the shape of Boundary's runtime behavior:

- trust thresholds use the same 0.30 isolation and 0.60 degraded bands as the Fulcrum router;
- repeated failures degrade trust instead of improving it;
- isolated or terminated agents are denied before protected tool execution;
- budget and privilege claims stay scoped to the invariant being enforced.

The proof lineage does not prove that every deployment is safe. Deployment
isolation, credential custody, policy quality, live service availability, and
operator configuration are still operational responsibilities.
