package securegithub

import (
	"context"
	"fmt"
)

type FixtureUpstream struct {
	Calls *[]Envelope
}

func (f FixtureUpstream) CallGitHub(_ context.Context, call ToolCall, envelope Envelope) (*MCPResult, error) {
	if f.Calls != nil {
		*f.Calls = append(*f.Calls, envelope)
	}
	text := fmt.Sprintf("fixture GitHub %s for %s", envelope.ToolName, envelope.TargetRepo())
	return &MCPResult{
		Content: []MCPContent{{Type: "text", Text: text}},
		StructuredContent: map[string]any{
			"profile_id":       ProfileID,
			"profile_status":   StatusPreview,
			"tool":             envelope.ToolName,
			"target_repo":      envelope.TargetRepo(),
			"fixture_mode":     true,
			"live_github_call": false,
			"request_id":       call.ID,
		},
	}, nil
}
