package editboundary

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

type ApplyExecutor struct {
	Pipeline   *governance.Pipeline
	Applier    Applier
	RecordPath string
}

type ApplyResult struct {
	Inspection     Inspection
	Decision       *governance.GovernanceDecision
	Record         EditDecisionRecord
	RecordPath     string
	Applied        bool
	ApplierInvoked bool
	ExitCode       int
}

func (e ApplyExecutor) Apply(ctx context.Context, req ApplyRequest) (*ApplyResult, error) {
	cwd := req.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}
		req.CWD = cwd
	}
	recordPath := firstNonEmpty(req.RecordPath, e.RecordPath, DefaultEditDecisionRecordPath)
	if err := prepareRecordPath(recordPath); err != nil {
		return nil, err
	}

	inspection, err := InspectPatch(req.Patch)
	if err != nil {
		return nil, err
	}
	governanceReq := BuildGovernanceRequest(req, inspection)
	pipeline := e.Pipeline
	if pipeline == nil {
		pipeline = NewDefaultPreviewPipeline()
	}
	decision, err := pipeline.Evaluate(ctx, governanceReq)
	if err != nil {
		decision = &governance.GovernanceDecision{
			RequestID:  governanceReq.RequestID,
			Action:     "deny",
			Reason:     fmt.Sprintf("governance pipeline error: %v", err),
			EnvelopeID: governanceReq.EnvelopeID,
		}
	}

	result := &ApplyResult{
		Inspection: inspection,
		Decision:   decision,
		RecordPath: recordPath,
		ExitCode:   126,
	}
	applyAllowed, reason := applyAllowed(req, inspection, decision)
	var applicationErr string
	switch {
	case req.DryRun:
		result.ExitCode = 0
	case applyAllowed:
		applier := e.Applier
		if applier == nil {
			applier = InternalApplier{}
		}
		result.ApplierInvoked = true
		if _, err := applier.Apply(cwd, req.Patch); err != nil {
			applicationErr = err.Error()
			reason = applicationErr
			result.ExitCode = 1
		} else {
			result.Applied = true
			result.ExitCode = 0
		}
	case reason != "":
		decision.Reason = reason
	}

	record := EditDecisionRecord{
		RecordType:       "edit_decision",
		SchemaVersion:    SchemaVersionDecision,
		RequestID:        decision.RequestID,
		EnvelopeID:       decision.EnvelopeID,
		PatchSHA256:      inspection.PatchSHA256,
		CWD:              cwd,
		FilesTouched:     inspection.FilesTouched,
		Files:            inspection.RecordPaths(),
		RedactedPaths:    inspection.RedactedPaths(),
		Class:            inspection.HighestClass,
		Risk:             inspection.Risk,
		Action:           decision.Action,
		ApprovalPresent:  req.ApprovalPresent,
		ApprovalMode:     approvalMode(req.ApprovalPresent),
		DryRun:           req.DryRun,
		ApplierInvoked:   result.ApplierInvoked,
		Applied:          result.Applied,
		IndexChanged:     false,
		ExitCode:         result.ExitCode,
		Reason:           firstNonEmpty(reason, decision.Reason),
		MatchedRule:      decision.MatchedRule,
		PolicyID:         decision.PolicyID,
		ApplicationError: applicationErr,
		Timestamp:        time.Now(),
	}
	if err := AppendDecisionRecord(recordPath, record); err != nil {
		return nil, err
	}
	result.Record = record
	return result, nil
}

func applyAllowed(req ApplyRequest, inspection Inspection, decision *governance.GovernanceDecision) (allowed bool, reason string) {
	if hardDenyClass(inspection.HighestClass) {
		return false, "hard-deny edit class cannot be applied"
	}
	if decision == nil {
		return false, "missing governance decision"
	}
	if decision.Allowed() {
		return true, ""
	}
	if decision.Action == "require_approval" {
		if req.ApprovalPresent {
			return true, ""
		}
		return false, "approval required before edit can be applied"
	}
	return false, decision.Reason
}

func hardDenyClass(class Class) bool {
	return class == ClassSecretBearing || class == ClassDestructive || class == ClassOutsideProjectScope
}

func approvalMode(present bool) string {
	if present {
		return "local_flag"
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
