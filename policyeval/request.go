package policyeval

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// RequestProvenance records how a Boundary request reached PolicyEval.
type RequestProvenance struct {
	Source  string `json:"source,omitempty"`
	Adapter string `json:"adapter,omitempty"`
	TraceID string `json:"trace_id,omitempty"`
}

func cloneAttributes(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringifyArgument(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return string(encoded)
		}
		return fmt.Sprint(typed)
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
