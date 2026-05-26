package governance

import (
	"fmt"
	"strings"
)

// StaticPolicyRule defines a simple allow/deny/warn rule evaluated before the
// full PolicyEval engine. Mirrors securemcp.PolicyRule while keeping the
// launch policy surface intentionally narrow.
type StaticPolicyRule struct {
	Name       string              `json:"name" yaml:"name"`
	Tool       string              `json:"tool" yaml:"tool"`
	Action     string              `json:"action" yaml:"action"` // "allow", "deny", "warn", "audit"
	Reason     string              `json:"reason,omitempty" yaml:"reason,omitempty"`
	Match      *StaticPolicyMatch  `json:"match,omitempty" yaml:"match,omitempty"`
	Conditions []StaticPolicyMatch `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	PolicyFile string              `json:"policy_file,omitempty" yaml:"-"`
	Metadata   map[string]string   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// StaticPolicyMatch is the launch-grade matcher used by YAML static policies.
// It supports contains checks against fields on GovernanceRequest, including
// arguments.<name>. This is intentionally not a full SQL firewall or policy DSL.
type StaticPolicyMatch struct {
	Type            string `json:"type,omitempty" yaml:"type,omitempty"`
	Field           string `json:"field" yaml:"field"`
	Contains        string `json:"contains,omitempty" yaml:"contains,omitempty"`
	Value           string `json:"value,omitempty" yaml:"value,omitempty"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty" yaml:"case_insensitive,omitempty"`
}

func (r StaticPolicyRule) matchesRequest(req *GovernanceRequest) bool {
	if !toolMatches(r.Tool, req.ToolName) {
		return false
	}

	matches := r.Conditions
	if r.Match != nil {
		matches = append([]StaticPolicyMatch{*r.Match}, matches...)
	}
	if len(matches) == 0 {
		return true
	}

	for _, match := range matches {
		if !match.matches(req) {
			return false
		}
	}
	return true
}

func (m StaticPolicyMatch) matches(req *GovernanceRequest) bool {
	matchType := strings.ToLower(strings.TrimSpace(m.Type))
	if matchType == "" {
		matchType = "contains"
	}
	if matchType != "contains" {
		return false
	}

	needle := m.Contains
	if needle == "" {
		needle = m.Value
	}
	if needle == "" || m.Field == "" {
		return false
	}

	haystack, ok := requestField(req, m.Field)
	if !ok {
		return false
	}
	if m.CaseInsensitive {
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}
	return strings.Contains(haystack, needle)
}

func requestField(req *GovernanceRequest, field string) (string, bool) {
	if req == nil {
		return "", false
	}
	switch field {
	case "tool", "tool_name":
		return req.ToolName, true
	case "action":
		return req.Action, true
	case "agent_id":
		return req.AgentID, true
	case "tenant_id":
		return req.TenantID, true
	case "transport":
		return string(req.Transport), true
	case "command":
		return req.Command, true
	case "code":
		return req.Code, true
	case "input.text", "arguments.text", "arguments.sql":
		key := strings.TrimPrefix(field, "arguments.")
		key = strings.TrimPrefix(key, "input.")
		if key == "text" && req.Arguments != nil {
			if value, ok := req.Arguments["sql"]; ok {
				return fmt.Sprint(value), true
			}
		}
		if req.Arguments != nil {
			value, ok := req.Arguments[key]
			if ok {
				return fmt.Sprint(value), true
			}
		}
	}

	if strings.HasPrefix(field, "arguments.") {
		key := strings.TrimPrefix(field, "arguments.")
		value, ok := req.Arguments[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	}
	if strings.HasPrefix(field, "input.") {
		key := strings.TrimPrefix(field, "input.")
		value, ok := req.Arguments[key]
		if !ok {
			return "", false
		}
		return fmt.Sprint(value), true
	}
	return "", false
}
