package boundarycli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/hookboundary"
)

// defaultServeRecordDir is where serve writes decision records unless
// --record-dir or server.record_dir names another directory. It follows the
// .boundary/<surface> layout the hook, command and edit surfaces use.
const defaultServeRecordDir = ".boundary/serve"

// serveRecordPublisher writes every serve decision as a decision record that
// `boundary verify-record` accepts, then logs it. Each record lands as one JSON
// object under <dir>/records/ and one line in <dir>/decision-records.jsonl.
// A write failure is reported to the pipeline as an audit delivery failure.
type serveRecordPublisher struct {
	log  *governance.SlogAuditPublisher
	sink hookboundary.Sink
	// startupNote is the one startup line serve prints when it tightened an
	// existing record dir to 0700; empty when nothing changed.
	startupNote string
}

func newServeRecordPublisher(log *governance.SlogAuditPublisher, dir string) (*serveRecordPublisher, error) {
	note, err := prepareServeRecordDir(dir)
	if err != nil {
		return nil, err
	}
	return &serveRecordPublisher{log: log, sink: hookboundary.Sink{Dir: dir}, startupNote: note}, nil
}

// serveCurrentUID is the user serve runs as; a test overrides it to exercise
// the not-owned refusal without a second account.
var serveCurrentUID = os.Geteuid

// prepareServeRecordDir makes dir an owner-only directory before serve opens
// its listener. Records carry decision identities and reasons, so:
//
//   - a missing dir is created 0700;
//   - an existing dir owned by the current user that grants any group or other
//     permission is chmodded to 0700, and the returned note says so;
//   - a symlink, a non-directory, or a dir owned by another user is refused.
//
// Only dir itself is checked; with dir at 0700 nothing beneath it is reachable
// by other users. Record files are written 0600 by hookboundary.Sink.
func prepareServeRecordDir(dir string) (string, error) {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("create record dir %s: %w", dir, err)
		}
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("inspect record dir %s: %w", dir, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return "", fmt.Errorf("record dir %s is a symlink; point --record-dir at a real directory", dir)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("record dir %s is not a directory", dir)
	}
	if uid, ok := fileOwnerUID(info); ok && uid != serveCurrentUID() {
		return "", fmt.Errorf("record dir %s is not owned by the current user (owner uid %d, serve uid %d); records could be read or replaced by its owner", dir, uid, serveCurrentUID())
	}
	perm := info.Mode().Perm()
	if perm&0o077 == 0 {
		return "", nil
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("restrict record dir %s to 0700: %w", dir, err)
	}
	return fmt.Sprintf("boundary serve: tightened record dir %s permissions from %#o to 0700", dir, perm), nil
}

func (p *serveRecordPublisher) Publish(ctx context.Context, event governance.AuditEvent) {
	_ = p.PublishChecked(ctx, event)
}

func (p *serveRecordPublisher) PublishChecked(ctx context.Context, event governance.AuditEvent) error {
	p.log.Publish(ctx, event)
	_, err := p.sink.Write(governance.BuildDecisionRecord(event))
	return err
}

func (p *serveRecordPublisher) logPath() string {
	return filepath.Join(p.sink.Dir, hookboundary.DecisionLogName)
}
