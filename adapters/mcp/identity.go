package mcp

import (
	"net/http"

	"github.com/fulcrum-governance/boundary/governance"
)

// Identity holds transport identity extracted from the MCP HTTP envelope.
type Identity struct {
	AgentID  string
	TenantID string
	TraceID  string
}

// ExtractIdentity reads the governance identity headers accepted by Boundary.
func ExtractIdentity(r *http.Request, defaultTenantID string) Identity {
	if r == nil {
		return Identity{TenantID: defaultTenantID}
	}
	agentID := firstHeader(r, governance.HeaderGovernanceAgentID, governance.HeaderLegacyAgentID, "X-MCP-Agent-ID")
	tenantID := firstHeader(r, governance.HeaderGovernanceTenantID, governance.HeaderLegacyTenantID, "X-MCP-Tenant-ID")
	if tenantID == "" {
		tenantID = defaultTenantID
	}
	return Identity{
		AgentID:  agentID,
		TenantID: tenantID,
		TraceID:  firstHeader(r, "X-Governance-Trace-ID", "X-Trace-ID", "Traceparent"),
	}
}

func firstHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if value := r.Header.Get(name); value != "" {
			return value
		}
	}
	return ""
}
