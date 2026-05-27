package securegithub

import (
	"context"
	"strings"
	"testing"
)

func TestSecureGitHubDeniesW1WriteAfterTaintBeforeUpstream(t *testing.T) {
	var calls []Envelope
	adapter := NewFixtureAdapter(Config{})
	adapter.upstream = FixtureUpstream{Calls: &calls}

	read, err := adapter.GovernToolCall(context.Background(), ToolCall{
		ID:       "read-1",
		ToolName: "get_issue",
		Arguments: map[string]any{
			"owner":              DefaultOwner,
			"repo":               DefaultRepo,
			"author_association": "CONTRIBUTOR",
		},
	})
	if err != nil {
		t.Fatalf("read call: %v", err)
	}
	if read.Response.Error != nil || !read.UpstreamCalled {
		t.Fatalf("read should be allowed and forwarded: %#v", read)
	}
	if len(calls) != 1 {
		t.Fatalf("read upstream calls = %d, want 1", len(calls))
	}

	write, err := adapter.GovernToolCall(context.Background(), ToolCall{
		ID:       "write-1",
		ToolName: "create_or_update_file",
		Arguments: map[string]any{
			"owner":   DefaultOwner,
			"repo":    DefaultRepo,
			"path":    "README.md",
			"private": true,
		},
	})
	if err != nil {
		t.Fatalf("write call: %v", err)
	}
	if write.Response.Error == nil {
		t.Fatalf("write should be denied: %#v", write.Response)
	}
	if write.UpstreamCalled || len(calls) != 1 {
		t.Fatalf("denied write reached upstream: result=%#v calls=%d", write, len(calls))
	}
	if write.Decision.Action != "deny" || write.Decision.MatchedRule != "deny-github-write-after-taint-fixture" {
		t.Fatalf("unexpected decision: %#v", write.Decision)
	}
	data := write.Response.Error.Data
	for key, want := range map[string]string{
		"target_repo":      DefaultOwner + "/" + DefaultRepo,
		"target_sink":      "private_repo",
		"capability_class": "W1",
		"risk_class":       "W1",
		"mutation_class":   "private_repo_content_write",
	} {
		if got := data[key]; got != want {
			t.Fatalf("denial data %s = %#v, want %q; data=%#v", key, got, want, data)
		}
	}
	if sources, ok := data["taint_sources"].([]string); !ok || len(sources) != 1 || sources[0] != "github.issue_body" {
		t.Fatalf("taint sources not recorded: %#v", data["taint_sources"])
	}
	if write.DecisionRecord.RecordID == "" || !strings.HasPrefix(write.DecisionRecord.DecisionHash, "sha256:") {
		t.Fatalf("decision record missing receipt fields: %#v", write.DecisionRecord)
	}
}

func TestSecureGitHubDeniesW2AfterTaint(t *testing.T) {
	adapter := NewFixtureAdapter(Config{})
	if _, err := adapter.GovernToolCall(context.Background(), ToolCall{ToolName: "get_pull_request"}); err != nil {
		t.Fatalf("read call: %v", err)
	}
	result, err := adapter.GovernToolCall(context.Background(), ToolCall{ToolName: "merge_pull_request"})
	if err != nil {
		t.Fatalf("merge call: %v", err)
	}
	if result.Response.Error == nil {
		t.Fatalf("merge should be denied: %#v", result.Response)
	}
	if result.Response.Error.Data["capability_class"] != "W2" {
		t.Fatalf("expected W2 denial metadata: %#v", result.Response.Error.Data)
	}
	if result.Decision.MatchedRule != "deny-github-critical-write-after-taint-fixture" {
		t.Fatalf("unexpected matched rule %q", result.Decision.MatchedRule)
	}
}

func TestSecureGitHubOneRepoPerSessionDeniesRepoSwitch(t *testing.T) {
	var calls []Envelope
	adapter := NewFixtureAdapter(Config{})
	adapter.upstream = FixtureUpstream{Calls: &calls}

	if _, err := adapter.GovernToolCall(context.Background(), ToolCall{
		ToolName:  "get_file_contents",
		SessionID: "repo-bound",
		Arguments: map[string]any{
			"owner": DefaultOwner,
			"repo":  DefaultRepo,
		},
	}); err != nil {
		t.Fatalf("first repo call: %v", err)
	}
	result, err := adapter.GovernToolCall(context.Background(), ToolCall{
		ToolName:  "get_file_contents",
		SessionID: "repo-bound",
		Arguments: map[string]any{
			"owner": "other-org",
			"repo":  "other-repo",
		},
	})
	if err != nil {
		t.Fatalf("repo switch call: %v", err)
	}
	if result.Response.Error == nil {
		t.Fatalf("repo switch should be denied: %#v", result.Response)
	}
	if result.UpstreamCalled || len(calls) != 1 {
		t.Fatalf("repo switch reached upstream: calls=%d result=%#v", len(calls), result)
	}
	if result.Decision.MatchedRule != "deny-github-repo-scope-violation" {
		t.Fatalf("unexpected rule: %#v", result.Decision)
	}
}

func TestSecureGitHubUnsupportedToolFailsClosed(t *testing.T) {
	adapter := NewFixtureAdapter(Config{})
	result, err := adapter.GovernToolCall(context.Background(), ToolCall{ToolName: "create_repository"})
	if err != nil {
		t.Fatalf("unsupported call: %v", err)
	}
	if result.Response.Error == nil {
		t.Fatalf("unsupported tool should return MCP-shaped error: %#v", result.Response)
	}
	if got := result.Response.Error.Data["upstream_called"]; got != false {
		t.Fatalf("unsupported call should not reach upstream: %#v", result.Response.Error.Data)
	}
}
