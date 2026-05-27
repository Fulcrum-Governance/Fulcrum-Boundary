package securegithub

import (
	"fmt"
	"strings"
)

type classification struct {
	ToolName          string
	Action            string
	CapabilityClass   string
	SourceClass       string
	TargetSink        string
	MutationClass     string
	TaintsEnvelope    bool
	TaintSource       string
	ProtectedMutation bool
}

func classifyTool(tool string, args map[string]any) (classification, error) {
	name := normalizeToolName(tool)
	if name == "" {
		return classification{}, fmt.Errorf("github tool name is required")
	}
	out := classification{
		ToolName:      name,
		Action:        "github." + name,
		SourceClass:   "agent_generated",
		TargetSink:    "none",
		MutationClass: "none",
	}
	switch name {
	case "get_issue":
		out.CapabilityClass = "R0"
		out.SourceClass = sourceClass(args, "external_collaborator")
		out.TaintsEnvelope = out.SourceClass == "external_collaborator" || out.SourceClass == "public_resource"
		out.TaintSource = "github.issue_body"
	case "get_pull_request":
		out.CapabilityClass = "R0"
		out.SourceClass = sourceClass(args, "external_collaborator")
		out.TaintsEnvelope = out.SourceClass == "external_collaborator" || out.SourceClass == "public_resource"
		out.TaintSource = "github.pull_request_body"
	case "get_file_contents":
		out.CapabilityClass = "R0"
		out.SourceClass = sourceClass(args, "allowlisted_resource")
		out.TaintsEnvelope = out.SourceClass == "public_resource" || out.SourceClass == "external_collaborator"
		out.TaintSource = "github.file_contents"
	case "create_issue", "create_pull_request":
		out.CapabilityClass = "W0"
		out.SourceClass = sourceClass(args, "agent_generated")
		out.TargetSink = sinkClass(args)
		out.MutationClass = "issue_or_pr_create"
	case "create_or_update_file", "push_files":
		out.CapabilityClass = "W1"
		out.SourceClass = sourceClass(args, "agent_generated")
		out.TargetSink = sinkClass(args)
		out.MutationClass = "private_repo_content_write"
		out.ProtectedMutation = true
	case "merge_pull_request":
		out.CapabilityClass = "W2"
		out.SourceClass = sourceClass(args, "agent_generated")
		out.TargetSink = sinkClass(args)
		out.MutationClass = "merge_or_release"
		out.ProtectedMutation = true
	default:
		return classification{}, fmt.Errorf("unsupported Secure GitHub MCP tool %q", name)
	}
	return out, nil
}

func normalizeToolName(tool string) string {
	tool = strings.TrimSpace(tool)
	tool = strings.TrimPrefix(tool, "github.")
	return tool
}

func sourceClass(args map[string]any, fallback string) string {
	if got := stringArg(args, "source_class"); got != "" {
		return got
	}
	if got := stringArg(args, "source"); got != "" {
		return got
	}
	if strings.EqualFold(stringArg(args, "author_association"), "MEMBER") ||
		strings.EqualFold(stringArg(args, "author_association"), "OWNER") ||
		strings.EqualFold(stringArg(args, "author_association"), "COLLABORATOR") ||
		boolArg(args, "collaborator") {
		return "allowlisted_resource"
	}
	return fallback
}

func sinkClass(args map[string]any) string {
	if got := stringArg(args, "target_sink"); got != "" {
		return got
	}
	if got := stringArg(args, "sink"); got != "" {
		return got
	}
	if v, ok := args["private"]; ok && !truthy(v) {
		return "public_repo"
	}
	return "private_repo"
}

func ownerRepo(args map[string]any, cfg Config) (owner string, repo string) {
	owner = stringArg(args, "owner")
	repo = stringArg(args, "repo")
	if owner == "" {
		owner = cfg.Owner
	}
	if repo == "" {
		repo = cfg.Repo
	}
	return owner, repo
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, ok := args[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	value, ok := args[key]
	if !ok {
		return false
	}
	return truthy(value)
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.EqualFold(strings.TrimSpace(typed), "yes")
	default:
		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(typed)), "true")
	}
}
