package firewall

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExternalInventoryIngestOptions struct {
	File         string
	Source       string
	AllowPartial bool
}

func IngestExternalInventoryFile(options ExternalInventoryIngestOptions) (ExternalInventoryIngestResult, error) {
	if strings.TrimSpace(options.File) == "" {
		return ExternalInventoryIngestResult{}, fmt.Errorf("inventory ingest requires --file")
	}
	body, err := os.ReadFile(options.File)
	if err != nil {
		return ExternalInventoryIngestResult{}, err
	}
	result, err := IngestExternalInventoryNDJSON(body, options)
	if err != nil {
		return ExternalInventoryIngestResult{}, err
	}
	result.File = options.File
	return result, nil
}

func IngestExternalInventoryNDJSON(body []byte, options ExternalInventoryIngestOptions) (ExternalInventoryIngestResult, error) {
	source := normalizeExternalInventorySource(options.Source)
	if source == "" {
		return ExternalInventoryIngestResult{}, fmt.Errorf("unsupported inventory source %q", options.Source)
	}
	generated := time.Now().UTC().Format(time.RFC3339)
	builder := newExternalInventoryBuilder(source, generated)
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	recordsRead := 0
	boundaryRecords := 0
	sawCompleteSummary := false
	sawSummary := false
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		recordsRead++
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			builder.addWarning(fmt.Sprintf("line %d is not JSON and was skipped: %v", recordsRead, err))
			continue
		}
		_, summaryOnly := raw["scan_summary"]
		if complete, ok := summaryStatus(raw); ok {
			sawSummary = true
			if complete {
				sawCompleteSummary = true
			}
			summaryOnly = summaryOnly || externalSummaryOnly(raw)
		}

		recordType := stringValue(raw, "record_type")
		if recordType != "" {
			var record InventoryRecord
			if err := json.Unmarshal(line, &record); err != nil {
				builder.addWarning(fmt.Sprintf("line %d has Boundary record_type but could not decode: %v", recordsRead, err))
				continue
			}
			boundaryRecords++
			if mapBoundaryRecord(builder, record) {
				continue
			}
			continue
		}

		if source == "boundary" {
			builder.addWarning(fmt.Sprintf("line %d is not a Boundary inventory record and was ignored", recordsRead))
			continue
		}
		if summaryOnly {
			continue
		}
		if !mapGenericRecord(builder, raw) {
			builder.addWarning(fmt.Sprintf("line %d did not contain recognizable MCP inventory fields", recordsRead))
		}
	}
	if err := scanner.Err(); err != nil {
		return ExternalInventoryIngestResult{}, err
	}
	if recordsRead == 0 {
		builder.addWarning("input did not contain any NDJSON records")
	}
	if !sawSummary {
		builder.addWarning("no complete scan_summary record found; snapshot marked partial")
	} else if !sawCompleteSummary {
		builder.addWarning("scan_summary was present but not complete; snapshot marked partial")
	}

	status := "partial"
	if sawCompleteSummary && len(builder.warnings) == 0 {
		status = "complete"
	}
	installRecommendationsEnabled := status == "complete" || options.AllowPartial
	root := filepath.Dir(options.File)
	if root == "." || root == "" {
		root = "external_inventory"
	}
	inventory := builder.inventory(root)
	records := BuildInventoryRecords(inventory)
	if !installRecommendationsEnabled {
		records = disableInstallRecommendationRecords(inventory, records)
	}
	result := ExternalInventoryIngestResult{
		SchemaVersion:                 externalInventoryIngestSchemaVersion,
		GeneratedAt:                   generated,
		Source:                        source,
		File:                          options.File,
		SnapshotStatus:                status,
		Complete:                      status == "complete",
		AllowPartial:                  options.AllowPartial,
		InstallRecommendationsEnabled: installRecommendationsEnabled,
		Warnings:                      append([]string(nil), builder.warnings...),
		Inventory:                     inventory,
		Records:                       records,
		ExternalInventoryComponents:   append([]ExternalInventoryComponent(nil), builder.components...),
		ExternalExposureFindings:      append([]ExternalExposureFinding(nil), builder.findings...),
	}
	result.Summary = ExternalInventorySummary{
		RecordsRead:                   recordsRead,
		BoundaryRecords:               boundaryRecords,
		MCPConfigs:                    len(inventory.Configs),
		MCPServers:                    len(inventory.Servers),
		ExternalInventoryComponents:   len(builder.components),
		ExternalExposureFindings:      len(builder.findings),
		Warnings:                      len(builder.warnings),
		InstallRecommendationsEnabled: installRecommendationsEnabled,
	}
	return result, nil
}

func normalizeExternalInventorySource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "boundary":
		return "boundary"
	case "generic":
		return "generic"
	case "bumblebee", "bumblebee-style", "bumblebee_style":
		return "bumblebee"
	default:
		return ""
	}
}

func summaryStatus(raw map[string]any) (complete bool, found bool) {
	if summary, ok := raw["scan_summary"].(map[string]any); ok {
		if status := strings.ToLower(stringValue(summary, "status")); status != "" {
			return status == "complete", true
		}
		if complete, ok := summary["complete"].(bool); ok {
			return complete, true
		}
	}
	status := strings.ToLower(stringValue(raw, "status", "snapshot_status"))
	if status != "" && (status == "complete" || status == "partial" || status == "incomplete") {
		return status == "complete", true
	}
	if complete, ok := raw["complete"].(bool); ok {
		return complete, true
	}
	return false, false
}

func externalSummaryOnly(raw map[string]any) bool {
	for key := range raw {
		switch key {
		case "status", "snapshot_status", "complete", "records", "record_count", "scanner", "source", "generated_at", "timestamp":
			continue
		default:
			return false
		}
	}
	return true
}

func disableInstallRecommendationRecords(inventory Inventory, records []InventoryRecord) []InventoryRecord {
	filtered := make([]InventoryRecord, 0, len(records))
	for _, record := range records {
		if record.RecordType == InventoryRecordPolicyRecommendation {
			continue
		}
		if record.InstallStatus != nil {
			record.InstallStatus.Recommendation = "disabled for partial external inventory snapshot; rerun with --allow-partial only after operator review"
		}
		filtered = append(filtered, record)
	}
	scanID := inventoryScanID(inventory)
	recordCounts := map[string]int{}
	for i := range filtered {
		filtered[i].Sequence = i + 1
		filtered[i].RecordID = fmt.Sprintf("%s:%06d", scanID, i+1)
		recordCounts[filtered[i].RecordType]++
	}
	for i := range filtered {
		if filtered[i].ScanSummary != nil {
			filtered[i].ScanSummary.RecordCounts = cloneRecordCounts(recordCounts)
		}
	}
	return filtered
}

func RenderExternalInventoryIngestJSON(result ExternalInventoryIngestResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func RenderExternalInventoryIngestSummary(result ExternalInventoryIngestResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "inventory ingest: %s\n", result.File)
	fmt.Fprintf(&b, "source: %s\n", result.Source)
	fmt.Fprintf(&b, "snapshot status: %s\n", result.SnapshotStatus)
	fmt.Fprintf(&b, "records read: %d\n", result.Summary.RecordsRead)
	fmt.Fprintf(&b, "mcp configs: %d\n", result.Summary.MCPConfigs)
	fmt.Fprintf(&b, "mcp servers: %d\n", result.Summary.MCPServers)
	fmt.Fprintf(&b, "external components: %d\n", result.Summary.ExternalInventoryComponents)
	fmt.Fprintf(&b, "external findings: %d\n", result.Summary.ExternalExposureFindings)
	fmt.Fprintf(&b, "install recommendations enabled: %t\n", result.InstallRecommendationsEnabled)
	for _, warning := range result.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", warning)
	}
	return b.String()
}
