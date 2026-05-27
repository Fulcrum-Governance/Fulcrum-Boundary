package commandboundary

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

const DefaultDecisionRecordPath = ".boundary/command/decision-records.jsonl"

type RunRequest struct {
	Argv       []string
	CWD        string
	AgentID    string
	TenantID   string
	RecordPath string
	Env        []string
}

func BuildGovernanceRequest(req RunRequest, classification Classification, argvHash string) *governance.GovernanceRequest {
	cwd := req.CWD
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = "local"
	}
	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportCLI,
		AgentID:   req.AgentID,
		TenantID:  tenantID,
		ToolName:  classification.Command,
		Action:    string(classification.Class),
		Command:   classification.Command,
		Arguments: map[string]any{
			"command_class":      string(classification.Class),
			"command_risk":       string(classification.Risk),
			"recommended_action": string(classification.RecommendedAction),
			"args_redacted":      classification.ArgsRedacted,
			"argv_hash":          argvHash,
			"cwd":                cwd,
		},
		PipeChain: []governance.PipeSegment{
			{
				Command:   classification.Command,
				Args:      classification.ArgsRedacted,
				RiskLevel: classRiskLevel(classification.Class),
			},
		},
	}
}

func HashArgv(argv []string) string {
	hash := sha256.New()
	for i, part := range argv {
		if i > 0 {
			hash.Write([]byte{0})
		}
		hash.Write([]byte(part))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func classRiskLevel(class Class) string {
	switch class {
	case ClassObserveRead:
		return "read"
	case ClassLocalFileWrite:
		return "write"
	case ClassDestructiveMutation, ClassCredentialAccess:
		return "destructive"
	default:
		return "admin"
	}
}
