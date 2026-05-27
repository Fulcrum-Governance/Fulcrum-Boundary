package firewall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type InventoryRecord struct {
	SchemaVersion string `json:"schema_version"`
	RecordType    string `json:"record_type"`
	RecordID      string `json:"record_id"`
	ScanID        string `json:"scan_id"`
	Sequence      int    `json:"sequence"`
	Timestamp     string `json:"timestamp"`

	ScanStart            *ScanStartRecord            `json:"scan_start,omitempty"`
	AgentClient          *AgentClientRecord          `json:"agent_client,omitempty"`
	MCPConfig            *MCPConfigRecord            `json:"mcp_config,omitempty"`
	MCPServer            *MCPServerRecord            `json:"mcp_server,omitempty"`
	ToolDescriptor       *ToolDescriptorRecord       `json:"tool_descriptor,omitempty"`
	ToolCapability       *ToolCapabilityRecord       `json:"tool_capability,omitempty"`
	RiskPath             *RiskPathRecord             `json:"risk_path,omitempty"`
	PolicyRecommendation *PolicyRecommendationRecord `json:"policy_recommendation,omitempty"`
	DescriptorLockStatus *DescriptorLockStatusRecord `json:"descriptor_lock_status,omitempty"`
	InstallStatus        *InstallStatusRecord        `json:"install_status,omitempty"`
	DecisionRecordRef    *DecisionRecordRef          `json:"decision_record_ref,omitempty"`
	ScanSummary          *InventoryRunSummary        `json:"scan_summary,omitempty"`
}

type ScanStartRecord struct {
	Root          string   `json:"root"`
	GeneratedAt   string   `json:"generated_at"`
	SchemaVersion string   `json:"inventory_schema_version"`
	Scope         []string `json:"scope"`
}

type AgentClientRecord struct {
	Client      string `json:"client"`
	Scope       string `json:"scope"`
	ConfigFiles int    `json:"config_files"`
}

type MCPConfigRecord struct {
	Path        string `json:"path"`
	Client      string `json:"client"`
	Scope       string `json:"scope"`
	ServerCount int    `json:"server_count"`
}

type MCPServerRecord struct {
	Name            string   `json:"name"`
	Client          string   `json:"client"`
	ConfigPath      string   `json:"config_path"`
	Command         string   `json:"command,omitempty"`
	URL             string   `json:"url,omitempty"`
	Args            []string `json:"args,omitempty"`
	EnvKeys         []string `json:"env_keys,omitempty"`
	DescriptorTools []string `json:"descriptor_tools,omitempty"`
	HighestRisk     string   `json:"highest_risk"`
	GovernedRoute   bool     `json:"governed_route"`
}

type ToolDescriptorRecord struct {
	Server     string `json:"server"`
	Client     string `json:"client"`
	ConfigPath string `json:"config_path"`
	Tool       string `json:"tool"`
}

type ToolCapabilityRecord struct {
	Server        string `json:"server"`
	Client        string `json:"client"`
	ConfigPath    string `json:"config_path"`
	Tool          string `json:"tool"`
	Category      string `json:"category"`
	Class         string `json:"class"`
	SourceClass   string `json:"source_class,omitempty"`
	SinkClass     string `json:"sink_class,omitempty"`
	MutationClass string `json:"mutation_class,omitempty"`
	Reason        string `json:"reason"`
}

type RiskPathRecord struct {
	ID         string `json:"id"`
	Category   string `json:"category"`
	Server     string `json:"server"`
	Client     string `json:"client"`
	ConfigPath string `json:"config_path"`
	Tool       string `json:"tool"`
	Source     string `json:"source"`
	Sink       string `json:"sink"`
	RiskClass  string `json:"risk_class"`
	Reason     string `json:"reason"`
	Mitigation string `json:"mitigation"`
}

type PolicyRecommendationRecord struct {
	Name        string   `json:"name"`
	Filename    string   `json:"filename"`
	Description string   `json:"description"`
	Mode        string   `json:"mode"`
	RiskPaths   []string `json:"risk_paths,omitempty"`
	Review      string   `json:"review"`
}

type DescriptorLockStatusRecord struct {
	ConfigPath string `json:"config_path"`
	Client     string `json:"client"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
}

type InstallStatusRecord struct {
	Server         string `json:"server"`
	Client         string `json:"client"`
	ConfigPath     string `json:"config_path"`
	Status         string `json:"status"`
	Governed       bool   `json:"governed"`
	BypassPath     string `json:"bypass_path"`
	Recommendation string `json:"recommendation"`
}

type DecisionRecordRef struct {
	RecordID string `json:"record_id"`
	Path     string `json:"path,omitempty"`
	Note     string `json:"note,omitempty"`
}

func BuildInventoryRecords(inventory Inventory) []InventoryRecord {
	riskGraph := BuildRiskGraph(inventory)
	scanID := inventoryScanID(inventory)
	recordCounts := map[string]int{}
	sequence := 0
	records := make([]InventoryRecord, 0, 2+len(inventory.Configs)+len(inventory.Servers)*4+len(riskGraph.Paths))

	appendRecord := func(recordType string, apply func(*InventoryRecord)) {
		sequence++
		record := InventoryRecord{
			SchemaVersion: inventoryRecordSchemaVersion,
			RecordType:    recordType,
			RecordID:      fmt.Sprintf("%s:%06d", scanID, sequence),
			ScanID:        scanID,
			Sequence:      sequence,
			Timestamp:     inventory.GeneratedAt,
		}
		apply(&record)
		records = append(records, record)
		recordCounts[recordType]++
	}

	appendRecord(InventoryRecordScanStart, func(record *InventoryRecord) {
		record.ScanStart = &ScanStartRecord{
			Root:          inventory.Root,
			GeneratedAt:   inventory.GeneratedAt,
			SchemaVersion: inventory.SchemaVersion,
			Scope:         []string{"mcp_agent_clients", "mcp_configs", "mcp_servers", "tool_descriptors", "risk_paths", "policy_recommendations", "descriptor_locks", "install_routes"},
		}
	})

	for _, client := range agentClientRecords(inventory.Configs) {
		client := client
		appendRecord(InventoryRecordAgentClient, func(record *InventoryRecord) {
			record.AgentClient = &client
		})
	}
	for _, config := range inventory.Configs {
		configRecord := MCPConfigRecord{
			Path:        config.Path,
			Client:      string(config.Client),
			Scope:       config.Scope,
			ServerCount: config.ServerCount,
		}
		appendRecord(InventoryRecordMCPConfig, func(record *InventoryRecord) {
			record.MCPConfig = &configRecord
		})
		lockRecord := DescriptorLockStatusRecord{
			ConfigPath: config.Path,
			Client:     string(config.Client),
			Status:     "not_checked",
			Reason:     "inventory reports descriptor lock scope but does not verify a lockfile unless boundary verify-lock is run",
		}
		appendRecord(InventoryRecordDescriptorLockStatus, func(record *InventoryRecord) {
			record.DescriptorLockStatus = &lockRecord
		})
	}

	for _, server := range inventory.Servers {
		server := server
		serverRecord := MCPServerRecord{
			Name:            server.Name,
			Client:          string(server.Client),
			ConfigPath:      server.ConfigPath,
			Command:         redactSecretBearingValue(server.Command),
			URL:             redactURL(server.URL),
			Args:            append([]string(nil), server.Args...),
			EnvKeys:         append([]string(nil), server.EnvKeys...),
			DescriptorTools: append([]string(nil), server.DescriptorTools...),
			HighestRisk:     server.HighestRisk,
			GovernedRoute:   isGovernedRoute(server),
		}
		appendRecord(InventoryRecordMCPServer, func(record *InventoryRecord) {
			record.MCPServer = &serverRecord
		})
		for _, tool := range server.DescriptorTools {
			descriptorRecord := ToolDescriptorRecord{
				Server:     server.Name,
				Client:     string(server.Client),
				ConfigPath: server.ConfigPath,
				Tool:       tool,
			}
			appendRecord(InventoryRecordToolDescriptor, func(record *InventoryRecord) {
				record.ToolDescriptor = &descriptorRecord
			})
		}
		for _, capability := range server.Capabilities {
			capabilityRecord := ToolCapabilityRecord{
				Server:        server.Name,
				Client:        string(server.Client),
				ConfigPath:    server.ConfigPath,
				Tool:          capability.Name,
				Category:      capability.Category,
				Class:         capability.Class,
				SourceClass:   capability.SourceClass,
				SinkClass:     capability.SinkClass,
				MutationClass: capability.MutationClass,
				Reason:        capability.Reason,
			}
			appendRecord(InventoryRecordToolCapability, func(record *InventoryRecord) {
				record.ToolCapability = &capabilityRecord
			})
		}
		installRecord := installStatusRecord(server)
		appendRecord(InventoryRecordInstallStatus, func(record *InventoryRecord) {
			record.InstallStatus = &installRecord
		})
	}

	for _, path := range riskGraph.Paths {
		pathRecord := RiskPathRecord(path)
		appendRecord(InventoryRecordRiskPath, func(record *InventoryRecord) {
			record.RiskPath = &pathRecord
		})
	}

	for _, recommendation := range policyRecommendationRecords(riskGraph) {
		recommendation := recommendation
		appendRecord(InventoryRecordPolicyRecommendation, func(record *InventoryRecord) {
			record.PolicyRecommendation = &recommendation
		})
	}

	summaryCounts := cloneRecordCounts(recordCounts)
	summaryCounts[InventoryRecordScanSummary] = 1
	summary := buildInventoryRunSummary(inventory, riskGraph, summaryCounts)
	appendRecord(InventoryRecordScanSummary, func(record *InventoryRecord) {
		record.ScanSummary = &summary
	})
	return records
}

func agentClientRecords(configs []ConfigFile) []AgentClientRecord {
	type key struct {
		client string
		scope  string
	}
	counts := map[key]int{}
	for _, config := range configs {
		counts[key{client: string(config.Client), scope: config.Scope}]++
	}
	keys := make([]key, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].client == keys[j].client {
			return keys[i].scope < keys[j].scope
		}
		return keys[i].client < keys[j].client
	})
	records := make([]AgentClientRecord, 0, len(keys))
	for _, key := range keys {
		records = append(records, AgentClientRecord{
			Client:      key.client,
			Scope:       key.scope,
			ConfigFiles: counts[key],
		})
	}
	return records
}

func policyRecommendationRecords(graph RiskGraph) []PolicyRecommendationRecord {
	pathIDsByTemplate := map[string]map[string]bool{}
	for _, path := range graph.Paths {
		for _, template := range templatesForRiskPath(path) {
			if pathIDsByTemplate[template] == nil {
				pathIDsByTemplate[template] = map[string]bool{}
			}
			pathIDsByTemplate[template][path.ID] = true
		}
	}
	var recommendations []PolicyRecommendationRecord
	for _, template := range StarterPolicyTemplates() {
		pathIDs := sortedSet(pathIDsByTemplate[template.Name])
		if len(pathIDs) == 0 && template.Name != "descriptor-integrity" {
			continue
		}
		recommendations = append(recommendations, PolicyRecommendationRecord{
			Name:        template.Name,
			Filename:    template.Filename,
			Description: template.Description,
			Mode:        "starter_policy_for_operator_review",
			RiskPaths:   pathIDs,
			Review:      "starter policy recommendation only; operators must review before relying on it in production",
		})
	}
	return recommendations
}

func templatesForRiskPath(path RiskPath) []string {
	switch path.Category {
	case "descriptor_change":
		return []string{"descriptor-integrity"}
	case "destructive_db_action":
		return []string{"postgres"}
	case "external_sink":
		return []string{"slack"}
	case "filesystem_exfil":
		return []string{"filesystem"}
	case "repo_write_path", "untrusted_input_to_private_data", "untrusted_input_to_private_repo_mutation":
		return []string{"github"}
	case "privileged_mutation":
		if strings.Contains(strings.ToLower(path.Server), "shell") || strings.Contains(strings.ToLower(path.Tool), "command") {
			return []string{"shell"}
		}
		return []string{"github"}
	default:
		return nil
	}
}

func installStatusRecord(server Server) InstallStatusRecord {
	governed := isGovernedRoute(server)
	status := "ungoverned_route"
	recommendation := "route this server through boundary install before relying on pre-execution decisions"
	if governed {
		status = "governed_route_detected"
		recommendation = "verify descriptor locks and policy routing before enforcement"
	}
	return InstallStatusRecord{ // #nosec G101 -- BypassPath is explanatory output text, not a credential.
		Server:         server.Name,
		Client:         string(server.Client),
		ConfigPath:     server.ConfigPath,
		Status:         status,
		Governed:       governed,
		BypassPath:     "direct MCP client routes are outside Boundary unless deployment topology prevents direct routing",
		Recommendation: recommendation,
	}
}

func isGovernedRoute(server Server) bool {
	command := strings.ToLower(server.Command)
	if strings.Contains(command, "boundary") {
		return true
	}
	args := strings.ToLower(strings.Join(server.Args, " "))
	return strings.Contains(args, "boundary") && strings.Contains(args, "mcp")
}

func inventoryScanID(inventory Inventory) string {
	hash := sha256.New()
	hash.Write([]byte(inventory.SchemaVersion))
	hash.Write([]byte{0})
	hash.Write([]byte(inventory.GeneratedAt))
	hash.Write([]byte{0})
	hash.Write([]byte(inventory.Root))
	for _, config := range inventory.Configs {
		hash.Write([]byte{0})
		hash.Write([]byte(config.Path))
		hash.Write([]byte(config.Client))
		hash.Write([]byte(config.Scope))
	}
	for _, server := range inventory.Servers {
		hash.Write([]byte{0})
		hash.Write([]byte(server.ConfigPath))
		hash.Write([]byte(server.Name))
		hash.Write([]byte(server.HighestRisk))
	}
	sum := hash.Sum(nil)
	return "scan_" + hex.EncodeToString(sum[:8])
}

func cloneRecordCounts(counts map[string]int) map[string]int {
	cloned := make(map[string]int, len(counts))
	for key, value := range counts {
		cloned[key] = value
	}
	return cloned
}

func sortedSet(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func redactSecretBearingValue(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "token=") || strings.Contains(lower, "password=") ||
		strings.Contains(lower, "secret=") || strings.Contains(lower, "api_key=") ||
		strings.Contains(lower, "apikey=") {
		return "[redacted]"
	}
	return value
}
