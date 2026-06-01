package boundarycli

import (
	"fmt"
	"io"
)

// recordLocation is the uniform "where is my decision record" signal printed by
// every record-emitting Boundary command (the proof-lane demos, redteam, the
// command/edit boundary surfaces, and the demo report path line). It separates
// the concepts that previously collided under a single "decision record:"
// token: an identifier (the record's `record_id`), a single verifiable record
// file (the path a single-record JSON object was written to), and a multi-
// record log (a .jsonl audit/dashboard artifact).
//
// Callers print whichever lines they have evidence for:
//   - recordIDLine when they can name the emitted record's record_id;
//   - recordPathLine when a single-record JSON object was written to disk;
//   - recordLogLine when a multi-record .jsonl log was written to disk.
//
// Keeping the lines distinct ends the path-vs-id ambiguity and, crucially,
// keeps the `decision record path:` line's contract precise: it always points
// at a single-record JSON object that an operator can copy straight into
// `boundary verify-record` (exit 0). Multi-record logs, which verify-record
// rejects ("invalid character '{' after top-level value"), are surfaced under
// the separate `decision record log:` label instead.
const (
	// recordIDLabel prefixes a line whose value is a record_id (rec_...). It is
	// never a path.
	recordIDLabel = "decision record id: "
	// recordPathLabel prefixes a line whose value is an on-disk path to a
	// single-record decision record: one JSON object that `boundary
	// verify-record` consumes directly. It is never a record_id and never a
	// multi-record .jsonl log.
	recordPathLabel = "decision record path: "
	// recordLogLabel prefixes a line whose value is an on-disk path to a multi-
	// record decision-record log (a .jsonl file, one record per line). It is a
	// dashboard/audit artifact, not a `boundary verify-record` input.
	recordLogLabel = "decision record log: "
)

// printRecordID writes the uniform record-id line. id is a record_id such as
// "rec_4b68b9d63c69". The line is omitted entirely when id is empty so commands
// that produced no record do not emit a misleading blank identifier.
func printRecordID(w io.Writer, id string) {
	if id == "" {
		return
	}
	fmt.Fprintf(w, "%s%s\n", recordIDLabel, id)
}

// printRecordPath writes the uniform record-path line. path is the location a
// single-record decision record (one JSON object) was written to, which
// `boundary verify-record` consumes directly. The line is omitted when path is
// empty so in-memory-only surfaces do not claim a nonexistent file. For multi-
// record .jsonl logs use printRecordLog instead so the path line keeps its
// "verify-record-consumable single object" contract.
func printRecordPath(w io.Writer, path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(w, "%s%s\n", recordPathLabel, path)
}

// printRecordLog writes the uniform record-log line. path is the location a
// multi-record decision-record log (a .jsonl file, one record per line) was
// written to. It is a dashboard/audit artifact, not a verify-record input. The
// line is omitted when path is empty.
func printRecordLog(w io.Writer, path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(w, "%s%s\n", recordLogLabel, path)
}
