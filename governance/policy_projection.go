package governance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/fulcrum-governance/boundary/policyeval"
)

// ProjectPolicyEvalRequest turns the canonical GovernanceRequest into the
// richer PolicyEval context used by schema v1 policies and decision records.
func ProjectPolicyEvalRequest(req *GovernanceRequest, trustScore *float64, trustState TrustState, policyVersion string) *policyeval.EvaluationRequest {
	if req == nil {
		return &policyeval.EvaluationRequest{}
	}
	attributes := map[string]string{
		"request.id": req.RequestID,
		"transport":  string(req.Transport),
		"tool.name":  req.ToolName,
		"action":     req.Action,
		"agent.id":   req.AgentID,
		"tenant.id":  req.TenantID,
	}
	for key, value := range req.Arguments {
		attributes["argument."+key] = fmt.Sprint(value)
	}
	riskClass := projectedRiskClass(req)
	if riskClass != "" {
		attributes["risk.class"] = riskClass
	}
	resourceIDs := projectedResourceIDs(req)
	projected := &policyeval.EvaluationRequest{
		TenantID:      req.TenantID,
		UserID:        req.AgentID,
		WorkflowID:    req.ParentEnvID,
		EnvelopeID:    req.EnvelopeID,
		Phase:         policyeval.ExecutionPhase_EXECUTION_PHASE_PRE_TOOL_CALL,
		ToolNames:     []string{req.ToolName},
		AgentID:       req.AgentID,
		Transport:     string(req.Transport),
		ToolName:      req.ToolName,
		Action:        req.Action,
		Arguments:     cloneAnyMap(req.Arguments),
		TrustScore:    trustScore,
		TrustState:    trustState.String(),
		RiskClass:     riskClass,
		ResourceIDs:   resourceIDs,
		PolicyVersion: policyVersion,
		Attributes:    attributes,
		Provenance: policyeval.RequestProvenance{
			Source:  "boundary-governance-pipeline",
			Adapter: string(req.Transport),
			TraceID: req.TraceID,
		},
	}
	projected.RequestHash = hashPolicyEvalRequest(projected)
	projected.Attributes["request.hash"] = projected.RequestHash
	return projected
}

func projectedRiskClass(req *GovernanceRequest) string {
	if req == nil || req.Arguments == nil {
		return ""
	}
	for _, key := range []string{"sql_class", "risk_class", "pipe_risk"} {
		if value, ok := req.Arguments[key]; ok {
			return fmt.Sprint(value)
		}
	}
	if len(req.PipeChain) > 0 {
		return HighestRisk(req.PipeChain)
	}
	return ""
}

func projectedResourceIDs(req *GovernanceRequest) []string {
	if req == nil || req.Arguments == nil {
		return nil
	}
	keys := []string{"resource_id", "table", "table_name", "database", "schema"}
	seen := map[string]bool{}
	var out []string
	for _, key := range keys {
		value, ok := req.Arguments[key]
		if !ok {
			continue
		}
		appendResource(&out, seen, value)
	}
	if value, ok := req.Arguments["resource_ids"]; ok {
		appendResource(&out, seen, value)
	}
	sort.Strings(out)
	return out
}

func appendResource(out *[]string, seen map[string]bool, value any) {
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			appendResource(out, seen, item)
		}
	case []any:
		for _, item := range typed {
			appendResource(out, seen, item)
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(typed))
		if text == "" || seen[text] {
			return
		}
		seen[text] = true
		*out = append(*out, text)
	}
}

func hashPolicyEvalRequest(req *policyeval.EvaluationRequest) string {
	payload := struct {
		TenantID    string         `json:"tenant_id"`
		AgentID     string         `json:"agent_id"`
		Transport   string         `json:"transport"`
		ToolName    string         `json:"tool_name"`
		Action      string         `json:"action"`
		Arguments   map[string]any `json:"arguments"`
		TrustState  string         `json:"trust_state"`
		RiskClass   string         `json:"risk_class"`
		ResourceIDs []string       `json:"resource_ids"`
	}{
		TenantID:    req.TenantID,
		AgentID:     req.AgentID,
		Transport:   req.Transport,
		ToolName:    req.ToolName,
		Action:      req.Action,
		Arguments:   req.Arguments,
		TrustState:  req.TrustState,
		RiskClass:   req.RiskClass,
		ResourceIDs: append([]string{}, req.ResourceIDs...),
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
