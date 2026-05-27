package firewall

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

type descriptorHashInput struct {
	SchemaVersion string               `json:"schema_version"`
	Name          string               `json:"name"`
	Command       string               `json:"command,omitempty"`
	URL           string               `json:"url,omitempty"`
	Args          []string             `json:"args,omitempty"`
	EnvKeys       []string             `json:"env_keys,omitempty"`
	Tools         []descriptorToolHash `json:"tools,omitempty"`
}

type descriptorToolHash struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	InputSchema  any    `json:"input_schema,omitempty"`
	OutputSchema any    `json:"output_schema,omitempty"`
}

func DescriptorHash(server Server) (string, error) {
	input := descriptorHashInput{
		SchemaVersion: "boundary.firewall.descriptor.v1",
		Name:          server.Name,
		Command:       server.Command,
		URL:           redactURL(server.URL),
		Args:          append([]string(nil), server.Args...),
		EnvKeys:       append([]string(nil), server.EnvKeys...),
		Tools:         descriptorToolsFromNames(server.DescriptorTools),
	}
	return hashDescriptorInput(input)
}

func descriptorHashForRawServer(name string, raw rawServer) (string, error) {
	tools, err := descriptorTools(raw.Tools)
	if err != nil {
		return "", err
	}
	input := descriptorHashInput{
		SchemaVersion: "boundary.firewall.descriptor.v1",
		Name:          name,
		Command:       raw.Command,
		URL:           redactURL(raw.URL),
		Args:          redactArgs(raw.Args),
		EnvKeys:       envKeys(raw.Env),
		Tools:         tools,
	}
	return hashDescriptorInput(input)
}

func hashDescriptorInput(input descriptorHashInput) (string, error) {
	sort.Strings(input.EnvKeys)
	sort.Slice(input.Tools, func(i, j int) bool {
		if input.Tools[i].Name == input.Tools[j].Name {
			return input.Tools[i].Description < input.Tools[j].Description
		}
		return input.Tools[i].Name < input.Tools[j].Name
	})
	body, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func descriptorTools(tools []rawTool) ([]descriptorToolHash, error) {
	descriptors := make([]descriptorToolHash, 0, len(tools))
	for _, tool := range tools {
		if tool.Name == "" {
			continue
		}
		inputSchema, err := canonicalSchema(tool.InputSchema, tool.InputSchemaSnake)
		if err != nil {
			return nil, err
		}
		outputSchema, err := canonicalSchema(tool.OutputSchema, tool.OutputSchemaSnake)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptorToolHash{
			Name:         tool.Name,
			Description:  tool.Description,
			InputSchema:  inputSchema,
			OutputSchema: outputSchema,
		})
	}
	return descriptors, nil
}

func descriptorToolsFromNames(names []string) []descriptorToolHash {
	tools := make([]descriptorToolHash, 0, len(names))
	for _, name := range names {
		if name != "" {
			tools = append(tools, descriptorToolHash{Name: name})
		}
	}
	return tools
}

func canonicalSchema(primary, fallback json.RawMessage) (any, error) {
	raw := primary
	if len(raw) == 0 {
		raw = fallback
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func redactURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.User != nil {
		parsed.User = url.User("[redacted]")
	}
	query := parsed.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") || strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "apikey") || strings.Contains(lower, "key") {
			query.Set(key, "[redacted]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
