package securegithub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	LiveConformanceSchemaVersion = "boundary.secure_github.live_conformance.v1"
	LiveConformanceReason        = "lethal_trifecta_detected"
)

type LiveConformanceTranscript struct {
	SchemaVersion          string    `json:"schema_version"`
	Sanitized              bool      `json:"sanitized"`
	Mode                   string    `json:"mode"`
	GeneratedAt            time.Time `json:"generated_at"`
	ProfileID              string    `json:"profile_id"`
	ProfileStatus          string    `json:"profile_status"`
	Owner                  string    `json:"owner"`
	Repo                   string    `json:"repo"`
	IssueNumber            int       `json:"issue_number"`
	TaintSourceType        string    `json:"taint_source_type,omitempty"`
	ContentSHA256          string    `json:"content_sha256,omitempty"`
	ReadUpstreamCalled     bool      `json:"read_upstream_called"`
	ExpectedAction         string    `json:"expected_action"`
	ActualAction           string    `json:"actual_action"`
	Reason                 string    `json:"reason"`
	DecisionReason         string    `json:"decision_reason,omitempty"`
	MatchedRule            string    `json:"matched_rule,omitempty"`
	UpstreamCalled         bool      `json:"upstream_called"`
	GitHubMutationCalled   bool      `json:"github_mutation_called"`
	DecisionRecordHash     string    `json:"decision_record_hash,omitempty"`
	TranscriptSHA256       string    `json:"transcript_sha256,omitempty"`
	RawContentIncluded     bool      `json:"raw_content_included"`
	CredentialDataIncluded bool      `json:"credential_data_included"`
}

type LiveConformanceResult struct {
	Transcript       LiveConformanceTranscript
	TranscriptPath   string
	TranscriptSHA256 string
	DecisionRecord   string
}

func RunLiveReadConformance(ctx context.Context, cfg LiveConfig, client GitHubClient) (LiveConformanceResult, error) {
	if !cfg.Enabled {
		return LiveConformanceResult{}, fmt.Errorf("secure GitHub live conformance is not enabled")
	}
	if client == nil {
		return LiveConformanceResult{}, fmt.Errorf("GitHub client is required")
	}
	adapter := NewAdapter(cfg.adapterConfig(), nil, LiveUpstream{Client: client, IssueNumber: cfg.IssueNumber})
	result, err := adapter.GovernToolCall(ctx, liveReadCall(cfg))
	if err != nil {
		return LiveConformanceResult{}, err
	}
	if result.Response.Error != nil {
		return LiveConformanceResult{}, fmt.Errorf("live read conformance was denied: %s", result.Response.Error.Message)
	}
	if !result.UpstreamCalled {
		return LiveConformanceResult{}, fmt.Errorf("live read conformance did not call upstream GitHub read")
	}
	contentSHA, _ := result.Response.Result.StructuredContent["content_sha256"].(string)
	transcript := newBaseLiveTranscript(cfg, "live-read")
	transcript.ContentSHA256 = contentSHA
	transcript.TaintSourceType = "github.issue_body"
	transcript.ReadUpstreamCalled = true
	transcript.ExpectedAction = "ALLOW"
	transcript.ActualAction = "ALLOW"
	transcript.Reason = "live_read_completed"
	transcript.DecisionReason = result.Decision.Reason
	transcript.MatchedRule = result.Decision.MatchedRule
	transcript.UpstreamCalled = result.UpstreamCalled
	transcript.DecisionRecordHash = result.DecisionRecord.DecisionHash
	return writeConformanceTranscript(cfg, transcript, "live-read.sanitized.json")
}

func RunLiveDeniedWriteConformance(ctx context.Context, cfg LiveConfig, client GitHubClient) (LiveConformanceResult, error) {
	if !cfg.Enabled {
		return LiveConformanceResult{}, fmt.Errorf("secure GitHub live conformance is not enabled")
	}
	if client == nil {
		return LiveConformanceResult{}, fmt.Errorf("GitHub client is required")
	}
	instrumented := NewInstrumentedGitHubClient(client)
	adapter := NewAdapter(cfg.adapterConfig(), nil, LiveUpstream{Client: instrumented, IssueNumber: cfg.IssueNumber})
	read, err := adapter.GovernToolCall(ctx, liveReadCall(cfg))
	if err != nil {
		return LiveConformanceResult{}, err
	}
	if read.Response.Error != nil {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write pre-read was denied: %s", read.Response.Error.Message)
	}
	if !read.UpstreamCalled {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write pre-read did not call upstream GitHub read")
	}
	write, err := adapter.GovernToolCall(ctx, liveWriteCall(cfg))
	if err != nil {
		return LiveConformanceResult{}, err
	}
	if write.Response.Error == nil {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write conformance allowed protected mutation")
	}
	if write.Decision == nil || write.Decision.Action != "deny" {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write conformance expected deny, got %+v", write.Decision)
	}
	if write.UpstreamCalled {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write conformance called upstream after deny")
	}
	if instrumented.MutationCalled() {
		return LiveConformanceResult{}, fmt.Errorf("live denied-write conformance reached GitHub mutation client")
	}
	contentSHA := ""
	if read.Response.Result != nil {
		contentSHA, _ = read.Response.Result.StructuredContent["content_sha256"].(string)
	}
	transcript := newBaseLiveTranscript(cfg, "denied-write-after-taint")
	transcript.ContentSHA256 = contentSHA
	transcript.TaintSourceType = "github.issue_body"
	transcript.ReadUpstreamCalled = read.UpstreamCalled
	transcript.ExpectedAction = "DENY"
	transcript.ActualAction = "DENY"
	transcript.Reason = LiveConformanceReason
	transcript.DecisionReason = write.Decision.Reason
	transcript.MatchedRule = write.Decision.MatchedRule
	transcript.UpstreamCalled = write.UpstreamCalled
	transcript.GitHubMutationCalled = instrumented.MutationCalled()
	transcript.DecisionRecordHash = write.DecisionRecord.DecisionHash
	return writeConformanceTranscript(cfg, transcript, "denied-write-after-taint.sanitized.json")
}

func newBaseLiveTranscript(cfg LiveConfig, mode string) LiveConformanceTranscript {
	return LiveConformanceTranscript{
		SchemaVersion:          LiveConformanceSchemaVersion,
		Sanitized:              true,
		Mode:                   mode,
		GeneratedAt:            time.Now().UTC(),
		ProfileID:              ProfileID,
		ProfileStatus:          StatusPreview,
		Owner:                  cfg.Owner,
		Repo:                   cfg.Repo,
		IssueNumber:            cfg.IssueNumber,
		RawContentIncluded:     false,
		CredentialDataIncluded: false,
	}
}

func writeConformanceTranscript(cfg LiveConfig, transcript LiveConformanceTranscript, name string) (LiveConformanceResult, error) {
	if err := assertTranscriptSanitized(transcript); err != nil {
		return LiveConformanceResult{}, err
	}
	dir := firstNonEmpty(cfg.TranscriptDir, DefaultGitHubTranscriptDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return LiveConformanceResult{}, fmt.Errorf("create Secure GitHub conformance transcript directory: %w", err)
	}
	transcript.TranscriptSHA256 = ""
	body, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return LiveConformanceResult{}, err
	}
	sum := sha256.Sum256(body)
	transcript.TranscriptSHA256 = hex.EncodeToString(sum[:])
	finalBody, err := json.MarshalIndent(transcript, "", "  ")
	if err != nil {
		return LiveConformanceResult{}, err
	}
	if containsSecretLikeData(string(finalBody)) {
		return LiveConformanceResult{}, fmt.Errorf("refusing to write Secure GitHub transcript with secret-like data")
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, finalBody, 0o600); err != nil {
		return LiveConformanceResult{}, fmt.Errorf("write Secure GitHub conformance transcript: %w", err)
	}
	return LiveConformanceResult{
		Transcript:       transcript,
		TranscriptPath:   path,
		TranscriptSHA256: transcript.TranscriptSHA256,
		DecisionRecord:   transcript.DecisionRecordHash,
	}, nil
}

func assertTranscriptSanitized(transcript LiveConformanceTranscript) error {
	if !transcript.Sanitized {
		return fmt.Errorf("secure GitHub transcript must declare sanitized=true")
	}
	if transcript.RawContentIncluded || transcript.CredentialDataIncluded {
		return fmt.Errorf("secure GitHub transcript contains raw content or credential data")
	}
	return nil
}

func liveReadCall(cfg LiveConfig) ToolCall {
	return ToolCall{
		ID:        "secure-github-live-read",
		ToolName:  "get_issue",
		SessionID: cfg.adapterConfig().SessionID,
		AgentID:   cfg.adapterConfig().AgentID,
		TenantID:  cfg.adapterConfig().TenantID,
		TraceID:   "trace-secure-github-live-conformance",
		Arguments: map[string]any{
			"owner":                cfg.Owner,
			"repo":                 cfg.Repo,
			"issue_number":         cfg.IssueNumber,
			"source_class":         "external_collaborator",
			"request_id":           "secure-github-live-read",
			"envelope_id":          "env-secure-github-live-read",
			"live_github_evidence": true,
		},
	}
}

func liveWriteCall(cfg LiveConfig) ToolCall {
	return ToolCall{
		ID:        "secure-github-live-denied-write",
		ToolName:  "create_or_update_file",
		SessionID: cfg.adapterConfig().SessionID,
		AgentID:   cfg.adapterConfig().AgentID,
		TenantID:  cfg.adapterConfig().TenantID,
		TraceID:   "trace-secure-github-live-conformance",
		Arguments: map[string]any{
			"owner":                cfg.Owner,
			"repo":                 cfg.Repo,
			"path":                 ".boundary-live-conformance-denied.txt",
			"message":              "Boundary denied live conformance mutation",
			"content":              "this content must never be written by denied conformance",
			"private":              true,
			"source_class":         "agent_generated",
			"target_sink":          "private_repo",
			"request_id":           "secure-github-live-denied-write",
			"envelope_id":          "env-secure-github-live-denied-write",
			"live_github_evidence": true,
		},
	}
}
