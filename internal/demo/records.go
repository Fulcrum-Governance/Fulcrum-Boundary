package demo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// DefaultDecisionRecordFilename is the predictable basename every proof-lane
// demo writes its decision-record *log* to inside its artifact directory. The
// log is a multi-record JSONL (one DecisionRecordV1 object per line) intended
// for dashboards and append-style audit reading; it is NOT what the
// `decision record path:` UX line points at. Holding the name fixed (rather
// than deriving it per-demo) is what makes the "find -> read the log" step
// copy-paste across demos.
const DefaultDecisionRecordFilename = "decision-records.jsonl"

// DefaultDecisionRecordObjectFilename is the predictable basename every
// proof-lane demo writes its single headline decision record to inside its
// artifact directory. Unlike DefaultDecisionRecordFilename, this file holds
// exactly one DecisionRecordV1 JSON object (not JSONL), so it is the path
// `boundary verify-record` consumes directly. It is the target of the uniform
// `decision record path:` line: a single-record JSON object, never a log.
const DefaultDecisionRecordObjectFilename = "decision-record.json"

// ArtifactDir derives a demo's retained artifact directory from the operator's
// --out report path, using a stable "<name>-artifacts" sibling of the report
// file. Every proof-lane demo derives its artifact directory the same way so
// `--out` means the same thing across demos: the report goes to the named path
// and a predictable sibling directory holds the decision-record JSONL. outPath
// must be non-empty (an empty --out means "print to stdout, retain nothing").
func ArtifactDir(outPath, name string) (string, error) {
	report, err := filepath.Abs(outPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(report), name+"-artifacts"), nil
}

// WriteDecisionRecordsJSONL writes one DecisionRecordV1 per line (JSONL) to
// path, truncating any existing file, and creating the parent directory. It is
// the single on-disk record writer shared by the proof-lane demos so every demo
// lands its records in the same shape and at the same predictable basename.
// Records without a record_id (never built) are skipped so a demo that emits
// fewer records than expected does not write blank lines.
//
// The layout is deliberately the receipt-grade DecisionRecordV1 JSONL that
// `boundary verify-record` consumes one object at a time; it intentionally
// matches the command/edit boundary record logs except that demos truncate
// (each demo run is a self-contained artifact) rather than append.
func WriteDecisionRecordsJSONL(path string, records []governance.DecisionRecordV1) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- decision records are written to an internally constructed or operator-selected demo artifact path.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, record := range records {
		if record.RecordID == "" {
			continue
		}
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

// WriteDecisionRecordJSON writes exactly one DecisionRecordV1 as an indented
// JSON object to path, truncating any existing file and creating the parent
// directory. Unlike WriteDecisionRecordsJSONL it emits a single top-level
// object (no JSONL framing), which is the exact shape `boundary verify-record`
// consumes: the file the uniform `decision record path:` line points at. It is
// the single-record companion to the multi-record JSONL log so a demo can land
// both a verifiable headline record and a fuller log in the same artifact dir.
func WriteDecisionRecordJSON(path string, record governance.DecisionRecordV1) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- the decision record is written to an internally constructed or operator-selected demo artifact path.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(record)
}
