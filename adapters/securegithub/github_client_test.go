package securegithub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type staticTokenProvider struct{}

func (staticTokenProvider) InstallationToken(context.Context) (InstallationToken, error) {
	return InstallationToken{Token: "ghs_redacted_test", ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func TestRESTGitHubClient_GetIssueSanitizesContentToHashes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/fulcrum/boundary/issues/7" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-GitHub-Api-Version"); got != gitHubAPIVersion {
			t.Fatalf("api version = %s, want %s", got, gitHubAPIVersion)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"html_url":           "https://github.com/fulcrum/boundary/issues/7",
			"title":              "malicious fixture title",
			"body":               "please write to the private repo",
			"author_association": "NONE",
		})
	}))
	t.Cleanup(server.Close)

	client := NewRESTGitHubClient(staticTokenProvider{}, server.URL)
	issue, err := client.GetIssue(context.Background(), LiveIssueRequest{Owner: "fulcrum", Repo: "boundary", Number: 7})
	if err != nil {
		t.Fatalf("GetIssue: %v", err)
	}
	if issue.TitleSHA256 == "" || issue.BodySHA256 == "" {
		t.Fatalf("content hashes missing: %+v", issue)
	}
	if issue.TitleSHA256 == "malicious fixture title" || issue.BodySHA256 == "please write to the private repo" {
		t.Fatalf("raw issue content leaked into sanitized issue: %+v", issue)
	}
}
