package firewall

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type externalInventoryBuilder struct {
	source     string
	generated  string
	configs    map[string]ConfigFile
	servers    map[string]Server
	components []ExternalInventoryComponent
	findings   []ExternalExposureFinding
	warnings   []string
}

func newExternalInventoryBuilder(source, generated string) *externalInventoryBuilder {
	return &externalInventoryBuilder{
		source:    source,
		generated: generated,
		configs:   map[string]ConfigFile{},
		servers:   map[string]Server{},
	}
}

func (b *externalInventoryBuilder) addWarning(message string) {
	if strings.TrimSpace(message) != "" {
		b.warnings = append(b.warnings, message)
	}
}

func (b *externalInventoryBuilder) addConfig(config ConfigFile) {
	config.Path = cleanExternalPath(config.Path)
	if config.Path == "" {
		config.Path = "external-inventory.ndjson"
	}
	if config.Client == "" {
		config.Client = clientTypeFromPath(config.Path)
	}
	if config.Scope == "" {
		config.Scope = "external"
	}
	existing := b.configs[config.Path]
	existing.Path = config.Path
	existing.Client = config.Client
	existing.Scope = config.Scope
	if config.ServerCount > existing.ServerCount {
		existing.ServerCount = config.ServerCount
	}
	b.configs[config.Path] = existing
}

func (b *externalInventoryBuilder) addServer(server Server) {
	server.ConfigPath = cleanExternalPath(server.ConfigPath)
	if server.ConfigPath == "" {
		server.ConfigPath = "external-inventory.ndjson"
	}
	if server.Client == "" {
		server.Client = clientTypeFromPath(server.ConfigPath)
	}
	if server.Name == "" {
		server.Name = firstNonEmpty(filepath.Base(server.Command), "external-mcp-server")
	}
	server.Command = redactSecretBearingValue(server.Command)
	server.URL = redactURL(server.URL)
	server.Args = redactArgs(server.Args)
	sort.Strings(server.EnvKeys)
	sort.Strings(server.DescriptorTools)
	if len(server.Capabilities) == 0 {
		server.Capabilities = ClassifyServer(server)
	}
	server.HighestRisk = highestRisk(server.Capabilities)
	key := serverKey(server.ConfigPath, string(server.Client), server.Name)
	existing, ok := b.servers[key]
	if ok {
		server = mergeExternalServer(existing, server)
	}
	b.servers[key] = server
	config := b.configs[server.ConfigPath]
	config.Path = server.ConfigPath
	if config.Client == "" {
		config.Client = server.Client
	}
	if config.Scope == "" {
		config.Scope = "external"
	}
	b.configs[server.ConfigPath] = config
}

func (b *externalInventoryBuilder) addComponent(component ExternalInventoryComponent) {
	component.Kind = firstNonEmpty(component.Kind, "external_inventory_component")
	component.Note = firstNonEmpty(component.Note, "reporting-only external inventory component; not treated as an MCP action path")
	b.components = append(b.components, component)
}

func (b *externalInventoryBuilder) addFinding(finding ExternalExposureFinding) {
	finding.Kind = firstNonEmpty(finding.Kind, "external_exposure_finding")
	finding.Note = firstNonEmpty(finding.Note, "reporting-only external exposure finding unless it maps to an MCP action path")
	finding.ReportOnly = !finding.MCPMapped
	b.findings = append(b.findings, finding)
}

func (b *externalInventoryBuilder) inventory(root string) Inventory {
	configs := make([]ConfigFile, 0, len(b.configs))
	for path, config := range b.configs {
		count := 0
		for _, server := range b.servers {
			if server.ConfigPath == path {
				count++
			}
		}
		config.ServerCount = count
		configs = append(configs, config)
	}
	sort.Slice(configs, func(i, j int) bool { return configs[i].Path < configs[j].Path })

	servers := make([]Server, 0, len(b.servers))
	for _, server := range b.servers {
		servers = append(servers, server)
	}
	sort.Slice(servers, func(i, j int) bool {
		if servers[i].ConfigPath == servers[j].ConfigPath {
			return servers[i].Name < servers[j].Name
		}
		return servers[i].ConfigPath < servers[j].ConfigPath
	})

	errors := make([]DiscoveryError, 0, len(b.warnings))
	for _, warning := range b.warnings {
		errors = append(errors, DiscoveryError{Path: "external_inventory", Error: warning})
	}
	return Inventory{
		SchemaVersion: "boundary.firewall.inventory.v1",
		GeneratedAt:   b.generated,
		Root:          root,
		Configs:       configs,
		Servers:       servers,
		Summary:       summarize(servers, len(configs)),
		Errors:        errors,
	}
}

func mapBoundaryRecord(builder *externalInventoryBuilder, record InventoryRecord) bool {
	switch record.RecordType {
	case InventoryRecordMCPConfig:
		if record.MCPConfig == nil {
			return false
		}
		builder.addConfig(ConfigFile{
			Path:        record.MCPConfig.Path,
			Client:      ClientType(record.MCPConfig.Client),
			Scope:       record.MCPConfig.Scope,
			ServerCount: record.MCPConfig.ServerCount,
		})
		return true
	case InventoryRecordMCPServer:
		if record.MCPServer == nil {
			return false
		}
		builder.addServer(Server{
			Name:            record.MCPServer.Name,
			Client:          ClientType(record.MCPServer.Client),
			ConfigPath:      record.MCPServer.ConfigPath,
			Command:         record.MCPServer.Command,
			URL:             record.MCPServer.URL,
			Args:            append([]string(nil), record.MCPServer.Args...),
			EnvKeys:         append([]string(nil), record.MCPServer.EnvKeys...),
			DescriptorTools: append([]string(nil), record.MCPServer.DescriptorTools...),
			HighestRisk:     record.MCPServer.HighestRisk,
		})
		return true
	case InventoryRecordToolDescriptor:
		if record.ToolDescriptor == nil {
			return false
		}
		server := builder.serverFor(record.ToolDescriptor.ConfigPath, record.ToolDescriptor.Client, record.ToolDescriptor.Server)
		server.DescriptorTools = appendUnique(server.DescriptorTools, record.ToolDescriptor.Tool)
		builder.addServer(server)
		return true
	case InventoryRecordToolCapability:
		if record.ToolCapability == nil {
			return false
		}
		server := builder.serverFor(record.ToolCapability.ConfigPath, record.ToolCapability.Client, record.ToolCapability.Server)
		server.Capabilities = appendCapability(server.Capabilities, Capability{
			Name:          record.ToolCapability.Tool,
			Category:      record.ToolCapability.Category,
			Class:         record.ToolCapability.Class,
			SourceClass:   record.ToolCapability.SourceClass,
			SinkClass:     record.ToolCapability.SinkClass,
			MutationClass: record.ToolCapability.MutationClass,
			Reason:        record.ToolCapability.Reason,
		})
		builder.addServer(server)
		return true
	default:
		return false
	}
}

func mapGenericRecord(builder *externalInventoryBuilder, raw map[string]any) bool {
	if mapped := mapEmbeddedMCPConfig(builder, raw); mapped {
		return true
	}
	if server, ok := genericServerFromRecord(raw); ok {
		builder.addServer(server)
		return true
	}
	if finding, ok := exposureFindingFromRecord(raw); ok {
		builder.addFinding(finding)
		return true
	}
	if component, ok := externalComponentFromRecord(raw); ok {
		builder.addComponent(component)
		return true
	}
	return false
}

func mapEmbeddedMCPConfig(builder *externalInventoryBuilder, raw map[string]any) bool {
	for _, key := range []string{"mcpServers", "mcp_servers", "servers"} {
		if value, ok := raw[key]; ok && looksLikeServerMap(value) {
			return parseEmbeddedConfig(builder, raw, map[string]any{key: value})
		}
	}
	if mcp, ok := raw["mcp"].(map[string]any); ok {
		for _, key := range []string{"mcpServers", "mcp_servers", "servers"} {
			if value, ok := mcp[key]; ok && looksLikeServerMap(value) {
				return parseEmbeddedConfig(builder, raw, map[string]any{key: value})
			}
		}
	}
	return false
}

func parseEmbeddedConfig(builder *externalInventoryBuilder, raw map[string]any, config map[string]any) bool {
	configPath := stringValue(raw, "config_path", "path", "file", "source_path")
	if configPath == "" {
		configPath = stringValue(raw, "filename", "config")
	}
	if configPath == "" {
		configPath = "external-mcp.json"
	}
	client := clientTypeFromPath(configPath)
	body, err := json.Marshal(config)
	if err != nil {
		builder.addWarning(fmt.Sprintf("could not encode embedded MCP config %s: %v", configPath, err))
		return false
	}
	servers, err := parseConfig(configPath, client, body)
	if err != nil {
		builder.addWarning(fmt.Sprintf("could not parse embedded MCP config %s: %v", configPath, err))
		return false
	}
	builder.addConfig(ConfigFile{Path: configPath, Client: client, Scope: "external", ServerCount: len(servers)})
	for _, server := range servers {
		builder.addServer(server)
	}
	return len(servers) > 0
}

func genericServerFromRecord(raw map[string]any) (Server, bool) {
	name := stringValue(raw, "server_name", "server", "mcp_server")
	command := stringValue(raw, "command", "launcher", "executable")
	args := stringSliceValue(raw, "args", "arguments")
	for _, launcher := range []string{"npx", "uvx", "docker"} {
		if value := stringValue(raw, launcher); value != "" {
			if command == "" {
				command = launcher
			}
			args = append([]string{value}, args...)
		}
	}
	url := stringValue(raw, "url", "endpoint")
	configPath := stringValue(raw, "config_path", "path", "file", "source_path")
	tools := stringSliceValue(raw, "tools", "tool_names", "descriptor_tools")
	if len(tools) == 0 {
		tool := stringValue(raw, "tool", "tool_name")
		if tool != "" {
			tools = []string{tool}
		}
	}
	kind := strings.ToLower(stringValue(raw, "kind", "type"))
	if name == "" && strings.Contains(kind, "server") {
		name = stringValue(raw, "name")
	}
	if name == "" && command == "" && url == "" && len(tools) == 0 {
		return Server{}, false
	}
	server := Server{
		Name:            firstNonEmpty(name, stringValue(raw, "name"), command, url, "external-mcp-server"),
		Client:          clientTypeFromPath(configPath),
		ConfigPath:      firstNonEmpty(configPath, "external-mcp.json"),
		Command:         command,
		URL:             url,
		Args:            args,
		EnvKeys:         stringSliceValue(raw, "env_keys", "environment_keys"),
		DescriptorTools: tools,
	}
	return server, true
}

func externalComponentFromRecord(raw map[string]any) (ExternalInventoryComponent, bool) {
	name := stringValue(raw, "package", "package_name", "extension", "extension_id", "component", "name")
	if name == "" {
		return ExternalInventoryComponent{}, false
	}
	return ExternalInventoryComponent{
		Kind:     "external_inventory_component",
		Name:     name,
		Version:  stringValue(raw, "version"),
		Source:   stringValue(raw, "source", "scanner"),
		Severity: stringValue(raw, "severity", "risk"),
		Note:     "reporting-only package or extension finding; it is not used as an MCP action path",
	}, true
}

func exposureFindingFromRecord(raw map[string]any) (ExternalExposureFinding, bool) {
	name := stringValue(raw, "finding", "finding_type", "exposure", "rule", "title")
	if name == "" {
		return ExternalExposureFinding{}, false
	}
	mcpMapped := boolValue(raw, "mcp", "mcp_mapped")
	return ExternalExposureFinding{
		Kind:       "external_exposure_finding",
		Name:       name,
		Severity:   stringValue(raw, "severity", "risk"),
		Source:     stringValue(raw, "source", "scanner"),
		Target:     stringValue(raw, "target", "path", "package", "extension"),
		MCPMapped:  mcpMapped,
		ReportOnly: !mcpMapped,
		Note:       "reporting-only external exposure finding unless it maps to an MCP action path",
	}, true
}

func (b *externalInventoryBuilder) serverFor(configPath, client, name string) Server {
	configPath = firstNonEmpty(configPath, "external-inventory.ndjson")
	client = firstNonEmpty(client, string(clientTypeFromPath(configPath)))
	name = firstNonEmpty(name, "external-mcp-server")
	key := serverKey(configPath, client, name)
	if server, ok := b.servers[key]; ok {
		return server
	}
	return Server{Name: name, Client: ClientType(client), ConfigPath: configPath}
}

func mergeExternalServer(existing, next Server) Server {
	existing.Command = firstNonEmpty(existing.Command, next.Command)
	existing.URL = firstNonEmpty(existing.URL, next.URL)
	existing.Args = appendUnique(existing.Args, next.Args...)
	existing.EnvKeys = appendUnique(existing.EnvKeys, next.EnvKeys...)
	existing.DescriptorTools = appendUnique(existing.DescriptorTools, next.DescriptorTools...)
	for _, capability := range next.Capabilities {
		existing.Capabilities = appendCapability(existing.Capabilities, capability)
	}
	if len(existing.Capabilities) == 0 {
		existing.Capabilities = ClassifyServer(existing)
	}
	existing.HighestRisk = highestRisk(existing.Capabilities)
	return existing
}

func serverKey(configPath, client, name string) string {
	return cleanExternalPath(configPath) + "\x00" + strings.ToLower(client) + "\x00" + strings.ToLower(name)
}

func clientTypeFromPath(path string) ClientType {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "claude_desktop_config.json"):
		return ClientClaudeDesktop
	case strings.Contains(lower, "cursor"):
		return ClientCursor
	case strings.Contains(lower, ".vscode"), strings.Contains(lower, "code/user"):
		return ClientVSCode
	case strings.HasSuffix(lower, ".mcp.json"), strings.HasSuffix(lower, "mcp.json"):
		return ClientRepoLocal
	default:
		return ClientCustom
	}
}

func cleanExternalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func appendUnique(values []string, additions ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range values {
		appendValue(value)
	}
	for _, value := range additions {
		appendValue(value)
	}
	sort.Strings(out)
	return out
}

func appendCapability(values []Capability, addition Capability) []Capability {
	if addition.Name == "" {
		return values
	}
	for _, value := range values {
		if value.Name == addition.Name && value.Category == addition.Category && value.Class == addition.Class {
			return values
		}
	}
	return append(values, addition)
}

func looksLikeServerMap(value any) bool {
	entries, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if len(entries) == 0 {
		return false
	}
	for _, candidate := range entries {
		if _, ok := candidate.(map[string]any); ok {
			return true
		}
	}
	return false
}

func stringValue(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if text := strings.TrimSpace(typed.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func stringSliceValue(raw map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return appendUnique(nil, typed...)
		case []any:
			var out []string
			for _, item := range typed {
				switch item := item.(type) {
				case string:
					out = append(out, item)
				case map[string]any:
					out = append(out, stringValue(item, "name", "tool", "tool_name"))
				}
			}
			return appendUnique(nil, out...)
		case string:
			if strings.Contains(typed, ",") {
				return appendUnique(nil, strings.Split(typed, ",")...)
			}
			return appendUnique(nil, typed)
		}
	}
	return nil
}

func boolValue(raw map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return typed == "true" || typed == "yes" || typed == "1"
		}
	}
	return false
}
