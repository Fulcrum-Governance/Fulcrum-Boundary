package madriver

import (
	"context"
	"io"
	"sync"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// stubUpstream is a fully offline fake of the upstream Managed Agents API. It
// emits a scripted event stream covering the conformance scenarios (allowed
// tool call, denied tool call, MCP tool call, thread creation, forced pipeline
// error, usage and idle events) and records the confirmations Boundary
// resolves so the transcript can prove them.
type stubUpstream struct {
	mu              sync.Mutex
	cfg             Config
	queue           []managedagents.Event
	toolUseTypes    map[string]string // tool-use event id -> event type
	confirmations   []managedagents.ToolConfirmation
	sentEvents      []map[string]any
	pendingGoverned int // scripted governable events still awaiting a confirmation
	streamOpened    bool
}

func newStubUpstream(cfg Config) *stubUpstream {
	return &stubUpstream{cfg: cfg}
}

func (s *stubUpstream) CreateSession(_ context.Context, params CreateSessionParams) (*SessionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	const sessionID = "sesn_stub_0000000001"
	s.queue = s.script(sessionID)
	return &SessionInfo{ID: sessionID, AgentID: params.AgentID}, nil
}

func (s *stubUpstream) OpenEventStream(_ context.Context, _ string) (managedagents.EventSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.streamOpened = true
	return &stubSource{up: s}, nil
}

func (s *stubUpstream) SendEvents(_ context.Context, _ string, events []map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentEvents = append(s.sentEvents, events...)
	return nil
}

func (s *stubUpstream) Forwarder() managedagents.ConfirmationForwarder { return s }

// SendConfirmation records the resolved confirmation and enqueues the
// tool_result the upstream would emit once the pause is resolved. When the
// last scripted governable event is answered it appends the closing usage and
// idle events so the stream terminates like a real session.
func (s *stubUpstream) SendConfirmation(_ context.Context, sessionID string, confirmation managedagents.ToolConfirmation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmations = append(s.confirmations, confirmation)

	resultType := managedagents.EventToolResult
	if s.toolUseTypes[confirmation.ToolUseID] == managedagents.EventAgentMCPToolUse {
		resultType = "agent.mcp_tool_result"
	}
	content := "stub tool result"
	if confirmation.Result == managedagents.ConfirmationDeny {
		content = "denied by Boundary: " + confirmation.DenyMessage
	}
	s.queue = append(s.queue, managedagents.Event{
		ID:              "stubres-" + confirmation.ToolUseID,
		Type:            resultType,
		SessionID:       sessionID,
		SessionThreadID: confirmation.SessionThreadID,
		Data: map[string]any{
			"tool_use_id": confirmation.ToolUseID,
			"result":      confirmation.Result,
			"content":     content,
		},
	})

	s.pendingGoverned--
	if s.pendingGoverned == 0 {
		s.queue = append(s.queue, s.closingEvents(sessionID)...)
	}
	return nil
}

func (s *stubUpstream) RequestIDs() []string { return nil }

func (s *stubUpstream) Close() error { return nil }

// Confirmations returns a copy of the confirmations recorded so far.
func (s *stubUpstream) Confirmations() []managedagents.ToolConfirmation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]managedagents.ToolConfirmation, len(s.confirmations))
	copy(out, s.confirmations)
	return out
}

// script is the deterministic event sequence the fake upstream emits. Governable
// events carry Data["evaluated_permission"]="ask" exactly as the upstream
// permission-policies doc describes for always_ask calls.
func (s *stubUpstream) script(sessionID string) []managedagents.Event {
	const agentID = "agt_stub_conformance"
	const threadID = "thread-stub-worker-1"
	s.pendingGoverned = 5
	events := []managedagents.Event{
		{
			ID:        "stub-evt-running",
			Type:      "session.status_running",
			SessionID: sessionID,
		},
		{
			ID:        "stub-evt-allow",
			Type:      managedagents.EventAgentToolUse,
			AgentID:   agentID,
			SessionID: sessionID,
			ToolName:  "read_issue",
			Input:     map[string]any{"name": "read_issue", "estimated_cost_usd": 0.05},
			Usage:     &managedagents.Usage{CostUSD: 0.05},
			Data:      map[string]any{"evaluated_permission": "ask"},
		},
		{
			ID:              "stub-evt-thread",
			Type:            managedagents.EventThreadCreated,
			AgentID:         agentID,
			SessionID:       sessionID,
			SessionThreadID: threadID,
			ParentThreadID:  sessionID,
			Data:            map[string]any{"budget_limit": 1.00},
		},
		{
			ID:              "stub-evt-allow2",
			Type:            managedagents.EventAgentToolUse,
			AgentID:         agentID,
			SessionID:       sessionID,
			SessionThreadID: threadID,
			ToolName:        "search_codebase",
			Input:           map[string]any{"name": "search_codebase", "estimated_cost_usd": 0.02},
			Usage:           &managedagents.Usage{CostUSD: 0.02},
			Data:            map[string]any{"evaluated_permission": "ask"},
		},
		{
			ID:              "stub-evt-mcp",
			Type:            managedagents.EventAgentMCPToolUse,
			AgentID:         agentID,
			SessionID:       sessionID,
			SessionThreadID: threadID,
			ToolName:        "mcp_docs_lookup",
			Input:           map[string]any{"name": "mcp_docs_lookup", "estimated_cost_usd": 0.03},
			Usage:           &managedagents.Usage{CostUSD: 0.03},
			Data:            map[string]any{"evaluated_permission": "ask"},
		},
		{
			ID:              "stub-evt-deny",
			Type:            managedagents.EventAgentToolUse,
			AgentID:         agentID,
			SessionID:       sessionID,
			SessionThreadID: threadID,
			ToolName:        s.cfg.DenyTool,
			Input:           map[string]any{"name": s.cfg.DenyTool},
			Data:            map[string]any{"evaluated_permission": "ask"},
		},
		{
			ID:              "stub-evt-failclosed",
			Type:            managedagents.EventAgentToolUse,
			AgentID:         agentID,
			SessionID:       sessionID,
			SessionThreadID: threadID,
			ToolName:        s.cfg.ErrorTool,
			Input:           map[string]any{"name": s.cfg.ErrorTool},
			Data:            map[string]any{"evaluated_permission": "ask"},
		},
	}
	s.toolUseTypes = map[string]string{}
	for _, e := range events {
		if e.Type == managedagents.EventAgentToolUse || e.Type == managedagents.EventAgentMCPToolUse {
			s.toolUseTypes[e.ID] = e.Type
		}
	}
	return events
}

// closingEvents finishes the scripted session once every governable event has
// been answered: one span with priced usage, the cumulative session.usage
// snapshot (list_cost in whole cents per the budgets doc), and idle stops.
func (s *stubUpstream) closingEvents(sessionID string) []managedagents.Event {
	return []managedagents.Event{
		{
			ID:        "stub-evt-span",
			Type:      "span.model_request_end",
			SessionID: sessionID,
			Data: map[string]any{
				"model_usage": map[string]any{
					"model":         "claude-sonnet-4-5-20250929",
					"input_tokens":  4200,
					"output_tokens": 310,
				},
			},
		},
		{
			ID:        "stub-evt-usage",
			Type:      "session.usage",
			SessionID: sessionID,
			Data: map[string]any{
				"usage": map[string]any{
					"list_cost":     "18",
					"currency":      "USD",
					"input_tokens":  4200,
					"output_tokens": 310,
				},
			},
		},
		{
			ID:              "stub-evt-thread-idle",
			Type:            "session.thread_status_idle",
			SessionID:       sessionID,
			SessionThreadID: "thread-stub-worker-1",
			StopReason:      &managedagents.StopReason{Type: "end_turn"},
		},
		{
			ID:         "stub-evt-idle",
			Type:       managedagents.EventStatusIdle,
			SessionID:  sessionID,
			StopReason: &managedagents.StopReason{Type: "end_turn"},
		},
	}
}

// stubSource pops scripted events; confirmations may append more.
type stubSource struct {
	up *stubUpstream
}

func (s *stubSource) Next(context.Context) (managedagents.Event, error) {
	s.up.mu.Lock()
	defer s.up.mu.Unlock()
	if len(s.up.queue) == 0 {
		return managedagents.Event{}, io.EOF
	}
	event := s.up.queue[0]
	s.up.queue = s.up.queue[1:]
	return event, nil
}
