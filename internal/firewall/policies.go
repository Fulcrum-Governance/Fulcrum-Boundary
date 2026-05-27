package firewall

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type PolicyTemplate struct {
	Name        string `json:"name"`
	Filename    string `json:"filename"`
	Description string `json:"description"`
	Body        string `json:"-"`
}

type PolicyGenerationResult struct {
	SchemaVersion string           `json:"schema_version"`
	Directory     string           `json:"directory"`
	Mode          string           `json:"mode"`
	Templates     []PolicyTemplate `json:"templates"`
	Files         []string         `json:"files"`
}

func StarterPolicyTemplates() []PolicyTemplate {
	return []PolicyTemplate{
		{
			Name:        "filesystem",
			Filename:    "filesystem.yaml",
			Description: "Restricts local filesystem reads and denies local file mutation starter paths.",
			Body: `schema_version: "1"
policy:
  name: firewall-filesystem-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: audit-local-file-read
      tool: read_file
      action: audit
      reason: Local file reads can move private local content into agent context.
      metadata:
        template: filesystem
        graph_path: filesystem_exfil
    - name: deny-local-file-write
      tool: write_file
      action: deny
      reason: Local filesystem writes are denied by the starter policy.
      metadata:
        template: filesystem
        graph_path: filesystem_mutation
    - name: deny-local-file-delete
      tool: delete_file
      action: deny
      reason: Local filesystem deletes are denied by the starter policy.
      metadata:
        template: filesystem
        graph_path: filesystem_mutation
`,
		},
		{
			Name:        "github",
			Filename:    "github.yaml",
			Description: "Denies high-impact GitHub repository mutation starter paths.",
			Body: `schema_version: "1"
policy:
  name: firewall-github-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: deny-github-file-write
      tool: create_or_update_file
      action: deny
      reason: Private repository content writes require an explicit operator policy.
      metadata:
        template: github
        graph_path: repo_write_path
    - name: deny-github-push-files
      tool: push_files
      action: deny
      reason: Multi-file repository pushes require an explicit operator policy.
      metadata:
        template: github
        graph_path: repo_write_path
    - name: deny-github-merge
      tool: merge_pull_request
      action: deny
      reason: Pull-request merges are W2 mutations and are denied by default.
      metadata:
        template: github
        graph_path: privileged_mutation
`,
		},
		{
			Name:        "postgres",
			Filename:    "postgres.yaml",
			Description: "Denies destructive SQL classes and common destructive SQL text patterns.",
			Body: `schema_version: "1"
policy:
  name: firewall-postgres-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: deny-destructive-sql-class
      tool: query
      action: deny
      reason: Destructive SQL classes are denied before upstream execution.
      metadata:
        template: postgres
        graph_path: destructive_db_action
      match:
        type: ast_class
        value: DESTRUCTIVE
    - name: deny-drop-table-text
      tool: query
      action: deny
      reason: DROP TABLE is denied by the starter policy.
      metadata:
        template: postgres
        graph_path: destructive_db_action
      match:
        type: contains
        field: arguments.sql
        contains: DROP TABLE
        case_insensitive: true
    - name: deny-truncate-text
      tool: query
      action: deny
      reason: TRUNCATE is denied by the starter policy.
      metadata:
        template: postgres
        graph_path: destructive_db_action
      match:
        type: contains
        field: arguments.sql
        contains: TRUNCATE
        case_insensitive: true
`,
		},
		{
			Name:        "slack",
			Filename:    "slack.yaml",
			Description: "Requires approval before external message publication.",
			Body: `schema_version: "1"
policy:
  name: firewall-slack-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: require-approval-for-message-send
      tool: send_message
      action: require_approval
      reason: External publication paths require operator approval in the starter policy.
      metadata:
        template: slack
        graph_path: external_sink
`,
		},
		{
			Name:        "shell",
			Filename:    "shell.yaml",
			Description: "Denies arbitrary shell or command execution starter paths.",
			Body: `schema_version: "1"
policy:
  name: firewall-shell-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: deny-command-execution
      tool: run_command
      action: deny
      reason: Shell command execution is W2 and denied by the starter policy.
      metadata:
        template: shell
        graph_path: privileged_mutation
`,
		},
		{
			Name:        "descriptor-integrity",
			Filename:    "descriptor-integrity.yaml",
			Description: "Requires approval when a routed request reports descriptor drift.",
			Body: `schema_version: "1"
policy:
  name: firewall-descriptor-integrity-starter
  version: "0.1.0"
  transport: mcp
  rules:
    - name: require-approval-on-descriptor-change
      tool: "*"
      action: require_approval
      reason: Descriptor changes can alter tool capabilities and policy projection.
      metadata:
        template: descriptor-integrity
        graph_path: descriptor_change
      match:
        type: equals
        field: arguments.descriptor_changed
        value: "true"
`,
		},
	}
}

func GenerateStarterPolicies(outDir string, force bool, mode string) (PolicyGenerationResult, error) {
	if outDir == "" {
		outDir = "boundary-firewall-policies"
	}
	if mode == "" {
		mode = "balanced"
	}
	if mode != "balanced" {
		return PolicyGenerationResult{}, fmt.Errorf("unsupported policy generation mode %q; supported mode: balanced", mode)
	}
	absDir, err := filepath.Abs(outDir)
	if err != nil {
		return PolicyGenerationResult{}, err
	}
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return PolicyGenerationResult{}, fmt.Errorf("create policy directory: %w", err)
	}

	templates := StarterPolicyTemplates()
	result := PolicyGenerationResult{
		SchemaVersion: "boundary.firewall.policy_generation.v1",
		Directory:     absDir,
		Mode:          mode,
		Templates:     templates,
	}
	for _, template := range templates {
		path := filepath.Join(absDir, template.Filename)
		if !force {
			if _, err := os.Stat(path); err == nil {
				return PolicyGenerationResult{}, fmt.Errorf("%s already exists; use --force to overwrite", path)
			} else if !os.IsNotExist(err) {
				return PolicyGenerationResult{}, fmt.Errorf("stat policy file %s: %w", path, err)
			}
		}
		if err := os.WriteFile(path, []byte(template.Body), 0o600); err != nil {
			return PolicyGenerationResult{}, fmt.Errorf("write policy file %s: %w", path, err)
		}
		result.Files = append(result.Files, path)
	}
	sort.Strings(result.Files)
	return result, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
