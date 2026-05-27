package firewall

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

const descriptorLockSchema = "boundary.firewall.descriptor_lock.v1"

type LockOptions struct {
	ConfigPath string
	Client     ClientType
	OutPath    string
	Servers    []string
	DryRun     bool
	Now        time.Time
}

type DescriptorLockFile struct {
	SchemaVersion    string         `json:"schema_version"`
	GeneratedAt      string         `json:"generated_at"`
	ConfigPath       string         `json:"config_path"`
	Client           ClientType     `json:"client"`
	HashAlgorithm    string         `json:"hash_algorithm"`
	Canonicalization string         `json:"canonicalization"`
	Servers          []LockedServer `json:"servers"`
}

type LockedServer struct {
	Name            string               `json:"name"`
	DescriptorHash  string               `json:"descriptor_hash"`
	Command         string               `json:"command,omitempty"`
	URL             string               `json:"url,omitempty"`
	Args            []string             `json:"args,omitempty"`
	EnvKeys         []string             `json:"env_keys,omitempty"`
	Tools           []string             `json:"tools,omitempty"`
	ToolDescriptors []descriptorToolHash `json:"tool_descriptors,omitempty"`
	HighestRisk     string               `json:"highest_risk"`
	Capabilities    []Capability         `json:"capabilities"`
}

type LockResult struct {
	LockFile DescriptorLockFile `json:"lock_file"`
	Path     string             `json:"path,omitempty"`
	DryRun   bool               `json:"dry_run"`
	Written  bool               `json:"written"`
}

type VerifyLockOptions struct {
	LockPath   string
	ConfigPath string
	OnChange   string
}

type LockVerification struct {
	SchemaVersion string            `json:"schema_version"`
	LockPath      string            `json:"lock_path"`
	ConfigPath    string            `json:"config_path"`
	OnChange      string            `json:"on_change"`
	Status        string            `json:"status"`
	Allowed       bool              `json:"allowed"`
	Matches       []DescriptorMatch `json:"matches"`
	Summary       LockSummary       `json:"summary"`
}

type DescriptorMatch struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ExpectedHash   string `json:"expected_hash,omitempty"`
	ActualHash     string `json:"actual_hash,omitempty"`
	PolicyBehavior string `json:"policy_behavior,omitempty"`
}

type LockSummary struct {
	Servers    int `json:"servers"`
	Unchanged  int `json:"unchanged"`
	Changed    int `json:"changed"`
	Missing    int `json:"missing"`
	Unexpected int `json:"unexpected"`
}

func CreateDescriptorLock(options LockOptions) (LockResult, error) {
	configPath, err := cleanAbsPath(options.ConfigPath)
	if err != nil {
		return LockResult{}, err
	}
	client := options.Client
	if client == "" {
		client = ClientCustom
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	lockPath := options.OutPath
	if lockPath == "" {
		lockPath, err = defaultWorkspacePath(".boundary/firewall", "locks", "descriptor-lock.json")
		if err != nil {
			return LockResult{}, err
		}
	} else {
		lockPath, err = cleanAbsPath(lockPath)
		if err != nil {
			return LockResult{}, err
		}
	}
	servers, err := lockedServersFromConfig(configPath, client, stringSet(options.Servers))
	if err != nil {
		return LockResult{}, err
	}
	if len(servers) == 0 {
		return LockResult{}, fmt.Errorf("no MCP servers matched lock selection")
	}
	lock := DescriptorLockFile{
		SchemaVersion:    descriptorLockSchema,
		GeneratedAt:      now.UTC().Format(time.RFC3339),
		ConfigPath:       configPath,
		Client:           client,
		HashAlgorithm:    "sha256",
		Canonicalization: "canonical-json/redacted-secrets",
		Servers:          servers,
	}
	result := LockResult{LockFile: lock, Path: lockPath, DryRun: options.DryRun, Written: !options.DryRun}
	if options.DryRun {
		result.Path = ""
		result.Written = false
		return result, nil
	}
	body, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return LockResult{}, err
	}
	if err := writeFileAtomic(lockPath, append(body, '\n'), 0o600); err != nil {
		return LockResult{}, fmt.Errorf("write descriptor lock: %w", err)
	}
	return result, nil
}

func VerifyDescriptorLock(options VerifyLockOptions) (LockVerification, error) {
	lockPath, err := cleanAbsPath(options.LockPath)
	if err != nil {
		return LockVerification{}, err
	}
	// #nosec G304 -- lock verification reads the operator-selected Boundary descriptor lock path.
	body, err := os.ReadFile(lockPath)
	if err != nil {
		return LockVerification{}, err
	}
	var lock DescriptorLockFile
	if err := json.Unmarshal(body, &lock); err != nil {
		return LockVerification{}, err
	}
	if lock.SchemaVersion != descriptorLockSchema {
		return LockVerification{}, fmt.Errorf("unsupported descriptor lock schema %q", lock.SchemaVersion)
	}
	configPath := options.ConfigPath
	if configPath == "" {
		configPath = lock.ConfigPath
	}
	configPath, err = cleanAbsPath(configPath)
	if err != nil {
		return LockVerification{}, err
	}
	onChange := options.OnChange
	if onChange == "" {
		onChange = "deny"
	}
	switch onChange {
	case "warn", "require_approval", "deny":
	default:
		return LockVerification{}, fmt.Errorf("unsupported descriptor change mode %q", onChange)
	}
	current, err := lockedServersFromConfig(configPath, lock.Client, nil)
	if err != nil {
		return LockVerification{}, err
	}
	currentByName := map[string]LockedServer{}
	for _, server := range current {
		currentByName[server.Name] = server
	}
	lockedByName := map[string]LockedServer{}
	for _, server := range lock.Servers {
		lockedByName[server.Name] = server
	}

	var matches []DescriptorMatch
	summary := LockSummary{Servers: len(lock.Servers)}
	for _, expected := range lock.Servers {
		actual, ok := currentByName[expected.Name]
		match := DescriptorMatch{Name: expected.Name, ExpectedHash: expected.DescriptorHash, PolicyBehavior: onChange}
		if !ok {
			match.Status = "missing"
			summary.Missing++
		} else {
			match.ActualHash = actual.DescriptorHash
			if expected.DescriptorHash == actual.DescriptorHash {
				match.Status = "unchanged"
				match.PolicyBehavior = ""
				summary.Unchanged++
			} else {
				match.Status = "changed"
				summary.Changed++
			}
		}
		matches = append(matches, match)
	}
	for _, actual := range current {
		if _, ok := lockedByName[actual.Name]; !ok {
			summary.Unexpected++
			matches = append(matches, DescriptorMatch{
				Name:           actual.Name,
				Status:         "unexpected",
				ActualHash:     actual.DescriptorHash,
				PolicyBehavior: onChange,
			})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Name < matches[j].Name })

	status := "ok"
	allowed := true
	if summary.Changed > 0 || summary.Missing > 0 || summary.Unexpected > 0 {
		status = "drift"
		allowed = onChange == "warn"
	}
	return LockVerification{
		SchemaVersion: "boundary.firewall.lock_verification.v1",
		LockPath:      lockPath,
		ConfigPath:    configPath,
		OnChange:      onChange,
		Status:        status,
		Allowed:       allowed,
		Matches:       matches,
		Summary:       summary,
	}, nil
}

func lockedServersFromConfig(configPath string, client ClientType, serverFilter map[string]bool) ([]LockedServer, error) {
	body, _, err := readFileBytes(configPath)
	if err != nil {
		return nil, err
	}
	config, _, err := parseRawMCPConfig(body)
	if err != nil {
		return nil, err
	}
	var locked []LockedServer
	appendServers := func(entries map[string]rawServer) error {
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if len(serverFilter) > 0 && !serverFilter[name] {
				continue
			}
			raw := entries[name]
			hash, err := descriptorHashForRawServer(name, raw)
			if err != nil {
				return err
			}
			toolDescriptors, err := descriptorTools(raw.Tools)
			if err != nil {
				return err
			}
			server := Server{
				Name:            name,
				Client:          client,
				ConfigPath:      configPath,
				Command:         raw.Command,
				URL:             raw.URL,
				Args:            redactArgs(raw.Args),
				EnvKeys:         envKeys(raw.Env),
				DescriptorTools: toolNames(raw.Tools),
			}
			server.Capabilities = ClassifyServer(server)
			server.HighestRisk = highestRisk(server.Capabilities)
			locked = append(locked, LockedServer{
				Name:            name,
				DescriptorHash:  hash,
				Command:         server.Command,
				URL:             redactURL(server.URL),
				Args:            server.Args,
				EnvKeys:         server.EnvKeys,
				Tools:           server.DescriptorTools,
				ToolDescriptors: toolDescriptors,
				HighestRisk:     server.HighestRisk,
				Capabilities:    server.Capabilities,
			})
		}
		return nil
	}
	if err := appendServers(config.MCPServers); err != nil {
		return nil, err
	}
	if err := appendServers(config.Servers); err != nil {
		return nil, err
	}
	sort.Slice(locked, func(i, j int) bool { return locked[i].Name < locked[j].Name })
	return locked, nil
}
