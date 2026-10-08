package madriver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// transcriptDoc mirrors the shape tests/conformance/managed_agents/
// conformance_test.go unmarshals, plus the mode field the live gate requires.
type transcriptDoc struct {
	Sanitized      bool              `json:"sanitized"`
	Mode           string            `json:"mode"`
	SessionCreated bool              `json:"session_created_through_boundary"`
	SessionID      string            `json:"session_id"`
	ThreadID       string            `json:"thread_id"`
	AgentID        string            `json:"agent_id"`
	Events         []transcriptEvent `json:"events"`
	Confirmations  []confirmationDoc `json:"confirmations"`
	Decisions      []decisionDoc     `json:"decisions"`
	Budget         budgetDoc         `json:"budget"`
	Trust          trustDoc          `json:"trust"`
	FailClosed     failClosedDoc     `json:"fail_closed"`
	// Provenance links this sanitized transcript back to the run that produced
	// it: the pseudonymized session id plus the SHA-256 of the sanitized raw
	// event log written beside it. The linkage prevents accidents (accepting
	// a stub or mismatched file as live evidence); a determined human could
	// still forge a file, so it is not tamper evidence.
	Provenance *provenanceLink `json:"provenance"`
	// NotObserved lists conformance criteria the driver could not observe
	// evidence for in this run. The driver reports absence rather than
	// fabricating or defaulting evidence (for example thread creation, which
	// upstream may never emit).
	NotObserved    []string `json:"criteria_not_observed,omitempty"`
	TranscriptHash string   `json:"transcript_sha256,omitempty"`
}

// provenanceLink is the transcript-embedded half of the transcript ↔
// provenance.json linkage. SessionID is the same pseudonym as the top-level
// session_id so no raw identifier enters the sanitized document.
type provenanceLink struct {
	Mode         string `json:"mode"`
	SessionID    string `json:"session_id"`
	RawLogSHA256 string `json:"raw_log_sha256"`
}

type transcriptEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	ThreadID  string `json:"thread_id,omitempty"`
	Tool      string `json:"tool,omitempty"`
}

type confirmationDoc struct {
	ToolUseID string `json:"tool_use_id"`
	Result    string `json:"result"`
	Tool      string `json:"tool,omitempty"`
	// Delivered is false when Boundary resolved a confirmation but the upstream
	// did not accept one for that call (e.g. the call's evaluated_permission
	// was not "ask"). Stub mode always delivers to the in-process fake.
	Delivered *bool `json:"delivered,omitempty"`
}

type decisionDoc struct {
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

type budgetDoc struct {
	Ceiling float64 `json:"ceiling"`
	Used    float64 `json:"used"`
	// UsageObserved records that at least one usable upstream usage signal
	// arrived. A Used of 0 without it is not evidence and must never pass the
	// budget criterion.
	UsageObserved bool `json:"usage_observed"`
	// SpendUnknown marks runs that ended without trustworthy spend evidence
	// (fail-closed stop or no usage signal at all); Used is then a lower
	// bound, not an observed figure.
	SpendUnknown bool `json:"spend_unknown,omitempty"`
}

type trustDoc struct {
	Tracked bool    `json:"tracked"`
	Score   float64 `json:"score"`
}

type failClosedDoc struct {
	Observed bool   `json:"observed"`
	Action   string `json:"action"`
	Reason   string `json:"reason,omitempty"`
}

// pseudonym replaces a real identifier with a stable, irreversible placeholder
// so the sanitized transcript carries shape without carrying session secrets.
// Equal inputs map to equal outputs so cross-references inside the transcript
// stay consistent.
func pseudonym(prefix, raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(prefix + ":" + raw))
	return prefix + "-" + hex.EncodeToString(sum[:])[:12]
}

// writeTranscriptFile marshals the sanitized transcript, redacts the
// serialized bytes, computes transcript_sha256 over exactly those redacted
// bytes (hash field absent), then writes the final document (hash included)
// to <out-dir>/transcript.sanitized.json. Re-hashing the file with the hash
// field cleared and the same marshal+redact steps therefore reproduces the
// recorded digest — the hash is over sanitized content, never raw JSON.
func writeTranscriptFile(dir string, doc *transcriptDoc) (path string, err error) {
	doc.TranscriptHash = ""
	unsigned, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal transcript: %w", err)
	}
	unsigned = []byte(RedactString(string(unsigned)))
	sum := sha256.Sum256(unsigned)
	doc.TranscriptHash = hex.EncodeToString(sum[:])

	final, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal transcript: %w", err)
	}
	final = []byte(RedactString(string(final)))
	path = filepath.Join(dir, "transcript.sanitized.json")
	if err := os.WriteFile(path, append(final, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write transcript: %w", err)
	}
	return path, nil
}
