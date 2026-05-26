package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

		path := filepath.Join(dir, name)
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read policy file %s: %w", path, err)
		}

		var doc StaticPolicyDocument
		if err := yaml.Unmarshal(body, &doc); err != nil {
			return nil, fmt.Errorf("parse policy file %s: %w", path, err)
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
			if rule.Match != nil && rule.Match.Field == "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: rule %q has empty match field", path, rule.Name))
			}
			for _, condition := range rule.Conditions {
				if condition.Field == "" {
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
