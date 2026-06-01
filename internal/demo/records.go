package demo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// DefaultDecisionRecordFilename is the predictable basename every proof-lane
// demo writes its decision record(s) to inside its artifact directory. Holding
// the name fixed (rather than deriving it per-demo) is what makes the
// "find -> verify" step copy-paste across demos.
const DefaultDecisionRecordFilename = "decision-records.jsonl"

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
