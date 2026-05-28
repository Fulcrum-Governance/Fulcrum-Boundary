package securegithub

import (
	"context"
	"fmt"
)

type LiveUpstream struct {
	Client      GitHubClient
	IssueNumber int
}

func (u LiveUpstream) CallGitHub(ctx context.Context, call ToolCall, envelope Envelope) (*MCPResult, error) {
	if u.Client == nil {
		return nil, fmt.Errorf("live GitHub client is required")
	}
	switch envelope.ToolName {
	case "get_issue":
		issue, err := u.Client.GetIssue(ctx, LiveIssueRequest{
			Owner:  envelope.Owner,
			Repo:   envelope.Repo,
			Number: issueNumberFromArgs(call.Arguments, u.IssueNumber),
		})
		if err != nil {
			return nil, err
		}
		return &MCPResult{
			Content: []MCPContent{{
				Type: "text",
				Text: fmt.Sprintf("live GitHub issue read for %s/%s#%d; body_sha256=%s", issue.Owner, issue.Repo, issue.Number, issue.BodySHA256),
			}},
			StructuredContent: map[string]any{
				"profile_id":         ProfileID,
				"profile_status":     StatusPreview,
				"tool":               envelope.ToolName,
				"target_repo":        envelope.TargetRepo(),
				"fixture_mode":       false,
				"live_github_call":   true,
				"issue_number":       issue.Number,
				"issue_url":          issue.URL,
				"author_association": issue.AuthorAssociation,
				"content_sha256":     issue.BodySHA256,
				"title_sha256":       issue.TitleSHA256,
				"taint_source_type":  "github.issue_body",
			},
		}, nil
	case "create_or_update_file":
		err := u.Client.CreateOrUpdateFile(ctx, LiveFileMutationRequest{
			Owner:   envelope.Owner,
			Repo:    envelope.Repo,
			Path:    stringArg(call.Arguments, "path"),
			Message: stringArg(call.Arguments, "message"),
			Content: stringArg(call.Arguments, "content"),
			Branch:  stringArg(call.Arguments, "branch"),
		})
		if err != nil {
			return nil, err
		}
		return &MCPResult{
			Content: []MCPContent{{Type: "text", Text: "live GitHub file mutation completed"}},
			StructuredContent: map[string]any{
				"profile_id":       ProfileID,
				"profile_status":   StatusPreview,
				"tool":             envelope.ToolName,
				"target_repo":      envelope.TargetRepo(),
				"fixture_mode":     false,
				"live_github_call": true,
			},
		}, nil
	default:
		return nil, fmt.Errorf("live GitHub upstream does not implement tool %q", envelope.ToolName)
	}
}
