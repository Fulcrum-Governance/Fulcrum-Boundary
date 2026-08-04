package securegithub

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// MCPJSONLEvidenceSink keeps decision records and independently observed
// forwarder events in distinct append-only local files.
type MCPJSONLEvidenceSink struct {
	decisionPath string
	forwardPath  string
	decisionMu   sync.Mutex
	forwardMu    sync.Mutex
}

func NewMCPJSONLEvidenceSink(decisionPath, forwardPath string) (*MCPJSONLEvidenceSink, error) {
	if decisionPath == "" || forwardPath == "" {
		return nil, errors.New("secure GitHub MCP: decision and forward evidence paths are required")
	}
	decisionAbs, err := filepath.Abs(decisionPath)
	if err != nil {
		return nil, err
	}
	forwardAbs, err := filepath.Abs(forwardPath)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(decisionAbs) == filepath.Clean(forwardAbs) {
		return nil, errors.New("secure GitHub MCP: decision and forward evidence paths must be distinct")
	}
	return &MCPJSONLEvidenceSink{decisionPath: decisionAbs, forwardPath: forwardAbs}, nil
}

func (s *MCPJSONLEvidenceSink) WriteDecision(_ context.Context, record governance.DecisionRecordV1) error {
	s.decisionMu.Lock()
	defer s.decisionMu.Unlock()
	return appendMCPJSONL(s.decisionPath, record)
}

func (s *MCPJSONLEvidenceSink) WriteForwardEvent(_ context.Context, event MCPForwardEvent) error {
	s.forwardMu.Lock()
	defer s.forwardMu.Unlock()
	return appendMCPJSONL(s.forwardPath, event)
}

func appendMCPJSONL(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G304 -- evidence is written only to the operator-selected local path.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(value)
}
