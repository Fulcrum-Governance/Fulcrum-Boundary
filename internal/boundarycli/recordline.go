package boundarycli

import (
	"fmt"
	"io"
)

// recordLocation is the uniform "where is my decision record" signal printed by
// every record-emitting Boundary command (the proof-lane demos, redteam, the
// command/edit boundary surfaces, and the demo report path line). It separates
// the two concepts that previously collided under a single "decision record:"
// token: an identifier (the record's `record_id`) and a filesystem location
// (the path a record file was written to).
//
// Callers print whichever lines they have evidence for:
//   - recordIDLine when they can name the emitted record's record_id;
//   - recordPathLine when a record file was actually written to disk.
//
// Keeping the two lines distinct ends the path-vs-id ambiguity: an operator can
// always copy the path line into `boundary verify-record` and the id line into a
// log search, and the same shape appears across commands.
const (
	// recordIDLabel prefixes a line whose value is a record_id (rec_...). It is
	// never a path.
	recordIDLabel = "decision record id: "
	// recordPathLabel prefixes a line whose value is an on-disk path to a
	// written decision record (a single JSON object or a .jsonl log). It is
	// never a record_id.
	recordPathLabel = "decision record path: "
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
// decision record (or .jsonl record log) was written to. The line is omitted
// when path is empty so in-memory-only surfaces do not claim a nonexistent file.
func printRecordPath(w io.Writer, path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(w, "%s%s\n", recordPathLabel, path)
}
