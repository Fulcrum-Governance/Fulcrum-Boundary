package a2a

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func GovernanceRequestFromEnvelope(envelope *TaskEnvelope, rawPayload []byte, tenantID string) (*governance.GovernanceRequest, error) {
	if _, _, err := validateEnvelope(envelope, rawPayload); err != nil {
		return nil, err
	}
	args := map[string]any{}
	for k, v := range envelope.Input {
		args[k] = v
	}
	args["task_id"] = envelope.TaskID
	args["context_id"] = envelope.ContextID
	args["message_id"] = envelope.MessageID
	args["receiver"] = envelope.Receiver
	if envelope.Metadata != nil {
		args["metadata"] = envelope.Metadata
	}
	traceID := firstNonEmpty(envelope.TaskID, envelope.ContextID, envelope.MessageID)
	return &governance.GovernanceRequest{
		RequestID:  uuid.New().String(),
		Transport:  governance.TransportA2A,
		AgentID:    envelope.SenderAgentID,
		TenantID:   tenantID,
		ToolName:   envelope.Action,
		Action:     "a2a/task",
		Arguments:  args,
		RawPayload: rawPayload,
		TraceID:    traceID,
		BudgetKey:  fmt.Sprintf("%s/%s", tenantID, traceID),
	}, nil
}

func envelopeFromRequest(req *governance.GovernanceRequest) TaskEnvelope {
	envelope := TaskEnvelope{
		TaskID:        stringArg(req.Arguments, "task_id"),
		ContextID:     stringArg(req.Arguments, "context_id"),
		MessageID:     stringArg(req.Arguments, "message_id"),
		SenderAgentID: req.AgentID,
		Receiver:      stringArg(req.Arguments, "receiver"),
		Action:        req.ToolName,
		Input:         map[string]any{},
	}
	for k, v := range req.Arguments {
		switch k {
		case "task_id", "context_id", "message_id", "receiver", "metadata":
			continue
		default:
			envelope.Input[k] = v
		}
	}
	if len(envelope.Input) == 0 {
		envelope.Input = nil
	}
	if metadata, ok := req.Arguments["metadata"].(map[string]any); ok {
		envelope.Metadata = metadata
	}
	return envelope
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}
