package commandboundary

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersionDecision = "boundary.command_decision.v1"

type CommandDecisionRecord struct {
	RecordType    string    `json:"record_type"`
	SchemaVersion string    `json:"schema_version"`
	RequestID     string    `json:"request_id"`
	EnvelopeID    string    `json:"envelope_id"`
	Command       string    `json:"command"`
	ArgsHash      string    `json:"args_hash"`
	ArgsRedacted  []string  `json:"args_redacted,omitempty"`
	CWD           string    `json:"cwd"`
	Class         Class     `json:"class"`
	Risk          Risk      `json:"risk"`
	Action        string    `json:"action"`
	Executed      bool      `json:"executed"`
	ExitCode      int       `json:"exit_code"`
	Reason        string    `json:"reason,omitempty"`
	MatchedRule   string    `json:"matched_rule,omitempty"`
	PolicyID      string    `json:"policy_id,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

func AppendDecisionRecord(path string, record CommandDecisionRecord) error {
	if path == "" {
		path = DefaultDecisionRecordPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- command decision records are written to an operator-selected local path.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	return enc.Encode(record)
}

func prepareRecordPath(path string) error {
	if path == "" {
		path = DefaultDecisionRecordPath
	}
	return os.MkdirAll(filepath.Dir(path), 0o700)
}
