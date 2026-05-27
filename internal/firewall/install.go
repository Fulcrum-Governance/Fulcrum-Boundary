package firewall

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const installReceiptSchema = "boundary.firewall.install_receipt.v1"

type InstallOptions struct {
	ConfigPath      string
	Client          ClientType
	OutDir          string
	ReceiptPath     string
	BoundaryCommand string
	Mode            string
	Servers         []string
	DryRun          bool
	Force           bool
	Now             time.Time
}

type InstallResult struct {
	SchemaVersion      string            `json:"schema_version"`
	GeneratedAt        string            `json:"generated_at"`
	ConfigPath         string            `json:"config_path"`
	Client             ClientType        `json:"client"`
	BackupPath         string            `json:"backup_path,omitempty"`
	ReceiptPath        string            `json:"receipt_path,omitempty"`
	ConfigSHA256Before string            `json:"config_sha256_before"`
	ConfigSHA256After  string            `json:"config_sha256_after,omitempty"`
	Mode               string            `json:"mode"`
	State              string            `json:"state"`
	DryRun             bool              `json:"dry_run"`
	Mutated            bool              `json:"mutated"`
	Servers            []InstalledServer `json:"servers"`
}

type InstalledServer struct {
	Name                    string   `json:"name"`
	OriginalDescriptorHash  string   `json:"original_descriptor_hash"`
	InstalledDescriptorHash string   `json:"installed_descriptor_hash"`
	OriginalCommand         string   `json:"original_command,omitempty"`
	OriginalURL             string   `json:"original_url,omitempty"`
	OriginalArgs            []string `json:"original_args,omitempty"`
	OriginalEnvKeys         []string `json:"original_env_keys,omitempty"`
	BoundaryCommand         string   `json:"boundary_command"`
	BoundaryArgs            []string `json:"boundary_args"`
}

type UninstallOptions struct {
	ReceiptPath string
	DryRun      bool
	Force       bool
}

type UninstallResult struct {
	SchemaVersion       string     `json:"schema_version"`
	ConfigPath          string     `json:"config_path"`
	BackupPath          string     `json:"backup_path"`
	ConfigSHA256Current string     `json:"config_sha256_current,omitempty"`
	ConfigSHA256Restore string     `json:"config_sha256_restore"`
	DryRun              bool       `json:"dry_run"`
	Forced              bool       `json:"forced"`
	Restored            bool       `json:"restored"`
	Servers             []string   `json:"servers"`
	RestoredAt          string     `json:"restored_at,omitempty"`
	Receipt             ReceiptRef `json:"receipt"`
}

type ReceiptRef struct {
	Path string `json:"path"`
}

func InstallConfig(options InstallOptions) (InstallResult, error) {
	configPath, err := cleanAbsPath(options.ConfigPath)
	if err != nil {
		return InstallResult{}, err
	}
	client := options.Client
	if client == "" {
		client = ClientCustom
	}
	mode := options.Mode
	if mode == "" {
		mode = "balanced"
	}
	boundaryCommand := options.BoundaryCommand
	if boundaryCommand == "" {
		boundaryCommand = "boundary"
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	originalBody, beforeHash, err := readFileBytes(configPath)
	if err != nil {
		return InstallResult{}, err
	}
	_, topLevel, err := parseRawMCPConfig(originalBody)
	if err != nil {
		return InstallResult{}, err
	}

	receiptPath := options.ReceiptPath
	if receiptPath == "" {
		base := strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
		name := fmt.Sprintf("%s.%s.%d.json", sanitizeFilename(base), beforeHash[:12], now.UnixNano())
		receiptPath, err = defaultWorkspacePath(options.OutDir, "install-receipts", name)
		if err != nil {
			return InstallResult{}, err
		}
	} else {
		receiptPath, err = cleanAbsPath(receiptPath)
		if err != nil {
			return InstallResult{}, err
		}
	}
	backupPath, err := defaultWorkspacePath(options.OutDir, "backups", fmt.Sprintf("%s.%s.bak", sanitizeFilename(filepath.Base(configPath)), beforeHash[:12]))
	if err != nil {
		return InstallResult{}, err
	}

	serverFilter := stringSet(options.Servers)
	servers, err := rewriteSelectedServers(topLevel, client, configPath, receiptPath, boundaryCommand, mode, serverFilter, options.Force)
	if err != nil {
		return InstallResult{}, err
	}
	if len(servers) == 0 {
		return InstallResult{}, fmt.Errorf("no MCP servers matched install selection")
	}
	rewrittenBody, err := encodeTopLevel(topLevel)
	if err != nil {
		return InstallResult{}, err
	}
	afterHash := sha256Hex(rewrittenBody)
	result := InstallResult{
		SchemaVersion:      installReceiptSchema,
		GeneratedAt:        now.UTC().Format(time.RFC3339),
		ConfigPath:         configPath,
		Client:             client,
		BackupPath:         backupPath,
		ReceiptPath:        receiptPath,
		ConfigSHA256Before: beforeHash,
		ConfigSHA256After:  afterHash,
		Mode:               mode,
		State:              "installed",
		DryRun:             options.DryRun,
		Mutated:            !options.DryRun,
		Servers:            servers,
	}
	if options.DryRun {
		result.BackupPath = ""
		result.ReceiptPath = ""
		result.State = "planned"
		result.Mutated = false
		return result, nil
	}

	if err := writeFileAtomic(backupPath, originalBody, 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("write backup: %w", err)
	}
	pending := result
	pending.State = "pending"
	pending.Mutated = false
	pendingBody, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return InstallResult{}, err
	}
	if err := writeFileAtomic(receiptPath, append(pendingBody, '\n'), 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("write pending install receipt: %w", err)
	}
	if err := writeFileAtomic(configPath, rewrittenBody, 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("rewrite MCP config: %w", err)
	}
	receiptBody, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return InstallResult{}, err
	}
	if err := writeFileAtomic(receiptPath, append(receiptBody, '\n'), 0o600); err != nil {
		return InstallResult{}, fmt.Errorf("finalize install receipt: %w", err)
	}
	return result, nil
}

func UninstallConfig(options UninstallOptions) (UninstallResult, error) {
	receiptPath, err := cleanAbsPath(options.ReceiptPath)
	if err != nil {
		return UninstallResult{}, err
	}
	body, err := os.ReadFile(receiptPath)
	if err != nil {
		return UninstallResult{}, err
	}
	var receipt InstallResult
	if err := json.Unmarshal(body, &receipt); err != nil {
		return UninstallResult{}, err
	}
	if receipt.SchemaVersion != installReceiptSchema {
		return UninstallResult{}, fmt.Errorf("unsupported install receipt schema %q", receipt.SchemaVersion)
	}
	if receipt.ConfigPath == "" || receipt.BackupPath == "" {
		return UninstallResult{}, fmt.Errorf("install receipt is missing config_path or backup_path")
	}
	if receipt.State != "installed" || !receipt.Mutated {
		return UninstallResult{}, fmt.Errorf("install receipt state %q is not restorable", receipt.State)
	}
	backupBody, restoreHash, err := readFileBytes(receipt.BackupPath)
	if err != nil {
		return UninstallResult{}, fmt.Errorf("read backup: %w", err)
	}
	var currentHash string
	if currentBody, err := os.ReadFile(receipt.ConfigPath); err == nil {
		currentHash = sha256Hex(currentBody)
	}
	serverNames := make([]string, 0, len(receipt.Servers))
	for _, server := range receipt.Servers {
		serverNames = append(serverNames, server.Name)
	}
	sort.Strings(serverNames)
	result := UninstallResult{
		SchemaVersion:       "boundary.firewall.uninstall.v1",
		ConfigPath:          receipt.ConfigPath,
		BackupPath:          receipt.BackupPath,
		ConfigSHA256Current: currentHash,
		ConfigSHA256Restore: restoreHash,
		DryRun:              options.DryRun,
		Forced:              options.Force,
		Restored:            !options.DryRun,
		Servers:             serverNames,
		Receipt:             ReceiptRef{Path: receiptPath},
	}
	if options.DryRun {
		return result, nil
	}
	if restoreHash != receipt.ConfigSHA256Before {
		return UninstallResult{}, fmt.Errorf("backup hash %s does not match receipt pre-install hash %s", restoreHash, receipt.ConfigSHA256Before)
	}
	if !options.Force && currentHash != receipt.ConfigSHA256After {
		return UninstallResult{}, fmt.Errorf("current config hash %s does not match installed hash %s; use force only after preserving post-install edits", currentHash, receipt.ConfigSHA256After)
	}
	if err := writeFileAtomic(receipt.ConfigPath, backupBody, 0o600); err != nil {
		return UninstallResult{}, fmt.Errorf("restore MCP config: %w", err)
	}
	result.RestoredAt = time.Now().UTC().Format(time.RFC3339)
	return result, nil
}

func rewriteSelectedServers(topLevel map[string]json.RawMessage, client ClientType, configPath, receiptPath, boundaryCommand, mode string, serverFilter map[string]bool, force bool) ([]InstalledServer, error) {
	var installed []InstalledServer
	rewriteMap := func(topLevelKey string) error {
		rawEntries, ok := topLevel[topLevelKey]
		if !ok {
			return nil
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(rawEntries, &entries); err != nil {
			return err
		}
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if len(serverFilter) > 0 && !serverFilter[name] {
				continue
			}
			rawMessage := entries[name]
			var raw rawServer
			if err := json.Unmarshal(rawMessage, &raw); err != nil {
				return err
			}
			if isBoundaryProxy(raw, boundaryCommand) && !force {
				return fmt.Errorf("server %q is already routed through Boundary; use --force to rewrite", name)
			}
			originalHash, err := descriptorHashForRawServer(name, raw)
			if err != nil {
				return err
			}
			boundaryArgs := []string{
				"mcp", "proxy",
				"--install-receipt", receiptPath,
				"--server", name,
				"--mode", mode,
			}
			serverObject := map[string]json.RawMessage{}
			if len(rawMessage) > 0 {
				if err := json.Unmarshal(rawMessage, &serverObject); err != nil {
					return err
				}
			}
			commandBody, _ := json.Marshal(boundaryCommand)
			argsBody, _ := json.Marshal(boundaryArgs)
			envBody, _ := json.Marshal(map[string]string{
				"BOUNDARY_MCP_CONFIG": configPath,
				"BOUNDARY_MCP_SERVER": name,
			})
			serverObject["command"] = commandBody
			serverObject["args"] = argsBody
			serverObject["env"] = envBody
			delete(serverObject, "url")
			replacementBody, err := json.Marshal(serverObject)
			if err != nil {
				return err
			}
			var replacement rawServer
			if err := json.Unmarshal(replacementBody, &replacement); err != nil {
				return err
			}
			installedHash, err := descriptorHashForRawServer(name, replacement)
			if err != nil {
				return err
			}
			entries[name] = replacementBody
			installed = append(installed, InstalledServer{
				Name:                    name,
				OriginalDescriptorHash:  originalHash,
				InstalledDescriptorHash: installedHash,
				OriginalCommand:         raw.Command,
				OriginalURL:             redactURL(raw.URL),
				OriginalArgs:            redactArgs(raw.Args),
				OriginalEnvKeys:         envKeys(raw.Env),
				BoundaryCommand:         boundaryCommand,
				BoundaryArgs:            boundaryArgs,
			})
		}
		body, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		topLevel[topLevelKey] = body
		return nil
	}
	if err := rewriteMap("mcpServers"); err != nil {
		return nil, err
	}
	if err := rewriteMap("servers"); err != nil {
		return nil, err
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })
	return installed, nil
}

func isBoundaryProxy(raw rawServer, boundaryCommand string) bool {
	if raw.Command != boundaryCommand || len(raw.Args) < 2 {
		return false
	}
	return raw.Args[0] == "mcp" && raw.Args[1] == "proxy"
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func sanitizeFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "config"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}
