package firewall

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type RiskGraph struct {
	SchemaVersion    string           `json:"schema_version"`
	GeneratedAt      string           `json:"generated_at"`
	Root             string           `json:"root"`
	InventorySummary Summary          `json:"inventory_summary"`
	Errors           []DiscoveryError `json:"errors,omitempty"`
	Nodes            []RiskNode       `json:"nodes"`
	Edges            []RiskEdge       `json:"edges"`
	Paths            []RiskPath       `json:"paths"`
	Summary          RiskSummary      `json:"summary"`
}

type RiskNode struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	Server string `json:"server,omitempty"`
	Client string `json:"client,omitempty"`
}

type RiskEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Label     string `json:"label"`
	RiskClass string `json:"risk_class"`
	PathID    string `json:"path_id"`
}

type RiskPath struct {
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

type RiskSummary struct {
	Paths                 int `json:"paths"`
	HighRiskPaths         int `json:"high_risk_paths"`
	DescriptorChangePaths int `json:"descriptor_change_paths"`
	RepoWritePaths        int `json:"repo_write_paths"`
	ExternalSinkPaths     int `json:"external_sink_paths"`
}

func BuildRiskGraph(inventory Inventory) RiskGraph {
	paths := DetectRiskPaths(inventory)
	graph := RiskGraph{
		SchemaVersion:    "boundary.firewall.risk_graph.v1",
		GeneratedAt:      inventory.GeneratedAt,
		Root:             inventory.Root,
		InventorySummary: inventory.Summary,
		Errors:           append([]DiscoveryError{}, inventory.Errors...),
		Paths:            paths,
	}

	nodes := map[string]RiskNode{}
	edges := map[string]RiskEdge{}
	addNode := func(node RiskNode) {
		if _, ok := nodes[node.ID]; !ok {
			nodes[node.ID] = node
		}
	}
	addEdge := func(edge RiskEdge) {
		key := edge.From + "->" + edge.To + ":" + edge.PathID
		if _, ok := edges[key]; !ok {
			edges[key] = edge
		}
	}

	for _, path := range paths {
		sourceID := graphNodeID("source", path.Source)
		serverID := graphNodeID("server", path.Client, path.Server)
		sinkID := graphNodeID("sink", path.Sink)
		addNode(RiskNode{ID: sourceID, Label: path.Source, Type: "source"})
		addNode(RiskNode{ID: serverID, Label: path.Server, Type: "server", Server: path.Server, Client: path.Client})
		addNode(RiskNode{ID: sinkID, Label: path.Sink, Type: "sink"})
		addEdge(RiskEdge{From: sourceID, To: serverID, Label: path.Tool, RiskClass: path.RiskClass, PathID: path.ID})
		addEdge(RiskEdge{From: serverID, To: sinkID, Label: path.Category, RiskClass: path.RiskClass, PathID: path.ID})
	}

	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].ID < graph.Nodes[j].ID })
	for _, edge := range edges {
		graph.Edges = append(graph.Edges, edge)
	}
	sort.Slice(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].From == graph.Edges[j].From {
			if graph.Edges[i].To == graph.Edges[j].To {
				return graph.Edges[i].PathID < graph.Edges[j].PathID
			}
			return graph.Edges[i].To < graph.Edges[j].To
		}
		return graph.Edges[i].From < graph.Edges[j].From
	})
	graph.Summary = summarizeRiskPaths(paths)
	return graph
}

func DetectRiskPaths(inventory Inventory) []RiskPath {
	var paths []RiskPath
	for _, server := range inventory.Servers {
		paths = append(paths, RiskPath{
			ID:         riskPathID(server, "descriptor_change", "descriptor"),
			Category:   "descriptor_change",
			Server:     server.Name,
			Client:     string(server.Client),
			Tool:       "descriptor",
			Source:     "mcp_descriptor",
			Sink:       "policy_projection",
			RiskClass:  "W1",
			ConfigPath: server.ConfigPath,
			Reason:     "MCP descriptors define available tools and can change the policy projection Boundary relies on. This graph flags why descriptor locking matters; it does not verify descriptor hashes.",
			Mitigation: "Use descriptor locking once installed, and require approval or fail closed when descriptors change.",
		})
		if hasUnknownCapability(server.Capabilities) {
			paths = append(paths, RiskPath{
				ID:         riskPathID(server, "review_required", "unknown"),
				Category:   "review_required",
				Server:     server.Name,
				Client:     string(server.Client),
				ConfigPath: server.ConfigPath,
				Tool:       "unknown",
				Source:     "unclassified_mcp_server",
				Sink:       "operator_review",
				RiskClass:  "unknown",
				Reason:     "Boundary could not classify this server or tool from the config descriptor.",
				Mitigation: "Review the server manually before routing privileged tools through it.",
			})
		}
		if hasExternalGitHubRead(server.Capabilities) && hasPrivateRepoWrite(server.Capabilities) {
			paths = append(paths, RiskPath{
				ID:         riskPathID(server, "untrusted_input_to_private_repo_mutation", "github"),
				Category:   "untrusted_input_to_private_repo_mutation",
				Server:     server.Name,
				Client:     string(server.Client),
				ConfigPath: server.ConfigPath,
				Tool:       "github_read_to_write",
				Source:     "external_collaborator",
				Sink:       "private_repo",
				RiskClass:  "W2",
				Reason:     "A GitHub MCP server can bring untrusted issue, PR, or repository content into context and later expose private-repo mutation tools.",
				Mitigation: "Track taint from GitHub reads and deny or require approval before private repository writes after taint.",
			})
		}
		for _, capability := range server.Capabilities {
			paths = append(paths, riskPathsForCapability(server, capability)...)
		}
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].ID < paths[j].ID })
	return paths
}

func riskPathsForCapability(server Server, capability Capability) []RiskPath {
	base := RiskPath{
		Server:     server.Name,
		Client:     string(server.Client),
		ConfigPath: server.ConfigPath,
		Tool:       capability.Name,
		RiskClass:  capability.Class,
	}
	var paths []RiskPath

	if capability.SourceClass == "external_collaborator" {
		path := base
		path.ID = riskPathID(server, "untrusted_input_to_private_data", capability.Name)
		path.Category = "untrusted_input_to_private_data"
		path.Source = firstNonEmpty(capability.SourceClass, "external_input")
		path.Sink = "agent_context_before_private_tools"
		path.Reason = "Untrusted repository or collaborator content can enter agent context before private data or mutation tools are called."
		path.Mitigation = "Track taint from read tools and deny or require approval for later private-data or repo-write actions."
		paths = append(paths, path)
	}

	if capability.SinkClass == "external_publication" {
		path := base
		path.ID = riskPathID(server, "external_sink", capability.Name)
		path.Category = "external_sink"
		path.Source = firstNonEmpty(capability.SourceClass, "agent_output")
		path.Sink = capability.SinkClass
		path.Reason = "External publication tools can send agent-controlled content outside the workspace."
		path.Mitigation = "Require approval or deny publication tools until the source and destination are trusted."
		paths = append(paths, path)
	}

	if capability.Class == "W2" {
		path := base
		path.ID = riskPathID(server, "privileged_mutation", capability.Name)
		path.Category = "privileged_mutation"
		path.Source = firstNonEmpty(capability.SourceClass, "agent_output")
		path.Sink = firstNonEmpty(capability.SinkClass, "privileged_system")
		path.Reason = "W2 tools perform critical mutations that should be denied or explicitly approved."
		path.Mitigation = "Deny W2 tools by default and route exceptions through explicit operator approval."
		paths = append(paths, path)
	}

	if capability.MutationClass == "database_query" {
		path := base
		path.ID = riskPathID(server, "destructive_db_action", capability.Name)
		path.Category = "destructive_db_action"
		path.Source = "agent_sql"
		path.Sink = firstNonEmpty(capability.SinkClass, "database")
		path.Reason = "Database query tools can issue destructive statements when statement class is not constrained."
		path.Mitigation = "Use AST classification and deny destructive SQL classes before upstream execution."
		paths = append(paths, path)
	}

	if capability.Category == "filesystem" && capability.SourceClass == "local_file" {
		path := base
		path.ID = riskPathID(server, "filesystem_exfil", capability.Name)
		path.Category = "filesystem_exfil"
		path.Source = capability.SourceClass
		path.Sink = "agent_context_or_external_sink"
		path.Reason = "Local filesystem reads can place private local data into agent context for later exfiltration."
		path.Mitigation = "Restrict filesystem roots and require review for reads from credential or private-data paths."
		paths = append(paths, path)
	}

	if capability.SinkClass == "private_repo" {
		path := base
		path.ID = riskPathID(server, "repo_write_path", capability.Name)
		path.Category = "repo_write_path"
		path.Source = firstNonEmpty(capability.SourceClass, "agent_output")
		path.Sink = capability.SinkClass
		path.Reason = "Repository write tools can mutate private source state."
		path.Mitigation = "Deny write-after-taint paths and require explicit approval for private repository mutations."
		paths = append(paths, path)
	}

	return paths
}

func RenderRiskGraph(graph RiskGraph, format string) ([]byte, error) {
	switch strings.ToLower(format) {
	case "", "json":
		return json.MarshalIndent(graph, "", "  ")
	case "mermaid":
		return []byte(renderMermaid(graph)), nil
	default:
		return nil, fmt.Errorf("unsupported graph format %q", format)
	}
}

func renderMermaid(graph RiskGraph) string {
	var b strings.Builder
	fmt.Fprintln(&b, "flowchart LR")
	for _, node := range graph.Nodes {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", mermaidID(node.ID), mermaidLabel(node.Label))
	}
	for _, edge := range graph.Edges {
		label := edge.Label
		if edge.RiskClass != "" {
			label = label + " " + edge.RiskClass
		}
		fmt.Fprintf(&b, "  %s -->|%s| %s\n", mermaidID(edge.From), mermaidLabel(label), mermaidID(edge.To))
	}
	return b.String()
}

func summarizeRiskPaths(paths []RiskPath) RiskSummary {
	summary := RiskSummary{Paths: len(paths)}
	for _, path := range paths {
		if path.RiskClass == "W1" || path.RiskClass == "W2" {
			summary.HighRiskPaths++
		}
		switch path.Category {
		case "descriptor_change":
			summary.DescriptorChangePaths++
		case "repo_write_path":
			summary.RepoWritePaths++
		case "external_sink":
			summary.ExternalSinkPaths++
		}
	}
	return summary
}

func riskPathID(server Server, category, tool string) string {
	return graphNodeID("path", string(server.Client), server.Name, category, tool)
}

func graphNodeID(parts ...string) string {
	joined := strings.ToLower(strings.Join(parts, "-"))
	var b strings.Builder
	lastDash := false
	for _, r := range joined {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func hasUnknownCapability(capabilities []Capability) bool {
	for _, capability := range capabilities {
		if capability.Class == "unknown" || capability.Category == "unknown" {
			return true
		}
	}
	return false
}

func hasExternalGitHubRead(capabilities []Capability) bool {
	for _, capability := range capabilities {
		if capability.Category == "github" && capability.SourceClass == "external_collaborator" {
			return true
		}
	}
	return false
}

func hasPrivateRepoWrite(capabilities []Capability) bool {
	for _, capability := range capabilities {
		if capability.Category == "github" && capability.SinkClass == "private_repo" {
			return true
		}
	}
	return false
}

func mermaidID(id string) string {
	return "n_" + strings.ReplaceAll(id, "-", "_")
}

func mermaidLabel(label string) string {
	label = strings.ReplaceAll(label, "\\", "\\\\")
	label = strings.ReplaceAll(label, "\"", "\\\"")
	label = strings.ReplaceAll(label, "\n", " ")
	return label
}
