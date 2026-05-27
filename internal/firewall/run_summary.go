package firewall

const (
	inventoryRecordSchemaVersion = "boundary.inventory.record.v1"

	InventoryRecordScanStart            = "scan_start"
	InventoryRecordAgentClient          = "agent_client"
	InventoryRecordMCPConfig            = "mcp_config"
	InventoryRecordMCPServer            = "mcp_server"
	InventoryRecordToolDescriptor       = "tool_descriptor"
	InventoryRecordToolCapability       = "tool_capability"
	InventoryRecordRiskPath             = "risk_path"
	InventoryRecordPolicyRecommendation = "policy_recommendation"
	InventoryRecordDescriptorLockStatus = "descriptor_lock_status"
	InventoryRecordInstallStatus        = "install_status"
	InventoryRecordDecisionRecordRef    = "decision_record_ref"
	InventoryRecordScanSummary          = "scan_summary"
)

type InventoryRunSummary struct {
	Status          string         `json:"status"`
	Complete        bool           `json:"complete"`
	ConfigFiles     int            `json:"config_files"`
	Servers         int            `json:"servers"`
	GitHubServers   int            `json:"github_servers"`
	HighRiskServers int            `json:"high_risk_servers"`
	UnknownServers  int            `json:"unknown_servers"`
	RiskPaths       int            `json:"risk_paths"`
	Errors          int            `json:"errors"`
	RecordCounts    map[string]int `json:"record_counts,omitempty"`
}

func buildInventoryRunSummary(inventory Inventory, riskGraph RiskGraph, recordCounts map[string]int) InventoryRunSummary {
	status := "complete"
	if len(inventory.Errors) > 0 {
		status = "partial"
	}
	return InventoryRunSummary{
		Status:          status,
		Complete:        status == "complete",
		ConfigFiles:     inventory.Summary.ConfigFiles,
		Servers:         inventory.Summary.Servers,
		GitHubServers:   inventory.Summary.GitHubServers,
		HighRiskServers: inventory.Summary.HighRiskServers,
		UnknownServers:  inventory.Summary.UnknownServers,
		RiskPaths:       riskGraph.Summary.Paths,
		Errors:          len(inventory.Errors),
		RecordCounts:    recordCounts,
	}
}
