package firewall

import "strings"

var riskRank = map[string]int{
	"unknown": 0,
	"R0":      1,
	"R1":      2,
	"W0":      3,
	"W1":      4,
	"W2":      5,
}

func ClassifyServer(server Server) []Capability {
	if len(server.DescriptorTools) > 0 {
		capabilities := make([]Capability, 0, len(server.DescriptorTools))
		for _, name := range server.DescriptorTools {
			capabilities = append(capabilities, ClassifyTool(name))
		}
		return capabilities
	}

	fingerprint := strings.ToLower(server.Name + " " + server.Command + " " + server.URL + " " + strings.Join(server.Args, " "))
	switch {
	case strings.Contains(fingerprint, "github"):
		return githubPreviewCapabilities()
	case containsAny(fingerprint, "filesystem", "file-system", "file_system", "fs-server"):
		return []Capability{
			{Name: "read_file", Category: "filesystem", Class: "R0", SourceClass: "local_file", Reason: "filesystem read can bring local content into agent context"},
			{Name: "write_file", Category: "filesystem", Class: "W1", SinkClass: "local_filesystem", MutationClass: "file_write", Reason: "filesystem write mutates local files"},
			{Name: "delete_file", Category: "filesystem", Class: "W1", SinkClass: "local_filesystem", MutationClass: "file_delete", Reason: "filesystem delete mutates local files"},
		}
	case containsAny(fingerprint, "postgres", "postgresql", "sqlite", "mysql", "database", "db"):
		return []Capability{
			{Name: "query", Category: "database", Class: "W1", SinkClass: "database", MutationClass: "database_query", Reason: "database query tools can read or mutate protected data depending on statement class"},
		}
	case containsAny(fingerprint, "slack", "discord", "email", "gmail"):
		return []Capability{
			{Name: "send_message", Category: "messaging", Class: "W0", SinkClass: "external_publication", MutationClass: "message_send", Reason: "messaging tools can publish agent-controlled content externally"},
		}
	case containsAny(fingerprint, "shell", "terminal", "bash", "powershell", "command", "exec"):
		return []Capability{
			{Name: "run_command", Category: "shell", Class: "W2", SinkClass: "runtime", MutationClass: "command_execution", Reason: "shell tools can execute arbitrary local or remote commands"},
		}
	default:
		return []Capability{
			{Name: "unknown", Category: "unknown", Class: "unknown", Reason: "server could not be classified from descriptor or command name"},
		}
	}
}

func ClassifyTool(name string) Capability {
	switch name {
	case "get_issue", "get_pull_request", "get_file_contents", "get_issue_comments",
		"get_pull_request_comments", "get_pull_request_diff", "get_pull_request_files",
		"list_issues", "list_pull_requests", "search_code", "search_issues", "search_repositories":
		return Capability{Name: name, Category: "github", Class: "R0", SourceClass: "external_collaborator", Reason: "GitHub read tools can introduce untrusted repository or collaborator content"}
	case "add_issue_comment", "create_pending_pull_request_review", "dismiss_notification":
		return Capability{Name: name, Category: "github", Class: "R1", SinkClass: "public_repo", MutationClass: "comment_write", Reason: "low-risk GitHub write still mutates discussion or notification state"}
	case "create_issue", "update_issue", "create_pull_request", "update_pull_request", "create_branch":
		return Capability{Name: name, Category: "github", Class: "W0", SourceClass: "agent_generated", SinkClass: "private_repo", MutationClass: "issue_or_pr_create", Reason: "GitHub W0 tools create or update user-visible repo state"}
	case "create_or_update_file", "push_files", "update_pull_request_branch":
		return Capability{Name: name, Category: "github", Class: "W1", SourceClass: "agent_generated", SinkClass: "private_repo", MutationClass: "private_repo_content_write", Reason: "GitHub W1 tools mutate repository contents"}
	case "merge_pull_request", "create_repository", "fork_repository":
		return Capability{Name: name, Category: "github", Class: "W2", SourceClass: "agent_generated", SinkClass: "private_repo", MutationClass: "merge_or_release", Reason: "GitHub W2 tools are critical repository mutations"}
	case "read_file", "list_directory":
		return Capability{Name: name, Category: "filesystem", Class: "R0", SourceClass: "local_file", Reason: "filesystem reads can add local content to agent context"}
	case "write_file":
		return Capability{Name: name, Category: "filesystem", Class: "W1", SinkClass: "local_filesystem", MutationClass: "file_write", Reason: "filesystem writes mutate local state"}
	case "delete_file":
		return Capability{Name: name, Category: "file_mutation", Class: "W1", SinkClass: "filesystem_or_repository", MutationClass: "file_delete", Reason: "file delete tools mutate local or repository state"}
	case "query":
		return Capability{Name: name, Category: "database", Class: "W1", SinkClass: "database", MutationClass: "database_query", Reason: "database query risk depends on statement class and data scope"}
	default:
		return Capability{Name: name, Category: "unknown", Class: "unknown", Reason: "tool is not in the built-in classifier"}
	}
}

func githubPreviewCapabilities() []Capability {
	tools := []string{
		"get_issue",
		"get_pull_request",
		"get_file_contents",
		"create_issue",
		"create_pull_request",
		"create_or_update_file",
		"push_files",
		"merge_pull_request",
	}
	capabilities := make([]Capability, 0, len(tools))
	for _, tool := range tools {
		capabilities = append(capabilities, ClassifyTool(tool))
	}
	return capabilities
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func highestRisk(capabilities []Capability) string {
	highest := "unknown"
	for _, capability := range capabilities {
		if riskRank[capability.Class] > riskRank[highest] {
			highest = capability.Class
		}
	}
	return highest
}
