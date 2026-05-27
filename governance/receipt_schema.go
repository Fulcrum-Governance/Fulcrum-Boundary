package governance

import "time"

const DecisionRecordSchemaVersion = "1"

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
}
