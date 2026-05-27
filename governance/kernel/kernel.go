package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fulcrum-governance/boundary/governance"
	"github.com/fulcrum-governance/boundary/governance/standalone"
)

type Publisher interface {
	Publish(ctx context.Context, subject string, payload []byte) error
}

type Bundle struct {
	Policies   governance.PolicyProvider
	Trust      governance.TrustBackend
	Cost       governance.CostPredictor
	Budget     governance.BudgetEnforcer
	Escalation governance.EscalationHandler
	Audit      governance.AuditPublisher
	Envelope   governance.EnvelopeManager
	Proofs     governance.ProofCorrespondence
}

type BundleConfig struct {
	PolicyStore     governance.RedisKV
	PolicyKeyPrefix string
	TrustStore      governance.RedisKV
	TrustConfig     governance.KernelTrustConfig
	BudgetEndpoint  string
	Publisher       Publisher
	EscalateSubject string
	AuditSubject    string
	EnvelopeSubject string
}

func NewBundle(cfg BundleConfig) (*Bundle, error) {
	if cfg.PolicyStore == nil {
		return nil, fmt.Errorf("kernel policy store is required")
	}
	if cfg.TrustStore == nil {
		return nil, fmt.Errorf("kernel trust store is required")
	}
	if cfg.BudgetEndpoint == "" {
		return nil, fmt.Errorf("kernel budget endpoint is required")
	}
	return &Bundle{
		Policies: RedisPolicyProvider{
			Store:     cfg.PolicyStore,
			KeyPrefix: firstNonEmpty(cfg.PolicyKeyPrefix, "fulcrum:policies:"),
		},
		Trust: governance.NewRedisTrustBackend(cfg.TrustStore, cfg.TrustConfig),
		Cost:  StaticCostPredictor{Estimate: governance.CostEstimate{Amount: 1, Unit: "operation", Confidence: 0.5, Source: "kernel-placeholder"}},
		Budget: HTTPBudgetEnforcer{
			Endpoint: cfg.BudgetEndpoint,
			Client:   http.DefaultClient,
		},
		Escalation: NATSEscalationHandler{
			Publisher: cfg.Publisher,
			Subject:   firstNonEmpty(cfg.EscalateSubject, "fulcrum.foundry.escalate"),
		},
		Audit: NATSAuditPublisher{
			Publisher: cfg.Publisher,
			Subject:   firstNonEmpty(cfg.AuditSubject, "fulcrum.audit.boundary"),
		},
		Envelope: NATSEnvelopeManager{
			Publisher: cfg.Publisher,
			Subject:   firstNonEmpty(cfg.EnvelopeSubject, "fulcrum.envelope"),
		},
		Proofs: standalone.StaticProofCorrespondence(),
	}, nil
}

type RedisPolicyProvider struct {
	Store     governance.RedisKV
	KeyPrefix string
	Key       string
}

func (p RedisPolicyProvider) LoadPolicies(ctx context.Context) ([]governance.StaticPolicyRule, error) {
	if p.Store == nil {
		return nil, fmt.Errorf("redis policy store is required")
	}
	key := p.Key
	if key == "" {
		key = firstNonEmpty(p.KeyPrefix, "fulcrum:policies:") + "active"
	}
	body, err := p.Store.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("policy bundle %q is empty", key)
	}
	doc, err := governance.ParseStaticPolicyDocument(key, []byte(body))
	if err != nil {
		return nil, err
	}
	return doc.Rules, nil
}

func (p RedisPolicyProvider) WatchPolicyUpdates(ctx context.Context) (<-chan governance.PolicyUpdate, error) {
	ch := make(chan governance.PolicyUpdate)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

type StaticCostPredictor struct {
	Estimate governance.CostEstimate
}

func (p StaticCostPredictor) PredictCost(context.Context, governance.GovernanceRequest) (governance.CostEstimate, error) {
	return p.Estimate, nil
}

type HTTPBudgetEnforcer struct {
	Endpoint string
	Client   *http.Client
}

func (e HTTPBudgetEnforcer) CheckBudget(ctx context.Context, tenantID, agentID string, cost governance.CostEstimate) (bool, error) {
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := e.post(ctx, "check", tenantID, agentID, cost.Amount, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

func (e HTTPBudgetEnforcer) RecordSpend(ctx context.Context, tenantID, agentID string, amount int64) error {
	return e.post(ctx, "record", tenantID, agentID, amount, nil)
}

func (e HTTPBudgetEnforcer) post(ctx context.Context, operation, tenantID, agentID string, amount int64, out any) error {
	if e.Endpoint == "" {
		return fmt.Errorf("budget endpoint is required")
	}
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	body, _ := json.Marshal(map[string]any{
		"operation": operation,
		"tenant_id": tenantID,
		"agent_id":  agentID,
		"amount":    amount,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("budget %s failed: %s: %s", operation, resp.Status, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

type NATSEscalationHandler struct {
	Publisher Publisher
	Subject   string
}

func (h NATSEscalationHandler) Escalate(ctx context.Context, req governance.GovernanceRequest, reason string) (*governance.GovernanceDecision, error) {
	if h.Publisher != nil {
		payload, _ := json.Marshal(map[string]any{"request": req, "reason": reason})
		if err := h.Publisher.Publish(ctx, firstNonEmpty(h.Subject, "fulcrum.foundry.escalate"), payload); err != nil {
			return nil, err
		}
	}
	return &governance.GovernanceDecision{
		RequestID:    req.RequestID,
		Action:       "escalate",
		Reason:       reason,
		TrustScore:   1,
		TrustState:   governance.TrustStateTrusted.String(),
		EnvelopeID:   req.EnvelopeID,
		DecisionMode: governance.DecisionModeClassified,
	}, nil
}

type NATSAuditPublisher struct {
	Publisher Publisher
	Subject   string
}

func (p NATSAuditPublisher) Publish(ctx context.Context, event governance.AuditEvent) {
	if p.Publisher == nil {
		return
	}
	payload, _ := json.Marshal(event)
	_ = p.Publisher.Publish(ctx, firstNonEmpty(p.Subject, "fulcrum.audit.boundary"), payload)
}

type NATSEnvelopeManager struct {
	Publisher Publisher
	Subject   string
}

func (m NATSEnvelopeManager) CreateEnvelope(ctx context.Context, req governance.GovernanceRequest) (governance.EnvelopeID, error) {
	id := req.EnvelopeID
	if id == "" {
		id = uuid.New().String()
	}
	return governance.EnvelopeID(id), m.publish(ctx, id, governance.EnvelopeStateCreated)
}

func (m NATSEnvelopeManager) TransitionEnvelope(ctx context.Context, id governance.EnvelopeID, state governance.EnvelopeState) error {
	return m.publish(ctx, string(id), state)
}

func (m NATSEnvelopeManager) publish(ctx context.Context, id string, state governance.EnvelopeState) error {
	if m.Publisher == nil {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{
		"envelope_id": id,
		"state":       state,
		"timestamp":   time.Now().UTC(),
	})
	return m.Publisher.Publish(ctx, firstNonEmpty(m.Subject, "fulcrum.envelope"), payload)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
