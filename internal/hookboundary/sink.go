package hookboundary

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Record sink layout. Both artifacts are written for every decided event: the
// log is the audit/dashboard stream, the per-record file is what `boundary
// verify-record` consumes directly (it rejects a multi-record .jsonl).
const (
	// DefaultRecordDir is where hook decision records are written, relative to
	// the hook process's own working directory.
	DefaultRecordDir = ".boundary/hook"
	// DecisionLogName is the append-only multi-record log, one record per line.
	DecisionLogName = "decision-records.jsonl"
	// RecordsDirName holds one single-record JSON object per decision.
	RecordsDirName = "records"
)

// File modes for the sink. Records name governed commands and paths, so they are
// owner-only.
const (
	recordDirMode  fs.FileMode = 0o700
	recordFileMode fs.FileMode = 0o600
)

// Paths names where one decision record was persisted.
type Paths struct {
	// Record is the single-record JSON object `boundary verify-record` reads.
	Record string
	// Log is the append-only multi-record .jsonl log.
	Log string
}

// errSymlinkTarget is the cause recorded when the sink refuses a path that
// already exists as a symlink.
var errSymlinkTarget = errors.New("path exists as a symlink")

// errNotADirectory is the cause recorded when a record directory exists as
// something other than a directory.
var errNotADirectory = errors.New("path exists and is not a directory")

// WriteError reports a decision record that could not be persisted.
//
// It exists so one failure can be described two ways. Error carries the full
// detail — what the sink was doing, which path, and why — for an operator
// reading stderr or a diagnostic. Summary carries the same failure WITHOUT the
// path, for the hook's escalation reason, which is handed to the model and
// written into the record: the agent learns that recording failed, not where the
// operator keeps records.
type WriteError struct {
	// Op names what the sink was doing, e.g. "create decision record".
	Op string
	// Path is the artifact the sink could not write.
	Path string
	// Err is the underlying cause.
	Err error
}

// Error renders the operation, the path, and the cause.
func (e *WriteError) Error() string {
	return e.Op + " " + e.Path + ": " + e.cause()
}

// Summary renders the operation and the cause, and never the path.
func (e *WriteError) Summary() string {
	return e.Op + ": " + e.cause()
}

// Unwrap exposes the underlying cause to errors.Is and errors.As.
func (e *WriteError) Unwrap() error { return e.Err }

// cause renders the failure reason without a path. A *fs.PathError repeats the
// path it was raised on, so only its inner cause ("permission denied") is used.
func (e *WriteError) cause() string {
	if e.Err == nil {
		return "unknown error"
	}
	var pathErr *fs.PathError
	if errors.As(e.Err, &pathErr) && pathErr.Err != nil {
		return pathErr.Err.Error()
	}
	return e.Err.Error()
}

// Sink persists hook decision records under Dir.
//
// It writes two artifacts per record and refuses a symlinked target: the record
// directories and both output paths are checked with os.Lstat, and the
// per-record file is created with O_EXCL so an existing file — symlink or not —
// is never followed or clobbered. The check covers the paths the sink itself
// creates and writes; it does not walk every parent directory component, so a
// symlinked ancestor above Dir is outside what this refuses.
type Sink struct {
	// Dir is the record directory. Empty means DefaultRecordDir.
	Dir string
}

// Write persists record as one single-record JSON object under the records
// directory and one appended line in the decision log, and returns both paths.
//
// It returns an error without partial success reporting: a caller that gets an
// error must treat the decision as unrecorded. That contract is why the ORDER is
// what it is. The per-record file is written first because it is the one artifact
// that can be undone: an appended log line cannot be retracted from a stream
// other readers may already have consumed, so appending it first would leave a
// hash-valid record of a verdict the caller then degraded — evidence strictly
// more permissive than what was enforced. When the append fails, the record file
// written a moment earlier is removed, and both artifacts are absent exactly as
// the contract says.
//
// The decision path degrades the verdict rather than crashing or allowing
// silently — see decide.go. Every failure is a *WriteError, whose Summary states
// the failure without the record path.
func (s Sink) Write(record governance.DecisionRecordV1) (Paths, error) {
	dir := s.Dir
	if dir == "" {
		dir = DefaultRecordDir
	}
	recordsDir := filepath.Join(dir, RecordsDirName)
	if err := ensureRecordDir(dir); err != nil {
		return Paths{}, err
	}
	if err := ensureRecordDir(recordsDir); err != nil {
		return Paths{}, err
	}

	body, err := json.Marshal(record)
	if err != nil {
		return Paths{}, &WriteError{Op: "encode decision record", Path: recordsDir, Err: err}
	}

	recordPath := filepath.Join(recordsDir, recordFileName(record, time.Now()))
	if err := writeRecordFile(recordPath, body); err != nil {
		return Paths{}, err
	}
	logPath := filepath.Join(dir, DecisionLogName)
	if err := appendRecordLine(logPath, body); err != nil {
		// Roll the record file back so the caller's "treat it as unrecorded" is
		// true of the filesystem too. A removal that itself fails leaves the
		// per-record file behind; the returned error is still the append failure,
		// because that is what the caller must act on.
		_ = os.Remove(recordPath)
		return Paths{}, err
	}
	return Paths{Record: recordPath, Log: logPath}, nil
}

// ensureRecordDir creates path with owner-only permissions when it is absent and
// refuses it when it exists as a symlink or as a non-directory.
func ensureRecordDir(path string) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if info.Mode()&fs.ModeSymlink != 0 {
			return &WriteError{Op: "refusing symlinked record directory", Path: path, Err: errSymlinkTarget}
		}
		if !info.IsDir() {
			return &WriteError{Op: "record directory", Path: path, Err: errNotADirectory}
		}
		return nil
	case errors.Is(err, fs.ErrNotExist):
		if mkErr := os.MkdirAll(path, recordDirMode); mkErr != nil {
			return &WriteError{Op: "create record directory", Path: path, Err: mkErr}
		}
		return nil
	default:
		return &WriteError{Op: "inspect record directory", Path: path, Err: err}
	}
}

// appendRecordLine appends one JSON line to the decision log, refusing a
// symlinked log path.
func appendRecordLine(path string, body []byte) error {
	if err := refuseSymlink(path, "decision record log"); err != nil {
		return err
	}
	// #nosec G304 -- the log path is composed from the operator-selected record
	// directory and a fixed file name; no event-supplied string reaches it.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, recordFileMode)
	if err != nil {
		return &WriteError{Op: "open decision record log", Path: path, Err: err}
	}
	line := make([]byte, 0, len(body)+1)
	line = append(line, body...)
	line = append(line, '\n')
	if _, err := file.Write(line); err != nil {
		_ = file.Close()
		return &WriteError{Op: "append decision record to log", Path: path, Err: err}
	}
	if err := file.Close(); err != nil {
		return &WriteError{Op: "close decision record log", Path: path, Err: err}
	}
	return nil
}

// writeRecordFile writes one single-record JSON object. O_EXCL means an existing
// path is never followed or overwritten, which refuses a planted symlink even if
// it appeared after the Lstat check.
func writeRecordFile(path string, body []byte) error {
	if err := refuseSymlink(path, "decision record"); err != nil {
		return err
	}
	// #nosec G304 -- the file name is derived from the record's own timestamp
	// and record_id (itself derived from decision_hash); no event-supplied
	// string reaches it.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, recordFileMode)
	if err != nil {
		return &WriteError{Op: "create decision record", Path: path, Err: err}
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return &WriteError{Op: "write decision record", Path: path, Err: err}
	}
	if err := file.Close(); err != nil {
		return &WriteError{Op: "close decision record", Path: path, Err: err}
	}
	return nil
}

// refuseSymlink reports an error when path exists and is a symlink. A missing
// path is fine — that is the ordinary first-write case. artifact names what the
// path holds, so the error reads the same way whichever artifact was refused.
func refuseSymlink(path, artifact string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return &WriteError{Op: "inspect " + artifact, Path: path, Err: err}
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return &WriteError{Op: "refusing symlinked " + artifact + " target", Path: path, Err: errSymlinkTarget}
	}
	return nil
}
