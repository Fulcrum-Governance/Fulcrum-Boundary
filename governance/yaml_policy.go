package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulcrum-governance/fulcrum-boundary/policyeval"
	"gopkg.in/yaml.v3"
)

// StaticPolicyDocument is the on-disk YAML shape for launch-grade static
// policies. It is intentionally small: one document, many simple rules.
type StaticPolicyDocument struct {
	Name    string             `yaml:"name"`
	Version string             `yaml:"version"`
	Rules   []StaticPolicyRule `yaml:"rules"`
}

// StaticPolicyLoadResult captures loaded rules plus validation detail that
// CLI commands can render without reparsing files.
type StaticPolicyLoadResult struct {
	Files    []string
	Rules    []StaticPolicyRule
	Warnings []string
}

// LoadStaticPoliciesFromDir returns all static policy rules found in YAML
// files under dir. Use LoadStaticPolicyFiles when validation detail matters.
func LoadStaticPoliciesFromDir(dir string) ([]StaticPolicyRule, error) {
	result, err := LoadStaticPolicyFiles(dir)
	if err != nil {
		return nil, err
	}
	return result.Rules, nil
}

// LoadStaticPolicyFiles loads .yaml and .yml files from dir in lexical order.
func LoadStaticPolicyFiles(dir string) (*StaticPolicyLoadResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read policy directory %s: %w", dir, err)
	}

	result := &StaticPolicyLoadResult{}
	seenRules := map[string]string{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: skipping symlinked policy file", filepath.Join(dir, name)))
			continue
		}

		path := filepath.Join(dir, name)
		// #nosec G304 -- path is assembled from os.ReadDir entries in the operator-selected policy directory; symlinks are skipped above.
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read policy file %s: %w", path, err)
		}

		doc, err := parseStaticPolicyDocument(path, body)
		if err != nil {
			return nil, err
		}

		result.Files = append(result.Files, path)
		if doc.Name == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: missing document name", path))
		}
		if len(doc.Rules) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: no rules", path))
		}

		for i := range doc.Rules {
			rule := doc.Rules[i]
			rule.PolicyFile = filepath.Base(path)
			if rule.Name == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %d has no name", path, i+1))
			} else if firstFile, ok := seenRules[rule.Name]; ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: duplicate rule name %q first seen in %s", path, rule.Name, firstFile))
			} else {
				seenRules[rule.Name] = path
			}
			if rule.Tool == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %q has empty tool", path, rule.Name))
			}
			if rule.Action == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %q has empty action", path, rule.Name))
			}
			if rule.Match != nil && rule.Match.Field == "" && staticMatchRequiresField(*rule.Match) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %q has empty match field", path, rule.Name))
			}
			for _, condition := range rule.Conditions {
				if condition.Field == "" && staticMatchRequiresField(condition) {
					result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %q has empty condition field", path, rule.Name))
				}
			}
			result.Rules = append(result.Rules, rule)
		}
	}

	if len(result.Files) == 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s: no YAML policy files found", dir))
	}

	return result, nil
}

// ParseStaticPolicyDocument parses either the legacy static-policy YAML shape
// or policy schema v1 into Boundary's in-process static rule representation.
func ParseStaticPolicyDocument(path string, body []byte) (*StaticPolicyDocument, error) {
	return parseStaticPolicyDocument(path, body)
}

func staticMatchRequiresField(match StaticPolicyMatch) bool {
	switch strings.ToLower(strings.TrimSpace(match.Type)) {
	case "transport_is", "agent_in", "agent_not_in", "ast_class":
		return false
	default:
		return true
	}
}

func parseStaticPolicyDocument(path string, body []byte) (*StaticPolicyDocument, error) {
	if policyeval.IsPolicyV1YAML(body) {
		doc, err := policyeval.ValidatePolicyV1YAML(path, body)
		if err != nil {
			return nil, err
		}
		return convertPolicyV1Document(doc), nil
	}

	var doc StaticPolicyDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse policy file %s: %w", path, err)
	}
	return &doc, nil
}

func convertPolicyV1Document(doc *policyeval.PolicyV1Document) *StaticPolicyDocument {
	if doc == nil {
		return &StaticPolicyDocument{}
	}
	out := &StaticPolicyDocument{
		Name:    doc.Policy.Name,
		Version: doc.Policy.Version,
		Rules:   make([]StaticPolicyRule, 0, len(doc.Policy.Rules)),
	}
	for _, rule := range doc.Policy.Rules {
		staticRule := StaticPolicyRule{
			Name:         rule.Name,
			Tool:         rule.Tool,
			Action:       strings.ToLower(strings.TrimSpace(rule.Action)),
			Reason:       rule.Reason,
			Transport:    firstNonEmpty(rule.Transport, doc.Policy.Transport),
			DecisionMode: DecisionMode(rule.DecisionMode),
			TenantScope:  append([]string{}, rule.TenantScope...),
			AgentScope:   append([]string{}, rule.AgentScope...),
			Conditions:   make([]StaticPolicyMatch, 0, len(rule.Conditions)),
			Metadata:     rule.Metadata,
		}
		if rule.Match != nil {
			match := convertPolicyV1Condition(*rule.Match)
			staticRule.Match = &match
		}
		for _, condition := range rule.Conditions {
			staticRule.Conditions = append(staticRule.Conditions, convertPolicyV1Condition(condition))
		}
		out.Rules = append(out.Rules, staticRule)
	}
	return out
}

func convertPolicyV1Condition(condition policyeval.PolicyV1Condition) StaticPolicyMatch {
	return StaticPolicyMatch{
		Type:            strings.ToLower(strings.TrimSpace(condition.Type)),
		Field:           condition.Field,
		Contains:        condition.Contains,
		Value:           condition.Value,
		Values:          append([]string{}, condition.Values...),
		Regex:           condition.Regex,
		CaseInsensitive: condition.CaseInsensitive,
	}
}
