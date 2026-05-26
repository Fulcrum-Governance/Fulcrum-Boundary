package governance

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// MiddlewareConfig configures GovernanceMiddleware. Zero-value fields fall
// back to documented defaults.
type MiddlewareConfig struct {
	// ToolNameHeader is read to populate GovernanceRequest.ToolName.
	// Default: "X-Tool-Name".
	ToolNameHeader string

	// AgentIDHeader is read to populate GovernanceRequest.AgentID.
	// Default: "X-Governance-Agent-ID".
	AgentIDHeader string

	// TenantIDHeader is read to populate GovernanceRequest.TenantID.
	// Default: "X-Governance-Tenant-ID".
	TenantIDHeader string

	// TransportType is the transport recorded on GovernanceRequest.
	// Default: TransportMCP.
	TransportType TransportType

	// ToolNameFromPath, if true, uses the URL path as the tool name when the
	// tool-name header is absent.
	ToolNameFromPath bool

	// RequestBuilder lets gateways parse transport-specific request bodies into
	// canonical governance requests before the downstream handler runs. If set,
	// the middleware uses it instead of the default header/path mapping.
	RequestBuilder func(*http.Request) (*GovernanceRequest, error)
}

// Response header names. These are also written on deny responses so that
// clients always see the governance verdict in the same fields.
const (
	HeaderToolName              = "X-Tool-Name"
	HeaderGovernanceAgentID     = "X-Governance-Agent-ID"
	HeaderGovernanceTenantID    = "X-Governance-Tenant-ID"
	HeaderLegacyAgentID         = "X-Agent-ID"
	HeaderLegacyTenantID        = "X-Tenant-ID"
	HeaderGovernanceAction      = "X-Governance-Action"
	HeaderGovernanceReason      = "X-Governance-Reason"
	HeaderGovernanceEnvelopeID  = "X-Governance-Envelope-ID"
	HeaderGovernanceRequestID   = "X-Governance-Request-ID"
	HeaderGovernanceDryRun      = "X-Governance-Dry-Run"
	HeaderGovernanceMatchedRule = "X-Governance-Matched-Rule"
)

// GovernanceMiddleware wraps an http.Handler with pre-execution governance.
// If Next is nil, the middleware acts as a standalone decision endpoint that
// returns the decision as a JSON body.
type GovernanceMiddleware struct {
	Pipeline *Pipeline
	Next     http.Handler
	Config   MiddlewareConfig
}

// NewMiddleware creates a GovernanceMiddleware. Zero-value config fields are
// filled with defaults; the returned middleware is safe to use as an
// http.Handler.
func NewMiddleware(pipeline *Pipeline, next http.Handler, cfg MiddlewareConfig) *GovernanceMiddleware {
	if cfg.ToolNameHeader == "" {
		cfg.ToolNameHeader = HeaderToolName
	}
	if cfg.AgentIDHeader == "" {
		cfg.AgentIDHeader = HeaderGovernanceAgentID
	}
	if cfg.TenantIDHeader == "" {
		cfg.TenantIDHeader = HeaderGovernanceTenantID
	}
	if cfg.TransportType == "" {
		cfg.TransportType = TransportMCP
	}
	return &GovernanceMiddleware{Pipeline: pipeline, Next: next, Config: cfg}
}

// ServeHTTP evaluates the request through the governance pipeline. On deny,
// it writes HTTP 403 with a JSON body. On allow/warn, it writes governance
// response headers and either forwards to Next or returns the decision as
// JSON when Next is nil.
func (m *GovernanceMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	agentID, agentHeader := readConfiguredIdentityHeader(r.Header, m.Config.AgentIDHeader, HeaderLegacyAgentID)
	tenantID, tenantHeader := readConfiguredIdentityHeader(r.Header, m.Config.TenantIDHeader, HeaderLegacyTenantID)

	gReq, err := m.buildGovernanceRequest(r, agentID, tenantID)
	if err != nil {
		http.Error(w, "governance request parse error: "+err.Error(), http.StatusBadRequest)
		return
	}

	decision, err := m.Pipeline.Evaluate(r.Context(), gReq)
	if err != nil {
		http.Error(w, "governance pipeline error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeGovernanceHeaders(w, decision)

	if !decision.Allowed() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"action":          decision.Action,
			"reason":          decision.Reason,
			"request_id":      decision.RequestID,
			"decision_mode":   string(decision.DecisionMode),
			"matched_rule":    decision.MatchedRule,
			"policy_file":     decision.PolicyFile,
			"gateway_version": decision.GatewayVersion,
		})
		return
	}

	if m.Next == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"action":      decision.Action,
			"reason":      decision.Reason,
			"request_id":  decision.RequestID,
			"envelope_id": decision.EnvelopeID,
			"dry_run":     decision.DryRun,
		})
		return
	}

	normalizeForwardedIdentityHeader(r.Header, m.Config.AgentIDHeader, agentHeader, agentID)
	normalizeForwardedIdentityHeader(r.Header, m.Config.TenantIDHeader, tenantHeader, tenantID)
	m.Next.ServeHTTP(w, r)
}

func (m *GovernanceMiddleware) buildGovernanceRequest(r *http.Request, agentID, tenantID string) (*GovernanceRequest, error) {
	if m.Config.RequestBuilder != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		req, err := m.Config.RequestBuilder(r)
		r.Body = io.NopCloser(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if req.AgentID == "" {
			req.AgentID = agentID
		}
		if req.TenantID == "" {
			req.TenantID = tenantID
		}
		if req.Transport == "" {
			req.Transport = m.Config.TransportType
		}
		return req, nil
	}

	toolName := r.Header.Get(m.Config.ToolNameHeader)
	if toolName == "" && m.Config.ToolNameFromPath {
		toolName = strings.TrimPrefix(r.URL.Path, "/")
	}

	return &GovernanceRequest{
		Transport: m.Config.TransportType,
		ToolName:  toolName,
		AgentID:   agentID,
		TenantID:  tenantID,
	}, nil
}

func readConfiguredIdentityHeader(headers http.Header, primaryHeader, legacyHeader string) (value string, source string) {
	value = headers.Get(primaryHeader)
	if value != "" {
		return value, primaryHeader
	}
	if primaryHeader != legacyHeader {
		value = headers.Get(legacyHeader)
		if value != "" {
			return value, legacyHeader
		}
	}
	return "", ""
}

func normalizeForwardedIdentityHeader(headers http.Header, primaryHeader, sourceHeader, value string) {
	if value == "" || primaryHeader == "" || sourceHeader == "" || sourceHeader == primaryHeader {
		return
	}
	headers.Set(primaryHeader, value)
}

func writeGovernanceHeaders(w http.ResponseWriter, d *GovernanceDecision) {
	w.Header().Set(HeaderGovernanceAction, d.Action)
	if d.Reason != "" {
		w.Header().Set(HeaderGovernanceReason, d.Reason)
	}
	if d.EnvelopeID != "" {
		w.Header().Set(HeaderGovernanceEnvelopeID, d.EnvelopeID)
	}
	if d.RequestID != "" {
		w.Header().Set(HeaderGovernanceRequestID, d.RequestID)
	}
	if d.DryRun {
		w.Header().Set(HeaderGovernanceDryRun, "true")
	}
	if d.MatchedRule != "" {
		w.Header().Set(HeaderGovernanceMatchedRule, d.MatchedRule)
	}
}
