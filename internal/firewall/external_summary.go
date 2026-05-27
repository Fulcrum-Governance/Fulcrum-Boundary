package firewall

const externalInventoryIngestSchemaVersion = "boundary.firewall.external_inventory_ingest.v1"

type ExternalInventorySummary struct {
	RecordsRead                   int  `json:"records_read"`
	BoundaryRecords               int  `json:"boundary_records"`
	MCPConfigs                    int  `json:"mcp_configs"`
	MCPServers                    int  `json:"mcp_servers"`
	ExternalInventoryComponents   int  `json:"external_inventory_components"`
	ExternalExposureFindings      int  `json:"external_exposure_findings"`
	Warnings                      int  `json:"warnings"`
	InstallRecommendationsEnabled bool `json:"install_recommendations_enabled"`
}

type ExternalInventoryComponent struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Source   string `json:"source,omitempty"`
	Severity string `json:"severity,omitempty"`
	Note     string `json:"note"`
}

type ExternalExposureFinding struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Severity   string `json:"severity,omitempty"`
	Source     string `json:"source,omitempty"`
	Target     string `json:"target,omitempty"`
	MCPMapped  bool   `json:"mcp_mapped"`
	ReportOnly bool   `json:"report_only"`
	Note       string `json:"note"`
}

type ExternalInventoryIngestResult struct {
	SchemaVersion                 string                       `json:"schema_version"`
	GeneratedAt                   string                       `json:"generated_at"`
	Source                        string                       `json:"source"`
	File                          string                       `json:"file"`
	SnapshotStatus                string                       `json:"snapshot_status"`
	Complete                      bool                         `json:"complete"`
	AllowPartial                  bool                         `json:"allow_partial"`
	InstallRecommendationsEnabled bool                         `json:"install_recommendations_enabled"`
	Warnings                      []string                     `json:"warnings,omitempty"`
	Inventory                     Inventory                    `json:"inventory"`
	Records                       []InventoryRecord            `json:"records"`
	ExternalInventoryComponents   []ExternalInventoryComponent `json:"external_inventory_components,omitempty"`
	ExternalExposureFindings      []ExternalExposureFinding    `json:"external_exposure_findings,omitempty"`
	Summary                       ExternalInventorySummary     `json:"summary"`
}
