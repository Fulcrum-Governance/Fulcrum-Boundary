package managedagents

import (
	"bytes"
	"strings"

	"github.com/fulcrum-governance/boundary/governance"
)

// InspectResponse flags obvious Managed Agents tool-result concerns for
// downstream policy or alerting. It intentionally stays conservative; the
// pre-execution decision remains the enforcement boundary.
func InspectResponse(resp *governance.ToolResponse) *governance.ResponseInspection {
	if resp == nil {
		return &governance.ResponseInspection{Safe: true}
	}
	inspection := &governance.ResponseInspection{Safe: true}
	body := strings.ToLower(string(resp.Content))
	if resp.ExitCode != 0 {
		inspection.Safe = false
		inspection.Concerns = append(inspection.Concerns, "non-zero tool result")
	}
	if bytes.Contains([]byte(body), []byte("api_key")) || bytes.Contains([]byte(body), []byte("secret")) {
		inspection.Safe = false
		inspection.SensitiveData = true
		inspection.Concerns = append(inspection.Concerns, "possible sensitive data in tool result")
	}
	return inspection
}
