package hookboundary

import (
	"context"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// operatorRuleUnavailable is the matched_rule of a decision denied because the
// operator policy bundle could not be read, parsed, or tied to one hash.
const operatorRuleUnavailable = "hook-operator-policy:unavailable"

// operatorOutcome is what the operator policy bundle said about one request.
type operatorOutcome struct {
	verdict     Verdict
	reason      string
	matchedRule string
	policyFile  string
	bundleHash  string
}

// evaluateOperatorPolicy runs one routed request past the operator's policy
// bundle in cfg.PolicyDir, loaded fresh from disk on this invocation, so a
// `boundary revoke` written before this tool call denies it (FUL-772).
//
// The bundle only ever adds restriction: the caller keeps the compiled-in
// posture's verdict unless this one is strictly stricter. A bundle that cannot
// be loaded, or an outcome the hook has no verdict for, denies — whatever the
// fail mode says, because a revocation that cannot be read is not known to be
// absent.
func evaluateOperatorPolicy(cfg Config, req *governance.GovernanceRequest) operatorOutcome {
	capture := &auditCapture{}
	pipeline := governance.NewPipeline(governance.PipelineConfig{
		PolicyDir:      cfg.PolicyDir,
		GatewayVersion: cfg.BoundaryVersion,
	}, nil, nil, capture)
	// Evaluate fills request and envelope ids it finds empty; work on a copy
	// so the record's request_hash covers the request the hook built.
	probe := *req
	decision, err := pipeline.Evaluate(context.Background(), &probe)
	out := operatorOutcome{bundleHash: capture.event.PolicyBundleHash, policyFile: capture.event.PolicyFile}
	if err != nil || decision == nil {
		out.verdict, out.matchedRule = VerdictDeny, operatorRuleUnavailable
		out.reason = fmt.Sprintf("Fulcrum Boundary denied this tool call: the operator policy bundle could not be evaluated (%v).", err)
		return out
	}
	if decision.Action == governance.ActionCheckIndeterminate {
		out.verdict, out.matchedRule = VerdictDeny, operatorRuleUnavailable
		out.reason = "Fulcrum Boundary denied this tool call: the operator policy bundle could not be loaded (" + decision.Reason + "). The call was not run."
		return out
	}
	verdict, ok := ParseVerdict(decision.Action)
	if decision.Action == "escalate" {
		verdict, ok = VerdictRequireApproval, true
	}
	if !ok {
		verdict = VerdictDeny
	}
	out.verdict = verdict
	out.matchedRule = decision.MatchedRule
	if out.matchedRule == "" {
		out.matchedRule = "hook-operator-policy:" + decision.Action
	}
	out.reason = fmt.Sprintf("Fulcrum Boundary operator policy: %s (%s). The call was not run.", decision.Reason, out.matchedRule)
	return out
}

// applyOperatorPolicy folds the operator bundle's outcome into in. It never
// loosens: the compiled-in verdict, reason, and matched rule stand unless the
// bundle's verdict is strictly stricter.
func applyOperatorPolicy(cfg Config, in *recordInput) {
	if cfg.PolicyDir == "" || in.Request == nil {
		return
	}
	out := evaluateOperatorPolicy(cfg, in.Request)
	in.PolicyBundleHash = out.bundleHash
	if verdictStrength(out.verdict) > verdictStrength(in.Verdict) {
		in.Verdict = out.verdict
		in.Reason = out.reason
		in.MatchedRule = out.matchedRule
		in.PolicyFile = out.policyFile
	}
}

// auditCapture keeps the decision event the throwaway pipeline emits first,
// which carries the bundle hash and source file of the rules that decided.
type auditCapture struct {
	event governance.AuditEvent
	seen  bool
}

func (c *auditCapture) Publish(_ context.Context, event governance.AuditEvent) {
	if !c.seen {
		c.event, c.seen = event, true
	}
}
