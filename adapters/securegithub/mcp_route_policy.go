package securegithub

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// MCPRoutePolicyFile is the file name the route's built-in rules are written
// to in an operator policy directory.
const MCPRoutePolicyFile = "secure-github-mcp.yaml"

//go:embed mcp_route_policy.yaml
var mcpRoutePolicyYAML []byte

// MCPRoutePolicyRules returns the Secure GitHub MCP route's built-in rules,
// parsed from the embedded default bundle.
func MCPRoutePolicyRules() ([]governance.StaticPolicyRule, error) {
	doc, err := governance.ParseStaticPolicyDocument(MCPRoutePolicyFile, mcpRoutePolicyYAML)
	if err != nil {
		return nil, fmt.Errorf("secure GitHub MCP: built-in policy: %w", err)
	}
	return doc.Rules, nil
}

// WriteMCPRoutePolicy writes the built-in rules into dir as
// MCPRoutePolicyFile, unless that file already exists. An existing file is
// left alone; the route still refuses it if it drops or changes a built-in
// rule.
func WriteMCPRoutePolicy(dir string) error {
	path := filepath.Join(dir, MCPRoutePolicyFile)
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("secure GitHub MCP: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("secure GitHub MCP: policy directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".secure-github-mcp-*.tmp")
	if err != nil {
		return fmt.Errorf("secure GitHub MCP: write built-in policy: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(mcpRoutePolicyYAML); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure GitHub MCP: write built-in policy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("secure GitHub MCP: write built-in policy: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("secure GitHub MCP: write built-in policy: %w", err)
	}
	return nil
}
