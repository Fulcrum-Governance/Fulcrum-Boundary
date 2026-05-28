package editboundary

import (
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const DefaultEditDecisionRecordPath = ".boundary/edit/decision-records.jsonl"

type ApplyRequest struct {
	Patch           []byte
	CWD             string
	AgentID         string
	TenantID        string
	RecordPath      string
	DryRun          bool
	ApprovalPresent bool
}

func BuildGovernanceRequest(req ApplyRequest, inspection Inspection) *governance.GovernanceRequest {
	cwd := req.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = "local"
	}
	return &governance.GovernanceRequest{
		RequestID:  uuid.New().String(),
		Transport:  governance.TransportCLI,
		AgentID:    req.AgentID,
		TenantID:   tenantID,
		ToolName:   "edit.apply",
		Action:     string(inspection.HighestClass),
		RawPayload: append([]byte(nil), req.Patch...),
		Command:    "boundary edit apply",
		Arguments: map[string]any{
			"edit_class":         string(inspection.HighestClass),
			"edit_risk":          string(inspection.Risk),
			"recommended_action": string(inspection.RecommendedAction),
			"patch_sha256":       inspection.PatchSHA256,
			"files_touched":      inspection.FilesTouched,
			"paths_redacted":     strings.Join(inspection.RedactedPaths(), ","),
			"cwd":                cwd,
			"dry_run":            req.DryRun,
			"approval_present":   req.ApprovalPresent,
		},
		PipeChain: []governance.PipeSegment{
			{
				Command:   "boundary edit apply",
				Args:      []string{inspection.PatchSHA256},
				RiskLevel: editRiskLevel(inspection.HighestClass),
			},
		},
	}
}

func editRiskLevel(class Class) string {
	switch class {
	case ClassNoop, ClassSafeContent:
		return "write"
	case ClassSecretBearing, ClassDestructive, ClassOutsideProjectScope:
		return "destructive"
	default:
		return "admin"
	}
}
