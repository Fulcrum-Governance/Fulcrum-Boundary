package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fulcrum-governance/boundary/governance"
	"github.com/fulcrum-governance/boundary/governance/kernel"
	"github.com/fulcrum-governance/boundary/internal/boundarycli"
	"github.com/stretchr/testify/require"
)

func TestKernelBundleBootsWithFulcrumBridgeContracts(t *testing.T) {
	ctx := context.Background()
	policies := newMemoryRedis()
	trust := newMemoryRedis()
	require.NoError(t, policies.Set(ctx, "fulcrum:policies:active", `
name: kernel
version: v1
rules:
  - name: block-drop
    tool: query
    action: deny
    match:
      field: arguments.sql
      contains: DROP TABLE
`, time.Minute))
	require.NoError(t, trust.Set(ctx, "agent:agent-1:circuit_state", "0", time.Minute))
	require.NoError(t, trust.Set(ctx, "agent:agent-1:trust_score", "0.99", time.Minute))

	budgetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.NotEmpty(t, payload["operation"])
		_ = json.NewEncoder(w).Encode(map[string]any{"allowed": true})
	}))
	defer budgetServer.Close()

	pub := &recordingPublisher{}
	bundle, err := kernel.NewBundle(kernel.BundleConfig{
		PolicyStore:     policies,
		PolicyKeyPrefix: "fulcrum:policies:",
		TrustStore:      trust,
		TrustConfig: governance.KernelTrustConfig{
			IPCPrefix:  "agent:",
			FailClosed: true,
		},
		BudgetEndpoint:  budgetServer.URL,
		Publisher:       pub,
		EscalateSubject: "fulcrum.foundry.escalate",
		AuditSubject:    "fulcrum.audit.boundary",
		EnvelopeSubject: "fulcrum.envelope",
	})
	require.NoError(t, err)

	rules, err := bundle.Policies.LoadPolicies(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	state, err := bundle.Trust.CheckAgentState(ctx, "agent-1")
	require.NoError(t, err)
	require.Equal(t, governance.TrustStateTrusted, state)

	allowed, err := bundle.Budget.CheckBudget(ctx, "tenant-1", "agent-1", governance.CostEstimate{Amount: 42})
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, bundle.Budget.RecordSpend(ctx, "tenant-1", "agent-1", 42))

	_, err = bundle.Escalation.Escalate(ctx, governance.GovernanceRequest{RequestID: "req-1"}, "needs foundry")
	require.NoError(t, err)
	bundle.Audit.Publish(ctx, governance.AuditEvent{RequestID: "req-1"})
	_, err = bundle.Envelope.CreateEnvelope(ctx, governance.GovernanceRequest{EnvelopeID: "env-1"})
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"fulcrum.foundry.escalate", "fulcrum.audit.boundary", "fulcrum.envelope"}, pub.subjects())
}

func TestRuntimeConfigValidationFailsUnsafeKernelConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "boundary.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
mode: kernel
kernel:
  trust:
    type: redis_ipc
    redis_url: redis://localhost:6379
    key_prefix: "agent:"
`), 0o600))

	_, err := boundarycli.LoadRuntimeConfig(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "kernel.policy_engine")
}

type memoryRedis struct {
	mu     sync.Mutex
	values map[string]string
}

func newMemoryRedis() *memoryRedis {
	return &memoryRedis{values: map[string]string{}}
}

func (m *memoryRedis) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[key], nil
}

func (m *memoryRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

func (m *memoryRedis) Del(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}

type recordingPublisher struct {
	mu       sync.Mutex
	messages map[string][][]byte
}

func (p *recordingPublisher) Publish(_ context.Context, subject string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.messages == nil {
		p.messages = map[string][][]byte{}
	}
	p.messages[subject] = append(p.messages[subject], payload)
	return nil
}

func (p *recordingPublisher) subjects() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	subjects := make([]string, 0, len(p.messages))
	for subject := range p.messages {
		subjects = append(subjects, subject)
	}
	return subjects
}
