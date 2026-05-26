package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/fulcrum-governance/boundary/governance"
)

// Forwarder sends an allowed MCP JSON-RPC request to an upstream server.
type Forwarder interface {
	Forward(ctx context.Context, body []byte) (*governance.ToolResponse, error)
}

// HTTPForwarder posts JSON-RPC payloads to an upstream HTTP MCP endpoint.
type HTTPForwarder struct {
	Endpoint string
	Client   *http.Client
}

// NewHTTPForwarder returns an HTTP MCP forwarder for endpoint.
func NewHTTPForwarder(endpoint string) *HTTPForwarder {
	return &HTTPForwarder{
		Endpoint: endpoint,
		Client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Forward posts body to the upstream MCP endpoint exactly once.
func (f *HTTPForwarder) Forward(ctx context.Context, body []byte) (*governance.ToolResponse, error) {
	if f == nil || f.Endpoint == "" {
		return nil, fmt.Errorf("MCP upstream endpoint is not configured")
	}
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &governance.ToolResponse{
		Content:     content,
		ContentType: resp.Header.Get("Content-Type"),
		ExitCode:    resp.StatusCode,
		Metadata:    map[string]string{"http_status": resp.Status},
	}, nil
}
