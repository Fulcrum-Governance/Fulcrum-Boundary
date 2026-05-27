package firewall

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type rawMCPConfig struct {
	MCPServers map[string]rawServer `json:"mcpServers"`
	Servers    map[string]rawServer `json:"servers"`
}

type rawServer struct {
	Command string         `json:"command"`
	URL     string         `json:"url"`
	Args    []string       `json:"args"`
	Env     map[string]any `json:"env"`
	Tools   []rawTool      `json:"tools"`
}

type rawTool struct {
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	InputSchema       json.RawMessage `json:"inputSchema"`
	OutputSchema      json.RawMessage `json:"outputSchema"`
	InputSchemaSnake  json.RawMessage `json:"input_schema"`
	OutputSchemaSnake json.RawMessage `json:"output_schema"`
}

func BuildInventory(options DiscoverOptions) (Inventory, error) {
	root := options.Root
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	home := options.Home
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = userHome
		}
	}

	candidates := make([]Candidate, 0)
	candidates = append(candidates, RepoLocalCandidates(absRoot)...)
	if options.IncludeDefaults {
		candidates = append(candidates, UserDefaultCandidates(home)...)
	}
	for _, path := range options.AdditionalConfigPaths {
		expanded := expandPath(path, home)
		abs, err := filepath.Abs(expanded)
		if err != nil {
			return Inventory{}, err
		}
		candidates = append(candidates, Candidate{
			Path:   abs,
			Client: ClientCustom,
			Scope:  "explicit",
		})
	}

	inventory := Inventory{
		SchemaVersion: "boundary.firewall.inventory.v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Root:          absRoot,
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		path := filepath.Clean(candidate.Path)
		if seen[path] {
			continue
		}
		seen[path] = true
		stat, err := os.Stat(path)
		if err != nil || stat.IsDir() {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			inventory.Errors = append(inventory.Errors, DiscoveryError{Path: path, Error: err.Error()})
			continue
		}
		servers, err := parseConfig(path, candidate.Client, body)
		if err != nil {
			inventory.Errors = append(inventory.Errors, DiscoveryError{Path: path, Error: err.Error()})
			continue
		}
		inventory.Configs = append(inventory.Configs, ConfigFile{
			Path:        path,
			Client:      candidate.Client,
			Scope:       candidate.Scope,
			ServerCount: len(servers),
		})
		inventory.Servers = append(inventory.Servers, servers...)
	}

	sort.Slice(inventory.Configs, func(i, j int) bool {
		return inventory.Configs[i].Path < inventory.Configs[j].Path
	})
	sort.Slice(inventory.Servers, func(i, j int) bool {
		if inventory.Servers[i].ConfigPath == inventory.Servers[j].ConfigPath {
			return inventory.Servers[i].Name < inventory.Servers[j].Name
		}
		return inventory.Servers[i].ConfigPath < inventory.Servers[j].ConfigPath
	})
	inventory.Summary = summarize(inventory.Servers, len(inventory.Configs))
	return inventory, nil
}

func DefaultCandidates(root, home string) []Candidate {
	var candidates []Candidate
	candidates = append(candidates, RepoLocalCandidates(root)...)
	candidates = append(candidates, UserDefaultCandidates(home)...)
	return candidates
}

func RepoLocalCandidates(root string) []Candidate {
	var candidates []Candidate
	if root != "" {
		candidates = append(candidates,
			Candidate{Path: filepath.Join(root, ".mcp.json"), Client: ClientRepoLocal, Scope: "repo"},
			Candidate{Path: filepath.Join(root, "mcp.json"), Client: ClientRepoLocal, Scope: "repo"},
			Candidate{Path: filepath.Join(root, ".cursor", "mcp.json"), Client: ClientCursor, Scope: "repo"},
			Candidate{Path: filepath.Join(root, ".vscode", "mcp.json"), Client: ClientVSCode, Scope: "repo"},
		)
	}
	return candidates
}

func UserDefaultCandidates(home string) []Candidate {
	var candidates []Candidate
	if home != "" {
		candidates = append(candidates,
			Candidate{Path: filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), Client: ClientClaudeDesktop, Scope: "user"},
			Candidate{Path: filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), Client: ClientClaudeDesktop, Scope: "user"},
			Candidate{Path: filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json"), Client: ClientClaudeDesktop, Scope: "user"},
			Candidate{Path: filepath.Join(home, "Library", "Application Support", "Cursor", "User", "mcp.json"), Client: ClientCursor, Scope: "user"},
			Candidate{Path: filepath.Join(home, ".cursor", "mcp.json"), Client: ClientCursor, Scope: "user"},
			Candidate{Path: filepath.Join(home, ".config", "Cursor", "User", "mcp.json"), Client: ClientCursor, Scope: "user"},
			Candidate{Path: filepath.Join(home, "Library", "Application Support", "Code", "User", "mcp.json"), Client: ClientVSCode, Scope: "user"},
			Candidate{Path: filepath.Join(home, ".config", "Code", "User", "mcp.json"), Client: ClientVSCode, Scope: "user"},
			Candidate{Path: filepath.Join(home, "AppData", "Roaming", "Code", "User", "mcp.json"), Client: ClientVSCode, Scope: "user"},
		)
	}
	return candidates
}

func parseConfig(path string, client ClientType, body []byte) ([]Server, error) {
	var config rawMCPConfig
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}
	servers := make([]Server, 0, len(config.MCPServers))
	appendServers := func(entries map[string]rawServer) {
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			raw := entries[name]
			server := Server{
				Name:            name,
				Client:          client,
				ConfigPath:      filepath.Clean(path),
				Command:         raw.Command,
				URL:             raw.URL,
				Args:            redactArgs(raw.Args),
				EnvKeys:         envKeys(raw.Env),
				DescriptorTools: toolNames(raw.Tools),
			}
			server.Capabilities = ClassifyServer(server)
			server.HighestRisk = highestRisk(server.Capabilities)
			servers = append(servers, server)
		}
	}
	appendServers(config.MCPServers)
	appendServers(config.Servers)
	return servers, nil
}

func envKeys(env map[string]any) []string {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func toolNames(tools []rawTool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool.Name != "" {
			names = append(names, tool.Name)
		}
	}
	sort.Strings(names)
	return names
}

func redactArgs(args []string) []string {
	redacted := append([]string(nil), args...)
	for i, arg := range args {
		lower := strings.ToLower(arg)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") || strings.Contains(lower, "api_key") ||
			strings.Contains(lower, "apikey") {
			redacted[i] = "[redacted]"
			continue
		}
		if i > 0 {
			prev := strings.ToLower(args[i-1])
			if prev == "--token" || prev == "--password" || prev == "--api-key" || prev == "--secret" {
				redacted[i] = "[redacted]"
			}
		}
	}
	return redacted
}

func expandPath(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") && home != "" {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func summarize(servers []Server, configFiles int) Summary {
	summary := Summary{ConfigFiles: configFiles, Servers: len(servers)}
	for _, server := range servers {
		if server.HighestRisk == "W1" || server.HighestRisk == "W2" {
			summary.HighRiskServers++
		}
		if strings.Contains(strings.ToLower(server.Name+" "+server.Command+" "+strings.Join(server.Args, " ")), "github") {
			summary.GitHubServers++
		}
		if server.HighestRisk == "unknown" {
			summary.UnknownServers++
		}
	}
	return summary
}
