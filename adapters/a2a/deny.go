package a2a

import (
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func DeniedTaskResponse(envelope TaskEnvelope, decision *governance.GovernanceDecision) *TaskResponse {
	message := "denied by Boundary"
	if decision != nil && decision.Reason != "" {
		message = decision.Reason
	}
	response := &TaskResponse{
		TaskID:    envelope.TaskID,
		ContextID: envelope.ContextID,
		Status:    StatusDenied,
		Error: &TaskError{
			Code:    "governance_denied",
			Message: message,
		},
		Governance: MetadataFromDecision(decision),
	}
	if response.Governance == nil {
		response.Governance = &GovernanceMetadata{Action: "deny", Reason: message}
	}
	return response
}

func UnsupportedTaskResponse(taskID string, err error) *TaskResponse {
	message := "unsupported A2A request"
	if err != nil {
		message = err.Error()
	}
	return &TaskResponse{
		TaskID: taskID,
		Status: StatusUnsupported,
		Error: &TaskError{
			Code:    "unsupported",
			Message: message,
		},
		Governance: &GovernanceMetadata{Action: "deny", Reason: message},
	}
}

func ErrorTaskResponse(envelope TaskEnvelope, code, message string) *TaskResponse {
	if code == "" {
		code = "a2a_error"
	}
	if message == "" {
		message = fmt.Sprintf("%s while governing A2A task", code)
	}
	return &TaskResponse{
		TaskID:    envelope.TaskID,
		ContextID: envelope.ContextID,
		Status:    StatusError,
		Error:     &TaskError{Code: code, Message: message},
		Governance: &GovernanceMetadata{
			Action: "deny",
			Reason: message,
		},
	}
}
