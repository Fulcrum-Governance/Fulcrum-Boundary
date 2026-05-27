package firewall

type ClientType string

const (
	ClientClaudeDesktop ClientType = "claude_desktop"
	ClientCursor        ClientType = "cursor"
	ClientVSCode        ClientType = "vscode"
	ClientRepoLocal     ClientType = "repo_local"
	ClientCustom        ClientType = "custom"
)

type DiscoverOptions struct {
	Root                  string
	Home                  string
	AdditionalConfigPaths []string
	IncludeDefaults       bool
}

type Candidate struct {
	Path   string     `json:"path"`
	Client ClientType `json:"client"`
	Scope  string     `json:"scope"`
}

type Inventory struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Root          string           `json:"root"`
	Configs       []ConfigFile     `json:"configs"`
	Servers       []Server         `json:"servers"`
	Summary       Summary          `json:"summary"`
	Errors        []DiscoveryError `json:"errors,omitempty"`
}

type ConfigFile struct {
	Path        string     `json:"path"`
	Client      ClientType `json:"client"`
	Scope       string     `json:"scope"`
	ServerCount int        `json:"server_count"`
}

type Server struct {
	Name            string       `json:"name"`
	Client          ClientType   `json:"client"`
	ConfigPath      string       `json:"config_path"`
	Command         string       `json:"command,omitempty"`
	URL             string       `json:"url,omitempty"`
	Args            []string     `json:"args,omitempty"`
	EnvKeys         []string     `json:"env_keys,omitempty"`
	DescriptorTools []string     `json:"descriptor_tools,omitempty"`
	Capabilities    []Capability `json:"capabilities"`
	HighestRisk     string       `json:"highest_risk"`
}

type Capability struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Class         string `json:"class"`
	SourceClass   string `json:"source_class,omitempty"`
	SinkClass     string `json:"sink_class,omitempty"`
	MutationClass string `json:"mutation_class,omitempty"`
	Reason        string `json:"reason"`
}

type Summary struct {
	ConfigFiles     int `json:"config_files"`
	Servers         int `json:"servers"`
	GitHubServers   int `json:"github_servers"`
	HighRiskServers int `json:"high_risk_servers"`
	UnknownServers  int `json:"unknown_servers"`
}

type DiscoveryError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}
