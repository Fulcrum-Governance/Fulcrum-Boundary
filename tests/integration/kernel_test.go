package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
	"github.com/fulcrum-governance/fulcrum-boundary/governance/kernel"
	"github.com/fulcrum-governance/fulcrum-boundary/internal/boundarycli"
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

// TestKernelBundleAwaitEscalationResolvesThroughBundle drives the awaiting
// escalation mode end-to-end through NewBundle: a Subscriber in the config
// selects the awaiting handler, the escalate envelope is published on the
// frozen subject, and a resolution delivered on the resolved subject maps
// approved->allow(human_approved), denied->deny(human_approved), and
// expired->deny(deterministic). The resolution is delivered synchronously
// inside Publish — before the await select runs — so this also proves the
// waiter is registered before the envelope is published. Malformed, unknown,
// and pending messages delivered first are ignored without unblocking the
// waiter.
func TestKernelBundleAwaitEscalationResolvesThroughBundle(t *testing.T) {
	cases := []struct {
		name       string
		resolution string
		wantAction string
		wantMode   governance.DecisionMode
		wantReason string
	}{
		{
			name:       "approved resolves to allow with human_approved",
			resolution: `{"request_id":"req-await","status":"approved","reviewer_id":"reviewer-9","review_note":"scoped change"}`,
			wantAction: "allow",
			wantMode:   governance.DecisionModeHumanApproved,
			wantReason: "escalation approved by reviewer-9: scoped change",
		},
		{
			name:       "denied resolves to deny with human_approved",
			resolution: `{"request_id":"req-await","status":"denied","reviewer_id":"reviewer-9"}`,
			wantAction: "deny",
			wantMode:   governance.DecisionModeHumanApproved,
			wantReason: "escalation denied by reviewer-9",
		},
		{
			name:       "expired resolves to deny with deterministic",
			resolution: `{"request_id":"req-await","status":"expired"}`,
			wantAction: "deny",
			wantMode:   governance.DecisionModeDeterministic,
			wantReason: "escalation expired",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sub := &fakeAwaitSubscriber{}
			pub := &hookedPublisher{}
			pub.afterPublish = func(string, []byte) {
				sub.deliver([]byte(`malformed{`))                                     // ignored
				sub.deliver([]byte(`{"request_id":"req-other","status":"approved"}`)) // unknown id, ignored
				sub.deliver([]byte(`{"request_id":"req-await","status":"pending"}`))  // non-resolution, ignored
				sub.deliver([]byte(tc.resolution))
			}
			bundle := newAwaitBundle(t, pub, sub, 2*time.Second)

			decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-await", EnvelopeID: "env-await"}, "needs foundry review")
			require.NoError(t, err)
			require.Equal(t, tc.wantAction, decision.Action)
			require.Equal(t, tc.wantMode, decision.DecisionMode)
			require.Equal(t, tc.wantReason, decision.Reason)
			require.Equal(t, "req-await", decision.RequestID)
			require.Equal(t, "env-await", decision.EnvelopeID)
			// The awaiting handler asserts no trust; trust fields are
			// pipeline-owned.
			require.Zero(t, decision.TrustScore)
			require.Empty(t, decision.TrustState)

			require.ElementsMatch(t, []string{"fulcrum.foundry.escalate"}, pub.subjects())
			require.Equal(t, "fulcrum.foundry.escalate.resolved", sub.subscribedSubject())
		})
	}
}

// TestKernelBundleAwaitEscalationTimesOutDenying covers the bounded-window
// default through the bundle: with no resolution the escalation denies naming
// the window, a late resolution after timeout is ignored without panic or
// leak, and the same request id can escalate again afresh.
func TestKernelBundleAwaitEscalationTimesOutDenying(t *testing.T) {
	sub := &fakeAwaitSubscriber{}
	var respond atomic.Bool
	pub := &hookedPublisher{}
	pub.afterPublish = func(string, []byte) {
		if respond.Load() {
			sub.deliver([]byte(`{"request_id":"req-window","status":"approved"}`))
		}
	}
	bundle := newAwaitBundle(t, pub, sub, 50*time.Millisecond)

	decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-window"}, "needs foundry review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	require.Equal(t, governance.DecisionModeDeterministic, decision.DecisionMode)
	require.Equal(t, "escalation timed out after 50ms awaiting resolution", decision.Reason)

	// Late resolution after the window is ignored: no panic, no stale waiter.
	require.NotPanics(t, func() {
		sub.deliver([]byte(`{"request_id":"req-window","status":"approved"}`))
	})

	respond.Store(true)
	decision, err = bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-window"}, "needs foundry review")
	require.NoError(t, err)
	require.Equal(t, "allow", decision.Action)
}

// TestKernelBundleAwaitEscalationFaultsFailClosed covers the fault paths
// through the bundle: publish failure and subscribe failure surface as errors
// (faults, never approvals), and a duplicate in-flight request id is rejected
// while the original await still resolves.
func TestKernelBundleAwaitEscalationFaultsFailClosed(t *testing.T) {
	t.Run("publish error is a fault", func(t *testing.T) {
		bundle := newAwaitBundle(t, failingPublisher{err: errPublishDown}, &fakeAwaitSubscriber{}, time.Second)
		decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-pub-fault"}, "needs foundry review")
		require.Error(t, err)
		require.Nil(t, decision)
		require.Contains(t, err.Error(), "publish failed")
	})

	t.Run("subscribe error is a fault", func(t *testing.T) {
		sub := &fakeAwaitSubscriber{err: errSubscribeDown}
		pub := &hookedPublisher{}
		bundle := newAwaitBundle(t, pub, sub, time.Second)
		decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-sub-fault"}, "needs foundry review")
		require.Error(t, err)
		require.Nil(t, decision)
		require.Contains(t, err.Error(), "subscribe failed")
		require.Empty(t, pub.subjects(), "no envelope may be published without a subscription")
	})

	t.Run("duplicate in-flight request id is a fault", func(t *testing.T) {
		sub := &fakeAwaitSubscriber{}
		published := make(chan struct{})
		var once sync.Once
		pub := &hookedPublisher{}
		pub.afterPublish = func(string, []byte) { once.Do(func() { close(published) }) }
		bundle := newAwaitBundle(t, pub, sub, 5*time.Second)

		type outcome struct {
			decision *governance.GovernanceDecision
			err      error
		}
		first := make(chan outcome, 1)
		go func() {
			d, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-dup"}, "needs foundry review")
			first <- outcome{decision: d, err: err}
		}()
		<-published

		decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-dup"}, "needs foundry review")
		require.Error(t, err)
		require.Nil(t, decision)
		require.Contains(t, err.Error(), "already in flight")

		sub.deliver([]byte(`{"request_id":"req-dup","status":"approved"}`))
		res := <-first
		require.NoError(t, res.err)
		require.Equal(t, "allow", res.decision.Action)
	})
}

// TestKernelBundleCloseReleasesAwaitSubscription proves the additive
// Bundle.Close releases the awaiting handler's resolution subscription and
// that post-close escalations fault rather than hang, while a routing-mode
// bundle (no Subscriber) treats Close as a clean no-op.
func TestKernelBundleCloseReleasesAwaitSubscription(t *testing.T) {
	sub := &fakeAwaitSubscriber{}
	pub := &hookedPublisher{}
	pub.afterPublish = func(string, []byte) {
		sub.deliver([]byte(`{"request_id":"req-close","status":"approved"}`))
	}
	bundle := newAwaitBundle(t, pub, sub, time.Second)

	_, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-close"}, "needs foundry review")
	require.NoError(t, err)

	require.NoError(t, bundle.Close())
	require.Equal(t, 1, sub.unsubscribeCount(), "Bundle.Close must release the resolution subscription")
	require.NoError(t, bundle.Close(), "Close is idempotent")
	require.Equal(t, 1, sub.unsubscribeCount())

	decision, err := bundle.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-post-close"}, "needs foundry review")
	require.Error(t, err)
	require.Nil(t, decision)

	// Routing mode (no Subscriber): Close is a no-op and the routing handler
	// still answers afterwards, byte-identical to before the seam existed.
	routing := newRoutingBundle(t, &recordingPublisher{})
	require.NoError(t, routing.Close())
	resolved, err := routing.Escalation.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-routing"}, "needs foundry")
	require.NoError(t, err)
	require.Equal(t, "escalate", resolved.Action)
}

// TestKernelBundleAwaitTimeoutValidation pins the BundleConfig contract: a
// negative await timeout is rejected only in awaiting mode (Subscriber set);
// routing-mode bundles ignore the field entirely.
func TestKernelBundleAwaitTimeoutValidation(t *testing.T) {
	cfg := kernel.BundleConfig{
		PolicyStore:          newMemoryRedis(),
		TrustStore:           newMemoryRedis(),
		BudgetEndpoint:       "http://budget.example.invalid",
		Publisher:            &recordingPublisher{},
		Subscriber:           &fakeAwaitSubscriber{},
		EscalateAwaitTimeout: -time.Second,
	}
	_, err := kernel.NewBundle(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "must not be negative")

	cfg.Subscriber = nil
	_, err = kernel.NewBundle(cfg)
	require.NoError(t, err, "routing mode ignores the await timeout")
}

var (
	errPublishDown   = fmt.Errorf("publish transport down")
	errSubscribeDown = fmt.Errorf("subscribe transport down")
)

// newAwaitBundle builds a kernel bundle in awaiting escalation mode with
// in-process fakes and default subjects. Policy, trust, and budget seams are
// present (NewBundle hard-fails without them) but unexercised.
func newAwaitBundle(t *testing.T, pub kernel.Publisher, sub kernel.Subscriber, timeout time.Duration) *kernel.Bundle {
	t.Helper()
	bundle, err := kernel.NewBundle(kernel.BundleConfig{
		PolicyStore:          newMemoryRedis(),
		TrustStore:           newMemoryRedis(),
		BudgetEndpoint:       "http://budget.example.invalid",
		Publisher:            pub,
		Subscriber:           sub,
		EscalateAwaitTimeout: timeout,
	})
	require.NoError(t, err)
	return bundle
}

// newRoutingBundle builds a kernel bundle with no Subscriber, selecting the
// existing routing-mode escalation handler.
func newRoutingBundle(t *testing.T, pub kernel.Publisher) *kernel.Bundle {
	t.Helper()
	bundle, err := kernel.NewBundle(kernel.BundleConfig{
		PolicyStore:    newMemoryRedis(),
		TrustStore:     newMemoryRedis(),
		BudgetEndpoint: "http://budget.example.invalid",
		Publisher:      pub,
	})
	require.NoError(t, err)
	return bundle
}

// hookedPublisher records like recordingPublisher and additionally invokes
// afterPublish synchronously once the message is recorded, letting tests
// deliver resolutions while the escalating goroutine is still inside Publish.
// afterPublish must be set before any concurrent use and not mutated after.
type hookedPublisher struct {
	recordingPublisher
	afterPublish func(subject string, payload []byte)
}

func (p *hookedPublisher) Publish(ctx context.Context, subject string, payload []byte) error {
	if err := p.recordingPublisher.Publish(ctx, subject, payload); err != nil {
		return err
	}
	if p.afterPublish != nil {
		p.afterPublish(subject, payload)
	}
	return nil
}

// failingPublisher always fails, standing in for a down transport.
type failingPublisher struct{ err error }

func (p failingPublisher) Publish(context.Context, string, []byte) error { return p.err }

// fakeAwaitSubscriber captures the awaiting handler's subscription in-process
// and lets tests deliver raw payloads to it, standing in for the resolver
// publishing on the resolved subject.
type fakeAwaitSubscriber struct {
	mu           sync.Mutex
	subject      string
	handler      func([]byte)
	unsubscribes int
	err          error
}

func (s *fakeAwaitSubscriber) Subscribe(_ context.Context, subject string, handler func(payload []byte)) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.subject = subject
	s.handler = handler
	return func() {
		s.mu.Lock()
		s.unsubscribes++
		s.mu.Unlock()
	}, nil
}

func (s *fakeAwaitSubscriber) deliver(payload []byte) {
	s.mu.Lock()
	handler := s.handler
	s.mu.Unlock()
	if handler != nil {
		handler(payload)
	}
}

func (s *fakeAwaitSubscriber) subscribedSubject() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subject
}

func (s *fakeAwaitSubscriber) unsubscribeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unsubscribes
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
