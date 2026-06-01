package explain

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Options configures a single explain run.
type Options struct {
	// Path is the decision-record JSON file to describe. Required.
	Path string
}

// fixedDoesNotProve is the load-bearing limitation footer. It is constant so the
// honesty caveats ship identically in text and JSON and cannot drift per record.
// Phrasing is negation-framed to satisfy the public-language lint and to match
// docs/DECISION_RECORDS.md and docs/RECEIPTS.md.
var fixedDoesNotProve = []string{
	"explain renders a record; it does not verify the record's hashes. Run boundary verify-record to recompute them.",
	"explain does not prove the verdict was correct; it only describes the recorded decision.",
	"explain does not prove enforcement: a deny record is not evidence the action was blocked, and direct access to the same tool is a bypass a record cannot see.",
	"Hashes cover integrity, not authenticity; a hash match does not prove who produced the record.",
	"execution_claim (upstream_called, executed) is an adapter self-report, not corroborated; it does not prove no upstream bytes moved.",
	"topology_profile is asserted, not attested; the record does not verify the deployment matches the named posture.",
}

// Run reads the decision record at opts.Path and returns a boundary.explain.v1
// description of it. It is read-only: it parses the record (V1 or V2 through the
// shared superset struct) and describes the fields it carries. It does not
// evaluate policy, touch the network, mutate anything, or recompute any hash.
func Run(opts Options) (*Result, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, fmt.Errorf("record path is required")
	}
	body, err := os.ReadFile(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("read record: %w", err)
	}
	return describe(body)
}

// describe builds the explain Result from raw decision-record bytes. It is split
// out so tests can describe in-memory records without a temp file.
func describe(body []byte) (*Result, error) {
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(body, &record); err != nil {
		return nil, fmt.Errorf("parse record: %w", err)
	}
	if !governance.SupportedDecisionRecordSchemaVersion(record.SchemaVersion) {
		return nil, fmt.Errorf(
			"unsupported record schema_version %q: explain describes %q and %q records",
			record.SchemaVersion,
			governance.DecisionRecordSchemaVersion,
			governance.DecisionRecordSchemaV2,
		)
	}
	if strings.TrimSpace(record.Action) == "" {
		return nil, fmt.Errorf("record is missing the required action field")
	}

	result := &Result{
		SchemaVersion:       SchemaVersion,
		Status:              "ok",
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		RecordSchemaVersion: record.SchemaVersion,
		RecordID:            record.RecordID,
		Decision: Decision{
			Action:       record.Action,
			Reason:       record.Reason,
			DecisionMode: string(record.DecisionMode),
			MatchedRule:  record.MatchedRule,
			PolicyFile:   record.PolicyFile,
			Tool:         record.Tool,
			Adapter:      string(record.Adapter),
			EventType:    record.EventType,
		},
		Hashes:       describeHashes(record),
		VerifyHint:   "run boundary verify-record <record.json> to recompute and check these hashes; explain does not verify them",
		DoesNotProve: fixedDoesNotProve,
	}

	if record.HasRouteContext() {
		result.RouteContext = describeRouteContext(record)
	}
	return result, nil
}

// describeHashes lists the stable hashes a record can carry, in a fixed order,
// with a one-line description of what each covers. It reports whether each is
// present without recomputing any of them.
func describeHashes(record governance.DecisionRecordV1) []HashDescription {
	return []HashDescription{
		{
			Field:   "decision_hash",
			Value:   record.DecisionHash,
			Present: record.DecisionHash != "",
			Covers:  "the record's own decision-defining fields (record_id, decision_hash, and signature fields are blanked before hashing); covers route-context fields when present. Integrity, not authenticity.",
		},
		{
			Field:   "request_hash",
			Value:   record.RequestHash,
			Present: record.RequestHash != "",
			Covers:  "the canonical governed request. Verifiable only by supplying the request to boundary verify-record --request.",
		},
		{
			Field:   "policy_bundle_hash",
			Value:   record.PolicyBundleHash,
			Present: record.PolicyBundleHash != "",
			Covers:  "the canonical policy bundle the pipeline was configured with. Verifiable only by supplying the bundle to boundary verify-record --policies.",
		},
		{
			Field:   "raw_shape_hash",
			Value:   record.RawShapeHash,
			Present: record.RawShapeHash != "",
			Covers:  "the trimmed raw input bytes on a parse-rejection record, where no governed request was built.",
		},
	}
}

// describeRouteContext maps a record's schema_version "2" fields into the
// explain envelope. It copies; it does not validate the asserted values.
func describeRouteContext(record governance.DecisionRecordV1) *RouteContext {
	rc := &RouteContext{
		AdapterID:       record.AdapterID,
		RouteID:         record.RouteID,
		TopologyProfile: record.TopologyProfile,
	}
	if record.ExecutionClaim != nil {
		rc.ExecutionClaim = &ExecutionClaim{
			UpstreamCalled: record.ExecutionClaim.UpstreamCalled,
			Executed:       record.ExecutionClaim.Executed,
			Source:         record.ExecutionClaim.Source,
		}
	}
	return rc
}
