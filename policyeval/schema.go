package policyeval

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const PolicySchemaVersionV1 = "1"

// PolicyV1Document is the canonical v1 YAML policy envelope.
type PolicyV1Document struct {
	SchemaVersion string         `yaml:"schema_version" json:"schema_version"`
	Policy        PolicyV1Block  `yaml:"policy" json:"policy"`
	Metadata      map[string]any `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type PolicyV1Block struct {
	Name      string         `yaml:"name" json:"name"`
	Version   string         `yaml:"version" json:"version"`
	Transport string         `yaml:"transport,omitempty" json:"transport,omitempty"`
	Rules     []PolicyV1Rule `yaml:"rules" json:"rules"`
}

type PolicyV1Rule struct {
	Name         string              `yaml:"name" json:"name"`
	Tool         string              `yaml:"tool" json:"tool"`
	Action       string              `yaml:"action" json:"action"`
	Reason       string              `yaml:"reason,omitempty" json:"reason,omitempty"`
	Transport    string              `yaml:"transport,omitempty" json:"transport,omitempty"`
	DecisionMode string              `yaml:"decision_mode,omitempty" json:"decision_mode,omitempty"`
	TenantScope  []string            `yaml:"tenant_scope,omitempty" json:"tenant_scope,omitempty"`
	AgentScope   []string            `yaml:"agent_scope,omitempty" json:"agent_scope,omitempty"`
	Match        *PolicyV1Condition  `yaml:"match,omitempty" json:"match,omitempty"`
	Conditions   []PolicyV1Condition `yaml:"conditions,omitempty" json:"conditions,omitempty"`
	Metadata     map[string]string   `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type PolicyV1Condition struct {
	Type            string   `yaml:"type,omitempty" json:"type,omitempty"`
	Field           string   `yaml:"field,omitempty" json:"field,omitempty"`
	Contains        string   `yaml:"contains,omitempty" json:"contains,omitempty"`
	Value           string   `yaml:"value,omitempty" json:"value,omitempty"`
	Values          []string `yaml:"values,omitempty" json:"values,omitempty"`
	Regex           string   `yaml:"regex,omitempty" json:"regex,omitempty"`
	CaseInsensitive bool     `yaml:"case_insensitive,omitempty" json:"case_insensitive,omitempty"`
}

// IsPolicyV1YAML reports whether data uses the schema_version/policy envelope.
func IsPolicyV1YAML(data []byte) bool {
	var header struct {
		SchemaVersion any `yaml:"schema_version"`
		Policy        any `yaml:"policy"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.SchemaVersion != nil || header.Policy != nil
}

// ValidatePolicyV1YAML validates and returns a v1 policy document.
func ValidatePolicyV1YAML(path string, data []byte) (*PolicyV1Document, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("%s: empty policy file", path)
	}
	var doc PolicyV1Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: parse policy v1: %w", path, err)
	}
	if doc.SchemaVersion != PolicySchemaVersionV1 {
		return nil, fmt.Errorf("%s: schema_version must be %q", path, PolicySchemaVersionV1)
	}
	if strings.TrimSpace(doc.Policy.Name) == "" {
		return nil, fmt.Errorf("%s: policy.name is required", path)
	}
	if strings.TrimSpace(doc.Policy.Version) == "" {
		return nil, fmt.Errorf("%s: policy.version is required", path)
	}
	if len(doc.Policy.Rules) == 0 {
		return nil, fmt.Errorf("%s: policy.rules must contain at least one rule", path)
	}
	for i, rule := range doc.Policy.Rules {
		if err := validatePolicyV1Rule(rule); err != nil {
			return nil, fmt.Errorf("%s: policy.rules[%d]: %w", path, i, err)
		}
	}
	return &doc, nil
}

func validatePolicyV1Rule(rule PolicyV1Rule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.TrimSpace(rule.Tool) == "" {
		return fmt.Errorf("tool is required")
	}
	if !allowedPolicyV1Action(rule.Action) {
		return fmt.Errorf("unsupported action %q", rule.Action)
	}
	if rule.Match != nil {
		if err := validatePolicyV1Condition(*rule.Match); err != nil {
			return fmt.Errorf("match: %w", err)
		}
	}
	for i, condition := range rule.Conditions {
		if err := validatePolicyV1Condition(condition); err != nil {
			return fmt.Errorf("conditions[%d]: %w", i, err)
		}
	}
	return nil
}

func validatePolicyV1Condition(condition PolicyV1Condition) error {
	conditionType := strings.ToLower(strings.TrimSpace(condition.Type))
	if conditionType == "" {
		conditionType = "contains"
	}
	switch conditionType {
	case "contains", "not_contains", "equals", "not_equals", "regex":
		if strings.TrimSpace(condition.Field) == "" {
			return fmt.Errorf("field is required for %s", conditionType)
		}
	case "transport_is":
	case "agent_in", "agent_not_in":
		if len(condition.Values) == 0 && strings.TrimSpace(condition.Value) == "" {
			return fmt.Errorf("value or values is required for %s", conditionType)
		}
	case "ast_class":
		if strings.TrimSpace(condition.Value) == "" && len(condition.Values) == 0 {
			return fmt.Errorf("value or values is required for ast_class")
		}
	default:
		return fmt.Errorf("unsupported condition type %q", condition.Type)
	}
	if conditionType == "regex" {
		pattern := condition.Regex
		if pattern == "" {
			pattern = condition.Value
		}
		if pattern == "" {
			pattern = condition.Contains
		}
		if pattern == "" {
			return fmt.Errorf("regex pattern is required")
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	return nil
}

func allowedPolicyV1Action(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "allow", "deny", "warn", "audit", "escalate", "require_approval":
		return true
	default:
		return false
	}
}
