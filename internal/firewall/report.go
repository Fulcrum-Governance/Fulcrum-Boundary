package firewall

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func RenderInventory(inventory Inventory, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "", "json":
		return json.MarshalIndent(inventory, "", "  ")
	case "ndjson":
		return RenderInventoryNDJSON(inventory)
	case "markdown", "md":
		return []byte(renderMarkdown(inventory)), nil
	case "sarif":
		return json.MarshalIndent(renderSARIF(inventory), "", "  ")
	default:
		return nil, fmt.Errorf("unsupported inventory format %q", format)
	}
}

func renderMarkdown(inventory Inventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Boundary MCP Inventory\n\n")
	fmt.Fprintf(&b, "- Generated: `%s`\n", inventory.GeneratedAt)
	fmt.Fprintf(&b, "- Root: `%s`\n", inventory.Root)
	fmt.Fprintf(&b, "- Config files: `%d`\n", inventory.Summary.ConfigFiles)
	fmt.Fprintf(&b, "- Servers: `%d`\n", inventory.Summary.Servers)
	fmt.Fprintf(&b, "- High-risk servers: `%d`\n", inventory.Summary.HighRiskServers)
	fmt.Fprintf(&b, "- GitHub servers: `%d`\n\n", inventory.Summary.GitHubServers)

	if len(inventory.Errors) > 0 {
		fmt.Fprintln(&b, "## Discovery Warnings")
		fmt.Fprintln(&b)
		for _, err := range inventory.Errors {
			fmt.Fprintf(&b, "- `%s`: %s\n", err.Path, err.Error)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintln(&b, "## Servers")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Client | Server | Highest risk | Capabilities | Config |")
	fmt.Fprintln(&b, "|---|---|---|---|---|")
	for _, server := range inventory.Servers {
		fmt.Fprintf(&b, "| `%s` | `%s` | `%s` | %s | `%s` |\n",
			server.Client,
			server.Name,
			server.HighestRisk,
			markdownCapabilities(server.Capabilities),
			server.ConfigPath,
		)
	}
	return b.String()
}

func markdownCapabilities(capabilities []Capability) string {
	parts := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		parts = append(parts, fmt.Sprintf("`%s:%s`", capability.Name, capability.Class))
	}
	sort.Strings(parts)
	return strings.Join(parts, "<br>")
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	ShortDescription sarifMessage `json:"shortDescription"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

func renderSARIF(inventory Inventory) sarifLog {
	rules := map[string]sarifRule{
		"boundary.mcp.high-risk": {
			ID:               "boundary.mcp.high-risk",
			Name:             "High-risk MCP capability",
			ShortDescription: sarifMessage{Text: "MCP server exposes W1 or W2 capabilities"},
		},
		"boundary.mcp.unknown": {
			ID:               "boundary.mcp.unknown",
			Name:             "Unclassified MCP server",
			ShortDescription: sarifMessage{Text: "MCP server could not be classified"},
		},
	}
	var results []sarifResult
	for _, server := range inventory.Servers {
		switch server.HighestRisk {
		case "W1", "W2":
			results = append(results, sarifResult{
				RuleID:  "boundary.mcp.high-risk",
				Level:   "warning",
				Message: sarifMessage{Text: fmt.Sprintf("%s exposes %s MCP capabilities", server.Name, server.HighestRisk)},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{URI: server.ConfigPath},
					},
				}},
			})
		case "unknown":
			results = append(results, sarifResult{
				RuleID:  "boundary.mcp.unknown",
				Level:   "note",
				Message: sarifMessage{Text: fmt.Sprintf("%s could not be classified", server.Name)},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{URI: server.ConfigPath},
					},
				}},
			})
		}
	}

	ruleList := make([]sarifRule, 0, len(rules))
	for _, rule := range rules {
		ruleList = append(ruleList, rule)
	}
	sort.Slice(ruleList, func(i, j int) bool { return ruleList[i].ID < ruleList[j].ID })

	return sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "Fulcrum Boundary MCP Inventory",
				InformationURI: "https://github.com/Fulcrum-Governance/Fulcrum-Boundary",
				Rules:          ruleList,
			}},
			Results: results,
		}},
	}
}
