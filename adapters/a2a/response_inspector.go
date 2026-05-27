package a2a

import (
	"bytes"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

func InspectResponse(resp *governance.ToolResponse) *governance.ResponseInspection {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}
	}
	inspection := &governance.ResponseInspection{Safe: true}
	body := strings.ToLower(string(resp.Content))
	if resp.ExitCode != 0 {
		inspection.Safe = false
		inspection.Concerns = append(inspection.Concerns, "non-zero A2A downstream result")
	}
	for _, marker := range [][]byte{[]byte("api_key"), []byte("secret"), []byte("token")} {
		if bytes.Contains([]byte(body), marker) {
			inspection.Safe = false
			inspection.SensitiveData = true
			inspection.Concerns = append(inspection.Concerns, "possible sensitive data in A2A response")
			break
		}
	}
	if strings.Contains(body, "policy_violation") || strings.Contains(body, "unsafe") {
		inspection.Safe = false
		inspection.Concerns = append(inspection.Concerns, "policy-relevant A2A response signal")
	}
	return inspection
}
