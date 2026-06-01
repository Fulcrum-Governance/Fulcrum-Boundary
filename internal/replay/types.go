// Package replay re-evaluates a recorded governed request locally and compares
// the reproduced verdict against the decision a record carries. It is read-only
// and fixture-safe: it parses a committed decision record plus the canonical
// GovernanceRequest that was recorded, recomputes the request and policy-bundle
// hashes, rebuilds the request, runs it through the same governance pipeline in
// a hermetic in-process configuration with no audit side effects, and reports
// whether the decision-defining fields match.
//
// Replay reproduces the *decision*, not enforcement and not the absence of
// upstream side effects. A match proves only that the same inputs reproduce the
// same decision for a routed request; it does not prove the original verdict was
// correct, that the action was blocked, or that no upstream bytes moved. Direct
// access to the same tool is a bypass a record cannot see.
package replay

// SchemaVersion is the stable schema_version of the boundary.replay.v1 JSON
// envelope. It mirrors the boundary.doctor.v1 / boundary.selftest.v1 /
// boundary.explain.v1 convention: the envelope version is independent of the
// decision-record schema version it replays (carried separately in
// RecordSchemaVersion).
const SchemaVersion = "boundary.replay.v1"

// Result is the boundary.replay.v1 envelope: the outcome of re-evaluating one
// recorded request against the recorded policy bundle. Every field is derived
// locally; replay asserts nothing about a live deployment.
type Result struct {
	SchemaVersion string `json:"schema_version"`
	// Status is "ok" when the recorded request reproduced the recorded decision
	// and every checked hash matched; "mismatch" when any decision-defining
	// field or hash differed. A non-ok Status corresponds to a non-zero exit.
	Status string `json:"status"`
	// Matched reports whether replay reproduced the recorded decision and every
	// requested hash check passed. It is the single boolean the exit code keys on.
	Matched bool `json:"matched"`

	// Boundary's standard local-diagnostic truth flags. Replay rebuilds and
	// re-evaluates a request in-process against a local policy directory; it
	// uses no credentials, no network, and mutates no live system, so all three
	// are false.
	RequiresCredentials bool `json:"requires_credentials"`
	RequiresNetwork     bool `json:"requires_network"`
	MutatesLiveSystems  bool `json:"mutates_live_systems"`

	// RecordSchemaVersion is the schema_version carried by the replayed record
	// ("1" without route-context, "2" with route-context). Route-context fields
	// are not decision-defining and are not re-evaluated; replay compares the
	// verdict, not the asserted posture.
	RecordSchemaVersion string `json:"record_schema_version"`
	// RecordID is the record's derived identifier (record_id).
	RecordID string `json:"record_id,omitempty"`

	// HashChecks reports the outcome of each hash gate replay ran (request_hash
	// always; policy_bundle_hash only when the record carries one). A failed
	// gate sets Matched false and Status "mismatch".
	HashChecks []HashCheck `json:"hash_checks"`

	// Recorded is the decision-defining content as the record carries it.
	Recorded Decision `json:"recorded"`
	// Reproduced is the decision-defining content produced by re-evaluating the
	// recorded request through the pipeline.
	Reproduced Decision `json:"reproduced"`
	// FieldChecks reports, per decision-defining field, whether the recorded and
	// reproduced values agree. Replay compares action, reason, decision_mode,
	// matched_rule, and policy_file (the last two only when the record carries
	// them) — never action alone.
	FieldChecks []FieldCheck `json:"field_checks"`

	// Mismatches lists, in human-readable form, every gate or field that did not
	// match. Empty on a successful replay.
	Mismatches []string `json:"mismatches,omitempty"`

	// DoesNotProve is the fixed limitation footer. It is load-bearing honesty
	// copy: replay reproduces the decision, not enforcement, and a match does
	// not prove the verdict was correct or that no upstream bytes moved.
	DoesNotProve []string `json:"does_not_prove"`
}

// Decision is the decision-defining content compared on each side of a replay.
type Decision struct {
	// Action is the verdict: allow, deny, warn, escalate, or require_approval.
	Action string `json:"action"`
	// Reason is the human-readable rationale.
	Reason string `json:"reason,omitempty"`
	// DecisionMode is how the verdict was reached (deterministic or classified).
	DecisionMode string `json:"decision_mode,omitempty"`
	// MatchedRule is the static policy rule that drove the verdict, when one
	// matched.
	MatchedRule string `json:"matched_rule,omitempty"`
	// PolicyFile is the YAML file that supplied the matched rule, when present.
	PolicyFile string `json:"policy_file,omitempty"`
}

// HashCheck records the outcome of one hash gate: the field compared, the value
// the record carries, the value replay recomputed, and whether they matched.
type HashCheck struct {
	// Field is the record field the gate compares (request_hash or
	// policy_bundle_hash).
	Field string `json:"field"`
	// Recorded is the value stored on the record.
	Recorded string `json:"recorded"`
	// Recomputed is the value replay derived from the supplied input.
	Recomputed string `json:"recomputed"`
	// Matched reports whether the two values are equal.
	Matched bool `json:"matched"`
}

// FieldCheck records whether one decision-defining field matched between the
// recorded and reproduced decisions.
type FieldCheck struct {
	// Field names the decision-defining field (action, reason, decision_mode,
	// matched_rule, or policy_file).
	Field string `json:"field"`
	// Recorded is the value on the record.
	Recorded string `json:"recorded"`
	// Reproduced is the value re-evaluation produced.
	Reproduced string `json:"reproduced"`
	// Matched reports whether the two values are equal.
	Matched bool `json:"matched"`
}
