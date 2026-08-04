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
	for _, path := range []string{decisionAbs, forwardAbs} {
		if err := prepareExistingMCPJSONL(path); err != nil {
			return nil, err
		}
	}
	return &MCPJSONLEvidenceSink{decisionPath: decisionAbs, forwardPath: forwardAbs}, nil
}

func prepareExistingMCPJSONL(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("secure GitHub MCP: evidence path must be a regular file")
	}
	return os.Chmod(path, 0o600)
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
	if err := validateMCPJSONLEvidencePath(path); err != nil {
		return err
	}
	// #nosec G703 -- path is absolute and clean per validateMCPJSONLEvidencePath.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// #nosec G703 -- path is absolute and clean per validateMCPJSONLEvidencePath.
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("secure GitHub MCP: evidence path must be a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// #nosec G304 G703 -- path is operator-selected, absolute, and clean per validateMCPJSONLEvidencePath.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("secure GitHub MCP: evidence path must be a regular file")
	}
	return json.NewEncoder(file).Encode(value)
}

func validateMCPJSONLEvidencePath(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("secure GitHub MCP: evidence path must be absolute and clean")
	}
	return nil
}
