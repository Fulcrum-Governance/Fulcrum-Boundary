package governance

import (
	"fmt"

	"github.com/google/uuid"
)

// unconfiguredDecision is what a nil *Pipeline returns. Nothing was evaluated,
// so the request is CHECK_INDETERMINATE/missing_config and blocks (ADR-047).
// No decision record is emitted: a nil pipeline has no audit publisher.
func unconfiguredDecision(req *GovernanceRequest) *GovernanceDecision {
	if req.EnvelopeID == "" {
		req.EnvelopeID = uuid.New().String()
	}
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	cf := &CheckFailure{
		Stage:    CheckStageConfig,
		Class:    CheckClassConfig,
		Category: FailureMissingConfig,
		Cause:    "governance pipeline is not configured",
	}
	return &GovernanceDecision{
		RequestID:    req.RequestID,
		Action:       ActionCheckIndeterminate,
		Reason:       fmt.Sprintf("check indeterminate (%s/%s): %s", cf.Stage, cf.Category, cf.Cause),
		EnvelopeID:   req.EnvelopeID,
		DecisionMode: DecisionModeDeterministic,
		TrustState:   TrustStateUnknown.String(),
		Check:        cf,
	}
}
