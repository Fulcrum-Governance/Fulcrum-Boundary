// Package explain renders a human-readable and JSON account of a decision
// record. It is read-only: it parses a committed decision-record JSON object and
// describes its fields. It does not evaluate policy, call the network, mutate
// anything, or re-verify the record's hashes. Hash verification stays the
// responsibility of boundary verify-record.
package explain

// SchemaVersion is the stable schema_version of the boundary.explain.v1 JSON
// envelope. It mirrors the boundary.doctor.v1 / boundary.selftest.v1 convention:
// the envelope version is independent of the decision-record schema version it
// describes (which is carried separately in RecordSchemaVersion).
const SchemaVersion = "boundary.explain.v1"

// Result is the boundary.explain.v1 envelope: a stable, read-only description of
// one decision record. Every field is derived from the record on disk; explain
// adds no claims of its own and verifies nothing.
type Result struct {
	SchemaVersion string `json:"schema_version"`
	// Status is "ok" when a record was parsed and described.
	Status string `json:"status"`

	// Boundary's standard local-diagnostic truth flags. explain reads a local
	// file and asserts nothing about a live deployment, so all three are false.
	RequiresCredentials bool `json:"requires_credentials"`
	RequiresNetwork     bool `json:"requires_network"`
	MutatesLiveSystems  bool `json:"mutates_live_systems"`

	// RecordSchemaVersion is the schema_version carried by the described record
	// ("1" without route-context, "2" with route-context). It is the record's
	// own version, distinct from this envelope's SchemaVersion.
	RecordSchemaVersion string `json:"record_schema_version"`
	// RecordID is the record's derived identifier (record_id).
	RecordID string `json:"record_id"`

	// Decision is the decision-defining content of the record: the fields that
	// describe what Boundary decided and why.
	Decision Decision `json:"decision"`

	// RouteContext is populated only for schema_version "2" records. It is
	// descriptive context, not attestation; see the per-field caveats and the
	// DoesNotProve footer.
	RouteContext *RouteContext `json:"route_context,omitempty"`

	// Hashes describes the stable hashes the record carries and exactly what
	// each one covers. explain only describes them; it does not recompute them.
	Hashes []HashDescription `json:"hashes"`

	// VerifyHint is a one-line pointer to the command that actually recomputes
	// the hashes. explain renders; verify-record verifies.
	VerifyHint string `json:"verify_hint"`

	// DoesNotProve is the fixed limitation footer. It is load-bearing honesty
	// copy: integrity is not authenticity, self-reports are not corroborated,
	// and rendering a record proves neither enforcement nor a correct verdict.
	DoesNotProve []string `json:"does_not_prove"`
}

// Decision is the decision-defining content of a record.
type Decision struct {
	// Action is the verdict: allow, deny, warn, escalate, or require_approval.
	Action string `json:"action"`
	// Reason is the human-readable rationale, when the record carries one.
	Reason string `json:"reason,omitempty"`
	// DecisionMode is how the verdict was reached (deterministic or classified).
	// It records how, not whether the verdict was correct.
	DecisionMode string `json:"decision_mode,omitempty"`
	// MatchedRule is the static policy rule that drove the verdict, when one
	// matched.
	MatchedRule string `json:"matched_rule,omitempty"`
	// PolicyFile is the YAML file that supplied the matched rule, when present.
	PolicyFile string `json:"policy_file,omitempty"`
	// Tool is the governed tool name, when present.
	Tool string `json:"tool,omitempty"`
	// Adapter is the transport that carried the request, when present.
	Adapter string `json:"adapter,omitempty"`
	// EventType is the record's event_type (governance_decision by default).
	EventType string `json:"event_type,omitempty"`
}

// RouteContext mirrors the schema_version "2" route-context fields. Each field
// is descriptive; recording it does not make it attested or corroborated.
type RouteContext struct {
	// AdapterID names the adapter that parsed and routed the request.
	// Descriptive only.
	AdapterID string `json:"adapter_id,omitempty"`
	// RouteID names the specific governed route the request traveled.
	// Descriptive only.
	RouteID string `json:"route_id,omitempty"`
	// TopologyProfile is the named deployment posture asserted at emission.
	// Asserted, not attested: it does not verify the deployment matches it.
	TopologyProfile string `json:"topology_profile,omitempty"`
	// ExecutionClaim is the adapter's structured execution self-report.
	// Self-report, not corroborated.
	ExecutionClaim *ExecutionClaim `json:"execution_claim,omitempty"`
}

// ExecutionClaim mirrors the record's execution self-report. Self-report, not
// corroborated.
type ExecutionClaim struct {
	UpstreamCalled bool   `json:"upstream_called"`
	Executed       bool   `json:"executed"`
	Source         string `json:"source,omitempty"`
}

// HashDescription names one stable hash on the record and what it covers. The
// Present flag reports whether the record actually carries the hash; explain
// describes coverage without recomputing it.
type HashDescription struct {
	// Field is the JSON field name on the record (e.g. decision_hash).
	Field string `json:"field"`
	// Value is the stored hash string, when present.
	Value string `json:"value,omitempty"`
	// Present reports whether the record carries this hash.
	Present bool `json:"present"`
	// Covers is a one-line description of exactly what the hash covers.
	Covers string `json:"covers"`
}
