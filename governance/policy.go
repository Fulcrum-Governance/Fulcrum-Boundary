package governance

import (
	"fmt"
	"regexp"
	"strings"
)

// StaticPolicyRule defines a simple allow/deny/warn rule evaluated before the
// full PolicyEval engine. Mirrors securemcp.PolicyRule while keeping the
// launch policy surface intentionally narrow.
type StaticPolicyRule struct {
	Name         string              `json:"name" yaml:"name"`
	Tool         string              `json:"tool" yaml:"tool"`
	Action       string              `json:"action" yaml:"action"` // allow, deny, warn, audit, escalate, require_approval
	Reason       string              `json:"reason,omitempty" yaml:"reason,omitempty"`
	Transport    string              `json:"transport,omitempty" yaml:"transport,omitempty"`
	DecisionMode DecisionMode        `json:"decision_mode,omitempty" yaml:"decision_mode,omitempty"`
	TenantScope  []string            `json:"tenant_scope,omitempty" yaml:"tenant_scope,omitempty"`
	AgentScope   []string            `json:"agent_scope,omitempty" yaml:"agent_scope,omitempty"`
	Match        *StaticPolicyMatch  `json:"match,omitempty" yaml:"match,omitempty"`
	Conditions   []StaticPolicyMatch `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	PolicyFile   string              `json:"policy_file,omitempty" yaml:"-"`
	Metadata     map[string]string   `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// StaticPolicyMatch is the launch-grade matcher used by YAML static policies.
// It supports typed field checks against GovernanceRequest, including
// arguments.<name>. It remains intentionally smaller than the PolicyEval DSL.
type StaticPolicyMatch struct {
	Type            string   `json:"type,omitempty" yaml:"type,omitempty"`
	Field           string   `json:"field,omitempty" yaml:"field,omitempty"`
	Contains        string   `json:"contains,omitempty" yaml:"contains,omitempty"`
	Value           string   `json:"value,omitempty" yaml:"value,omitempty"`
	Values          []string `json:"values,omitempty" yaml:"values,omitempty"`
	Regex           string   `json:"regex,omitempty" yaml:"regex,omitempty"`
	CaseInsensitive bool     `json:"case_insensitive,omitempty" yaml:"case_insensitive,omitempty"`
}

func (r StaticPolicyRule) matchesRequest(req *GovernanceRequest) bool {
	if req == nil {
		return false
	}
	if !toolMatches(r.Tool, req.ToolName) {
		return false
	}
	if r.Transport != "" && !strings.EqualFold(r.Transport, string(req.Transport)) {
		return false
	}
	if len(r.TenantScope) > 0 && !stringInList(req.TenantID, r.TenantScope, true) {
		return false
	}
	if len(r.AgentScope) > 0 && !stringInList(req.AgentID, r.AgentScope, true) {
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

	switch matchType {
	case "transport_is":
		needle := firstNonEmpty(m.Value, m.Contains)
		return needle != "" && strings.EqualFold(needle, string(req.Transport))
	case "agent_in":
		return stringInList(req.AgentID, matchValues(m), true)
	case "agent_not_in":
		return !stringInList(req.AgentID, matchValues(m), true)
	case "ast_class":
		field := m.Field
		if field == "" {
			field = "arguments.sql_class"
		}
		got, ok := requestField(req, field)
		return ok && valueMatchesAny(got, matchValues(m), true)
	}

	needle := firstNonEmpty(m.Contains, m.Value)
	if needle == "" || m.Field == "" {
		return false
	}
	haystack, ok := requestField(req, m.Field)
	if !ok {
		return false
	}
	switch matchType {
	case "contains":
		return contains(haystack, needle, m.CaseInsensitive)
	case "not_contains":
		return !contains(haystack, needle, m.CaseInsensitive)
	case "equals":
		return equalString(haystack, needle, m.CaseInsensitive)
	case "not_equals":
		return !equalString(haystack, needle, m.CaseInsensitive)
	case "regex":
		pattern := firstNonEmpty(m.Regex, m.Value, m.Contains)
		if pattern == "" {
			return false
		}
		if m.CaseInsensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(haystack)
	default:
		return false
	}
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
	case "risk_class", "sql.class":
		if req.Arguments != nil {
			if value, ok := req.Arguments["sql_class"]; ok {
				return fmt.Sprint(value), true
			}
			if value, ok := req.Arguments["risk_class"]; ok {
				return fmt.Sprint(value), true
			}
		}
		return "", false
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func contains(haystack, needle string, insensitive bool) bool {
	if insensitive {
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}
	return strings.Contains(haystack, needle)
}

func equalString(left, right string, insensitive bool) bool {
	if insensitive {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func matchValues(match StaticPolicyMatch) []string {
	values := append([]string{}, match.Values...)
	if match.Value != "" {
		values = append(values, match.Value)
	}
	if match.Contains != "" {
		values = append(values, match.Contains)
	}
	return values
}

func valueMatchesAny(value string, values []string, insensitive bool) bool {
	for _, candidate := range values {
		if equalString(value, candidate, insensitive) {
			return true
		}
	}
	return false
}

func stringInList(value string, values []string, insensitive bool) bool {
	if value == "" {
		return false
	}
	return valueMatchesAny(value, values, insensitive)
}
