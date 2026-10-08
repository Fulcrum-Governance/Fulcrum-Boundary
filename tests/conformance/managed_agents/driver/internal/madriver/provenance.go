package madriver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// provenanceDoc is the provenance.json written beside each transcript. It
// ties the evidence to the code that produced it and is never mistakable for
// live evidence in stub mode because Mode is recorded verbatim.
type provenanceDoc struct {
	Mode      string `json:"mode"`
	SessionID string `json:"session_id"`
	// TranscriptSessionID is the pseudonymized session id the sanitized
	// transcript carries; it links the raw operator evidence here to the
	// sanitized document without putting a raw id into the transcript.
	TranscriptSessionID string `json:"transcript_session_id"`
	AgentID             string `json:"agent_id,omitempty"`
	StartedUTC          string `json:"started_utc"`
	EndedUTC            string `json:"ended_utc"`
	DriverCommit        string `json:"driver_git_commit"`
	AdapterCommit       string `json:"adapter_git_commit"`
	TranscriptSHA256    string `json:"transcript_sha256"`
	// RawLogFile and RawLogSHA256 tie this provenance record to the sanitized
	// raw event log written beside it.
	RawLogFile         string   `json:"raw_log_file"`
	RawLogSHA256       string   `json:"raw_log_sha256"`
	UpstreamRequestIDs []string `json:"upstream_request_ids,omitempty"`
	BetaHeader         string   `json:"beta_header"`
	StopReason         string   `json:"stop_reason"`
	SpendUSD           float64  `json:"spend_usd"`
}

func writeProvenanceFile(dir string, doc provenanceDoc) (string, error) {
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal provenance: %w", err)
	}
	raw = []byte(RedactString(string(raw)))
	path := filepath.Join(dir, "provenance.json")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("write provenance: %w", err)
	}
	return path, nil
}
