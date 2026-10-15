package boundarycli

import (
	"context"
	"fmt"
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
}

func newServeRecordPublisher(log *governance.SlogAuditPublisher, dir string) (*serveRecordPublisher, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create record dir: %w", err)
	}
	return &serveRecordPublisher{log: log, sink: hookboundary.Sink{Dir: dir}}, nil
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
