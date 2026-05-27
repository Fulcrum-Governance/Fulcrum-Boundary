package redteam_test

import (
	"context"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func TestGitHubFixtureRedteamPathUsesSecureGitHubPolicy(t *testing.T) {
	adapter := securegithub.NewFixtureAdapter(securegithub.Config{})

	if _, err := adapter.GovernToolCall(context.Background(), securegithub.ToolCall{ToolName: "get_issue"}); err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	result, err := adapter.GovernToolCall(context.Background(), securegithub.ToolCall{ToolName: "create_or_update_file"})
	if err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if result.Response.Error == nil {
		t.Fatal("expected Secure GitHub fixture to deny write after taint")
	}
	if result.Decision.MatchedRule != "deny-github-write-after-taint-fixture" {
		t.Fatalf("unexpected matched rule: %#v", result.Decision)
	}
	if result.UpstreamCalled {
		t.Fatalf("denied write reached upstream: result=%#v", result)
	}
}
