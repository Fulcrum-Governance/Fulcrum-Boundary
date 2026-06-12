package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/governance"
)

// Subscriber delivers messages published on a subject to a handler. It is a
// bare injection seam mirroring Publisher: Boundary ships no NATS
// implementation in-repo; deployment provides the transport.
type Subscriber interface {
	// Subscribe registers handler for subject and returns an unsubscribe func
	// that stops delivery and releases the subscription. handler is invoked
	// once per received message with the raw payload bytes; it must not block
	// (the awaiting handler hands off to a buffered channel and returns). ctx
	// covers establishment of the subscription, not its lifetime. A non-nil
	// error means the subscription was not established.
	Subscribe(ctx context.Context, subject string, handler func(payload []byte)) (unsubscribe func(), err error)
}

// resolvedMessage is the decoded resolution event published on the resolved
// subject by the upstream Foundry layer (fulcrum-io). request_id correlates
// back to the originating GovernanceRequest; status carries the verdict.
// reviewer_id and review_note, when present, are folded into the decision
// reason string. resolved_at is advisory audit provenance only and is never a
// control input. The message carries no trust fields and none may be read
// from it.
type resolvedMessage struct {
	RequestID  string `json:"request_id"`
	Status     string `json:"status"` // approved | denied | expired (| pending, ignored)
	ReviewerID string `json:"reviewer_id,omitempty"`
	ReviewNote string `json:"review_note,omitempty"`
	ResolvedAt string `json:"resolved_at,omitempty"`
}

// statusKnown reports whether Status is one of the three resolution verdicts.
// "pending" (a progress event, not a resolution) and any unknown value return
// false and the message is ignored.
func (m resolvedMessage) statusKnown() bool {
	switch m.Status {
	case "approved", "denied", "expired":
		return true
	}
	return false
}

// AwaitingEscalationHandler is the awaiting-mode escalation handler: it
// publishes the same frozen escalate envelope as NATSEscalationHandler
// ({"request": ..., "reason": ...} on EscalateSubject), then blocks for a
// bounded window awaiting a resolution message on ResolvedSubject (default
// fulcrum.foundry.escalate.resolved), correlated by the request's RequestID.
//
// An approved resolution maps to allow and a denied resolution to deny, both
// DecisionModeHumanApproved (a human review resolved them). An expired
// resolution and a local timeout map to deny with DecisionModeDeterministic:
// they are mechanical outcomes, and labeling them human_approved would assert
// a review that never happened. Returned decisions never assert trust — a
// reviewer attests a verdict, not trust — so TrustScore stays 0 and
// TrustState stays empty; trust fields are pipeline-owned.
//
// Errors returned by Escalate are faults, never approvals (publish failure,
// subscription failure, a duplicate in-flight RequestID, or escalating after
// Close); callers deny fail-closed on them per the EscalationHandler
// contract. Construct with NewAwaitingEscalationHandler, which validates the
// publisher, subscriber, and timeout.
type AwaitingEscalationHandler struct {
	// Publisher publishes the escalate envelope on EscalateSubject.
	Publisher Publisher
	// Subscriber delivers resolution messages from ResolvedSubject.
	Subscriber Subscriber
	// EscalateSubject is the escalate-envelope subject; empty means the
	// canonical default fulcrum.foundry.escalate.
	EscalateSubject string
	// ResolvedSubject is the resolution subject; empty means the canonical
	// default fulcrum.foundry.escalate.resolved.
	ResolvedSubject string
	// Timeout bounds the synchronous hold per escalation. When it elapses
	// with no resolution the escalation denies, naming the window.
	Timeout time.Duration

	subscribeOnce sync.Once
	subErr        error  // captured from the lazy Subscribe; replayed to every caller
	unsubscribe   func() // set by the lazy Subscribe; called by Close

	mu      sync.Mutex
	waiters map[string]chan resolvedMessage // RequestID -> delivery channel
	closed  bool
}

// NewAwaitingEscalationHandler builds the synchronous-await escalation
// handler. Empty subjects fall back to the canonical defaults at use time;
// NewBundle supplies them already resolved. A nil pub or sub, or a
// non-positive timeout, is a configuration fault the constructor rejects: an
// awaiting handler that cannot publish, cannot subscribe, or cannot hold a
// window can only ever fault.
func NewAwaitingEscalationHandler(pub Publisher, sub Subscriber, escalateSubject, resolvedSubject string, timeout time.Duration) (*AwaitingEscalationHandler, error) {
	if pub == nil {
		return nil, fmt.Errorf("awaiting escalation handler requires a publisher")
	}
	if sub == nil {
		return nil, fmt.Errorf("awaiting escalation handler requires a subscriber")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("awaiting escalation handler requires a positive timeout")
	}
	return &AwaitingEscalationHandler{
		Publisher:       pub,
		Subscriber:      sub,
		EscalateSubject: escalateSubject,
		ResolvedSubject: resolvedSubject,
		Timeout:         timeout,
		waiters:         make(map[string]chan resolvedMessage),
	}, nil
}

// Escalate publishes the escalate envelope for req and blocks until a
// resolution arrives on the resolved subject, the configured Timeout elapses
// (deny naming the window), or ctx ends (deny fault). The resolution
// subscription is established lazily on first use; a subscribe failure is
// sticky and is replayed to every subsequent call as a fault until the
// process restarts. The waiter is registered before the envelope is
// published, so a resolution arriving immediately after publish is never
// missed, and it is always deregistered before return.
func (h *AwaitingEscalationHandler) Escalate(ctx context.Context, req governance.GovernanceRequest, reason string) (*governance.GovernanceDecision, error) {
	if h.Publisher == nil || h.Subscriber == nil {
		return nil, fmt.Errorf("awaiting escalation handler requires both a publisher and a subscriber")
	}
	if err := h.ensureSubscribed(ctx); err != nil {
		return nil, err
	}

	ch, err := h.register(req.RequestID)
	if err != nil {
		return nil, err
	}
	defer h.deregister(req.RequestID)

	payload, _ := json.Marshal(map[string]any{"request": req, "reason": reason})
	if err := h.Publisher.Publish(ctx, h.escalateSubjectOrDefault(), payload); err != nil {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

	timer := time.NewTimer(h.Timeout)
	defer timer.Stop()
	select {
	case msg := <-ch:
		return h.decide(req, msg), nil
	case <-timer.C:
		return h.timeoutDecision(req), nil
	case <-ctx.Done():
		return h.ctxDecision(req, ctx.Err()), nil
	}
}

// Close stops the resolution subscription and refuses further escalations. It
// is idempotent and safe to call before any escalation has subscribed.
// In-flight Escalate calls are not force-unblocked; they resolve via their
// own timeout. After Close no further dispatch fires and new escalations
// return a fault (deny fail-closed upstream). The error is always nil today;
// the signature satisfies io.Closer so Bundle.Close can release the seam
// uniformly.
func (h *AwaitingEscalationHandler) Close() error {
	h.mu.Lock()
	h.closed = true
	unsub := h.unsubscribe
	h.unsubscribe = nil
	h.mu.Unlock()
	if unsub != nil {
		unsub()
	}
	return nil
}

// ensureSubscribed lazily establishes the resolution subscription on first
// use, exactly once even under concurrent first escalations. A failure is
// sticky: no resubscribe loop runs on the evaluation hot path; operators fix
// the transport and the next process gets a fresh subscription.
func (h *AwaitingEscalationHandler) ensureSubscribed(ctx context.Context) error {
	h.subscribeOnce.Do(func() {
		if h.isClosed() {
			h.setSubErr(fmt.Errorf("escalation handler is closed"))
			return
		}
		unsub, err := h.Subscriber.Subscribe(ctx, h.resolvedSubjectOrDefault(), h.dispatch)
		if err != nil {
			h.setSubErr(fmt.Errorf("subscribe failed: %w", err))
			return
		}
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			// Close raced the lazy subscribe: release the subscription
			// immediately and refuse the escalation.
			unsub()
			h.setSubErr(fmt.Errorf("escalation handler is closed"))
			return
		}
		h.unsubscribe = unsub
		h.mu.Unlock()
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.subErr
}

func (h *AwaitingEscalationHandler) isClosed() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closed
}

func (h *AwaitingEscalationHandler) setSubErr(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.subErr = err
}

// register installs the waiter channel for id before the escalate envelope is
// published, closing the missed-fast-resolution race. A duplicate in-flight
// id is a fault — it means two concurrent governances of the same RequestID —
// as is registering after Close.
func (h *AwaitingEscalationHandler) register(id string) (chan resolvedMessage, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, fmt.Errorf("escalation handler is closed")
	}
	if h.waiters == nil {
		h.waiters = make(map[string]chan resolvedMessage)
	}
	if _, exists := h.waiters[id]; exists {
		return nil, fmt.Errorf("escalation already in flight for request %q", id)
	}
	// Buffered so dispatch never blocks, even when the waiter has already
	// timed out and will never read.
	ch := make(chan resolvedMessage, 1)
	h.waiters[id] = ch
	return ch, nil
}

// deregister removes the waiter for id. Idempotent. It never closes the
// channel: a late dispatch sends harmlessly into the buffered slot of a
// now-unreferenced channel, which is simply garbage collected.
func (h *AwaitingEscalationHandler) deregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.waiters, id)
}

// dispatch is the Subscriber callback. It decodes a resolution message and
// hands it to the registered waiter without blocking. Malformed payloads,
// messages with an empty or unknown request_id, and unknown status values
// (including the non-resolution "pending") are ignored so the waiter keeps
// awaiting until its own timeout. A late message after timeout finds no
// waiter and is dropped without leak or panic.
func (h *AwaitingEscalationHandler) dispatch(payload []byte) {
	var msg resolvedMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	if msg.RequestID == "" || !msg.statusKnown() {
		return
	}
	h.mu.Lock()
	ch, ok := h.waiters[msg.RequestID]
	h.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- msg:
	default:
		// A resolution was already delivered to this waiter; the first
		// delivered resolution wins and the duplicate is dropped.
	}
}

// decide maps a resolution message to the decision the pipeline adopts.
func (h *AwaitingEscalationHandler) decide(req governance.GovernanceRequest, msg resolvedMessage) *governance.GovernanceDecision {
	switch msg.Status {
	case "approved":
		return h.decision(req, "allow", resolvedReason("approved", msg), governance.DecisionModeHumanApproved)
	case "denied":
		return h.decision(req, "deny", resolvedReason("denied", msg), governance.DecisionModeHumanApproved)
	default:
		// dispatch filtered to the three known verdicts, so this is
		// "expired": a mechanical record-expiry on the resolver side, not a
		// human verdict — deterministic, not human_approved.
		return h.decision(req, "deny", resolvedReason("expired", msg), governance.DecisionModeDeterministic)
	}
}

// timeoutDecision is the bounded-window default: no resolution arrived before
// Timeout elapsed, so the action denies fail-closed with a reason naming the
// window. DecisionModeDeterministic — a mechanical local default, not a
// relayed human verdict.
func (h *AwaitingEscalationHandler) timeoutDecision(req governance.GovernanceRequest) *governance.GovernanceDecision {
	reason := fmt.Sprintf("escalation timed out after %s awaiting resolution", h.Timeout)
	return h.decision(req, "deny", reason, governance.DecisionModeDeterministic)
}

// ctxDecision denies fail-closed when the caller's context ends before any
// resolution arrives (transport cancellation or deadline). The reason carries
// the standard escalation fault prefix.
func (h *AwaitingEscalationHandler) ctxDecision(req governance.GovernanceRequest, err error) *governance.GovernanceDecision {
	reason := "escalation fault (fail-closed): " + err.Error()
	return h.decision(req, "deny", reason, governance.DecisionModeDeterministic)
}

// decision assembles the returned GovernanceDecision. Trust is deliberately
// not asserted: a reviewer attests a verdict, not trust, so TrustScore stays
// 0 and TrustState stays empty — trust fields are pipeline-owned (stage 1 and
// the deferred trust update).
func (h *AwaitingEscalationHandler) decision(req governance.GovernanceRequest, action, reason string, mode governance.DecisionMode) *governance.GovernanceDecision {
	return &governance.GovernanceDecision{
		RequestID:    req.RequestID,
		Action:       action,
		Reason:       reason,
		EnvelopeID:   req.EnvelopeID,
		DecisionMode: mode,
	}
}

// resolvedReason renders the deterministic reason string for a resolution:
// "escalation <verb>", plus " by <reviewer_id>" and ": <review_note>" when
// the optional fields are present.
func resolvedReason(verb string, m resolvedMessage) string {
	r := "escalation " + verb
	if m.ReviewerID != "" {
		r += " by " + m.ReviewerID
	}
	if m.ReviewNote != "" {
		r += ": " + m.ReviewNote
	}
	return r
}

// escalateSubjectOrDefault returns the configured escalate subject or the
// canonical default.
func (h *AwaitingEscalationHandler) escalateSubjectOrDefault() string {
	return firstNonEmpty(h.EscalateSubject, "fulcrum.foundry.escalate")
}

// resolvedSubjectOrDefault returns the configured resolved subject or the
// canonical default.
func (h *AwaitingEscalationHandler) resolvedSubjectOrDefault() string {
	return firstNonEmpty(h.ResolvedSubject, "fulcrum.foundry.escalate.resolved")
}
