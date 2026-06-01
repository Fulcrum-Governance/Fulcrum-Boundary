package governance

import "time"

// DecisionRecordSchemaVersion is the schema_version value of a record that
// carries no route-context fields. A record is V1 when it is byte-compatible
// with the original decision-record shape.
const DecisionRecordSchemaVersion = "1"

// DecisionRecordSchemaV2 is the schema_version value emitted when any of the
// additive route-context fields (adapter_id, route_id, topology_profile,
// execution_claim) is populated. V2 is a strict superset of V1: a V1 record is
// simply a V2 record without the route-context fields.
const DecisionRecordSchemaV2 = "2"

// DecisionRecordV1 is the canonical decision record. It is a single versioned
// superset struct: the route-context fields appended at the end are present
// only in schema_version "2" records (all are omitempty), and a record that
// leaves them empty marshals byte-for-byte as a schema_version "1" record, so
// existing V1 records and their decision_hash values remain valid unchanged.
//
// The discriminator is the SchemaVersion field's value, not the Go type. Both
// versions share this one struct and one ComputeDecisionHash code path.
type DecisionRecordV1 struct {
	SchemaVersion       string        `json:"schema_version"`
	EventType           string        `json:"event_type,omitempty"`
	RecordID            string        `json:"record_id"`
	Timestamp           time.Time     `json:"timestamp"`
	BoundaryVersion     string        `json:"boundary_version,omitempty"`
	BoundaryBuildDigest string        `json:"boundary_build_digest,omitempty"`
	Adapter             TransportType `json:"adapter,omitempty"`
	AgentID             string        `json:"agent_id,omitempty"`
	TenantID            string        `json:"tenant_id,omitempty"`
	TraceID             string        `json:"trace_id,omitempty"`
	Tool                string        `json:"tool,omitempty"`
	Action              string        `json:"action"`
	Reason              string        `json:"reason,omitempty"`
	DecisionMode        DecisionMode  `json:"decision_mode,omitempty"`
	MatchedRule         string        `json:"matched_rule,omitempty"`
	PolicyFile          string        `json:"policy_file,omitempty"`
	PolicyBundleHash    string        `json:"policy_bundle_hash,omitempty"`
	RequestHash         string        `json:"request_hash,omitempty"`
	RawShapeHash        string        `json:"raw_shape_hash,omitempty"`
	DecisionHash        string        `json:"decision_hash"`
	TrustScore          float64       `json:"trust_score"`
	TrustState          string        `json:"trust_state,omitempty"`
	Signature           string        `json:"signature,omitempty"`
	SignatureKeyID      string        `json:"signature_key_id,omitempty"`

	// Route-context fields (schema_version "2", strictly additive).
	//
	// These describe the governed route the request traveled. They are
	// descriptive context, not attestation: recording them extends
	// tamper-detection (they are covered by decision_hash) but does not make
	// the deployment posture verified or the adapter self-report corroborated.

	// AdapterID names the adapter that parsed and routed the request.
	// Descriptive only.
	AdapterID string `json:"adapter_id,omitempty"`
	// RouteID names the specific governed route the request traveled.
	// Descriptive only.
	RouteID string `json:"route_id,omitempty"`
	// TopologyProfile is the named deployment posture asserted at emission.
	// Asserted, not attested: the field does not verify that the running
	// deployment matches the named posture.
	TopologyProfile string `json:"topology_profile,omitempty"`
	// ExecutionClaim is the structured form of the adapter's execution
	// self-report. Self-report, not corroborated: recording it explicitly does
	// not make it independently verified.
	ExecutionClaim *ExecutionClaim `json:"execution_claim,omitempty"`
}

// DecisionRecordV2 is the versioned superset record. It is the same Go type as
// DecisionRecordV1; the alias exists so callers can name the version they mean.
// The on-the-wire version is carried in SchemaVersion, not the Go type.
type DecisionRecordV2 = DecisionRecordV1

// ExecutionClaim is the structured form of an adapter's execution self-report.
//
// Every field is a component reporting on itself, not an independently observed
// network fact. UpstreamCalled and Executed are adapter self-reports; nothing
// in the hashed record corroborates them. Treat them as self-attested adapter
// signals, not verifiable properties of the record.
type ExecutionClaim struct {
	// UpstreamCalled is the adapter's self-report of whether it called the
	// upstream tool. Self-report, not corroborated.
	UpstreamCalled bool `json:"upstream_called"`
	// Executed is the adapter's self-report of whether the governed action ran.
	// Self-report, not corroborated.
	Executed bool `json:"executed"`
	// Source names the adapter or surface that produced this self-report, so a
	// reader can see which component is asserting it.
	Source string `json:"source,omitempty"`
}

// HasRouteContext reports whether the record carries any schema_version "2"
// route-context field. It is the discriminator used to decide whether a record
// is emitted as "2" (route-context present) or "1" (byte-compatible with V1).
func (r DecisionRecordV1) HasRouteContext() bool {
	return r.AdapterID != "" || r.RouteID != "" || r.TopologyProfile != "" || r.ExecutionClaim != nil
}
