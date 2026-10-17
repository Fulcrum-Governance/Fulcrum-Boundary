# Lean proof files (reference copy)

This folder is a verbatim copy of the Lean files and theorem inventory that
`docs/PROOF_BOUNDARY.md` cites. They come from the Fulcrum-Proofs repository.

- Source repository: `Fulcrum-Governance/Fulcrum-Proofs`
- Source commit: `30e40ffe8ab533d913a6c4691a2be9a48ea131fb`
- License: MIT. The license text is in `LICENSE` in this folder.

| File here | Path at the source commit |
|---|---|
| `lean/Proofs/BasicInvariants.lean` | `proofs/lean/Proofs/BasicInvariants.lean` |
| `lean/Proofs/TrustTermination.lean` | `proofs/lean/Proofs/TrustTermination.lean` |
| `lean/Proofs/GameTheory/CoordinationEfficiency.lean` | `proofs/lean/Proofs/GameTheory/CoordinationEfficiency.lean` |
| `lean/Proofs/GameTheory/CoordinationEfficiencyExact.lean` | `proofs/lean/Proofs/GameTheory/CoordinationEfficiencyExact.lean` |
| `lean/Proofs/GameTheory/NashExistence.lean` | `proofs/lean/Proofs/GameTheory/NashExistence.lean` |
| `lean/Proofs/GameTheory/MixedNashExistence.lean` | `proofs/lean/Proofs/GameTheory/MixedNashExistence.lean` |
| `theorem_inventory.yaml` | `claims/theorem_inventory.yaml` |

The files are not edited. Boundary does not build or check them: there is no
Lean toolchain in Boundary CI, and some files import modules that are not
copied here. They are kept so readers can see the theorem statements that
`docs/PROOF_BOUNDARY.md` refers to.
