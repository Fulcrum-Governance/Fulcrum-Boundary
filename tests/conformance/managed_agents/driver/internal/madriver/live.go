package madriver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// liveUpstream talks to the real Managed Agents beta API. It runs inside the
// driver process, which acts as the Boundary front for this adapter: the
// adapter ships as a library (SessionProxy over EventSource/EventSink plus a
// ConfirmationForwarder), and cmd/boundary exposes no managed-agents listener,
// so the governed path here is in-process embedding with the driver holding
// the upstream credential.
//
// Protocol references (docs.claude.com / platform.claude.com managed-agents
// docs, checked 2026-10-08):
//   - POST /v1/sessions (beta header managed-agents-2026-04-01) creates a
//     session; body {agent, environment_id, budget?, initial_events?}.
//   - GET /v1/sessions/{id}/events/stream?beta=true opens the SSE stream
//     (accept: text/event-stream); only post-open events arrive.
//   - POST /v1/sessions/{id}/events?beta=true sends {events: [...]}.
//   - Confirmations: {type:"user.tool_confirmation", tool_use_id, result:
//     "allow"|"deny", deny_message?}; upstream rejects a confirmation with 400
//     when the referenced call's evaluated_permission is not "ask".
type liveUpstream struct {
	base   string
	key    string
	client *http.Client

	mu         sync.Mutex
	requestIDs []string
	// pendingAsk holds tool-use event ids the upstream told us are awaiting a
	// confirmation (evaluated_permission == "ask", plus ids named by
	// session.status_idle stop_reason.requires_action). The forwarder refuses
	// to confirm anything outside this set because the API answers 400.
	pendingAsk map[string]bool
	// delivered holds tool-use ids whose confirmation was actually posted
	// upstream, so the transcript can distinguish "Boundary resolved" from
	// "upstream accepted".
	delivered map[string]bool
}

func newLiveUpstream(base, key string) *liveUpstream {
	return &liveUpstream{
		base:       strings.TrimRight(base, "/"),
		key:        key,
		client:     &http.Client{},
		pendingAsk: map[string]bool{},
		delivered:  map[string]bool{},
	}
}

func (l *liveUpstream) CreateSession(ctx context.Context, params CreateSessionParams) (*SessionInfo, error) {
	// The upstream session budget is a second, independent ceiling: the driver
	// converts its own USD ceiling to the API's whole-cents string form.
	cents := int64(params.BudgetUSD*100 + 0.5)
	if cents < 1 {
		cents = 1
	}
	body := map[string]any{
		"agent":          params.AgentID,
		"environment_id": params.EnvironmentID,
		"budget": map[string]any{
			"type":          "limit",
			"max_list_cost": map[string]any{"amount": strconv.FormatInt(cents, 10), "currency": "USD"},
		},
	}
	if strings.TrimSpace(params.Prompt) != "" {
		body["initial_events"] = []map[string]any{{
			"type":    "user.message",
			"content": []map[string]any{{"type": "text", "text": params.Prompt}},
		}}
	}
	resp, err := l.do(ctx, http.MethodPost, "/v1/sessions", "", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var parsed struct {
		ID    string `json:"id"`
		Agent string `json:"agent"`
	}
	if err := decodeJSON(resp.Body, &parsed); err != nil {
		return nil, fmt.Errorf("decode create-session response: %w", err)
	}
	if parsed.ID == "" {
		return nil, errors.New("create-session response carried no session id")
	}
	return &SessionInfo{ID: parsed.ID, AgentID: parsed.Agent}, nil
}

func (l *liveUpstream) OpenEventStream(ctx context.Context, sessionID string) (managedagents.EventSource, error) {
	resp, err := l.do(ctx, http.MethodGet, "/v1/sessions/"+sessionID+"/events/stream", "beta=true", nil)
	if err != nil {
		return nil, err
	}
	return &sseSource{up: l, resp: resp, scan: bufio.NewScanner(resp.Body)}, nil
}

func (l *liveUpstream) SendEvents(ctx context.Context, sessionID string, events []map[string]any) error {
	resp, err := l.do(ctx, http.MethodPost, "/v1/sessions/"+sessionID+"/events", "beta=true", map[string]any{"events": events})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// The send endpoint accepts or rejects the batch; a rejection is returned
	// as an error by do() before this point.
	return nil
}

func (l *liveUpstream) Forwarder() managedagents.ConfirmationForwarder { return l }

func (l *liveUpstream) RequestIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.requestIDs))
	copy(out, l.requestIDs)
	return out
}

func (l *liveUpstream) Close() error { return nil }

// SendConfirmation implements managedagents.ConfirmationForwarder. It only
// posts a confirmation for an event the upstream marked as awaiting one
// (evaluated_permission "ask" or listed in a requires_action stop_reason);
// sending anything else is a protocol error the upstream rejects with 400, so
// unconfirmed-but-evaluated tool calls are recorded as decisions without a
// confirmation attempt.
func (l *liveUpstream) SendConfirmation(ctx context.Context, sessionID string, confirmation managedagents.ToolConfirmation) error {
	l.mu.Lock()
	pending := l.pendingAsk[confirmation.ToolUseID]
	l.mu.Unlock()
	if !pending {
		return nil
	}
	event := map[string]any{
		"type":        managedagents.ConfirmationEventType,
		"tool_use_id": confirmation.ToolUseID,
		"result":      confirmation.Result,
	}
	if confirmation.DenyMessage != "" {
		event["deny_message"] = confirmation.DenyMessage
	}
	if err := l.SendEvents(ctx, sessionID, []map[string]any{event}); err != nil {
		return err
	}
	l.mu.Lock()
	l.delivered[confirmation.ToolUseID] = true
	l.mu.Unlock()
	return nil
}

// ConfirmationSent reports whether the confirmation for toolUseID was actually
// posted upstream (true) or skipped because the call was not awaiting one.
func (l *liveUpstream) ConfirmationSent(toolUseID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.delivered[toolUseID]
}

// do performs one authenticated API call. The key only ever travels in the
// x-api-key request header; response bodies are decoded by the caller and any
// error text returned upward is redacted before it can surface.
func (l *liveUpstream) do(ctx context.Context, method, path, query string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	url := l.base + path
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("x-api-key", l.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", BetaHeader)
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if strings.HasSuffix(path, "/events/stream") {
		req.Header.Set("accept", "text/event-stream")
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if id := resp.Header.Get("request-id"); id != "" {
		l.mu.Lock()
		l.requestIDs = append(l.requestIDs, id)
		l.mu.Unlock()
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		text, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("%s %s: upstream status %d: %s", method, path, resp.StatusCode, RedactString(string(text)))
	}
	return resp, nil
}

// sseSource adapts the session SSE stream to managedagents.EventSource.
type sseSource struct {
	up   *liveUpstream
	resp *http.Response
	scan *bufio.Scanner
}

func (s *sseSource) Next(ctx context.Context) (managedagents.Event, error) {
	for s.scan.Scan() {
		select {
		case <-ctx.Done():
			return managedagents.Event{}, ctx.Err()
		default:
		}
		line := strings.TrimSpace(s.scan.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // event:, comment, and blank lines carry no payload
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return managedagents.Event{}, io.EOF
		}
		event, err := eventFromSSE([]byte(payload))
		if err != nil {
			return managedagents.Event{}, err
		}
		s.noteAsk(event)
		return event, nil
	}
	if err := s.scan.Err(); err != nil {
		return managedagents.Event{}, err
	}
	return managedagents.Event{}, io.EOF
}

// noteAsk records tool-use events the upstream flagged as awaiting a
// confirmation, so the forwarder only confirms calls that can accept one.
func (s *sseSource) noteAsk(event managedagents.Event) {
	mark := func(id string) {
		if id == "" {
			return
		}
		s.up.mu.Lock()
		s.up.pendingAsk[id] = true
		s.up.mu.Unlock()
	}
	if perm, ok := event.Data["evaluated_permission"].(string); ok && perm == "ask" {
		mark(event.ID)
	}
	if event.StopReason != nil && event.StopReason.Type == "requires_action" {
		for _, id := range event.StopReason.EventIDs {
			mark(id)
		}
	}
}

// eventFromSSE converts one SSE data payload into the adapter's Event shape.
// Unknown upstream fields are preserved in Data so the proxy, tracker, and
// spend guard can still see them (evaluated_permission, model_usage,
// list_cost, ...).
//
// UNVERIFIED field names (checked against the published docs, not a live
// capture): session_id, session_thread_id vs thread_id, tool_name vs name,
// agent_id location, and the input block's key. Anything absent simply
// degrades to an empty field — the adapter's parser then treats the event as
// non-governable rather than guessing.
func eventFromSSE(data []byte) (managedagents.Event, error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return managedagents.Event{}, fmt.Errorf("unmarshal stream event: %w", err)
	}
	event := managedagents.Event{Data: m, Raw: append([]byte(nil), data...)}
	event.Type = stringField(m, "type")
	event.ID = stringField(m, "id")
	event.SessionID = firstNonEmpty(stringField(m, "session_id"), stringField(m, "session"))
	event.SessionThreadID = firstNonEmpty(stringField(m, "session_thread_id"), stringField(m, "thread_id"))
	event.ParentThreadID = stringField(m, "parent_thread_id")
	event.TenantID = stringField(m, "tenant_id")
	event.AgentID = stringField(m, "agent_id")
	event.ToolName = firstNonEmpty(stringField(m, "tool_name"), stringField(m, "name"), stringField(m, "tool"))
	if input, ok := m["input"].(map[string]any); ok {
		event.Input = input
	}
	if sr, ok := m["stop_reason"].(map[string]any); ok {
		stop := &managedagents.StopReason{Type: stringField(sr, "type")}
		if ids, ok := sr["event_ids"].([]any); ok {
			for _, id := range ids {
				if s, ok := id.(string); ok {
					stop.EventIDs = append(stop.EventIDs, s)
				}
			}
		}
		event.StopReason = stop
	}
	return event, nil
}

func decodeJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(io.LimitReader(r, 1<<20))
	return dec.Decode(v)
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
