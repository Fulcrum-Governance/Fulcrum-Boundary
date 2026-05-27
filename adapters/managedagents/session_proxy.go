package managedagents

import (
	"context"
	"errors"
	"io"
	"strings"
)

// EventSource provides the inbound Managed Agents session stream.
type EventSource interface {
	Next(context.Context) (Event, error)
}

// EventSink receives the governed session stream forwarded to the customer app.
type EventSink interface {
	Emit(context.Context, Event) error
}

// SessionProxy governs a Managed Agents session stream event by event.
type SessionProxy struct {
	Resolver *ToolResolver
	Tracker  *ThreadTracker
}

func NewSessionProxy(resolver *ToolResolver, tracker *ThreadTracker) *SessionProxy {
	return &SessionProxy{Resolver: resolver, Tracker: tracker}
}

func (p *SessionProxy) Proxy(ctx context.Context, source EventSource, sink EventSink) error {
	for {
		event, err := source.Next(ctx)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if p.Tracker != nil {
			p.Tracker.TrackEvent(event)
		}
		if isGovernableToolEvent(event.Type) {
			if p.Resolver == nil {
				return errors.New("managed agents proxy requires a tool resolver")
			}
			_, decision, err := p.Resolver.Resolve(ctx, event)
			if err != nil {
				return err
			}
			event.Governance = metadataFromDecision(decision)
		}
		if sink != nil {
			if err := sink.Emit(ctx, event); err != nil {
				return err
			}
		}
	}
}

func isGovernableToolEvent(eventType string) bool {
	return eventType == EventAgentToolUse || eventType == EventAgentMCPToolUse
}

// BypassConfig records the deployment controls needed to keep customer apps
// from sending tool confirmations directly to the upstream Managed Agents API.
type BypassConfig struct {
	BoundaryOwnsAPIKey           bool
	CustomerCanSendConfirmations bool
}

func VerifyBypassConfig(cfg BypassConfig) error {
	var gaps []string
	if !cfg.BoundaryOwnsAPIKey {
		gaps = append(gaps, "Boundary must be the only component with the upstream Managed Agents API key")
	}
	if cfg.CustomerCanSendConfirmations {
		gaps = append(gaps, "customer app must not be able to call the sessions events send endpoint directly")
	}
	if len(gaps) > 0 {
		return errors.New(strings.Join(gaps, "; "))
	}
	return nil
}

// SliceSource and SliceSink are small deterministic helpers used by examples
// and integration tests.
type SliceSource struct {
	Events []Event
	index  int
}

func (s *SliceSource) Next(context.Context) (Event, error) {
	if s.index >= len(s.Events) {
		return Event{}, io.EOF
	}
	event := s.Events[s.index]
	s.index++
	return event, nil
}

type SliceSink struct {
	Events []Event
}

func (s *SliceSink) Emit(_ context.Context, event Event) error {
	s.Events = append(s.Events, event)
	return nil
}
