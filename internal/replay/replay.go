package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Options configures a single replay run.
type Options struct {
	// RecordPath is the decision-record JSON file to reproduce. Required.
	RecordPath string
	// RequestPath is the canonical GovernanceRequest JSON that was recorded.
	// Required: a V1/V2 record carries request_hash but not the request body,
	// so the request must be supplied to rebuild and re-evaluate it.
	RequestPath string
	// PolicyDir is the operator's policy directory. Required: replay re-evaluates
	// against the recorded policy bundle, and when the record carries a
	// policy_bundle_hash it is recomputed from this directory and must match.
	PolicyDir string
}

// fixedDoesNotProve is the load-bearing limitation footer. It is constant so the
// honesty caveats ship identically in text and JSON and cannot drift per record.
// Phrasing is negation-framed to satisfy the public-language lint and to match
// docs/DECISION_RECORDS.md and docs/RECEIPTS.md.
var fixedDoesNotProve = []string{
	"replay reproduces the recorded decision; it does not prove enforcement. A reproduced deny is not evidence the action was blocked.",
	"replay does not prove that no upstream bytes moved; it re-evaluates the decision, not the absence of upstream side effects.",
	"replay reproduces the decision only for routed requests; direct access to the same tool is a bypass a record cannot see.",
	"a match does not prove the original verdict was correct; it proves only that the same inputs reproduce the same decision.",
}

// Run reads the record and the recorded request, recomputes the request and
// (when present) policy-bundle hashes, rebuilds the request, re-evaluates it
// through the same governance pipeline in a hermetic in-process configuration
// with no audit side effects, and reports whether the decision-defining fields
// match. It is read-only and fixture-safe: no credentials, no network, no live
// mutation.
//
// Run returns an error only for malformed inputs (unreadable or unparseable
// files, an unsupported schema version). A hash mismatch or a decision-field
// mismatch is a normal, reported outcome: Result.Matched is false and the error
// is nil, so the caller maps the outcome to an exit code via Result.Matched.
func Run(opts Options) (*Result, error) {
	if strings.TrimSpace(opts.RecordPath) == "" {
		return nil, fmt.Errorf("record path is required")
	}
	if strings.TrimSpace(opts.RequestPath) == "" {
		return nil, fmt.Errorf("--request is required: a record carries request_hash but not the request body; supply the recorded GovernanceRequest JSON")
	}
	if strings.TrimSpace(opts.PolicyDir) == "" {
		return nil, fmt.Errorf("--policies is required: replay re-evaluates the recorded request against the recorded policy bundle")
	}

	recordBytes, err := os.ReadFile(opts.RecordPath)
	if err != nil {
		return nil, fmt.Errorf("read record: %w", err)
	}
	var record governance.DecisionRecordV1
	if err := json.Unmarshal(recordBytes, &record); err != nil {
		return nil, fmt.Errorf("parse record: %w", err)
	}
	if !governance.SupportedDecisionRecordSchemaVersion(record.SchemaVersion) {
		return nil, fmt.Errorf(
			"unsupported record schema_version %q: replay reproduces %q and %q records",
			record.SchemaVersion,
			governance.DecisionRecordSchemaVersion,
			governance.DecisionRecordSchemaV2,
		)
	}
	if strings.TrimSpace(record.Action) == "" {
		return nil, fmt.Errorf("record is missing the required action field")
	}

	requestBytes, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return nil, fmt.Errorf("read request: %w", err)
	}
	// The recorded request is the canonical GovernanceRequest. Unmarshalling into
	// the struct both (a) rebuilds the request for re-evaluation and (b) lets us
	// recompute request_hash with ComputeRequestHash — the SAME function the
	// pipeline used to write it. ComputeRequestHash marshals in struct-field
	// order; ComputeRawRequestHash (used by verify-record --request) marshals via
	// `any` with sorted keys and yields a different hash, so replay deliberately
	// uses the struct path to reproduce the recorded bytes exactly.
	var req governance.GovernanceRequest
	if err := json.Unmarshal(requestBytes, &req); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}

	return reproduce(record, &req, opts.PolicyDir)
}

// reproduce runs the hash gates and the re-evaluation against a parsed record,
// rebuilt request, and policy directory. It is split out so tests can drive it
// with in-memory inputs.
func reproduce(record governance.DecisionRecordV1, req *governance.GovernanceRequest, policyDir string) (*Result, error) {
	result := &Result{
		SchemaVersion:       SchemaVersion,
		Status:              "ok",
		Matched:             true,
		RequiresCredentials: false,
		RequiresNetwork:     false,
		MutatesLiveSystems:  false,
		RecordSchemaVersion: record.SchemaVersion,
		RecordID:            record.RecordID,
		Recorded: Decision{
			Action:       record.Action,
			Reason:       record.Reason,
			DecisionMode: string(record.DecisionMode),
			MatchedRule:  record.MatchedRule,
			PolicyFile:   record.PolicyFile,
		},
		DoesNotProve: fixedDoesNotProve,
	}

	// Gate 1: recompute request_hash from the supplied request and confirm it
	// matches the record, so replay is reproducing THE recorded request.
	recomputedRequestHash := governance.ComputeRequestHash(req)
	requestMatched := record.RequestHash != "" && record.RequestHash == recomputedRequestHash
	result.HashChecks = append(result.HashChecks, HashCheck{
		Field:      "request_hash",
		Recorded:   record.RequestHash,
		Recomputed: recomputedRequestHash,
		Matched:    requestMatched,
	})
	if !requestMatched {
		if record.RequestHash == "" {
			result.fail("request_hash: record carries no request_hash to bind the supplied request to")
		} else {
			result.fail(fmt.Sprintf("request_hash mismatch: record %s != supplied request %s", record.RequestHash, recomputedRequestHash))
		}
	}

	// Gate 2: when the record carries a policy_bundle_hash, recompute it from the
	// supplied directory and confirm it matches, so replay is reproducing against
	// THE recorded policy bundle (not a stale or different one). When the record
	// carries no policy_bundle_hash there is nothing to bind; the re-evaluation
	// still runs against the supplied directory.
	if record.PolicyBundleHash != "" {
		recomputedPolicyHash, err := governance.PolicyBundleHashFromDir(policyDir)
		if err != nil {
			return nil, fmt.Errorf("hash policy bundle: %w", err)
		}
		policyMatched := record.PolicyBundleHash == recomputedPolicyHash
		result.HashChecks = append(result.HashChecks, HashCheck{
			Field:      "policy_bundle_hash",
			Recorded:   record.PolicyBundleHash,
			Recomputed: recomputedPolicyHash,
			Matched:    policyMatched,
		})
		if !policyMatched {
			result.fail(fmt.Sprintf("policy_bundle_hash mismatch: record %s != supplied bundle %s", record.PolicyBundleHash, recomputedPolicyHash))
		}
	}

	// Rebuild and re-evaluate: load the static policies from the supplied
	// directory and run the recorded request through the same pipeline, in a
	// hermetic in-process configuration — no trust checker, no PolicyEvaluator
	// override, and a no-op auditor so re-evaluation has no side effects.
	policies, err := governance.LoadStaticPoliciesFromDir(policyDir)
	if err != nil {
		return nil, fmt.Errorf("load policies: %w", err)
	}
	// GatewayVersion is copied onto the decision but is not a decision-defining
	// field replay compares; carry the recorded value so the reproduced decision
	// reads faithfully without affecting the comparison.
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		StaticPolicies: policies,
		GatewayVersion: record.BoundaryVersion,
	}, nil, nil, nil)

	decision, err := pipeline.Evaluate(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("re-evaluate request: %w", err)
	}

	result.Reproduced = Decision{
		Action:       decision.Action,
		Reason:       decision.Reason,
		DecisionMode: string(decision.DecisionMode),
		MatchedRule:  decision.MatchedRule,
		PolicyFile:   decision.PolicyFile,
	}

	// Compare the decision-defining fields — never action alone. action,
	// reason, and decision_mode are always compared. matched_rule and policy_file
	// are compared whenever EITHER side carries a value: an allow with no matched
	// rule legitimately has neither (empty == empty still holds), but a record
	// that recorded no rule while re-evaluation now matches one (or vice versa) is
	// genuine drift and must fail. This is what makes replay catch a stale or
	// different bundle that reaches the same action through a different rule.
	result.compareField("action", record.Action, decision.Action)
	result.compareField("reason", record.Reason, decision.Reason)
	result.compareField("decision_mode", string(record.DecisionMode), string(decision.DecisionMode))
	if record.MatchedRule != "" || decision.MatchedRule != "" {
		result.compareField("matched_rule", record.MatchedRule, decision.MatchedRule)
	}
	if record.PolicyFile != "" || decision.PolicyFile != "" {
		result.compareField("policy_file", record.PolicyFile, decision.PolicyFile)
	}

	return result, nil
}

// fail records a mismatch reason and flips the result to a non-matching,
// non-ok outcome.
func (r *Result) fail(reason string) {
	r.Matched = false
	r.Status = "mismatch"
	r.Mismatches = append(r.Mismatches, reason)
}

// compareField records a per-field check and, on inequality, a mismatch reason.
func (r *Result) compareField(field, recorded, reproduced string) {
	matched := recorded == reproduced
	r.FieldChecks = append(r.FieldChecks, FieldCheck{
		Field:      field,
		Recorded:   recorded,
		Reproduced: reproduced,
		Matched:    matched,
	})
	if !matched {
		r.fail(fmt.Sprintf("%s mismatch: record %q != reproduced %q", field, recorded, reproduced))
	}
}
