package editboundary

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const SchemaVersionDecision = "boundary.edit_decision.v1"

type EditDecisionRecord struct {
	RecordType       string    `json:"record_type"`
	SchemaVersion    string    `json:"schema_version"`
	RequestID        string    `json:"request_id"`
	EnvelopeID       string    `json:"envelope_id"`
	PatchSHA256      string    `json:"patch_sha256"`
	CWD              string    `json:"cwd"`
	FilesTouched     int       `json:"files_touched"`
	Files            []string  `json:"files"`
	RedactedPaths    []string  `json:"redacted_paths,omitempty"`
	Class            Class     `json:"class"`
	Risk             Risk      `json:"risk"`
	Action           string    `json:"action"`
	ApprovalPresent  bool      `json:"approval_present"`
	ApprovalMode     string    `json:"approval_mode,omitempty"`
	DryRun           bool      `json:"dry_run"`
	ApplierInvoked   bool      `json:"applier_invoked"`
	Applied          bool      `json:"applied"`
	IndexChanged     bool      `json:"index_changed"`
	ExitCode         int       `json:"exit_code"`
	Reason           string    `json:"reason,omitempty"`
	MatchedRule      string    `json:"matched_rule,omitempty"`
	PolicyID         string    `json:"policy_id,omitempty"`
	ApplicationError string    `json:"application_error,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

func AppendDecisionRecord(path string, record EditDecisionRecord) error {
	if path == "" {
		path = DefaultEditDecisionRecordPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- edit decision records are written to an operator-selected local path.
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
		path = DefaultEditDecisionRecordPath
	}
	return os.MkdirAll(filepath.Dir(path), 0o700)
}
