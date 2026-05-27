package managedagentsconformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	enableEnv     = "BOUNDARY_MA_CONFORMANCE"
	transcriptEnv = "BOUNDARY_MA_TRANSCRIPT"
)

type transcript struct {
	Sanitized      bool               `json:"sanitized"`
	SessionCreated bool               `json:"session_created_through_boundary"`
	SessionID      string             `json:"session_id"`
	ThreadID       string             `json:"thread_id"`
	AgentID        string             `json:"agent_id"`
	Events         []transcriptEvent  `json:"events"`
	Confirmations  []confirmation     `json:"confirmations"`
	Decisions      []decisionRecord   `json:"decisions"`
	Budget         budgetEvidence     `json:"budget"`
	Trust          trustEvidence      `json:"trust"`
	FailClosed     failClosedEvidence `json:"fail_closed"`
	TranscriptHash string             `json:"transcript_sha256,omitempty"`
}

type transcriptEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	Tool      string `json:"tool,omitempty"`
}

type confirmation struct {
	ToolUseID string `json:"tool_use_id"`
	Result    string `json:"result"`
	Tool      string `json:"tool,omitempty"`
}

type decisionRecord struct {
	AgentID   string  `json:"agent_id"`
	SessionID string  `json:"session_id"`
	ThreadID  string  `json:"thread_id"`
	Tool      string  `json:"tool"`
	Action    string  `json:"action"`
	Rule      string  `json:"rule"`
	Trust     float64 `json:"trust"`
	RequestID string  `json:"request_id,omitempty"`
	Envelope  string  `json:"envelope_id,omitempty"`
}

type budgetEvidence struct {
	Ceiling float64 `json:"ceiling"`
	Used    float64 `json:"used"`
}

type trustEvidence struct {
	Tracked bool    `json:"tracked"`
	Score   float64 `json:"score"`
}

type failClosedEvidence struct {
	Observed bool   `json:"observed"`
	Action   string `json:"action"`
	Reason   string `json:"reason,omitempty"`
}

func TestSessionCreationThroughBoundaryProxy(t *testing.T) {
	tr := loadTranscript(t)
	if !tr.SessionCreated || tr.SessionID == "" {
		t.Fatalf("session creation through Boundary proxy not recorded: %+v", tr)
	}
}

func TestToolConfirmationAllow(t *testing.T) {
	tr := loadTranscript(t)
	if !hasConfirmation(tr, "allow") {
		t.Fatalf("no allow confirmation recorded: %+v", tr.Confirmations)
	}
}

func TestToolConfirmationDeny(t *testing.T) {
	tr := loadTranscript(t)
	if !hasConfirmation(tr, "deny") {
		t.Fatalf("no deny confirmation recorded: %+v", tr.Confirmations)
	}
}

func TestMCPToolUseEvent(t *testing.T) {
	tr := loadTranscript(t)
	if !hasEvent(tr, "agent.mcp_tool_use") {
		t.Fatalf("no agent.mcp_tool_use event recorded: %+v", tr.Events)
	}
}

func TestThreadCreationAndTracking(t *testing.T) {
	tr := loadTranscript(t)
	if tr.ThreadID == "" && !hasEvent(tr, "session.thread_created") {
		t.Fatalf("thread creation/tracking evidence missing: thread_id=%q events=%+v", tr.ThreadID, tr.Events)
	}
}

func TestBudgetTrackingAgainstCeiling(t *testing.T) {
	tr := loadTranscript(t)
	if tr.Budget.Ceiling <= 0 {
		t.Fatalf("budget ceiling missing: %+v", tr.Budget)
	}
	if tr.Budget.Used < 0 || tr.Budget.Used > tr.Budget.Ceiling {
		t.Fatalf("budget usage outside ceiling: %+v", tr.Budget)
	}
}

func TestTrustTrackingInDecisionRecords(t *testing.T) {
	tr := loadTranscript(t)
	if !tr.Trust.Tracked {
		t.Fatal("trust tracking not recorded")
	}
	for _, decision := range tr.Decisions {
		if decision.Trust == 0 {
			t.Fatalf("decision missing trust score: %+v", decision)
		}
	}
}

func TestDecisionMetadata(t *testing.T) {
	tr := loadTranscript(t)
	if len(tr.Decisions) == 0 {
		t.Fatal("no decision records in transcript")
	}
	for _, decision := range tr.Decisions {
		missing := missingDecisionFields(decision)
		if len(missing) > 0 {
			t.Fatalf("decision missing metadata %v: %+v", missing, decision)
		}
	}
}

func TestFailClosedBehaviorOnPipelineError(t *testing.T) {
	tr := loadTranscript(t)
	if !tr.FailClosed.Observed {
		t.Fatal("fail-closed pipeline-error evidence missing")
	}
	if tr.FailClosed.Action != "deny" {
		t.Fatalf("pipeline error did not deny: %+v", tr.FailClosed)
	}
}

func TestSanitizedTranscriptEvidence(t *testing.T) {
	tr, data := loadTranscriptBytes(t)
	if !tr.Sanitized {
		t.Fatal("transcript does not declare sanitized=true")
	}
	if hasSecretLikeData(string(data)) {
		t.Fatal("transcript contains secret-like or raw personal data")
	}
	if tr.TranscriptHash != "" {
		if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(tr.TranscriptHash) {
			t.Fatalf("transcript_sha256 must be a lowercase sha256 hex digest, got %q", tr.TranscriptHash)
		}
	}
}

func loadTranscript(t *testing.T) transcript {
	t.Helper()
	tr, _ := loadTranscriptBytes(t)
	return tr
}

func loadTranscriptBytes(t *testing.T) (transcript, []byte) {
	t.Helper()
	if os.Getenv(enableEnv) != "true" {
		t.Skip(enableEnv + " not set")
	}
	path := os.Getenv(transcriptEnv)
	if path == "" {
		t.Skip(transcriptEnv + " not set; run live conformance and point this env var at the sanitized transcript")
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var tr transcript
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatalf("parse transcript: %v", err)
	}
	return tr, data
}

func hasEvent(tr transcript, eventType string) bool {
	for _, event := range tr.Events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func hasConfirmation(tr transcript, result string) bool {
	for _, confirmation := range tr.Confirmations {
		if confirmation.Result == result {
			return true
		}
	}
	return false
}

func missingDecisionFields(decision decisionRecord) []string {
	var missing []string
	if decision.AgentID == "" {
		missing = append(missing, "agent_id")
	}
	if decision.SessionID == "" {
		missing = append(missing, "session_id")
	}
	if decision.ThreadID == "" {
		missing = append(missing, "thread_id")
	}
	if decision.Tool == "" {
		missing = append(missing, "tool")
	}
	if decision.Action == "" {
		missing = append(missing, "action")
	}
	if decision.Rule == "" {
		missing = append(missing, "rule")
	}
	if decision.Trust == 0 {
		missing = append(missing, "trust")
	}
	return missing
}

func hasSecretLikeData(data string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)anthropic[_-]?api[_-]?key`),
		regexp.MustCompile(`(?i)bearer\s+[a-z0-9._~+/=-]{12,}`),
		regexp.MustCompile(`sk-ant-[a-zA-Z0-9_-]+`),
		regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(data) {
			return true
		}
	}
	for _, marker := range []string{"unsanitized", "raw_secret", "api_key", "bearer_token", "session_secret"} {
		if strings.Contains(strings.ToLower(data), marker) {
			return true
		}
	}
	return false
}
