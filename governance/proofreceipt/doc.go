// Package proofreceipt defines the proof-receipt-v0.1 sidecar: checker-
// validated invariant evidence attached to a Boundary decision record and
// bound to it by decision_hash. The sidecar is evidence, NOT a decision mode:
// it carries no decision_mode field, and Boundary standalone never emits a
// `proved` decision (see governance/decision_mode.go and the
// isAdoptableEscalationMode allow-set in governance/pipeline.go). The receipt
// attaches alongside DecisionRecordV1/V2 without re-encoding it, so it does not
// change decision_hash (see docs/PROOF_BOUNDARY.md and
// Fulcrum/docs/schemas/evidence-warehouse-v0.1.md, the verbatim 4.1 format law).
package proofreceipt
