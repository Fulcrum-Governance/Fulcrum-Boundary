// Package webhook provides a TransportAdapter and HTTP handler for
// webhook-style tool invocations: an upstream agent POSTs a JSON payload
// describing the tool call, the handler runs governance, and either
// forwards the request to a downstream service or returns the decision.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// WebhookPayload is the expected JSON body of an incoming webhook tool call.
type WebhookPayload struct {
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
	AgentID   string         `json:"agent_id"`
	TenantID  string         `json:"tenant_id"`
	TraceID   string         `json:"trace_id,omitempty"`
}

type Mode string

const (
	ModeExecution     Mode = "execution"
	ModeInformational Mode = "informational"
)

// HandlerConfig selects whether a webhook endpoint is an execution gate or an
// informational audit sink.
type HandlerConfig struct {
	Mode       Mode
	ForwardURL string
	Client     *http.Client
}

// Result is the JSON envelope returned by mode-aware webhook handlers.
type Result struct {
	Mode     Mode                           `json:"mode"`
	CanDeny  bool                           `json:"can_deny"`
	Control  string                         `json:"control"`
	Decision *governance.GovernanceDecision `json:"decision,omitempty"`
}

// Adapter implements governance.TransportAdapter for webhook payloads.
type Adapter struct {
	// DefaultTenantID is applied when the payload does not specify one.
	DefaultTenantID string
}

// NewAdapter returns a webhook adapter with an optional default tenant.
func NewAdapter(defaultTenantID string) *Adapter {
	return &Adapter{DefaultTenantID: defaultTenantID}
}

// Type returns TransportWebhook.
func (a *Adapter) Type() governance.TransportType { return governance.TransportWebhook }

// ParseRequest accepts *http.Request, *WebhookPayload, WebhookPayload, or
// a JSON byte slice. For *http.Request, it reads (and replaces) the body.
func (a *Adapter) ParseRequest(_ context.Context, raw any) (*governance.GovernanceRequest, error) {
	var payload *WebhookPayload
	switch v := raw.(type) {
	case *WebhookPayload:
		payload = v
	case WebhookPayload:
		payload = &v
	case json.RawMessage:
		payload = &WebhookPayload{}
		if err := json.Unmarshal(v, payload); err != nil {
			return nil, governance.NewParseError(governance.TransportWebhook, "unmarshal payload", err)
		}
	case []byte:
		payload = &WebhookPayload{}
		if err := json.Unmarshal(v, payload); err != nil {
			return nil, governance.NewParseError(governance.TransportWebhook, "unmarshal payload", err)
		}
	case *http.Request:
		body, err := io.ReadAll(v.Body)
		if err != nil {
			return nil, governance.NewParseError(governance.TransportWebhook, "read request body", err)
		}
		_ = v.Body.Close()
		// Restore body for downstream consumers.
		v.Body = io.NopCloser(bytes.NewReader(body))
		payload = &WebhookPayload{}
		if err := json.Unmarshal(body, payload); err != nil {
			return nil, governance.NewParseError(governance.TransportWebhook, "unmarshal request body", err)
		}
	default:
		return nil, governance.NewParseError(governance.TransportWebhook, fmt.Sprintf("unsupported raw type %T", raw), nil)
	}

	if payload.Tool == "" {
		return nil, governance.NewParseError(governance.TransportWebhook, "tool field is required", nil)
	}
	tenantID := payload.TenantID
	if tenantID == "" {
		tenantID = a.DefaultTenantID
	}

	return &governance.GovernanceRequest{
		RequestID: uuid.New().String(),
		Transport: governance.TransportWebhook,
		AgentID:   payload.AgentID,
		TenantID:  tenantID,
		ToolName:  payload.Tool,
		Action:    "webhook/invoke",
		Arguments: payload.Arguments,
		TraceID:   payload.TraceID,
	}, nil
}

// ForwardGoverned is a no-op; the Handler() function performs forwarding
// when a forwardURL is configured.
func (a *Adapter) ForwardGoverned(_ context.Context, _ *governance.GovernanceRequest, _ *governance.GovernanceDecision) (*governance.ToolResponse, error) {
	return nil, nil
}

// InspectResponse returns a benign result.
func (a *Adapter) InspectResponse(_ context.Context, _ *governance.ToolResponse) (*governance.ResponseInspection, error) {
	return &governance.ResponseInspection{Safe: true}, nil
}

// EmitGovernanceMetadata is a no-op; the Handler() function writes
// governance headers directly onto the http.ResponseWriter.
func (a *Adapter) EmitGovernanceMetadata(_ context.Context, _ *governance.ToolResponse, _ *governance.GovernanceDecision) error {
	return nil
}

// Handler returns an execution-mode http.HandlerFunc that runs the governance
// pipeline on incoming webhook payloads. It is retained for compatibility;
// new endpoints should prefer HandlerWithConfig so the mode is explicit.
//
// Behavior:
//   - Parse error → 400 Bad Request with JSON error body.
//   - Pipeline error → fail closed; the payload is not forwarded.
//   - Decision is not allowed → 403 Forbidden with the decision JSON.
//   - Decision is allowed and forwardURL == "" → 200 OK with the decision JSON.
//   - Decision is allowed and forwardURL != "" → POST the original payload
//     to forwardURL and stream the downstream response back to the caller.
//
// Governance headers (X-Governance-Action, X-Governance-Reason,
// X-Governance-Envelope-ID) are added to every response.
func Handler(pipeline *governance.Pipeline, forwardURL string) http.HandlerFunc {
	return HandlerWithConfig(pipeline, HandlerConfig{
		Mode:       ModeExecution,
		ForwardURL: forwardURL,
	})
}

// HandlerWithConfig returns an http.HandlerFunc for either informational or
// execution webhook mode.
//
// Informational mode records the governance verdict for an action that already
// happened. It never forwards, never returns 403, and must not be described as
// pre-execution denial.
//
// Execution mode treats Boundary as a pre-execution approval gate. Denied
// requests and governance failures are not forwarded.
func HandlerWithConfig(pipeline *governance.Pipeline, cfg HandlerConfig) http.HandlerFunc {
	adapter := NewAdapter("")
	cfg = normalizeConfig(cfg)
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "read body: "+err.Error())
			return
		}
		_ = r.Body.Close()

		req, err := adapter.ParseRequest(r.Context(), body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeModeHeaders(w, cfg.Mode)

		if pipeline == nil {
			if cfg.Mode == ModeInformational {
				writeJSONError(w, http.StatusInternalServerError, "governance: pipeline is required")
				return
			}
			writeJSONError(w, http.StatusServiceUnavailable, "governance: pipeline is required; execution webhook not forwarded")
			return
		}

		decision, err := pipeline.Evaluate(r.Context(), req)
		if err != nil {
			if cfg.Mode == ModeInformational {
				writeJSONError(w, http.StatusInternalServerError, "governance: "+err.Error())
				return
			}
			writeJSONError(w, http.StatusServiceUnavailable, "governance: "+err.Error()+"; execution webhook not forwarded")
			return
		}

		writeGovernanceHeaders(w, decision)
		if cfg.Mode == ModeInformational {
			writeJSON(w, http.StatusOK, Result{
				Mode:     ModeInformational,
				CanDeny:  false,
				Control:  "post_execution_audit_only",
				Decision: decision,
			})
			return
		}

		if !decision.Allowed() {
			writeJSON(w, http.StatusForbidden, decision)
			return
		}

		if cfg.ForwardURL == "" {
			writeJSON(w, http.StatusOK, decision)
			return
		}

		fwd, err := http.NewRequestWithContext(r.Context(), http.MethodPost, cfg.ForwardURL, bytes.NewReader(body))
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "build forward request: "+err.Error())
			return
		}
		fwd.Header.Set("Content-Type", "application/json")
		resp, err := cfg.Client.Do(fwd)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "forward: "+err.Error())
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func normalizeConfig(cfg HandlerConfig) HandlerConfig {
	if cfg.Mode != ModeInformational && cfg.Mode != ModeExecution {
		cfg.Mode = ModeExecution
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{}
	}
	return cfg
}

func writeModeHeaders(w http.ResponseWriter, mode Mode) {
	w.Header().Set("X-Governance-Webhook-Mode", string(mode))
	w.Header().Set("X-Governance-Can-Deny", fmt.Sprintf("%t", mode == ModeExecution))
}

func writeGovernanceHeaders(w http.ResponseWriter, d *governance.GovernanceDecision) {
	w.Header().Set("X-Governance-Action", d.Action)
	if d.Reason != "" {
		w.Header().Set("X-Governance-Reason", d.Reason)
	}
	if d.EnvelopeID != "" {
		w.Header().Set("X-Governance-Envelope-ID", d.EnvelopeID)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
