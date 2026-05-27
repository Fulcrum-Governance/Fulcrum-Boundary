package firewall

import (
	"encoding/json"
)

func parseRawMCPConfig(body []byte) (rawMCPConfig, map[string]json.RawMessage, error) {
	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(body, &topLevel); err != nil {
		return rawMCPConfig{}, nil, err
	}
	var config rawMCPConfig
	if raw, ok := topLevel["mcpServers"]; ok {
		if err := json.Unmarshal(raw, &config.MCPServers); err != nil {
			return rawMCPConfig{}, nil, err
		}
	}
	if raw, ok := topLevel["servers"]; ok {
		if err := json.Unmarshal(raw, &config.Servers); err != nil {
			return rawMCPConfig{}, nil, err
		}
	}
	if config.MCPServers == nil {
		config.MCPServers = map[string]rawServer{}
	}
	if config.Servers == nil {
		config.Servers = map[string]rawServer{}
	}
	return config, topLevel, nil
}

func encodeRawMCPConfig(config rawMCPConfig, topLevel map[string]json.RawMessage) ([]byte, error) {
	if _, ok := topLevel["mcpServers"]; ok {
		body, err := json.Marshal(config.MCPServers)
		if err != nil {
			return nil, err
		}
		topLevel["mcpServers"] = body
	}
	if _, ok := topLevel["servers"]; ok {
		body, err := json.Marshal(config.Servers)
		if err != nil {
			return nil, err
		}
		topLevel["servers"] = body
	}
	return encodeTopLevel(topLevel)
}

func encodeTopLevel(topLevel map[string]json.RawMessage) ([]byte, error) {
	body, err := json.MarshalIndent(topLevel, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}
