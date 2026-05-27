package a2a

import (
	"encoding/json"
	"fmt"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

var supportedRequiredFields = map[string]func(*TaskEnvelope) bool{
	"task_id":         func(e *TaskEnvelope) bool { return e.TaskID != "" },
	"context_id":      func(e *TaskEnvelope) bool { return e.ContextID != "" },
	"message_id":      func(e *TaskEnvelope) bool { return e.MessageID != "" },
	"sender_agent_id": func(e *TaskEnvelope) bool { return e.SenderAgentID != "" },
	"receiver":        func(e *TaskEnvelope) bool { return e.Receiver != "" },
	"action":          func(e *TaskEnvelope) bool { return e.Action != "" },
	"input":           func(e *TaskEnvelope) bool { return e.Input != nil },
}

func ParseTaskEnvelope(raw any) (*TaskEnvelope, []byte, error) {
	switch v := raw.(type) {
	case *TaskEnvelope:
		return validateEnvelope(v, nil)
	case TaskEnvelope:
		return validateEnvelope(&v, nil)
	case *TaskMessage:
		return validateEnvelope(envelopeFromLegacy(v), nil)
	case TaskMessage:
		return validateEnvelope(envelopeFromLegacy(&v), nil)
	case json.RawMessage:
		return parseJSONEnvelope(v)
	case []byte:
		return parseJSONEnvelope(json.RawMessage(v))
	default:
		return nil, nil, governance.NewParseError(governance.TransportA2A, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}
}

func parseJSONEnvelope(raw json.RawMessage) (*TaskEnvelope, []byte, error) {
	var rpc jsonRPCRequest
	if err := json.Unmarshal(raw, &rpc); err == nil && rpc.Method != "" {
		envelope, err := envelopeFromJSONRPC(rpc)
		if err != nil {
			return nil, nil, err
		}
		envelope.Raw = append(json.RawMessage(nil), raw...)
		return validateEnvelope(envelope, raw)
	}
	envelope := &TaskEnvelope{Raw: append(json.RawMessage(nil), raw...)}
	if err := json.Unmarshal(raw, envelope); err != nil {
		return nil, nil, governance.NewParseError(governance.TransportA2A, "unmarshal task envelope", err)
	}
	if envelope.SenderAgentID == "" {
		var legacy TaskMessage
		if err := json.Unmarshal(raw, &legacy); err == nil && legacy.AgentCard.AgentID != "" {
			envelope = envelopeFromLegacy(&legacy)
			envelope.Raw = append(json.RawMessage(nil), raw...)
		}
	}
	return validateEnvelope(envelope, raw)
}

func validateEnvelope(envelope *TaskEnvelope, raw []byte) (*TaskEnvelope, []byte, error) {
	if envelope == nil {
		return nil, nil, governance.NewParseError(governance.TransportA2A, "TaskEnvelope is required", nil)
	}
	if envelope.Action == "" {
		return nil, nil, governance.NewParseError(governance.TransportA2A, "TaskEnvelope.Action is required", nil)
	}
	if envelope.SenderAgentID == "" {
		return nil, nil, governance.NewParseError(governance.TransportA2A, "TaskEnvelope.SenderAgentID is required", nil)
	}
	for _, field := range envelope.RequiredFields {
		check, ok := supportedRequiredFields[field]
		if !ok {
			return nil, nil, governance.NewParseError(governance.TransportA2A, fmt.Sprintf("unsupported required field %q", field), nil)
		}
		if !check(envelope) {
			return nil, nil, governance.NewParseError(governance.TransportA2A, fmt.Sprintf("required field %q is missing", field), nil)
		}
	}
	return envelope, raw, nil
}

func envelopeFromLegacy(msg *TaskMessage) *TaskEnvelope {
	if msg == nil {
		return nil
	}
	receiver := msg.AgentCard.Endpoint
	if receiver == "" {
		receiver = msg.AgentCard.Name
	}
	return &TaskEnvelope{
		TaskID:        msg.TaskID,
		SenderAgentID: msg.AgentCard.AgentID,
		Receiver:      receiver,
		Action:        msg.Action,
		Input:         msg.Input,
	}
}

func envelopeFromJSONRPC(rpc jsonRPCRequest) (*TaskEnvelope, error) {
	if rpc.Method != "message/send" && rpc.Method != "tasks/send" {
		return nil, governance.NewParseError(governance.TransportA2A, fmt.Sprintf("unsupported A2A method %q", rpc.Method), nil)
	}
	var params messageSendParams
	if len(rpc.Params) > 0 {
		if err := json.Unmarshal(rpc.Params, &params); err != nil {
			return nil, governance.NewParseError(governance.TransportA2A, "unmarshal A2A message params", err)
		}
	}
	metadata := mergeMetadata(params.Metadata, params.Message.Metadata)
	envelope := &TaskEnvelope{
		TaskID:        params.Message.TaskID,
		ContextID:     params.Message.ContextID,
		MessageID:     params.Message.MessageID,
		SenderAgentID: stringMetadata(metadata, "sender_agent_id"),
		Receiver:      stringMetadata(metadata, "receiver"),
		Action:        firstNonEmpty(stringMetadata(metadata, "action"), rpc.Method),
		Input:         inputFromParts(params.Message.Parts),
		Metadata:      metadata,
	}
	if required, ok := metadata["required_fields"].([]any); ok {
		for _, field := range required {
			if s, ok := field.(string); ok {
				envelope.RequiredFields = append(envelope.RequiredFields, s)
			}
		}
	}
	return envelope, nil
}

func mergeMetadata(maps ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func inputFromParts(parts []messagePart) map[string]any {
	input := map[string]any{}
	var texts []string
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
		for k, v := range part.Data {
			input[k] = v
		}
	}
	if len(texts) == 1 {
		input["text"] = texts[0]
	} else if len(texts) > 1 {
		input["text_parts"] = texts
	}
	if len(input) == 0 {
		return nil
	}
	return input
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return value
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
