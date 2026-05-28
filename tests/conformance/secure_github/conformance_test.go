package securegithubconformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/securegithub"
)

func TestLiveDeniedWriteTranscriptShape(t *testing.T) {
	tr := loadTranscript(t)
	if tr.SchemaVersion != securegithub.LiveConformanceSchemaVersion {
		t.Fatalf("schema_version = %q", tr.SchemaVersion)
	}
	if tr.Mode != "denied-write-after-taint" {
		t.Fatalf("mode = %q, want denied-write-after-taint", tr.Mode)
	}
	if tr.ProfileID != securegithub.ProfileID || tr.ProfileStatus != securegithub.StatusPreview {
		t.Fatalf("unexpected profile metadata: %+v", tr)
	}
	if tr.Owner == "" || tr.Repo == "" || tr.IssueNumber <= 0 {
		t.Fatalf("target repo metadata missing: %+v", tr)
	}
}

func TestLiveReadTaintEvidence(t *testing.T) {
	tr := loadTranscript(t)
	if !tr.ReadUpstreamCalled {
		t.Fatal("live read did not call upstream GitHub read")
	}
	if tr.TaintSourceType != "github.issue_body" {
		t.Fatalf("taint source = %q", tr.TaintSourceType)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(tr.ContentSHA256) {
		t.Fatalf("content_sha256 is not a sha256 hex digest: %q", tr.ContentSHA256)
	}
}

func TestDeniedWriteNoMutationEvidence(t *testing.T) {
	tr := loadTranscript(t)
	if tr.ExpectedAction != "DENY" || tr.ActualAction != "DENY" {
		t.Fatalf("expected/actual action mismatch: %+v", tr)
	}
	if tr.Reason != securegithub.LiveConformanceReason {
		t.Fatalf("reason = %q", tr.Reason)
	}
	if tr.UpstreamCalled {
		t.Fatal("denied write reports upstream_called=true")
	}
	if tr.GitHubMutationCalled {
		t.Fatal("denied write reports github_mutation_called=true")
	}
	if tr.MatchedRule == "" || tr.DecisionRecordHash == "" {
		t.Fatalf("decision evidence missing: %+v", tr)
	}
}

func TestSanitizedTranscriptEvidence(t *testing.T) {
	tr, data := loadTranscriptBytes(t)
	if !tr.Sanitized {
		t.Fatal("transcript does not declare sanitized=true")
	}
	if tr.RawContentIncluded || tr.CredentialDataIncluded {
		t.Fatalf("raw content or credential data included: %+v", tr)
	}
	if strings.Contains(strings.ToLower(string(data)), "-----begin") ||
		strings.Contains(strings.ToLower(string(data)), "authorization") ||
		strings.Contains(strings.ToLower(string(data)), "bearer ") {
		t.Fatal("transcript contains secret-like data")
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(tr.TranscriptSHA256) {
		t.Fatalf("transcript_sha256 is not a sha256 hex digest: %q", tr.TranscriptSHA256)
	}
}

func loadTranscript(t *testing.T) securegithub.LiveConformanceTranscript {
	t.Helper()
	tr, _ := loadTranscriptBytes(t)
	return tr
}

func loadTranscriptBytes(t *testing.T) (securegithub.LiveConformanceTranscript, []byte) {
	t.Helper()
	if os.Getenv(securegithub.EnvGitHubConformance) != "true" {
		t.Skip(securegithub.EnvGitHubConformance + " not set")
	}
	path := os.Getenv(securegithub.EnvGitHubTranscript)
	if path == "" {
		t.Fatal(securegithub.EnvGitHubTranscript + " not set; run boundary secure github conformance denied-write and point this env var at the sanitized transcript")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var tr securegithub.LiveConformanceTranscript
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("parse transcript: %v", err)
	}
	return tr, data
}
