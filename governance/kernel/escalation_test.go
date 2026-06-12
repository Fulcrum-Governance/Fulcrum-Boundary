package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// fakePublisher records publishes in-process and optionally fails or invokes a
// synchronous hook after recording, letting tests deliver resolutions while
// the escalating goroutine is still inside Publish (i.e. after register,
// before the await select).
type fakePublisher struct {
	mu        sync.Mutex
	messages  map[string][][]byte
	err       error
	onPublish func(subject string, payload []byte)
}

func (p *fakePublisher) Publish(_ context.Context, subject string, payload []byte) error {
	p.mu.Lock()
	if p.err != nil {
		err := p.err
		p.mu.Unlock()
		return err
	}
	if p.messages == nil {
		p.messages = map[string][][]byte{}
	}
	p.messages[subject] = append(p.messages[subject], payload)
	hook := p.onPublish
	p.mu.Unlock()
	if hook != nil {
		hook(subject, payload)
	}
	return nil
}

func (p *fakePublisher) count(subject string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.messages[subject])
}

func (p *fakePublisher) last(subject string) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	msgs := p.messages[subject]
	if len(msgs) == 0 {
		return nil
	}
	return msgs[len(msgs)-1]
}

// fakeSubscriber captures the subscription in-process and lets tests deliver
// raw payloads to the captured handler, standing in for the resolver
// publishing on the resolved subject.
type fakeSubscriber struct {
	mu             sync.Mutex
	subject        string
	handler        func([]byte)
	subscribeCalls int
	unsubscribes   int
	err            error
}

func (s *fakeSubscriber) Subscribe(_ context.Context, subject string, handler func(payload []byte)) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribeCalls++
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

func (s *fakeSubscriber) deliver(payload []byte) {
	s.mu.Lock()
	handler := s.handler
	s.mu.Unlock()
	if handler != nil {
		handler(payload)
	}
}

func (s *fakeSubscriber) stats() (subscribes, unsubscribes int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.subscribeCalls, s.unsubscribes
}

func newTestAwaitHandler(t *testing.T, pub *fakePublisher, sub *fakeSubscriber, timeout time.Duration) *AwaitingEscalationHandler {
	t.Helper()
	h, err := NewAwaitingEscalationHandler(pub, sub, "", "", timeout)
	require.NoError(t, err)
	return h
}

func waiterCount(h *AwaitingEscalationHandler) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.waiters)
}

func resolution(id, status string) []byte {
	return []byte(fmt.Sprintf(`{"request_id":%q,"status":%q}`, id, status))
}

func TestAwaitingEscalationApprovedAllowsWithHumanApprovedMode(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) {
		sub.deliver([]byte(`{"request_id":"req-1","status":"approved","reviewer_id":"reviewer-7","review_note":"looks safe","resolved_at":"2026-06-12T00:00:00Z"}`))
	}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-1", EnvelopeID: "env-1"}, "semantic review required")
	require.NoError(t, err)
	require.Equal(t, "allow", decision.Action)
	require.Equal(t, governance.DecisionModeHumanApproved, decision.DecisionMode)
	require.Equal(t, "escalation approved by reviewer-7: looks safe", decision.Reason)
	require.Equal(t, "req-1", decision.RequestID)
	require.Equal(t, "env-1", decision.EnvelopeID)
	// The awaiting handler asserts no trust: a reviewer attests a verdict,
	// not trust. Trust fields are pipeline-owned.
	require.Zero(t, decision.TrustScore)
	require.Empty(t, decision.TrustState)

	// Frozen escalate envelope on the default subject; resolution
	// subscription on the default resolved subject.
	require.Equal(t, 1, pub.count("fulcrum.foundry.escalate"))
	var envelope struct {
		Request governance.GovernanceRequest `json:"request"`
		Reason  string                       `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(pub.last("fulcrum.foundry.escalate"), &envelope))
	require.Equal(t, "req-1", envelope.Request.RequestID)
	require.Equal(t, "semantic review required", envelope.Reason)
	require.Equal(t, "fulcrum.foundry.escalate.resolved", sub.subject)

	require.Zero(t, waiterCount(h), "waiter must be deregistered after resolution")
}

func TestAwaitingEscalationDeniedDeniesWithHumanApprovedMode(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) {
		sub.deliver([]byte(`{"request_id":"req-2","status":"denied","reviewer_id":"reviewer-7"}`))
	}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-2"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	require.Equal(t, governance.DecisionModeHumanApproved, decision.DecisionMode)
	require.Equal(t, "escalation denied by reviewer-7", decision.Reason)
	require.Zero(t, decision.TrustScore)
	require.Empty(t, decision.TrustState)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationExpiredDeniesDeterministic(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) {
		sub.deliver(resolution("req-3", "expired"))
	}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-3"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	// A record expiry is a mechanical outcome, not a human verdict.
	require.Equal(t, governance.DecisionModeDeterministic, decision.DecisionMode)
	require.Equal(t, "escalation expired", decision.Reason)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationTimeoutDeniesNamingWindow(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	h := newTestAwaitHandler(t, pub, sub, 25*time.Millisecond)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-4"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	require.Equal(t, governance.DecisionModeDeterministic, decision.DecisionMode)
	require.Equal(t, "escalation timed out after 25ms awaiting resolution", decision.Reason)
	require.Zero(t, waiterCount(h), "timed-out waiter must be deregistered")
}

func TestAwaitingEscalationIgnoresMalformedUnknownAndPendingMessages(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) {
		sub.deliver([]byte(`not json at all`))                                                // malformed → ignored
		sub.deliver([]byte(`{"status":"approved"}`))                                          // empty request_id → ignored
		sub.deliver(resolution("req-other", "approved"))                                      // unknown request_id → ignored
		sub.deliver(resolution("req-5", "pending"))                                           // pending is not a resolution → ignored
		sub.deliver(resolution("req-5", "rubber-stamped"))                                    // unknown status → ignored
		sub.deliver([]byte(`{"request_id":"req-5","status":"approved","unknown_extra":"x"}`)) // resolves
	}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-5"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "allow", decision.Action)
	require.Equal(t, "escalation approved", decision.Reason)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationDuplicateResolutionFirstWins(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) {
		// Both arrive before the await select runs (delivered synchronously
		// inside Publish): the first fills the buffered slot and wins, the
		// duplicate is dropped without blocking dispatch.
		sub.deliver(resolution("req-6", "approved"))
		sub.deliver(resolution("req-6", "denied"))
	}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-6"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "allow", decision.Action)
	require.Equal(t, governance.DecisionModeHumanApproved, decision.DecisionMode)
}

func TestAwaitingEscalationLateResolutionAfterTimeoutIsIgnored(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	h := newTestAwaitHandler(t, pub, sub, 10*time.Millisecond)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-7"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	require.Zero(t, waiterCount(h))

	// Late resolution after the window: no waiter, no panic, no leak.
	require.NotPanics(t, func() { sub.deliver(resolution("req-7", "approved")) })
	require.Zero(t, waiterCount(h))

	// The same RequestID can escalate again afresh and resolve normally.
	pub.mu.Lock()
	pub.onPublish = func(string, []byte) { sub.deliver(resolution("req-7", "approved")) }
	pub.mu.Unlock()
	decision, err = h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-7"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "allow", decision.Action)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationPublishErrorIsFault(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{err: fmt.Errorf("nats down")}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-8"}, "needs review")
	require.Error(t, err)
	require.Nil(t, decision)
	require.Contains(t, err.Error(), "publish failed")
	require.Contains(t, err.Error(), "nats down")
	require.Zero(t, waiterCount(h), "failed publish must deregister its waiter")
}

func TestAwaitingEscalationSubscribeErrorIsStickyFault(t *testing.T) {
	sub := &fakeSubscriber{err: fmt.Errorf("no transport")}
	pub := &fakePublisher{}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	for i := 0; i < 2; i++ {
		decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-9"}, "needs review")
		require.Error(t, err)
		require.Nil(t, decision)
		require.Contains(t, err.Error(), "subscribe failed")
		require.Contains(t, err.Error(), "no transport")
	}
	require.Zero(t, pub.count("fulcrum.foundry.escalate"), "no envelope may be published without a subscription")
	subscribes, _ := sub.stats()
	require.Equal(t, 1, subscribes, "subscribe failure is sticky; no retry loop on the hot path")
}

func TestAwaitingEscalationDuplicateInFlightRequestIDIsFault(t *testing.T) {
	sub := &fakeSubscriber{}
	published := make(chan struct{})
	var once sync.Once
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) { once.Do(func() { close(published) }) }
	h := newTestAwaitHandler(t, pub, sub, 5*time.Second)

	type outcome struct {
		decision *governance.GovernanceDecision
		err      error
	}
	first := make(chan outcome, 1)
	go func() {
		d, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-dup"}, "needs review")
		first <- outcome{decision: d, err: err}
	}()

	<-published // first escalation has registered and published; it is now awaiting

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-dup"}, "needs review")
	require.Error(t, err)
	require.Nil(t, decision)
	require.Contains(t, err.Error(), `escalation already in flight for request "req-dup"`)

	sub.deliver(resolution("req-dup", "approved"))
	res := <-first
	require.NoError(t, res.err)
	require.Equal(t, "allow", res.decision.Action)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationContextCancellationDeniesFault(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) { cancel() }
	h := newTestAwaitHandler(t, pub, sub, 5*time.Second)

	decision, err := h.Escalate(ctx, governance.GovernanceRequest{RequestID: "req-ctx"}, "needs review")
	require.NoError(t, err)
	require.Equal(t, "deny", decision.Action)
	require.Equal(t, governance.DecisionModeDeterministic, decision.DecisionMode)
	require.Equal(t, "escalation fault (fail-closed): context canceled", decision.Reason)
	require.Zero(t, waiterCount(h))
}

func TestAwaitingEscalationConcurrentDistinctEscalationsCorrelateIndependently(t *testing.T) {
	const n = 8
	expected := make(map[string]string, n) // request_id -> status to deliver
	for i := 0; i < n; i++ {
		status := "approved"
		if i%2 == 1 {
			status = "denied"
		}
		expected[fmt.Sprintf("req-c-%d", i)] = status
	}

	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(_ string, payload []byte) {
		var envelope struct {
			Request governance.GovernanceRequest `json:"request"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return
		}
		sub.deliver(resolution(envelope.Request.RequestID, expected[envelope.Request.RequestID]))
	}
	h := newTestAwaitHandler(t, pub, sub, 5*time.Second)

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for id, status := range expected {
		wg.Add(1)
		go func(id, status string) {
			defer wg.Done()
			decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: id}, "needs review")
			if err != nil {
				errs <- fmt.Errorf("escalate %s: %w", id, err)
				return
			}
			wantAction := "allow"
			if status == "denied" {
				wantAction = "deny"
			}
			if decision.Action != wantAction {
				errs <- fmt.Errorf("escalate %s: action %q, want %q", id, decision.Action, wantAction)
				return
			}
			if decision.RequestID != id {
				errs <- fmt.Errorf("escalate %s: decision correlated to %q", id, decision.RequestID)
			}
		}(id, status)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Zero(t, waiterCount(h))
	subscribes, _ := sub.stats()
	require.Equal(t, 1, subscribes, "concurrent first escalations must subscribe exactly once")
}

func TestAwaitingEscalationCloseReleasesSubscriptionAndRefusesFurtherEscalations(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	pub.onPublish = func(string, []byte) { sub.deliver(resolution("req-close", "approved")) }
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	_, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-close"}, "needs review")
	require.NoError(t, err)

	require.NoError(t, h.Close())
	_, unsubscribes := sub.stats()
	require.Equal(t, 1, unsubscribes, "Close must release the resolution subscription")

	// Idempotent: a second Close does not unsubscribe again.
	require.NoError(t, h.Close())
	_, unsubscribes = sub.stats()
	require.Equal(t, 1, unsubscribes)

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-after-close"}, "needs review")
	require.Error(t, err)
	require.Nil(t, decision)
	require.Contains(t, err.Error(), "closed")
}

func TestAwaitingEscalationCloseBeforeFirstEscalateOpensNoSubscription(t *testing.T) {
	sub := &fakeSubscriber{}
	pub := &fakePublisher{}
	h := newTestAwaitHandler(t, pub, sub, time.Second)

	require.NoError(t, h.Close())

	decision, err := h.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-x"}, "needs review")
	require.Error(t, err)
	require.Nil(t, decision)
	require.Contains(t, err.Error(), "closed")

	subscribes, unsubscribes := sub.stats()
	require.Zero(t, subscribes, "a closed handler must not open a subscription")
	require.Zero(t, unsubscribes)
	require.Zero(t, pub.count("fulcrum.foundry.escalate"))
}

func TestNewAwaitingEscalationHandlerValidatesConfig(t *testing.T) {
	pub := &fakePublisher{}
	sub := &fakeSubscriber{}

	_, err := NewAwaitingEscalationHandler(nil, sub, "", "", time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "publisher")

	_, err = NewAwaitingEscalationHandler(pub, nil, "", "", time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscriber")

	_, err = NewAwaitingEscalationHandler(pub, sub, "", "", 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timeout")

	_, err = NewAwaitingEscalationHandler(pub, sub, "", "", -time.Second)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timeout")

	h, err := NewAwaitingEscalationHandler(pub, sub, "custom.escalate", "custom.resolved", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "custom.escalate", h.EscalateSubject)
	require.Equal(t, "custom.resolved", h.ResolvedSubject)
	require.Equal(t, time.Minute, h.Timeout)

	// A struct-literal handler with no seams faults instead of panicking.
	bare := &AwaitingEscalationHandler{Timeout: time.Second}
	decision, err := bare.Escalate(context.Background(), governance.GovernanceRequest{RequestID: "req-bare"}, "needs review")
	require.Error(t, err)
	require.Nil(t, decision)
}
