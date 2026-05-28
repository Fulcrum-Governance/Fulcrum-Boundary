package securegithub

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeLiveGitHubClient struct {
	issueCalls    int
	mutationCalls int
}

func (f *fakeLiveGitHubClient) GetIssue(_ context.Context, req LiveIssueRequest) (LiveIssue, error) {
	f.issueCalls++
	return LiveIssue{
		Owner:             req.Owner,
		Repo:              req.Repo,
		Number:            req.Number,
		URL:               "https://github.com/fulcrum/boundary/issues/7",
		AuthorAssociation: "NONE",
		TitleSHA256:       sha256Hex("external issue title"),
		BodySHA256:        sha256Hex("untrusted issue body"),
		FetchedAt:         time.Now().UTC(),
	}, nil
}

func (f *fakeLiveGitHubClient) CreateOrUpdateFile(context.Context, LiveFileMutationRequest) error {
	f.mutationCalls++
	return nil
}

func TestRunLiveReadConformanceWritesSanitizedTranscript(t *testing.T) {
	cfg := testLiveConfig(t)
	client := &fakeLiveGitHubClient{}
	result, err := RunLiveReadConformance(context.Background(), cfg, client)
	if err != nil {
		t.Fatalf("RunLiveReadConformance: %v", err)
	}
	if client.issueCalls != 1 || client.mutationCalls != 0 {
		t.Fatalf("unexpected calls: issue=%d mutation=%d", client.issueCalls, client.mutationCalls)
	}
	if result.Transcript.Mode != "live-read" || !result.Transcript.ReadUpstreamCalled {
		t.Fatalf("unexpected transcript: %+v", result.Transcript)
	}
	assertSanitizedTranscriptFile(t, result.TranscriptPath)
}

func TestRunLiveDeniedWriteConformanceDoesNotCallMutationClient(t *testing.T) {
	cfg := testLiveConfig(t)
	client := &fakeLiveGitHubClient{}
	result, err := RunLiveDeniedWriteConformance(context.Background(), cfg, client)
	if err != nil {
		t.Fatalf("RunLiveDeniedWriteConformance: %v", err)
	}
	if client.issueCalls != 1 {
		t.Fatalf("issue calls = %d, want 1", client.issueCalls)
	}
	if client.mutationCalls != 0 {
		t.Fatalf("mutation was called after deny")
	}
	tr := result.Transcript
	if tr.ExpectedAction != "DENY" || tr.ActualAction != "DENY" || tr.UpstreamCalled || tr.GitHubMutationCalled {
		t.Fatalf("unexpected denied-write transcript: %+v", tr)
	}
	if tr.Reason != LiveConformanceReason {
		t.Fatalf("reason = %s, want %s", tr.Reason, LiveConformanceReason)
	}
	if tr.MatchedRule != "deny-github-write-after-taint-fixture" {
		t.Fatalf("matched rule = %s", tr.MatchedRule)
	}
	assertSanitizedTranscriptFile(t, result.TranscriptPath)
}

func testLiveConfig(t *testing.T) LiveConfig {
	t.Helper()
	return LiveConfig{
		Enabled:        true,
		AppID:          123,
		InstallationID: 456,
		PrivateKeyPath: "/redacted/github-app.pem",
		Owner:          "fulcrum",
		Repo:           "boundary",
		IssueNumber:    7,
		APIBaseURL:     DefaultGitHubAPIBaseURL,
		TranscriptDir:  t.TempDir(),
	}
}

func assertSanitizedTranscriptFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if containsSecretLikeData(string(data)) {
		t.Fatalf("transcript contains secret-like data: %s", string(data))
	}
	if strings.Contains(string(data), "untrusted issue body") || strings.Contains(string(data), "external issue title") {
		t.Fatalf("transcript leaked raw issue content: %s", string(data))
	}
	var tr LiveConformanceTranscript
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("parse transcript: %v", err)
	}
	if !tr.Sanitized || tr.RawContentIncluded || tr.CredentialDataIncluded || tr.TranscriptSHA256 == "" {
		t.Fatalf("transcript not sanitized: %+v", tr)
	}
}
